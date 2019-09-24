// Author  <dorzheho@cisco.com>

package rancher

import (
	"encoding/json"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
)

// Config holds the main configuration
type Config struct {
	// Server configuration
	Servers map[string]*ServerConfig
	//Path to the config file
	Path string `json:"path,omitempty"`
	// CurrentServer the user has in focus
	CurrentServer string
}

//ServerConfig holds the config for each server the user has setup
type ServerConfig struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	TokenKey  string `json:"tokenKey"`
	URL       string `json:"url"`
	Project   string `json:"project"`
	CACerts   string `json:"cacert"`
}

// Write creates a new Rancher configuration file
func (c Config) Write() error {
	// Create directories tree
	err := os.MkdirAll(path.Dir(c.Path), 0700)
	if err != nil {
		return err
	}

	// Save configuration
	p := c.Path
	c.Path = ""

	logrus.Debugf("Saving config to %s", p)

	// Create the output file
	output, err := os.Create(p)
	if err != nil {
		return err
	}
	defer output.Close()

	// Write to the file
	return json.NewEncoder(output).Encode(c)
}

// Server returns server configuration
func (c Config) Server() *ServerConfig {
	return c.Servers[c.CurrentServer]
}

// Cluster returns name of the cluster
func (c ServerConfig) Cluster() string {
	return strings.Split(c.Project, ":")[0]
}

// EnvironmentURL returns URL for appropriate environment
func (c ServerConfig) EnvironmentURL() (string, error) {
	envUrl, err := baseURL(c.URL)
	if err != nil {
		return "", err
	}
	return envUrl, nil
}

// baseURL for API v3
func baseURL(fullURL string) (string, error) {
	idx := strings.LastIndex(fullURL, "/v3")
	if idx == -1 {
		u, err := url.Parse(fullURL)
		if err != nil {
			return "", err
		}
		newURL := url.URL{
			Scheme: u.Scheme,
			Host:   u.Host,
		}
		return newURL.String(), nil
	}
	return fullURL[:idx], nil
}
