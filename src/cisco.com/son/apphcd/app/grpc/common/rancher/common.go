// Author  <dorzheho@cisco.com>

package rancher

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/clientbase"
	ntypes "github.com/rancher/norman/types"
	clusterClient "github.com/rancher/types/client/cluster/v3"
	managementClient "github.com/rancher/types/client/management/v3"
	projectClient "github.com/rancher/types/client/project/v3"
	"github.com/spf13/viper"
)

// Get master client
func GetClient(projectName string) (*MasterClient, error) {
	// Lookup configuration file
	cf, err := lookupConfig(projectName)
	if nil != err {
		return nil, err
	}

	// Create Master client
	mc, err := NewMasterClient(cf)
	if nil != err {
		return nil, err
	}
	return mc, nil
}

// Load Rancher configuration
func loadConfig(path string) (Config, error) {
	// Construct client configuration
	cf := Config{
		Path:    path,
		Servers: make(map[string]*ServerConfig),
	}

	// Read the config file
	content, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return cf, nil
	}

	if err != nil {
		return cf, err
	}

	// Parse the file content
	err = json.Unmarshal(content, &cf)
	cf.Path = path
	return cf, err
}

// Find appropriate configuration file
func lookupConfig(projectName string) (*ServerConfig, error) {
	// Set configuration file path
	path := os.ExpandEnv("${HOME}/.apphc/adapters/rancher/projects/" + projectName + ".json")

	// Load configuration
	cf, err := loadConfig(path)
	if nil != err {
		return nil, err
	}

	// Find available server
	cs := cf.Server()
	if cs == nil {
		return nil, errors.New("no configuration found")
	}
	return cs, nil
}

// GetResourceType maps an incoming resource type to a valid one from the schema
func GetResourceType(c *MasterClient, resource string) (string, error) {
	// Management client
	if c.ManagementClient != nil {
		for key := range c.ManagementClient.APIBaseClient.Types {
			if strings.ToLower(key) == strings.ToLower(resource) {
				return key, nil
			}
		}
	}

	// Project client
	if c.ProjectClient != nil {
		for key := range c.ProjectClient.APIBaseClient.Types {
			if strings.ToLower(key) == strings.ToLower(resource) {
				return key, nil
			}
		}
	}

	// Cluster client
	if c.ProjectClient != nil {
		for key := range c.ClusterClient.APIBaseClient.Types {
			if strings.ToLower(key) == strings.ToLower(resource) {
				return key, nil
			}
		}
	}

	return "", fmt.Errorf("unknown resource type: %s", resource)
}

// Lookup finds appropriate resource according to schema
func Lookup(c *MasterClient, name string, types ...string) (*ntypes.Resource, error) {
	var byName *ntypes.Resource
	// Iterate over available types
	for _, schemaType := range types {
		// Get resource type according to schema
		rt, err := GetResourceType(c, schemaType)
		if err != nil {
			return nil, err
		}

		var schemaClient clientbase.APIBaseClientInterface
		// The schemaType dictates which client we need to use
		// Find appropriate schema related to the management client
		if c.ManagementClient != nil {
			if _, ok := c.ManagementClient.APIBaseClient.Types[rt]; ok {
				schemaClient = c.ManagementClient
			}
		}

		// Find appropriate schema related to the project client
		if c.ProjectClient != nil {
			if _, ok := c.ProjectClient.APIBaseClient.Types[rt]; ok {
				schemaClient = c.ProjectClient
			}
		}

		// Find appropriate schema related to the cluster client
		if c.ClusterClient != nil {
			if _, ok := c.ClusterClient.APIBaseClient.Types[rt]; ok {
				schemaClient = c.ClusterClient
			}
		}

		// Attempt to get the resource by ID
		var resource ntypes.Resource
		if err := schemaClient.ByID(schemaType, name, &resource); !clientbase.IsNotFound(err) && err != nil {
			return nil, err
		}
		if err == nil && resource.ID == name {
			return &resource, nil
		}

		// Resource was not found assuming the ID, check if it's the name of a resource
		var collection ntypes.ResourceCollection
		listOpts := &ntypes.ListOpts{
			Filters: map[string]interface{}{
				"name":         name,
				"removed_null": 1,
			},
		}

		// Find matches for the schemaType
		if err := schemaClient.List(schemaType, listOpts, &collection); err != nil {
			return nil, err
		}

		// If found
		if len(collection.Data) > 1 {
			var ids []string
			for _, data := range collection.Data {
				ids = append(ids, data.ID)
			}
			return nil, fmt.Errorf("multiple resources of type %s found for name %s: %v", schemaType, name, ids)
		}

		// No matches for this schemaType, try the next one
		if len(collection.Data) == 0 {
			continue
		}

		// If multiple resources for a particular name exist
		if byName != nil {
			return nil, fmt.Errorf("multiple resources named %s: %s:%s, %s:%s", name, collection.Data[0].Type,
				collection.Data[0].ID, byName.Type, byName.ID)
		}

		// Select the first element from the collection data array
		byName = &collection.Data[0]
	}

	// Cannot find appropriate resource
	if byName == nil {
		return nil, fmt.Errorf("not found: %s", name)
	}
	return byName, nil
}

