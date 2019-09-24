// Author  <dorzheho@cisco.com>

package controller

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"k8s.io/client-go/kubernetes"

	pbapphcmgr "cisco.com/son/apphcd/api/v1/apphcmanager"
	pbappmgr "cisco.com/son/apphcd/api/v1/appmanager"
	pbclumgr "cisco.com/son/apphcd/api/v1/clustermanager"
	appcommon "cisco.com/son/apphcd/app/common"
	"cisco.com/son/apphcd/app/grpc/apphcmanager"
	"cisco.com/son/apphcd/app/grpc/appmanager"
	rappmgr "cisco.com/son/apphcd/app/grpc/appmanager/adapters/rancher"
	appmgrcommon "cisco.com/son/apphcd/app/grpc/appmanager/common"
	"cisco.com/son/apphcd/app/grpc/clustermanager"
	rclumgr "cisco.com/son/apphcd/app/grpc/clustermanager/adapters/rancher"
	clumgrcommon "cisco.com/son/apphcd/app/grpc/clustermanager/common"
	grpccommon "cisco.com/son/apphcd/app/grpc/common"
	"cisco.com/son/apphcd/app/grpc/common/rancher"
)

const (
	defaultGitPort = 30080
	defaultApphDockerRegistryPort = 5000
)

// newGrpcServer creates new gRPC server
func newGrpcServer() (*grpc.Server, error) {
	logrus.Info("Instantiating gRPC server")

	var grpcServer *grpc.Server

	// Create new gRPC server
	if viper.GetBool(appcommon.EnvApphcInternalAuthorizationEnabled) {
		// Create with OAuth (bearer token) authorization support
		authFunc := func(ctx context.Context) (context.Context, error) {
			token, err := grpc_auth.AuthFromMD(ctx, "bearer")
			if err != nil {
				return nil, err
			}

			// Remove leading/trailing spaces
			token = strings.TrimSpace(token)

			// Compare against preconfigured token
			if token != viper.GetString(appcommon.EnvApphcBearerToken) {
				return nil, fmt.Errorf("%s: invalid bearer token", codes.Unauthenticated)
			}

			// Set the new context
			newCtx := context.WithValue(ctx, "tokenInfo", token)
			return newCtx, nil
		}

		grpcServer = grpc.NewServer(grpc.StreamInterceptor(grpc_auth.StreamServerInterceptor(authFunc)),
			grpc.UnaryInterceptor(grpc_auth.UnaryServerInterceptor(authFunc)))
	} else {
		// Create insecure server
		grpcServer = grpc.NewServer()
	}

	logrus.Info("Registering ApphcManager service to gRPC")

	pbapphcmgr.RegisterApphcManagerServer(grpcServer, &apphcmanager.ApphcManager{})

	var appmgrAdapter appmgrcommon.AppManagerAdapter
	var clumgrAdapter clumgrcommon.ClusterManagerAdapter
	var kubeClient *kubernetes.Clientset

	if viper.GetBool(appcommon.EnvApphcAdaptersRancherEnabled) {
		if err := rancher.LoginSetup(viper.GetString(rancher.EnvApphcAdaptersRancherServerEndpoint),
			viper.GetString(rancher.EnvApphcAdaptersRancherServerCredsToken), rancher.AppsProjectName); err != nil {
			return nil, err

		}
		if err := rancher.LoginSetup(viper.GetString(rancher.EnvApphcAdaptersRancherServerEndpoint),
			viper.GetString(rancher.EnvApphcAdaptersRancherServerCredsToken), rancher.SvcsProjectName); err != nil {
			return nil, err

		}
		// Create Rancher client for Services project
		svcsClient, err := rancher.GetClient(rancher.SvcsProjectName)
		if err != nil {
			return nil, err
		}

		if err := rancher.RelocateCoreServices(svcsClient); err != nil {
			return nil, err
		}

		gitEndpoint, err := rancher.GetEndpoint(svcsClient.ProjectClient, svcsClient.ManagementClient,
			grpccommon.ServiceGitNamespace, grpccommon.ServiceGitName, viper.GetString(appcommon.EnvApphExternalIp))
		if err != nil {
			if strings.Contains(err.Error(),"not found") {
				gitEndpoint = fmt.Sprintf("%s:%d",viper.GetString(appcommon.EnvApphExternalIp),defaultGitPort)
			} else {
				return nil, err
			}
		}

		viper.SetDefault(appcommon.EnvApphcGitServerEndpoint, gitEndpoint)

		collection, err := svcsClient.ManagementClient.Node.List(rancher.DefaultListOpts())
		if nil != err {
			return nil, err
		}

		var masterIpAddress string
		if len(collection.Data) == 1 {
			masterIpAddress = collection.Data[0].IPAddress
		} else {
			for _, n := range collection.Data {
				if l := appcommon.MapGet(n.Labels, appcommon.ApphNodeRoleLabel); l != "" {
					masterIpAddress = n.IPAddress
					break
				}
			}
		}

		if viper.GetString(appcommon.EnvApphcPrivateDockerRegistry) == "" {
			viper.SetDefault(appcommon.EnvApphcPrivateDockerRegistry, fmt.Sprintf("%s:%d", masterIpAddress, defaultApphDockerRegistryPort))
		}

		extIp := viper.GetString(appcommon.EnvApphExternalIp)
		if  extIp != "" {
			viper.SetDefault(appcommon.EnvApphMasterNodeIp, extIp)
		} else {
			viper.SetDefault(appcommon.EnvApphMasterNodeIp, masterIpAddress)
		}

		// Create Rancher client for Applications project
		appsMasterClient, err := rancher.GetClient(rancher.AppsProjectName)
		if err != nil {
			return nil, err
		}

		logrus.Debug("Instantiating Rancher adapters")

		clumgrAdapter = rclumgr.NewAdapter(appsMasterClient, svcsClient.ProjectClient)
		if err := createKubeConfigFile(clumgrAdapter); err != nil {
			return nil, err
		}

		kubeClient, err = grpccommon.KubeClientset()
		if err != nil {
			return nil, err
		}

		// Create adapter for Application manager
		appmgrAdapter, err = rappmgr.NewAdapter(appsMasterClient, svcsClient.ProjectClient, kubeClient)
		if err != nil {
			return nil, err
		}

		logrus.Debugf("Server endpoint: %s", viper.GetString(rancher.EnvApphcAdaptersRancherServerEndpoint))
	}

	logrus.Info("Registering AppManager service to gRPC")

	// Register Application manager gRPC server
	pbappmgr.RegisterAppManagerServer(grpcServer, appmanager.New(appmgrAdapter))

	logrus.Info("Registering ClusterManager service to gRPC")

	// Register Cluster manager gRPC server
	pbclumgr.RegisterClusterManagerServer(grpcServer, clustermanager.New(clumgrAdapter, kubeClient))

	// Return gRPC server
	return grpcServer, nil
}

func createKubeConfigFile(adapter clumgrcommon.ClusterManagerAdapter) error {
	// Fetch kubernetes configuration
	req := &pbclumgr.GetKubeConfigRequest{}
	cfg, err := adapter.GetKubeConfig(req)
	if err != nil {
		return err
	}

	// Create kubernetes config file
	return ioutil.WriteFile(appcommon.ApphcKubeconfigPath, []byte(cfg.Kubeconfig), 0644)
}
