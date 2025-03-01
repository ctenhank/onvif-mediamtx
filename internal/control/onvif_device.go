package control

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/ctenhank/mediamtx/internal/conf"
	"github.com/ctenhank/mediamtx/internal/logger"
	"github.com/icholy/digest"

	goonvif "github.com/IOTechSystems/onvif"

	device "github.com/IOTechSystems/onvif/device"
	"github.com/IOTechSystems/onvif/ptz"
	"github.com/IOTechSystems/onvif/xsd"

	media "github.com/IOTechSystems/onvif/media"
	recording "github.com/IOTechSystems/onvif/recording"

	xsdonvif "github.com/IOTechSystems/onvif/xsd/onvif"
)

const directory = "onvif-test"
const debug = true

var filename = time.Now().Format("20060102_150405")

type Profile struct {
	xsdonvif.Profile
	PathName string
}

type MediaUri struct {
	xsdonvif.MediaUri
	Profile *Profile
}

// 채널 사용해서 Intializing이 Done 됐는지
type onvifDevice struct {
	ctx       context.Context
	ctxCancel func()

	Conf   *conf.Path
	Url    url.URL
	dev    *goonvif.Device
	parent *Control

	SystemDateTime         *xsdonvif.SystemDateTime
	Capabilities           *xsdonvif.Capabilities
	Profiles               *[]Profile
	StreamUris             *[]MediaUri
	StorageConfigurations  *[]device.StorageConfiguration
	RecordingConfiguration *recording.RecordingConfiguration
	SnapshotUri            *xsdonvif.MediaUri

	onvifUrl url.URL
	client   *http.Client

	ptzRoom *PTZRoom
}

func (o *onvifDevice) isEnabledPTZ() bool {
	return o.Capabilities == nil || o.Capabilities.PTZ.XAddr != ""
}

func (o *onvifDevice) initialize() error {
	u, err := convertPathConfToUrl(*o.Conf)

	if err != nil {
		return err
	}

	o.Url = *u
	ctx, ctxCancel := context.WithCancel(context.Background())
	o.ctx = ctx
	o.ctxCancel = ctxCancel

	// Add Digest Auth
	transport := &digest.Transport{
		Username: o.Conf.Username,
		Password: o.Conf.Password,
	}

	o.client = &http.Client{
		Transport: &retryableTransport{
			transport: transport,
		},
	}

	dev, err := goonvif.NewDevice(goonvif.DeviceParams{
		Xaddr:    o.Url.Host,
		Username: o.Conf.Username,
		Password: o.Conf.Password,
		HttpClient: &http.Client{
			Transport: transport,
		},
	})

	if err != nil {
		return err
	}

	o.dev = dev
	var innerWg sync.WaitGroup
	innerWg.Add(1)
	go func() {
		defer innerWg.Done()

		dtResp, err := o.getSystemDateAndTime()
		if err != nil {
			o.parent.Log(logger.Error, "Failed to get system date and time of onvif device "+o.Conf.Name+": "+err.Error())
		}
		o.SystemDateTime = &dtResp.SystemDateAndTime

		capResp, err := o.getCapabilities()
		if err != nil {
			o.parent.Log(logger.Error, "Failed to get profiles of onvif device "+o.Conf.Name+": "+err.Error())
		}
		o.Capabilities = &capResp.Capabilities

		proResp, err := o.getProfiles()
		if err != nil {
			o.parent.Log(logger.Error, "Failed to get profiles of onvif device "+o.Conf.Name+": "+err.Error())
		}

		profiles := []Profile{}

		for i, profile := range proResp.Profiles {
			pathName := o.Conf.Name + "_" + fmt.Sprint(i)
			if i == 0 {
				pathName = o.Conf.Name
			}
			profiles = append(profiles, Profile{
				Profile:  profile,
				PathName: pathName,
			})
		}

		o.Profiles = &profiles

		streamUris := []MediaUri{}
		for _, profile := range *o.Profiles {
			stResp, err := o.getStreamUri(&profile.Token)

			if err != nil {
				o.parent.Log(logger.Error, "Failed to get stream uri of onvif device "+o.Conf.Name+": "+err.Error())
				continue
			}

			streamUris = append(streamUris, MediaUri{
				MediaUri: stResp.MediaUri,
				Profile:  &profile,
			})

		}
		o.StreamUris = &streamUris

		snResp, err := o.getSnapshotUri()
		if err != nil {
			o.parent.Log(logger.Error, "Failed to get snapshot uri of onvif device "+o.Conf.Name+": "+err.Error())
		}
		o.SnapshotUri = &snResp.MediaUri

		if o.isEnabledPTZ() {
			p := PTZRoom{
				available: o.isEnabledPTZ(),
				dev:       o,
				conf:      o.Conf,
			}

			err = p.initialize()
			if err != nil {
				o.parent.Log(logger.Error, "Failed to initialize PTZ room "+o.Conf.Name+": "+err.Error())
			}
			o.ptzRoom = &p
		}

		o.parent.Log(logger.Info, "onvif device "+o.Conf.Name+" initialized")
	}()

	innerWg.Wait()

	return nil
}