// Get Project context.
// Project name appears in the App Hoster controller config file
func GetProjectContext(c *MasterClient, projectName string) (*managementClient.Project, error) {

	// Get project name
	if projectName == "" {
		return nil, errors.New("missing project configuration: " + projectName)
	}

	// Set options
	opts := DefaultListOpts()
	opts.Filters["name"] = projectName

	// Check whether project already exists
	projectCollection, err := c.ManagementClient.Project.List(opts)
	if err != nil {
		return nil, err
	}

	// Create the project if doesn't exist
	if len(projectCollection.Data) == 0 {
		clusterName := viper.GetString(EnvApphcAdaptersRancherClusterName)
		if projectName == "" {
			return nil, errors.New("missing cluster configuration: " + EnvApphcAdaptersRancherClusterName)
		}

		// Find appropriate cluster
		resource, err := Lookup(c, clusterName, "cluster")
		if nil != err {
			return nil, err
		}

		p := &managementClient.Project{}
		p.Name = projectName
		p.ClusterID = resource.ID
		p, err = c.ManagementClient.Project.Create(p)
		if err != nil {
			return nil, err
		}

		return p, nil
	}

	// Handle only one project
	return &projectCollection.Data[0], nil
}

// Returns a list of essential options
func baseListOpts() *ntypes.ListOpts {
	return &ntypes.ListOpts{
		Filters: map[string]interface{}{
			"limit": -2,
			"all":   true,
		},
	}
}

// DefaultListOpts returns a list of default options.
func DefaultListOpts() *ntypes.ListOpts {
	listOpts := baseListOpts()
	listOpts.Filters["system"] = "false"
	return listOpts
}

func GetEndpoint(pc *projectClient.Client, mgt *managementClient.Client, namespace, appName, externalIp string) (endpoint string, err error) {
	opts := DefaultListOpts()
	opts.Filters["namespaceId"] = namespace
	opts.Filters["name"] = appName

	s, err := pc.Service.List(opts)
	if err != nil {
		return
	}

	if len(s.Data) == 0 {
		err = fmt.Errorf("service %s not found", appName)
		return
	}

	var endpointAddress string

	if externalIp == "" {
		endpointAddress = s.Data[0].PublicEndpoints[0].Addresses[0]
		if endpointAddress == "<nil>" {
			endpointAddress, err = getEndpointAddress(mgt)
			if err != nil {
				return
			}
		}
	} else {
		endpointAddress = externalIp
	}

	endpoint = fmt.Sprintf("%s:%d", endpointAddress, s.Data[0].PublicEndpoints[0].Port)
	return
}

// RelocateCoreServices moves all namespaces related to the core services to dedicated project
func RelocateCoreServices(c *MasterClient) error {
	// Get the project ID
	project, err := GetProjectContext(c, SvcsProjectName)
	if err != nil {
		return err
	}

	// Find all configured namespaces
	for _, n := range strings.Split(SvcsProjectNamespaces, ",") {
		// Find namespace related to the core service
		ns, err := findNamespace(c, strings.TrimSpace(n))
		if err != nil {
			return err
		}

		if ns != nil {
			// Move the namespace to dedicated project
			if err := c.ClusterClient.Namespace.ActionMove(ns, &clusterClient.NamespaceMove{project.ID}); err != nil {
				return err
			}
		}
	}

	return nil
}

func findNamespace(apiClient *MasterClient, nsName string) (*clusterClient.Namespace, error) {
	filter := DefaultListOpts()
	filter.Filters["name"] = nsName

	// Find namespace according to the filter
	namespaces, err := apiClient.ClusterClient.Namespace.List(filter)
	if err != nil {
		return nil, err
	}

	// If the namespace doesn't exist
	if len(namespaces.Data) == 0 {
		return nil, nil
	}

	return &namespaces.Data[0], nil
}

func getEndpointAddress(mgt *managementClient.Client) (endpointAddr string, err error) {
	c, err := mgt.Node.List(DefaultListOpts())
	if err != nil {
		return
	}

	if len(c.Data) == 0 {
		err = fmt.Errorf("no AppHoster nodes found")
		return
	}

	endpointAddr = c.Data[0].IPAddress
	return
}
