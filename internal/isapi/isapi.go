package isapi

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	gourl "net/url"

	"github.com/icholy/digest"
)

type OnvifUserType string

const (
	Admin     OnvifUserType = "administrator"
	Operator  OnvifUserType = "operator"
	MediaUser OnvifUserType = "mediaUser"
)

const (
	SYSTEM_INTEGRATE_ENDPOINT = "/ISAPI/System/Network/Integrate"
	ONVIF_USER_ENDPOINT       = "/ISAPI/Security/ONVIF/users"
	xmlns                     = "http://www.isapi.org/ver20/XMLSchema"
)

type CGI struct {
	Enable          bool   `xml:"enable"`
	CertificateType string `xml:"certificateType"`
}

type ONVIF struct {
	Enable          bool   `xml:"enable"`
	CertificateType string `xml:"certificateType"`
}

type ISAPI struct {
	Enable bool `xml:"enable"`
}

type NetworkIntegrate struct {
	XMLName xml.Name `xml:"Integrate"`
	CGI     CGI      `xml:"CGI"`
	ONVIF   ONVIF    `xml:"ONVIF"`
	ISAPI   ISAPI    `xml:"ISAPI"`
}

type UserList struct {
	User []User
}

type User struct {
	Id       int           `xml:"id"`
	UserName string        `xml:"userName"`
	UserType OnvifUserType `xml:"userType"`
}

type UserListForm struct {
	XMLName  xml.Name `xml:"UserList"`
	Version  string   `xml:"version,attr"`
	UserForm UserForm
}

type UserForm struct {
	XMLName xml.Name `xml:"User"`
	User
	Password string `xml:"password"`
}

type ResponseStatus struct {
	Version       string `xml:"version,attr"`
	StatusCode    int    `xml:"statusCode"`
	StatusString  string `xml:"statusString"`
	SubStatusCode string `xml:"subStatusCode"`
	RequestURL    string `xml:"requestURL"`
}

type HostParams struct {
	Host     string
	Username string
	Password string
}

type IntegrateParams struct {
	HostParams
	CGIEnable            bool
	CGICertificateType   string // "digest" or "digest/WSSE" or "WSSE"
	ONVIFEnable          bool
	ONVIFCertificateType string // "digest" or "digest/WSSE" or "WSSE"
	ISAPIEnable          bool
}

type UserCreateParams struct {
	HostParams
	UserForm UserForm
}

type UserDeleteParams struct {
	HostParams
	Id int
}

func ReadAndParse(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.New("http response read error")
	}

	err = xml.Unmarshal(data, v)
	if err != nil {
		return errors.New("xml unmarshal error")
	}

	return nil
}

func sendPutMethod(client http.Client, url string, data []byte) (*http.Response, error) {
	u, err := gourl.Parse(url)
	if err != nil {
		return nil, errors.New("url parse error")
	}

	req, err := http.NewRequest("PUT", u.String(), bytes.NewReader(data))

	if err != nil {
		return nil, errors.New("http request error")
	}

	return client.Do(req)
}

func sendDeleteMethod(client http.Client, url string, data []byte) (*http.Response, error) {
	u, err := gourl.Parse(url)
	if err != nil {
		return nil, errors.New("url parse error")
	}

	req, err := http.NewRequest("DELETE", u.String(), bytes.NewReader(data))

	if err != nil {
		return nil, errors.New("http request error")
	}

	return client.Do(req)
}

func GetNetworkIntegrate(params HostParams) (*NetworkIntegrate, error) {
	var networkIntegrate NetworkIntegrate

	client := http.Client{
		Transport: &digest.Transport{
			Username: params.Username,
			Password: params.Password,
		},
	}

	resp, err := client.Get(params.Host + SYSTEM_INTEGRATE_ENDPOINT)

	if err != nil && resp.StatusCode != http.StatusOK {
		return nil, err
	}

	ReadAndParse(resp, &networkIntegrate)
	return &networkIntegrate, nil
}

func SetNetworkIntegrate(params IntegrateParams) (*ResponseStatus, error) {
	xmlForm := NetworkIntegrate{
		CGI: CGI{
			Enable:          params.CGIEnable,
			CertificateType: params.CGICertificateType,
		},
		ONVIF: ONVIF{
			Enable:          params.ONVIFEnable,
			CertificateType: params.ONVIFCertificateType,
		},
		ISAPI: ISAPI{
			Enable: params.ISAPIEnable,
		},
	}

	client := http.Client{
		Transport: &digest.Transport{
			Username: params.Username,
			Password: params.Password,
		},
	}

	data, err := xml.Marshal(xmlForm)
	if err != nil {
		return nil, errors.New("xml marshal error")
	}

	resp, err := sendPutMethod(client, params.Host+SYSTEM_INTEGRATE_ENDPOINT, data)

	if err != nil && resp.StatusCode != http.StatusOK {
		return nil, err
	}

	var reply ResponseStatus
	err = ReadAndParse(resp, &reply)
	if err != nil {
		return nil, err
	}

	return &reply, nil
}

func GetOnvifUserList(params HostParams) (*UserList, error) {
	client := http.Client{
		Transport: &digest.Transport{
			Username: params.Username,
			Password: params.Password,
		},
	}

	resp, err := client.Get(params.Host + ONVIF_USER_ENDPOINT)

	if err != nil && resp.StatusCode != http.StatusOK {
		return nil, err
	}

	var userList UserList
	err = ReadAndParse(resp, &userList)
	if err != nil {

		return nil, errors.New("xml unmarshal error")
	}

	return &userList, nil
}

func CreateOnvifUser(params UserCreateParams) (*ResponseStatus, error) {
	client := http.Client{
		Transport: &digest.Transport{
			Username: params.Username,
			Password: params.Password,
		},
	}

	user := UserListForm{UserForm: UserForm{
		User: User{
			Id:       params.UserForm.Id,
			UserName: params.UserForm.UserName,
			UserType: params.UserForm.UserType,
		},
		Password: params.UserForm.Password,
	}, Version: "2.0"}

	data, err := xml.Marshal(user)

	if err != nil {
		return nil, errors.New("xml marshal error")
	}

	resp, err := sendPutMethod(client, params.Host+ONVIF_USER_ENDPOINT, data)

	if err != nil && resp.StatusCode != http.StatusOK {
		return nil, err
	}

	var reply ResponseStatus
	err = ReadAndParse(resp, &reply)
	if err != nil {
		return nil, errors.New("xml unmarshal error")
	}

	return &reply, nil
}

func DeleteOnvifUser(params UserDeleteParams) (*ResponseStatus, error) {
	client := http.Client{
		Transport: &digest.Transport{
			Username: params.Username,
			Password: params.Password,
		},
	}

	url := params.Host + ONVIF_USER_ENDPOINT + "/" + fmt.Sprint(params.Id)
	resp, err := sendDeleteMethod(client, url, nil)

	if err != nil && resp.StatusCode != http.StatusOK {
		return nil, err
	}

	var reply ResponseStatus
	err = ReadAndParse(resp, &reply)
	if err != nil {
		return nil, errors.New("xml unmarshal error")
	}

	return &reply, nil
}