func (o *onvifDevice) test(tag string, err error, t interface{}) {
	var data []byte
	filepath := directory + "/" + filename + "_" + o.Conf.Name + ".json"
	if _, err := os.Stat(filepath); err == nil {
		data, err = os.ReadFile(filepath)
		if err != nil {
			o.parent.Log(logger.Error, "Failed to read file "+filepath+": "+err.Error())
			// return
		}
	} else {
		data = []byte("{}")
	}

	if err != nil {
		o.parent.Log(logger.Error, "Failed to read file "+filepath+": "+err.Error())
		// return
	}

	var d map[string]interface{}
	err = json.Unmarshal(data, &d)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to get "+tag+" of onvif device "+o.Conf.Name+": "+err.Error())
		return
	}

	_, err = json.Marshal(t)
	if err != nil {
		o.parent.Log(logger.Error, "Failed to marshal capabilities of onvif device "+o.Conf.Name+": "+err.Error())
		return
	}

	// Get unique tag name
	tag = getUniqueTagName(tag, d)
	d[tag] = t

	bd, err := json.Marshal(d)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to marshal capabilities of onvif device "+o.Conf.Name+": %v", err)
		return
	}

	os.MkdirAll(directory, os.ModePerm)
	err = os.WriteFile(filepath, bd, 0644)
	if err != nil {
		o.parent.Log(logger.Error, "Failed to write file "+filename+": "+err.Error())
		return
	}

}

func (o *onvifDevice) callMethod(method interface{}, reply interface{}) error {
	resp, err := o.dev.CallMethod(method)

	tag := reflect.TypeOf(method).String()

	if err != nil {
		o.parent.Log(logger.Error, "Failed to call "+tag+" of onvif device "+o.Conf.Name+": "+err.Error())
		return err
	}

	b, err := io.ReadAll(resp.Body)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to read body of "+tag+" of onvif device "+o.Conf.Name+": "+err.Error())
		return err
	}

	// o.parent.Log(logger.Info, "onvif device "+o.Conf.Name+": "+tag+" response: "+string(b))

	if (resp.StatusCode / 100) == 2 { // 성공 경우
		err = xml.Unmarshal(b, reply)

		if err != nil {
			o.parent.Log(logger.Error, "Failed to parse "+tag+" of onvif device "+o.Conf.Name+": "+err.Error())
		}

		if debug {
			o.test(tag, err, reply)
		}

		return err
	}

	// 실패한 경우
	reply = nil

	if resp.StatusCode == http.StatusUnauthorized { // 에러: 인증 실패
		// TODO: 인증 실패 시 처리
	} else if resp.StatusCode == http.StatusBadRequest { // 에러: 잘못된 요청
		// TODO: 잘못된 요청 시 처리
	}

	return err // 그 외 모든 경우는 에러로 처리
}

func (o *onvifDevice) getSystemDateAndTime() (*device.GetSystemDateAndTimeResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			GetSystemDateAndTimeResponse device.GetSystemDateAndTimeResponse
		}
	}
	var reply Envelope
	err := o.callMethod(
		device.GetSystemDateAndTime{},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to get system date and time of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	// o.parent.Log(logger.Info, "onvif device %v: system date and time %v ", o.Conf.Name, reply.SystemDateAndTime.LocalDateTime)
	return &reply.Body.GetSystemDateAndTimeResponse, nil
}

