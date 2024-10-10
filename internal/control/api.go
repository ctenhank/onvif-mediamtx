package control

import (
	"net/http"

	"github.com/ctenhank/mediamtx/internal/defs"
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

			cams = append(cams, defs.IPCamera{
				Name:      dev.Conf.Name,
				PtzSupprt: dev.isEnabledPTZ(),
				Channels:  ch,
			})
		}
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
