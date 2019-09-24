// Author <dorzheho@cisco.com>

package common

import (
	"bytes"
	"text/template"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"cisco.com/son/apphcd/api/v1/clustermanager"
)

// GenerateResponse creates appropriate response
func GenerateResponse(statusCode clustermanager.Status, msg string, m proto.Message) (*clustermanager.Response, error) {
	var err error
	resp := new(clustermanager.Response)

	if m != nil {
		// Create body
		resp.Body, err = ptypes.MarshalAny(m)
		if err != nil {
			msg = err.Error()
		}
	}

	// Set timestamp
	resp.Timestamp = ptypes.TimestampNow()
	// Set status code
	resp.Status = statusCode
	// Set message
	resp.Message = msg

	// If the protobuf message is nil we assume that an error occurred
	// hence the error message is printed
	if statusCode == clustermanager.Status_ERROR {
		logrus.Error(msg)
	}

	logrus.WithFields(logrus.Fields{"service": "ClusterManager", "type": "grpc", "status": statusCode}).Info("Sending response")
	logrus.Debugf("Response message: %q", resp.String())

	return resp, nil
}

func GetState(state string) clustermanager.State {

	var s clustermanager.State

	switch state {
	case NodeStateActive:
		s = clustermanager.State_active
	case NodeStateCordoned:
		s = clustermanager.State_unschedulable
	case NodeStateDrained:
		s = clustermanager.State_maintenance
	}

	return s
}

func GetNodeMonitorUrl(endpoint, hostname string) (string, error) {

	type Node struct {
		MonitorEndpoint string
		Hostname        string
	}

	t, err := template.New("node").Parse(viper.GetString(TemplatesUrlMonitorNodes))
	if err != nil {
		return "", err
	}

	var url bytes.Buffer

	n := &Node{MonitorEndpoint: endpoint, Hostname: hostname}
	if err := t.Execute(&url, n); err != nil {
		return "", err
	}

	return url.String(), nil
}

func GetServicesMonitorUrl(endpoint string) (string, error) {

	type Services struct {
		MonitorEndpoint string
	}

	t, err := template.New("services").Parse(viper.GetString(TemplatesUrlMonitorServices))
	if err != nil {
		return "", err
	}

	var url bytes.Buffer

	n := &Services{MonitorEndpoint: endpoint}
	if err := t.Execute(&url, n); err != nil {
		return "", err
	}

	return url.String(), nil
}
