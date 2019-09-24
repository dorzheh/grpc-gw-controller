// Author  <dorzheho@cisco.com>

package rancher

import (
	"fmt"

	managementClient "github.com/rancher/types/client/management/v3"
)

// GetClusterName returns a name of the cluster to which the Controller is bound
func GetClusterName(c *MasterClient) (string, error) {
	// Get collection according to the default options
	collection, err := c.ManagementClient.Cluster.List(DefaultListOpts())
	if err != nil {
		return "", err
	}

	// Iterate over the collection
	for _, item := range collection.Data {
		// if item ID is equal to the cluster ID
		if item.ID == c.UserConfig.Cluster() {
			// get cluster name
			return getClusterName(&item), nil
		}
	}
	return "", fmt.Errorf("cannot resolve cluster")
}

// getClusterByID gets Cluster by ID
func getClusterByID(c *MasterClient, clusterID string) (*managementClient.Cluster, error) {
	// Get collection according to the default options
	cluster, err := c.ManagementClient.Cluster.ByID(clusterID)
	if err != nil {
		return nil, fmt.Errorf("no cluster found with the ID [%s]: %s", clusterID, err)
	}
	return cluster, nil
}

// getClusterName gets name of the cluster.If the name is empty, returns cluster ID
func getClusterName(cluster *managementClient.Cluster) string {
	if cluster.Name != "" {
		return cluster.Name
	}
	return cluster.ID
}

// ClusterKubeConfig returns Kubernetes cluster configuration
func ClusterKubeConfig(c *MasterClient) (string, error) {
	// Get cluster name
	clusterName, err := GetClusterName(c)
	if err != nil {
		return "", err
	}

	// Find resource
	resource, err := Lookup(c, clusterName, "cluster")
	if nil != err {
		return "", err
	}

	// Get cluster by ID
	cluster, err := getClusterByID(c, resource.ID)
	if nil != err {
		return "", err
	}

	// Get Kubernetes configuration for a particular cluster
	config, err := c.ManagementClient.Cluster.ActionGenerateKubeconfig(cluster)
	if nil != err {
		return "", err
	}
	return config.Config, nil
}
