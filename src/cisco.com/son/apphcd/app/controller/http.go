// Author  <dorzheho@cisco.com>

package controller

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	openapi "cisco.com/son/apphcd/api/v1/appmanager"
	"cisco.com/son/apphcd/pkg/ui/data/swagger"
	"github.com/philips/go-bindata-assetfs"
)

// HTTP server static IP.
// Since the server is running locally we use localhost for the endpoint
const httpServerAddr = "127.0.0.1"

// newHttpServer creates new HTTP server
func newHttpServer(ctx context.Context, serverPort int) (*http.Server, error) {
	logrus.Info("Instantiating HTTP server")

	// Create HTTP router
	router := http.NewServeMux()
	router.HandleFunc("/swagger.json", func(w http.ResponseWriter, req *http.Request) {
		_,_ = io.Copy(w, strings.NewReader(openapi.Swagger))
	})

	// Serve Swagger
	serveSwagger(router)

	// Initialize gRPC Gateway
	gw, err := newGateway(ctx, serverPort)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize gRPC Gateway: %v", err)
	}

	router.Handle("/", gw)

	// Return HTTP Server instance
	return &http.Server{
		Addr:        fmt.Sprintf("%s:%d", httpServerAddr, serverPort),
		Handler:     router,
		IdleTimeout: 120 * time.Second,
	}, nil
}

// serveSwagger serves Swagger WebUI
func serveSwagger(mux *http.ServeMux) {
	_ = mime.AddExtensionType(".svg", "image/svg+xml")

	// Expose files in third_party/swagger-ui/ on <host>/swagger-ui
	fileServer := http.FileServer(&assetfs.AssetFS{
		Asset:    swagger.Asset,
		AssetDir: swagger.AssetDir,
		Prefix:   "third_party/swagger-ui",
	})

	prefix := "/swagger-ui/"
	mux.Handle(prefix, http.StripPrefix(prefix, fileServer))
}
