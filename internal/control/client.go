package control

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/IOTechSystems/onvif/ptz"
	xsdonvif "github.com/IOTechSystems/onvif/xsd/onvif"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	ptzRoom *PTZRoom

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.ptzRoom.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		log.Printf("Received message %v, %v", message, string(message))
		var a PtzAction
		err = json.Unmarshal(message, &a)
		if err != nil {
			log.Println("Error: %v", err)
			// Invalid Json Format
		}

		c.handleAction(a)

		c.ptzRoom.hub.broadcast <- message
	}
}

func (c *Client) handleContinuousMove(direction string) (interface{}, error) {

	pan, tilt, zoom := .0, .0, .0
	if direction == "left" {
		pan = -c.ptzRoom.conf.PTZPanSpeed
	} else if direction == "right" {
		pan = c.ptzRoom.conf.PTZPanSpeed
	} else if direction == "up" {
		tilt = c.ptzRoom.conf.PTZTiltSpeed
	} else if direction == "down" {
		tilt = -c.ptzRoom.conf.PTZTiltSpeed
	} else if direction == "zoomIn" {
		zoom = c.ptzRoom.conf.PTZZoomSpeed
	} else if direction == "zoomOut" {
		zoom = -c.ptzRoom.conf.PTZZoomSpeed
	} else {
		return nil, errors.New("invalid direction")
	}

	speed := xsdonvif.PTZSpeed{
		PanTilt: &xsdonvif.Vector2D{
			X: pan,
			Y: tilt,
		},
		Zoom: &xsdonvif.Vector1D{
			X: zoom,
		},
	}

	return c.ptzRoom.dev.continuousMove(&speed)
}

func (c *Client) handleRelativeMove(direction string) (interface{}, error) {

	pan, tilt, zoom := .0, .0, .0
	if direction == "left" {
		pan = -0.05
	} else if direction == "right" {
		pan = 0.05
	} else if direction == "up" {
		tilt = 0.05
	} else if direction == "down" {
		tilt = -0.05
	} else if direction == "zoomIn" {
		zoom = 0.05
	} else if direction == "zoomOut" {
		zoom = -0.05
	} else {
		return nil, errors.New("invalid direction")
	}

	vector := ptz.Vector{
		PanTilt: &xsdonvif.Vector2D{
			X: pan,
			Y: tilt,
		},
		Zoom: &xsdonvif.Vector1D{
			X: zoom,
		},
	}

	return c.ptzRoom.dev.relativeMove(vector)
}

func (c *Client) handleAction(a PtzAction) error {
	// c.ptzRoom.mutex.Lock()
	log.Println("Handle Ptz Action: %v", a)

	go c.ptzRoom.setTicker()

	// PTZ Action을 처리
	if a.Action == "continuous" {
		resp, err := c.handleContinuousMove(a.Direction)
		if err != nil {
			log.Println("Error: %v", err)
			return err
		}

		log.Println("Response: %v", resp)
	} else if a.Action == "stop" {
		resp, err := c.ptzRoom.dev.stop()
		if err != nil {
			log.Println("Error: %v", err)
			return err
		}
		log.Println("Response: %v", resp)

	} else if a.Action == "relative" {
		resp, err := c.handleRelativeMove(a.Direction)
		if err != nil {
			log.Println("Error: %v", err)
			return err
		}
		log.Println("Response: %v", resp)
	} else if a.Action == "home" {
		resp, err := c.ptzRoom.dev.gotoHomePosition()
		if err != nil {
			log.Println("Error: %v", err)
			return err
		}
		log.Println("Response: %v", resp)

	} else if a.Action == "save" {
		resp, err := c.ptzRoom.dev.setHomePosition()
		if err != nil {
			log.Println("Error: %v", err)
			return err
		}

		log.Println("Response: %v", resp)
	}

	// curr, err := c.ptzRoom.dev.getPtzStatus()
	// if err != nil {
	// 	log.Println("Failed get ptz status: %v", err)
	// }

	// log.Printf("PTZ Status: %v -> %v", prev, curr)
	// log.Printf("PTZ Position: (%v, %v) -> (%v, %v)", prev.PTZStatus.Position.PanTilt.X, prev.PTZStatus.Position.PanTilt.Y, curr.PTZStatus.Position.PanTilt.X, curr.PTZStatus.Position.PanTilt.Y)
	// c.ptzRoom.mutex.Unlock()
	return nil

}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			log.Println("Received message %v", message)
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(ptzRoom *PTZRoom, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{ptzRoom: ptzRoom, conn: conn, send: make(chan []byte, 256)}
	// client.hub.register <- client
	log.Println("Client connected")

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
