package control_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/ctenhank/mediamtx/internal/conf"
	"github.com/ctenhank/mediamtx/internal/control"
	"github.com/ctenhank/mediamtx/internal/logger"
	"github.com/ctenhank/mediamtx/internal/test"
	"github.com/stretchr/testify/require"
)

type testParent struct{}

func (t *testParent) Log(level logger.Level, format string, args ...interface{}) {}

const tempConfStr = `
control: true
paths:
  test-path:
    source: http://192.168.1.64
    username: username
    password: password
`

func tempConf(t *testing.T, cnt string) *conf.Conf {
	fi, err := test.CreateTempFile([]byte(cnt))
	require.NoError(t, err)
	defer os.Remove(fi)

	cnf, _, err := conf.Load(fi, nil)
	require.NoError(t, err)

	return cnf
}

func TestControlServerInitialization(t *testing.T) {
	t.Log(tempConfStr)
	control := control.Control{
		Address: ":9995",
		Conf:    tempConf(t, tempConfStr),
		Parent:  &testParent{},
	}

	defer control.Close()
	err := control.Initialize()
	if err != nil {
		t.Errorf("Initialization Error: %v", err)
		return
	}

}

func TestGetIPCameras(t *testing.T) {
	control := control.Control{
		Address: ":9995",
		Conf:    tempConf(t, tempConfStr),
		Parent:  &testParent{},
	}

	defer control.Close()
	err := control.Initialize()
	if err != nil {
		t.Errorf("Initialization Error: %v", err)
		return
	}

	req, err := http.NewRequest("GET", "http://localhost:9995/ipcam", nil)
	require.NoError(t, err)

	client := &http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestGetIPCamera(t *testing.T) {
	control := control.Control{
		Address: ":9995",
		Conf:    tempConf(t, tempConfStr),
		Parent:  &testParent{},
	}

	defer control.Close()
	err := control.Initialize()
	if err != nil {
		t.Errorf("Initialization Error: %v", err)
		return
	}

	req, err := http.NewRequest("GET", "http://localhost:9995/ipcam/test-path", nil)
	require.NoError(t, err)

	client := &http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestGetChannels(t *testing.T) {
	control := control.Control{
		Address: ":9995",
		Conf:    tempConf(t, tempConfStr),
		Parent:  &testParent{},
	}

	defer control.Close()
	err := control.Initialize()
	if err != nil {
		t.Errorf("Initialization Error: %v", err)
		return
	}

	req, err := http.NewRequest("GET", "http://localhost:9995/ipcam/test-path/channel", nil)
	require.NoError(t, err)

	client := &http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, res.StatusCode)
}
