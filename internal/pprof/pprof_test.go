package pprof

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ctenhank/mediamtx/internal/conf"
	"github.com/ctenhank/mediamtx/internal/test"
	"github.com/stretchr/testify/require"
)

func TestPreflightRequest(t *testing.T) {
	s := &PPROF{
		Address:     "127.0.0.1:9999",
		AllowOrigin: "*",
		ReadTimeout: conf.StringDuration(10 * time.Second),
		Parent:      test.NilLogger,
	}
	err := s.Initialize()
	require.NoError(t, err)
	defer s.Close()

	tr := &http.Transport{}
	defer tr.CloseIdleConnections()
	hc := &http.Client{Transport: tr}

	req, err := http.NewRequest(http.MethodOptions, "http://localhost:9999", nil)
	require.NoError(t, err)

	req.Header.Add("Access-Control-Request-Method", "GET")

	res, err := hc.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusNoContent, res.StatusCode)

	byts, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	require.Equal(t, "*", res.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", res.Header.Get("Access-Control-Allow-Credentials"))
	require.Equal(t, "OPTIONS, GET", res.Header.Get("Access-Control-Allow-Methods"))
	require.Equal(t, "Authorization", res.Header.Get("Access-Control-Allow-Headers"))
	require.Equal(t, byts, []byte{})
}