func (o *onvifDevice) getCapabilities() (*device.GetCapabilitiesResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			GetCapabilitiesResponse device.GetCapabilitiesResponse
		}
	}

	var reply Envelope
	err := o.callMethod(
		device.GetCapabilities{Category: []xsdonvif.CapabilityCategory{"All"}},
		&reply,
	)

	if err != nil {
		return nil, err
	}

	return &reply.Body.GetCapabilitiesResponse, nil
}

func (o *onvifDevice) getStreamUri(profileToken *xsdonvif.ReferenceToken) (*media.GetStreamUriResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			GetStreamUriResponse media.GetStreamUriResponse
		}
	}

	var reply Envelope
	err := o.callMethod(
		media.GetStreamUri{
			ProfileToken: profileToken,
		},
		&reply,
	)

	if err != nil {
		return nil, err
	}

	return &reply.Body.GetStreamUriResponse, nil
}

func (o *onvifDevice) getProfiles() (*media.GetProfilesResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			GetProfilesResponse media.GetProfilesResponse
		}
	}

	var reply Envelope
	err := o.callMethod(
		media.GetProfiles{},
		&reply,
	)

	if err != nil {
		return nil, err
	}

	return &reply.Body.GetProfilesResponse, nil
}

func (o *onvifDevice) getStorageConfigurations() (*device.GetStorageConfigurationsResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			GetStorageConfigurationsResponse device.GetStorageConfigurationsResponse
		}
	}

	var reply Envelope
	err := o.callMethod(
		device.GetStorageConfigurations{},
		&reply,
	)

	if err != nil {
		return nil, err
	}

	return &reply.Body.GetStorageConfigurationsResponse, nil

}

func (o *onvifDevice) getRecordingConfiguration() (*recording.GetRecordingConfigurationResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			GetRecordingConfigurationResponse recording.GetRecordingConfigurationResponse
		}
	}

	var reply Envelope
	err := o.callMethod(
		recording.GetRecordingConfiguration{},
		&reply,
	)

	if err != nil {
		return nil, err
	}

	return &reply.Body.GetRecordingConfigurationResponse, nil

}

func (o *onvifDevice) stop() (*ptz.StopResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			StopResponse ptz.StopResponse
		}
	}
	p := *(o.Profiles)

	var reply Envelope
	err := o.callMethod(
		ptz.Stop{
			ProfileToken: p[0].Token,
			PanTilt:      true,
			Zoom:         true,
		},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to continuous move of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	return &reply.Body.StopResponse, nil

}

