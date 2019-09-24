// Author  <dorzheho@cisco.com>

package controller

import (
	"cisco.com/son/apphcd/app/common/mutex"
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
)

// Controller structure
type Controller struct {
	serverPort int          // Server port number
	listener   net.Listener // Network listener
	httpServer *http.Server // HTTP server
	grpcServer *grpc.Server // gRPC server
}

// New returns a Controller instance
func New(serverPort int, listener net.Listener) *Controller {
	return &Controller{
		serverPort: serverPort,
		listener:   listener,
	}
}

// Start the Controller
func (c *Controller) Start() error {
	logrus.Info("Starting Controller")

	// Set context
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create TCP multiplexer
	tcpMux := cmux.New(c.listener)

	// Instantiate appropriate listeners
	grpcL := tcpMux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldPrefixSendSettings("content-type", "application/grpc"))
	httpL := tcpMux.Match(cmux.HTTP1Fast("PATCH"))

	// Initialize gRPC server instance
	var err error
	c.grpcServer, err = newGrpcServer()
	if err != nil {
		return fmt.Errorf("unable to initialize gRPC server instance: %v", err)
	}

	// Initialize HTTP server instance
	c.httpServer, err = newHttpServer(ctx, c.serverPort)
	if err != nil {
		return fmt.Errorf("unable to initialize HTTP server instance: %v", err)
	}

	// Start gRPC server
	go func() {
		if err := c.grpcServer.Serve(grpcL); err != nil {
			logrus.Fatalln("unable to start external gRPC server")
		}
	}()

	// Start HTTP server
	go func() {
		if err := c.httpServer.Serve(httpL); err != nil {
			logrus.Fatalln("unable to start HTTP server")
		}
	}()

	// Create new mutex
	mutex.New()

	logrus.Info("Instantiating TCP Multiplexer")
	logrus.Infof("AppHoster Controller is listening on port %d", c.serverPort)
	return tcpMux.Serve()
}
