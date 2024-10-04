package rtmp

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/ctenhank/mediamtx/internal/asyncwriter"
	"github.com/ctenhank/mediamtx/internal/logger"
	"github.com/ctenhank/mediamtx/internal/protocols/rtmp/bytecounter"
	"github.com/ctenhank/mediamtx/internal/protocols/rtmp/message"
	"github.com/ctenhank/mediamtx/internal/stream"
	"github.com/ctenhank/mediamtx/internal/test"
	"github.com/stretchr/testify/require"
)

func TestFromStreamNoSupportedCodecs(t *testing.T) {
	stream, err := stream.New(
		1460,
		&description.Session{Medias: []*description.Media{{
			Type:    description.MediaTypeVideo,
			Formats: []format.Format{&format.VP8{}},
		}}},
		true,
		test.NilLogger,
	)
	require.NoError(t, err)

	writer := asyncwriter.New(0, nil)

	l := test.Logger(func(logger.Level, string, ...interface{}) {
		t.Error("should not happen")
	})

	err = FromStream(stream, writer, nil, nil, 0, l)
	require.Equal(t, errNoSupportedCodecsFrom, err)
}

func TestFromStreamSkipUnsupportedTracks(t *testing.T) {
	stream, err := stream.New(
		1460,
		&description.Session{Medias: []*description.Media{
			{
				Type:    description.MediaTypeVideo,
				Formats: []format.Format{&format.VP8{}},
			},
			{
				Type:    description.MediaTypeVideo,
				Formats: []format.Format{&format.H264{}},
			},
			{
				Type:    description.MediaTypeVideo,
				Formats: []format.Format{&format.H264{}},
			},
		}},
		true,
		test.NilLogger,
	)
	require.NoError(t, err)

	writer := asyncwriter.New(0, nil)

	n := 0

	l := test.Logger(func(l logger.Level, format string, args ...interface{}) {
		require.Equal(t, logger.Warn, l)
		switch n {
		case 0:
			require.Equal(t, "skipping track with codec VP8", fmt.Sprintf(format, args...))
		case 1:
			require.Equal(t, "skipping track with codec H264", fmt.Sprintf(format, args...))
		}
		n++
	})

	var buf bytes.Buffer
	bc := bytecounter.NewReadWriter(&buf)
	conn := &Conn{mrw: message.NewReadWriter(&buf, bc, false)}

	err = FromStream(stream, writer, conn, nil, 0, l)
	require.NoError(t, err)
	require.Equal(t, 2, n)
}
