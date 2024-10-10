package defs

type ControlError struct {
	Error string `json:"error"`
}

type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Channel struct {
	Name       string     `json:"name"`
	Resolution Resolution `json:"resolution"`
}

type IPCamera struct {
	Name      string    `json:"name"`
	PtzSupprt bool      `json:"ptz_support"`
	Channels  []Channel `json:"channels"`
}
