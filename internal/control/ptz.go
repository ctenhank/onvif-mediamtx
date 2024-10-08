package control

// TODO: PTZ 사용중일 때 다른 사용자에게 이를 알려줌

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ctenhank/mediamtx/internal/conf"
)

var (
	PtzActions = []string{
		"continuous", // Move continuously
		"stop",       // Stop continuous move
		"relative",
		"absolute",
		"geo",
		"home",
		"save", // Save home position
	}
)

// PtzManager PTZ 기능을 제어하는 구조체
type PtzAction struct {
	Action    string `json:"action"`
	Direction string `json:"direction"`
	Message   string `json:"message"`
}

// PTZ 이용가능한 Path 별로 PTZ WebSocket Channel 생성

type PTZRoom struct {
	available bool
	conf      *conf.Path
	dev       *onvifDevice
	mutex     *sync.RWMutex
	hub       *Hub
	ticker    *time.Ticker
}

func (pr *PTZRoom) initialize() error {
	// url에 요청 후 PTZ 기능 동작되는지 확인 필요
	pr.hub = newHub()
	go pr.hub.run()

	return nil
}

func (pr *PTZRoom) getPtzStatus() (string, error) {
	resp, err := pr.dev.getPtzStatus() // PTZ 상태 확인

	if err != nil {
		fmt.Println("Error:", err)
		return "", err
	}

	fmt.Printf("PTZ Position: (%v, %v ,%v)\n", resp.PTZStatus.Position.PanTilt.X, resp.PTZStatus.Position.PanTilt.Y, resp.PTZStatus.Position.Zoom.X)

	ptStatus := resp.PTZStatus.MoveStatus.PanTilt
	zoomStatus := resp.PTZStatus.MoveStatus.Zoom

	status := "IDLE"

	if ptStatus == "MOVING" || zoomStatus == "MOVING" {
		status = "MOVING"
	}

	return status, nil
}

func (pr *PTZRoom) setTicker() {
	if pr.ticker != nil {
		log.Println("Ticker is already set")
		return
	}

	// status, err := pr.getPtzStatus() // PTZ 상태 확인
	// pr.dev.parent.Log(logger.Info, "PTZ Status: %v", status)

	// if err != nil {
	// 	fmt.Println("Error:", err)
	// }
	pr.ticker = time.NewTicker(1 * time.Second)
	cnt := 0

	for range pr.ticker.C {
		status, err := pr.getPtzStatus() // PTZ 상태 확인

		if err != nil {
			fmt.Println("Error:", err)
		}

		// if status == "IDLE" {
		// 	cnt += 1
		// } else {
		// 	cnt = 0
		// }

		// if cnt > 2 && status == "IDLE" {
		// 	if pr.ticker != nil {
		// 		pr.ticker.Stop()
		// 		pr.ticker = nil
		// 	}
		// 	break
		// }

		if status == "IDLE" {
			cnt += 1
		} else {
			cnt = 0
			pr.available = false
		}

		if status == "IDLE" {
			if pr.ticker != nil {
				pr.ticker.Stop()
				pr.ticker = nil
				pr.available = true
				{
				}
				pr.hub.broadcast <- []byte("")
			}
			break
		}

		// if status == "MOVING" && pr.ticker == nil {
		// 	pr.ticker = time.NewTicker(1 * time.Second)
		// } else if status == "IDLE" && pr.ticker != nil {
		// 	if pr.ticker != nil {
		// 		pr.ticker.Stop()
		// 		pr.ticker = nil

		// 	}
		// 	break
		// }
	}
}
