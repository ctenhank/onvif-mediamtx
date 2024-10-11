package control

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ctenhank/mediamtx/internal/conf"
	"github.com/ctenhank/mediamtx/internal/defs"
	"github.com/ctenhank/mediamtx/internal/logger"
	"github.com/ctenhank/mediamtx/internal/protocols/httpp"
	"github.com/ctenhank/mediamtx/internal/restrictnetwork"
	"github.com/gin-gonic/gin"
)

type apiParent interface {
	logger.Writer
}

type Control struct {
	Address        string
	Encryption     bool
	ServerKey      string
	ServerCert     string
	AllowOrigin    string
	TrustedProxies conf.IPNetworks
	ReadTimeout    conf.StringDuration
	Conf           *conf.Conf

	Parent       apiParent
	httpServer   *httpp.WrappedServer
	OnvifDevices []onvifDevice
	ptzRoom      []PTZRoom
}

func convertPathConfToUrl(path conf.Path) (*url.URL, error) {
	u, err := url.Parse(path.Source)
	if err != nil {
		return nil, err
	}

	port := u.Port()

	if port == "" {
		port = strings.Replace(path.APIPort, ":", "", -1)
	}

	return &url.URL{
		Scheme: "http",
		Host:   u.Hostname() + ":" + port,
		User:   url.UserPassword(path.Username, path.Password),
	}, nil
}

func (c *Control) Initialize() error {
	router := gin.New()

	group := router.Group("/")

	// path := group.Group("/path")
	// path.GET("/:name")
	ipcam := group.Group("/ipcam")
	ipcam.GET("/", c.getIPCameras)
	ipcam.GET("/:name", c.getIPCamera)
	ipcam.GET("/:name/channel", c.getChannels)

	group.GET("/ptz/:name", c.getPTZ)

	// ptz := group.Group("/ptz/:name")
	// ptz.GET("/join", c.joinPTZ)

	network, address := restrictnetwork.Restrict("tcp", c.Address)

	c.httpServer = &httpp.WrappedServer{
		Network:     network,
		Address:     address,
		ReadTimeout: time.Duration(c.ReadTimeout),
		Encryption:  c.Encryption,
		ServerCert:  c.ServerCert,
		ServerKey:   c.ServerKey,
		Handler:     router,
		Parent:      c,
	}
	err := c.httpServer.Initialize()
	if err != nil {
		return err
	}

	paths := c.Conf.Paths
	for _, path := range paths {

		dev := &onvifDevice{
			Conf:   path,
			parent: c,
		}
		err := dev.initialize()
		if err != nil {
			c.Log(logger.Error, "failed to initialize onvif device: %v", err)
			continue
		}

		c.OnvifDevices = append(c.OnvifDevices, *dev)
	}

	c.Log(logger.Info, "listener opened on "+address)

	return nil
}

func (c *Control) Log(level logger.Level, format string, args ...interface{}) {
	c.Parent.Log(level, "[Control] "+format, args...)
}

func (c *Control) Close() {
	c.Log(logger.Info, "listener is closing")
	c.httpServer.Close()

}

func (c *Control) getPtzRoom(name string) *PTZRoom {
	for _, room := range c.ptzRoom {
		if room.conf.Name == name {
			return &room
		}
	}
	return nil
}

func (c *Control) getOnvifDevice(name string) *onvifDevice {
	for _, dev := range c.OnvifDevices {
		for _, profiles := range *dev.Profiles {
			if profiles.PathName == name {
				return &dev
			}
		}
	}
	return nil
}

func (c *Control) writeError(ctx *gin.Context, status int, err error) {
	c.Log(logger.Error, err.Error())

	ctx.JSON(status, defs.ControlError{
		Error: err.Error(),
	})
}

func (c *Control) getPTZ(ctx *gin.Context) {
	params := ctx.Params
	channelName := params.ByName("name")

	dev := c.getOnvifDevice(channelName)

	if dev == nil || dev.ptzRoom == nil {
		c.writeError(ctx, http.StatusNotFound, errors.New("찾을 수 없는 채널명입니다"))
		return
	} else if !dev.isEnabledPTZ() {
		c.writeError(ctx, http.StatusBadRequest, errors.New("PTZ가 지원되지 않는 채널입니다"))
	} else {
		serveWs(dev.ptzRoom, ctx.Writer, ctx.Request)
	}
}
