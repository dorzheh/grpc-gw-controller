// Author  <dorzheho@cisco.com>

package rancher

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/grantae/certinfo"
	managementClient "github.com/rancher/types/client/management/v3"
)

// LoginData stores appropriate data required for Rancher client
type LoginData struct {
	Project     *managementClient.Project
	Index       int
	ClusterName string
}

// CACertResponse stores CA Certificate
type CACertResponse struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// LoginSetup logs in to the Rancher server and creates appropriate identity file
func LoginSetup(serverName, token, projectName string) error {

	// path to the file
	path := os.ExpandEnv("${HOME}/.apphc/adapters/rancher/projects/" + projectName + ".json")

	// load configuration file from path
	cf, err := loadConfig(path)
	if err != nil {
		return err
	}

	// if no server name provided , use the default one
	if serverName == "" {
		serverName = "rancherDefault"
	}

	serverConfig := &ServerConfig{}

	// Validate the url and drop the path
	u, err := url.Parse(serverName)
	if err != nil {
		return err
	}

	u.Path = ""
	serverConfig.URL = u.String()

	// if token provided , split the token string
	if token != "" {
		auth := strings.Split(token, ":")
		if len(auth) != 2 {
			return errors.New("invalid token")
		}
		serverConfig.AccessKey = auth[0]
		serverConfig.SecretKey = auth[1]
		serverConfig.TokenKey = token
	} else {
		// This can be removed once username and password is accepted
		return errors.New("token is required")
	}

	// Create new management client
	c, err := NewManagementClient(serverConfig)
	if err != nil {
		if _, ok := err.(*url.Error); ok && strings.Contains(err.Error(), "certificate signed by unknown authority") {
			// no certificate was provided and it's most likely a self signed certificate if
			// we get here so grab the cacert and see if the user accepts the server
			c, err = getCertFromServer(serverConfig)
			if nil != err {
				return err
			}
		} else {
			return err
		}
	}

	// Get project context
	project, err := GetProjectContext(c, projectName)
	if err != nil {
		return err
	}

	// Set the default server and project for the user
	serverConfig.Project = project.ID
	cf.CurrentServer = serverName
	cf.Servers[serverName] = serverConfig

	return cf.Write()
}

// Get certificate from server
func getCertFromServer(cf *ServerConfig) (*MasterClient, error) {
	// Create a new HTTP request
	req, err := http.NewRequest("GET", cf.URL+"/v3/settings/cacerts", nil)
	if nil != err {
		return nil, err
	}

	// Set authentication
	req.SetBasicAuth(cf.AccessKey, cf.SecretKey)
	// create a RoundTripper
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Create client
	client := &http.Client{Transport: tr}
	// send the request
	res, err := client.Do(req)
	if nil != err {
		return nil, err
	}
	defer res.Body.Close()

	// Read the body
	content, err := ioutil.ReadAll(res.Body)
	if nil != err {
		return nil, err
	}

	// Decode JSON response
	var certReponse *CACertResponse
	err = json.Unmarshal(content, &certReponse)

	// Verify certificate
	cert, err := verifyCert([]byte(certReponse.Value))
	if nil != err {
		return nil, err
	}

	if _, err := processServerChain(res); err != nil {
		return nil, err
	}

	cf.CACerts = cert

	// Create new management client
	return NewManagementClient(cf)
}

// process HTTP response , fetch certificates
func processServerChain(res *http.Response) ([]string, error) {
	var allCerts []string

	for _, cert := range res.TLS.PeerCertificates {
		result, err := certinfo.CertificateText(cert)
		if err != nil {
			return allCerts, err
		}
		allCerts = append(allCerts, result)
	}
	return allCerts, nil
}

// Verify certificate
func verifyCert(caCert []byte) (string, error) {
	caCert = bytes.Replace(caCert, []byte(`\n`), []byte("\n"), -1)

	block, _ := pem.Decode(caCert)

	if block == nil {
		return "", errors.New("no cert was found")
	}

	// Parse the data
	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if nil != err {
		return "", err
	}

	// Verify that IsCA is valid
	if !parsedCert.IsCA {
		return "", errors.New("CACerts is not valid")
	}
	return string(caCert), nil
}
