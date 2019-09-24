// Author  <dorzheho@cisco.com>

package controller

import (
	"context"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"cisco.com/son/apphcd/api/v1/apphcmanager"
	pbappmgr "cisco.com/son/apphcd/api/v1/appmanager"
	"cisco.com/son/apphcd/api/v1/clustermanager"
	"cisco.com/son/apphcd/app/common"
)

// gRPC server static IP.
// Since the server is running locally we use localhost for the endpoint
const grpcServerAddr = "127.0.0.1"

// newGateway creates new gRPC gateway
func newGateway(ctx context.Context, serverPort int) (http.Handler, error) {
	logrus.Info("Instantiating gRPC Gateway")

	// gRPC dial up options
	opts := []grpc.DialOption{
		//grpc.WithBlock(),
		grpc.WithInsecure(),
	}
	// Create gRPC connection
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", grpcServerAddr, serverPort), opts...)
	if err != nil {
		return nil, err
	}

	// Changes json serializer to include empty fields with default values
	gwMux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{OrigName: true, EmitDefaults: true}),
		runtime.WithProtoErrorHandler(runtime.DefaultHTTPProtoErrorHandler),
	)

	// Register Gateway endpoints
	// If Rancher adapter enabled
	if viper.GetBool(common.EnvApphcAdaptersRancherEnabled) {
		// Register Application manager service
		logrus.Info("Registering HTTP handlers for service AppManager")
		if err := pbappmgr.RegisterAppManagerHandler(ctx, gwMux, conn); err != nil {
			return nil, err
		}
	}

	// Register AppHoster Controller manager service
	if err := apphcmanager.RegisterApphcManagerHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}

	// Register AppHoster Cluster manager service
	if err := clustermanager.RegisterClusterManagerHandler(ctx, gwMux, conn); err != nil {
		return nil, err
	}

	return gwMux, nil
}
