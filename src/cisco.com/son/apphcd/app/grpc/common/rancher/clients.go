// Author  <dorzheho@cisco.com>

package rancher

import (
	"strings"

	"github.com/rancher/norman/clientbase"
	clusterClient "github.com/rancher/types/client/cluster/v3"
	managementClient "github.com/rancher/types/client/management/v3"
	projectClient "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type MasterClient struct {
	ClusterClient    *clusterClient.Client    // Cluster client
	ManagementClient *managementClient.Client // Management client
	ProjectClient    *projectClient.Client    // Project client
	UserConfig       *ServerConfig            // Server configuration
}

// NewMasterClient returns a new MasterClient with Cluster,
// Management and Project clients populated
func NewMasterClient(config *ServerConfig) (*MasterClient, error) {
	mc := &MasterClient{
		UserConfig: config,
	}

	// Check the project existence
	clusterProject := CheckProject(config.Project)
	if clusterProject == nil {
		logrus.Warn("No context set, some commands will not work.")
	}

	var g errgroup.Group

	// Instantiate new clients
	g.Go(mc.newClusterClient)
	g.Go(mc.newManagementClient)
	g.Go(mc.newProjectClient)

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return mc, nil
}

// NewManagementClient returns a new MasterClient with only the Management client
func NewManagementClient(config *ServerConfig) (*MasterClient, error) {
	mc := &MasterClient{
		UserConfig: config,
	}

	err := mc.newManagementClient()
	if err != nil {
		return nil, err
	}
	return mc, nil
}

// Creates a new Cluster client
func (mc *MasterClient) newClusterClient() error {
	options := createClientOpts(mc.UserConfig)
	options.URL = options.URL + "/clusters/" + mc.UserConfig.Cluster()

	// Setup the project client
	cc, err := clusterClient.NewClient(options)
	if err != nil {
		return err
	}
	mc.ClusterClient = cc

	return nil
}

// Creates a new Management client
func (mc *MasterClient) newManagementClient() error {
	options := createClientOpts(mc.UserConfig)

	// Setup the management client
	mClient, err := managementClient.NewClient(options)
	if err != nil {
		return err
	}
	mc.ManagementClient = mClient

	return nil
}

// Creates a new Project client
func (mc *MasterClient) newProjectClient() error {
	options := createClientOpts(mc.UserConfig)
	options.URL = options.URL + "/projects/" + mc.UserConfig.Project

	// Setup the project client
	pc, err := projectClient.NewClient(options)
	if err != nil {
		return err
	}
	mc.ProjectClient = pc
	return nil
}

// Creates client options
func createClientOpts(config *ServerConfig) *clientbase.ClientOpts {
	serverURL := config.URL
	// set APIv3
	if !strings.HasSuffix(serverURL, "/v3") {
		serverURL = config.URL + "/v3"
	}

	options := &clientbase.ClientOpts{
		URL:       serverURL,
		AccessKey: config.AccessKey,
		SecretKey: config.SecretKey,
		CACerts:   config.CACerts,
	}
	return options
}

// CheckProject verifies s matches the valid project ID of <cluster>:<project>
func CheckProject(s string) []string {
	clusterProject := strings.Split(s, ":")
	//
	if len(s) == 0 || len(clusterProject) != 2 {
		return nil
	}
	return clusterProject
}
