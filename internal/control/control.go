package control

import (
	"net/url"
	"time"

	"github.com/ctenhank/mediamtx/internal/conf"
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

	return &url.URL{
		Scheme: "http",
		Host:   u.Host + path.APIPort,
		User:   url.UserPassword(path.Username, path.Password),
	}, nil
}

func (c *Control) Initialize() error {
	router := gin.New()

	group := router.Group("/")

	ptz := group.Group("/ptz/:name")
	ptz.GET("/join", c.joinPTZ)

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
		dev.initialize()

		c.OnvifDevices = append(c.OnvifDevices, *dev)
		p := PTZRoom{
			available: true,
			dev:       dev,
			conf:      path,
		}

		err := p.initialize()
		if err != nil {
			c.Log(logger.Error, "Failed to initialize PTZ room "+path.Name+": "+err.Error())
			continue
		}
		c.ptzRoom = append(c.ptzRoom, p)

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

func (c *Control) joinPTZ(ctx *gin.Context) {
	params := ctx.Params
	query := ctx.Request.URL.RawQuery

	channelName := params.ByName("name")

	if r := c.getPtzRoom(channelName); r == nil {
		c.Log(logger.Warn, "No such ptz room: %v", channelName)
		return
	} else {
		serveWs(r, ctx.Writer, ctx.Request)
	}

	c.Log(logger.Info, "joinPTZ: params(%v), query(%v)", params, query)

}
