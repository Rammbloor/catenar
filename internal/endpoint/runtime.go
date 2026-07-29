package endpoint

import (
	"context"
	"crypto/x509"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	grpccredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"catenar/internal/contracts"
)

type grpcRuntime struct {
	systemCertPool func() (*x509.CertPool, error)
}

func newGRPCRuntime(options grpcTransportAdapterOptions) GRPCRuntime {
	systemCertPool := options.systemCertPool
	if systemCertPool == nil {
		systemCertPool = x509.SystemCertPool
	}

	return &grpcRuntime{
		systemCertPool: systemCertPool,
	}
}

func (r *grpcRuntime) Dial(ctx context.Context, cfg EndpointRuntimeConfig) (GRPCClientConn, *endpointDiagnostic) {
	systemPool, tlsConfig, tlsErr := (&grpcTransportAdapter{
		systemCertPool: r.systemCertPool,
	}).buildClientTLSConfig(cfg)
	if tlsErr != nil {
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     tlsErr.Envelope.Code,
			Category: tlsErr.Envelope.Category,
			Message:  tlsErr.Envelope.Message,
			NextStep: tlsErr.NextStep,
			Details:  copyDetails(tlsErr.Envelope.Details),
		}
	}

	dialOptions := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if cfg.Endpoint.Authority != "" {
		dialOptions = append(dialOptions, grpc.WithAuthority(cfg.Endpoint.Authority))
	}

	if cfg.Endpoint.TLS.Mode == contracts.TLSModePlaintext {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		creds := newSplitAuthorityCredentials(grpccredentials.NewTLS(buildDialTLSConfig(tlsConfig, systemPool)), tlsConfig.ServerName)
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
	}

	connectCtx, cancel := context.WithTimeout(ctx, endpointConnectTimeout(cfg.Endpoint))
	defer cancel()

	clientConn, err := grpc.DialContext(connectCtx, cfg.Endpoint.Target, dialOptions...)
	if err != nil {
		return nil, classifyGRPCError(cfg.Endpoint, err)
	}

	if state := clientConn.GetState(); state != connectivity.Ready {
		_ = clientConn.Close()
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.grpc_not_ready",
			Category: contracts.ErrorCategoryTransport,
			Message:  "The endpoint accepted the network connection but gRPC channel readiness could not be proven.",
			NextStep: "Check for HTTP/2, TLS termination or proxy issues and confirm the target is a gRPC server rather than a generic HTTPS listener.",
			Details: map[string]string{
				"state": state.String(),
			},
		}
	}

	return clientConn, nil
}
