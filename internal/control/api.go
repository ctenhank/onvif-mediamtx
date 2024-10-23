package control

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/ctenhank/mediamtx/internal/defs"
	"github.com/ctenhank/mediamtx/internal/logger"
	"github.com/gin-gonic/gin"
)

func (c *Control) getIPCameras(ctx *gin.Context) {
	cams := make([]defs.IPCamera, 0)
	for _, dev := range c.OnvifDevices {
		ch := make([]defs.Channel, 0)
		for _, profile := range *dev.Profiles {
			ch = append(ch, defs.Channel{
				Name: profile.PathName,
				Resolution: defs.Resolution{
					Width:  int(*profile.VideoEncoderConfiguration.Resolution.Width),
					Height: int(*profile.VideoEncoderConfiguration.Resolution.Height),
				},
			})

		}
		cams = append(cams, defs.IPCamera{
			Id:        dev.Conf.Id,
			Name:      dev.Conf.Name,
			PtzSupprt: dev.isEnabledPTZ(),
			Channels:  ch,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"cameras": cams,
	})
}

func (c *Control) getIPCamera(ctx *gin.Context) {
	params := ctx.Params

	name := params.ByName("name")

	if name == "" {
		ctx.JSON(http.StatusBadRequest, defs.ControlError{
			Error: "Paramater `name` is required",
		})
		return
	}

	var cam *onvifDevice
	for _, dev := range c.OnvifDevices {
		if dev.Conf.Name == name {
			cam = &dev
		}
	}

	if cam == nil {
		ctx.JSON(http.StatusBadRequest, defs.ControlError{
			Error: "No such camera found: " + name,
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"camera": cam,
	})
}

func (c *Control) getChannels(ctx *gin.Context) {
	params := ctx.Params

	name := params.ByName("name")

	if name == "" {
		c.writeError(ctx, http.StatusBadRequest, errors.New("Paramater `name` is required"))
		return
	}

	var cam *onvifDevice
	for _, dev := range c.OnvifDevices {
		if dev.Conf.Name == name {
			cam = &dev
		}
	}

	if cam == nil {

		c.writeError(ctx, http.StatusBadRequest, errors.New("No such camera found: "+name))

		return
	}

	ch := make([]defs.Channel, 0)
	for _, profile := range *cam.Profiles {
		ch = append(ch, defs.Channel{
			Name: profile.PathName,
			Resolution: defs.Resolution{
				Width:  int(*profile.VideoEncoderConfiguration.Resolution.Width),
				Height: int(*profile.VideoEncoderConfiguration.Resolution.Height),
			},
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"channels": ch,
	})
}

func (c *Control) getSnapshot(ctx *gin.Context) {
	params := ctx.Params

	name := params.ByName("name")

	if name == "" {
		c.writeError(ctx, http.StatusBadRequest, errors.New("Paramater `name` is required"))

		return
	}

	var cam *onvifDevice
	for _, dev := range c.OnvifDevices {
		if dev.Conf.Name == name {
			cam = &dev
		}
	}

	if cam == nil {
		c.writeError(ctx, http.StatusBadRequest, errors.New("No such camera found: "+name))

		return
	}

	// directory := "snapshots"
	// filename := name + ".jpeg"
	// path := directory + "/" + filename
	// if fs, err := os.Stat(filename); err == nil {
	// 	file, err := os.Open("path/to/your/image.jpg")
	// 	if err != nil {
	// 		fmt.Println("Error opening file:", err)
	// 		return
	// 	}
	// 	defer file.Close()

	// 	img, err := jpeg.Decode(file)
	// 	if err != nil {
	// 		fmt.Println("Error decoding JPEG:", err)
	// 		return
	// 	}

	// } else if fs.ModTime().Before(time.Now().Add(-1 * time.Minute)) {

	// }

	directory := "snapshots"
	filename := name + ".jpeg"

	// path := directory + "/" + filename
	if fs, err := os.Stat(filename); err == nil && fs.ModTime().Before(time.Now().Add(-1*time.Minute)) {
		ctx.File("snapshots/" + name + ".jpeg")
		return
	}


	snapshotUrl, err := url.Parse(string(cam.SnapshotUri.Uri))

	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, errors.New("Error parsing snapshot URL: "+err.Error()))

		return
	}

	snapshotUrl.Host = cam.Url.Host

	resp, err := cam.client.Get(snapshotUrl.String())
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, errors.New("Error getting snapshot: "+err.Error()))

		return
	}

	if resp.StatusCode == 500 {
		c.writeError(ctx, http.StatusInternalServerError, errors.New("Error ip camera "+name))
		return

	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, errors.New("Error reading snapshot: "+err.Error()))

		return
	}

	c.Log(logger.Info, "Snapshot taken from "+name+"; "+resp.Status+", "+snapshotUrl.String()+"\n"+resp.Header.Get("Content-Type"))
  
	os.MkdirAll(directory, os.ModePerm)
	err = os.WriteFile(directory+"/"+filename, b, 0644)

	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, errors.New("Error writing snapshot: "+err.Error()))

		return

	}

	ctx.File("snapshots/" + name + ".jpeg")
}
