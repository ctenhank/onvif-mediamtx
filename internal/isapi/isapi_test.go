package isapi_test

import (
	"testing"

	"github.com/ctenhank/mediamtx/internal/isapi"
)

const (
	host     = "http://192.168.1.64"
	username = "username"
	password = "password"
)

func TestGetNetworkIntegrate(t *testing.T) {
	networkIntegrate, err := isapi.GetNetworkIntegrate(isapi.HostParams{
		Host:     host,
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Errorf("GetNetworkIntegrate() error = %v", err)
		return
	}
	t.Logf("GetNetworkIntegrate() = %v", networkIntegrate)
}

func TestSetNetworkIntegrate(t *testing.T) {
	isapiResponseStatus, err := isapi.SetNetworkIntegrate(isapi.IntegrateParams{
		HostParams: isapi.HostParams{
			Host:     host,
			Username: username,
			Password: password,
		},
		CGIEnable:            false,
		CGICertificateType:   "digest",
		ONVIFEnable:          true,
		ONVIFCertificateType: "digest",
		ISAPIEnable:          true,
	})
	if err != nil {
		t.Errorf("SetNetworkIntegrate() error = %v", err)
		return
	}
	t.Logf("SetNetworkIntegrate() = %v", isapiResponseStatus)
}

func TestGetOnvifUserList(t *testing.T) {
	resp, err := isapi.GetOnvifUserList(isapi.HostParams{
		Host:     host,
		Username: username,
		Password: password,
	})

	if err != nil {
		t.Errorf("GetOnvifUserList() error = %v", err)
		return
	}

	t.Logf("GetOnvifUserList() = %v", resp)
}

func TestCreateOnvifUser(t *testing.T) {
	resp, err := isapi.CreateOnvifUser(isapi.UserCreateParams{
		UserForm: isapi.UserForm{
			User: isapi.User{
				UserType: isapi.MediaUser,
				UserName: "onviftest4",
				Id:       4,
			},
			Password: "test1234",
		},
		HostParams: isapi.HostParams{
			Host:     host,
			Username: username,
			Password: password,
		},
	})

	if err != nil {
		t.Errorf("CreateOnvifUser() error = %v", err)
		return
	}

	t.Logf("CreateOnvifUser() = %v", resp)
}

func TestDeleteOnvifUser(t *testing.T) {
	resp, err := isapi.DeleteOnvifUser(isapi.UserDeleteParams{
		Id: 4,
		HostParams: isapi.HostParams{
			Host:     host,
			Username: username,
			Password: password,
		},
	})

	if err != nil {
		t.Errorf("DeleteOnvifUser() error = %v", err)
		return
	}

	t.Logf("DeleteOnvifUser() = %v", resp)
}
