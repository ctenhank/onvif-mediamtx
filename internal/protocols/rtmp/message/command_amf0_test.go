package message

import (
	"testing"

	"github.com/ctenhank/mediamtx/internal/protocols/rtmp/amf0"
	"github.com/ctenhank/mediamtx/internal/protocols/rtmp/rawmessage"
)

func BenchmarkCommandAMF0Marshal(b *testing.B) {
	msg := &CommandAMF0{
		ChunkStreamID: 3,
		Name:          "connect",
		CommandID:     1,
		Arguments: []interface{}{
			amf0.Object{
				{Key: "app", Value: "/stream"},
				{Key: "flashVer", Value: "LNX 9,0,124,2"},
				{Key: "tcUrl", Value: "http://example.com"},
				{Key: "fpad", Value: false},
				{Key: "capabilities", Value: 15},
				{Key: "audioCodecs", Value: 4071},
				{Key: "videoCodecs", Value: 252},
				{Key: "videoFunction", Value: 1},
			},
		},
	}

	for n := 0; n < b.N; n++ {
		msg.marshal() //nolint:errcheck
	}
}

func BenchmarkCommandAMF0Unmarshal(b *testing.B) {
	raw := &rawmessage.Message{
		ChunkStreamID:   0x3,
		Timestamp:       0,
		Type:            0x14,
		MessageStreamID: 0x0,
		Body: []uint8{
			0x02, 0x00, 0x07, 0x63, 0x6f, 0x6e, 0x6e, 0x65,
			0x63, 0x74, 0x00, 0x3f, 0xf0, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x03, 0x00, 0x03, 0x61, 0x70,
			0x70, 0x02, 0x00, 0x07, 0x2f, 0x73, 0x74, 0x72,
			0x65, 0x61, 0x6d, 0x00, 0x08, 0x66, 0x6c, 0x61,
			0x73, 0x68, 0x56, 0x65, 0x72, 0x02, 0x00, 0x0d,
			0x4c, 0x4e, 0x58, 0x20, 0x39, 0x2c, 0x30, 0x2c,
			0x31, 0x32, 0x34, 0x2c, 0x32, 0x00, 0x05, 0x74,
			0x63, 0x55, 0x72, 0x6c, 0x02, 0x00, 0x12, 0x68,
			0x74, 0x74, 0x70, 0x3a, 0x2f, 0x2f, 0x65, 0x78,
			0x61, 0x6d, 0x70, 0x6c, 0x65, 0x2e, 0x63, 0x6f,
			0x6d, 0x00, 0x04, 0x66, 0x70, 0x61, 0x64, 0x01,
			0x00, 0x00, 0x0c, 0x63, 0x61, 0x70, 0x61, 0x62,
			0x69, 0x6c, 0x69, 0x74, 0x69, 0x65, 0x73, 0x00,
			0x40, 0x2e, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x0b, 0x61, 0x75, 0x64, 0x69, 0x6f, 0x43,
			0x6f, 0x64, 0x65, 0x63, 0x73, 0x00, 0x40, 0xaf,
			0xce, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0b,
			0x76, 0x69, 0x64, 0x65, 0x6f, 0x43, 0x6f, 0x64,
			0x65, 0x63, 0x73, 0x00, 0x40, 0x6f, 0x80, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x0d, 0x76, 0x69,
			0x64, 0x65, 0x6f, 0x46, 0x75, 0x6e, 0x63, 0x74,
			0x69, 0x6f, 0x6e, 0x00, 0x3f, 0xf0, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09,
		},
	}

	msg := &CommandAMF0{}

	for n := 0; n < b.N; n++ {
		msg.unmarshal(raw) //nolint:errcheck
	}
}
