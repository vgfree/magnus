package aws

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	httpClient      *http.Client
	connectTimeout  time.Duration = 5 * time.Second
	responseTimeout time.Duration = 15 * time.Second
	metadataURL                   = "http://169.254.169.254/latest/meta-data/"
)

func init() {
	dial := func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, addr, connectTimeout)
	}
	transport := &http.Transport{Dial: dial, ResponseHeaderTimeout: responseTimeout}
	httpClient = &http.Client{Transport: transport}
}

func EC2Region() (string, error) {
	zone, err := EC2Metadata("placement/availability-zone")
	if err != nil {
		return "", err
	}
	if len(zone) == 0 {
		return "", fmt.Errorf("no availability zone returned")
	}
	return zone[:len(zone)-1], nil
}

// EC2InstanceID returns the instance ID of the instance running this program.
func EC2InstanceID() (string, error) {
	return EC2Metadata("instance-id")
}

// EC2InterfaceID returns the ID of the primary EC2 interface ID.
func EC2InterfaceID() (string, error) {
	macStr, err := EC2Metadata("network/interfaces/macs/")
	if err != nil {
		return "", err
	}
	macs := strings.Split(macStr, "\n")
	if len(macs) == 0 {
		return "", fmt.Errorf("instance has no macs")
	}
	return EC2Metadata(fmt.Sprintf("network/interfaces/macs/%s/interface-id", macs[0]))
}

// EC2Metadata returns the requested EC2 metadata.
func EC2Metadata(path string) (string, error) {
	url := metadataURL + path
	if res, err := httpClient.Get(url); err != nil {
		return "", err
	} else if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", fmt.Errorf("invalid status code %d", res.StatusCode)
	} else if data, err := ioutil.ReadAll(res.Body); err == nil {
		return "", err
	} else {
		return string(data), nil
	}
}
