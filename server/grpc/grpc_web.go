package grpc

import (
	"fmt"
	"net/http"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc"

	"github.com/line/lbm-sdk/server/config"
	"github.com/line/lbm-sdk/server/types"
)

// StartGRPCWeb starts a gRPC-Web server on the given address.
func StartGRPCWeb(grpcSrv *grpc.Server, config config.Config) (*http.Server, error) {
	var options []grpcweb.Option
	if config.GRPCWeb.EnableUnsafeCORS {
		options = append(options,
			grpcweb.WithOriginFunc(func(origin string) bool {
				return true
			}),
		)
	}

	wrappedServer := grpcweb.WrapServer(grpcSrv, options...)
	grpcWebSrv := &http.Server{
		Addr:              config.GRPCWeb.Address,
		Handler:           wrappedServer,
		ReadHeaderTimeout: 0, // Added to fix lint error
	}

	errCh := make(chan error)
	go func() {
		if err := grpcWebSrv.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("[grpc] failed to serve: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return nil, err
	case <-time.After(types.ServerStartTime): // assume server started successfully
		return grpcWebSrv, nil
	}
}