func (o *onvifDevice) absoluteMove(speed *xsdonvif.PTZSpeed) (*ptz.AbsoluteMoveResponse, error) {
	o.parent.Log(logger.Info, "Continuous Move")
	type Envelope struct {
		Header struct{}
		Body   struct {
			AbsoluteMoveResponse ptz.AbsoluteMoveResponse
		}
	}

	p := *(o.Profiles)

	var reply Envelope
	err := o.callMethod(
		ptz.AbsoluteMove{
			ProfileToken: p[0].Token,
			Position:     ptz.Vector{PanTilt: &xsdonvif.Vector2D{X: 0.5, Y: 0.5}},
			Speed:        ptz.Speed{PanTilt: &xsdonvif.Vector2D{X: speed.PanTilt.X, Y: speed.PanTilt.Y}},
		},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to continuous move of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	return &reply.Body.AbsoluteMoveResponse, nil
}

func (o *onvifDevice) relativeMove(translation ptz.Vector) (*ptz.RelativeMoveResponse, error) {
	o.parent.Log(logger.Info, "Relative Move")
	type Envelope struct {
		Header struct{}
		Body   struct {
			RelativeMoveResponse ptz.RelativeMoveResponse
		}
	}

	p := *(o.Profiles)

	var reply Envelope
	err := o.callMethod(
		ptz.RelativeMove{
			ProfileToken: p[0].Token,
			Translation:  translation,
			Speed:        ptz.Speed{PanTilt: &xsdonvif.Vector2D{X: 1, Y: 1}, Zoom: &xsdonvif.Vector1D{X: 1}},
		},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to relative move of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	return &reply.Body.RelativeMoveResponse, nil
}

func (o *onvifDevice) getPtzStatus() (*ptz.GetStatusResponse, error) {
	o.parent.Log(logger.Info, "GetPtzStatus")
	type Envelope struct {
		Header struct{}
		Body   struct {
			GetStatusResponse ptz.GetStatusResponse
		}
	}

	p := *(o.Profiles)

	var reply Envelope
	err := o.callMethod(
		ptz.GetStatus{
			ProfileToken: p[0].Token,
		},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to GetPtzStatus move of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	return &reply.Body.GetStatusResponse, nil
}

func (o *onvifDevice) geoMove(speed *xsdonvif.PTZSpeed) (*ptz.GeoMoveResponse, error) {
	o.parent.Log(logger.Info, "Continuous Move")
	type Envelope struct {
		Header struct{}
		Body   struct {
			GeoMoveResponse ptz.GeoMoveResponse
		}
	}

	p := *(o.Profiles)

	var reply Envelope
	err := o.callMethod(
		ptz.GeoMove{
			ProfileToken: p[0].Token,
			Target: xsdonvif.GeoLocation{
				Lon:       0.5,
				Lat:       0.5,
				Elevation: 0.5,
			},
			Speed:      *speed,
			AreaHeight: 0.5,
			AreaWidth:  0.5,
		},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to continuous move of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	return &reply.Body.GeoMoveResponse, nil
}

func (o *onvifDevice) continuousMove(speed *xsdonvif.PTZSpeed) (*ptz.ContinuousMoveResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			ContinuousMoveResponse ptz.ContinuousMoveResponse
		}
	}

	p := *(o.Profiles)

	timeout := xsd.Duration("").NewDateTime("0", "0", "0", "0", "0", "10")
	// o.parent.Log(logger.Info, "Continuous Move with speed %v and timeout %v", speed, timeout)

	var reply Envelope
	err := o.callMethod(
		ptz.ContinuousMove{
			ProfileToken: &p[0].Token,
			Velocity:     speed,
			Timeout:      &timeout,
		},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to continuous move of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	return &reply.Body.ContinuousMoveResponse, nil
}

func (o *onvifDevice) gotoHomePosition() (*ptz.GotoHomePositionResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			GotoHomePositionResponse ptz.GotoHomePositionResponse
		}
	}

	p := *(o.Profiles)

	var reply Envelope
	err := o.callMethod(
		ptz.GotoHomePosition{
			ProfileToken: &p[0].Token,
			Speed: &xsdonvif.PTZSpeed{
				PanTilt: &xsdonvif.Vector2D{
					X: 1,
					Y: 1,
				},
				Zoom: &xsdonvif.Vector1D{
					X: 1,
				},
			},
		},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to GotoHomePositionResponse move of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	return &reply.Body.GotoHomePositionResponse, nil
}

func (o *onvifDevice) setHomePosition() (*ptz.SetHomePositionResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			SetHomePositionResponse ptz.SetHomePositionResponse
		}
	}

	p := *(o.Profiles)

	var reply Envelope
	err := o.callMethod(
		ptz.SetHomePosition{
			ProfileToken: p[0].Token,
		},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to SetHomePosition move of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	return &reply.Body.SetHomePositionResponse, nil
}

func (o *onvifDevice) getSnapshotUri() (*media.GetSnapshotUriResponse, error) {
	type Envelope struct {
		Header struct{}
		Body   struct {
			GetSnapshotUriResponse media.GetSnapshotUriResponse
		}
	}

	p := *(o.Profiles)

	var reply Envelope
	err := o.callMethod(
		media.GetSnapshotUri{
			ProfileToken: p[0].Token,
		},
		&reply,
	)

	if err != nil {
		o.parent.Log(logger.Error, "Failed to GetSnapshotUriResponse move of onvif device "+o.Conf.Name+": "+err.Error())
		return nil, err
	}

	return &reply.Body.GetSnapshotUriResponse, nil
}

// 실패하면 채널을 통해서 에러 사실을 알림
func (o *onvifDevice) ping() error {
	resp, err := o.getSystemDateAndTime()
	if err != nil {
		o.parent.Log(logger.Error, "Failed to ping onvif device "+o.Conf.Name+": "+err.Error())
		return err
	}

	o.SystemDateTime = &resp.SystemDateAndTime
	return err
}

func getUniqueTagName(tag string, d map[string]interface{}) string {
	t := tag
	tag_cnt := 0
	for {
		if tag_cnt != 0 {
			t = tag + "_" + fmt.Sprint(tag_cnt)
		}

		if _, ok := d[t]; !ok {
			break
		}

		tag_cnt += 1
	}

	return t
}
