package endpoint

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	grpccredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"tether/internal/contracts"
)

type grpcTransportAdapterOptions struct {
	systemCertPool func() (*x509.CertPool, error)
}

type grpcTransportAdapter struct {
	systemCertPool func() (*x509.CertPool, error)
}

func newGRPCTransportAdapter(options grpcTransportAdapterOptions) TransportAdapter {
	systemCertPool := options.systemCertPool
	if systemCertPool == nil {
		systemCertPool = x509.SystemCertPool
	}

	return &grpcTransportAdapter{
		systemCertPool: systemCertPool,
	}
}

func (a *grpcTransportAdapter) TestEndpoint(ctx context.Context, cfg EndpointRuntimeConfig) EndpointProbeReport {
	startedAt := time.Now()
	tlsConfigured := cfg.Endpoint.TLS.Mode != contracts.TLSModePlaintext
	checks := []contracts.EndpointCheck{
		{Stage: contracts.EndpointCheckStageTargetResolution, Outcome: contracts.EndpointCheckOutcomeNotProven, Message: "Target resolution has not run yet."},
		{Stage: contracts.EndpointCheckStageTCPConnect, Outcome: contracts.EndpointCheckOutcomeNotProven, Message: "TCP connect has not run yet."},
		{Stage: contracts.EndpointCheckStageTLSHandshake, Outcome: contracts.EndpointCheckOutcomeSkipped, Message: "TLS is disabled for plaintext endpoints."},
		{Stage: contracts.EndpointCheckStageGRPCReadiness, Outcome: contracts.EndpointCheckOutcomeNotProven, Message: "gRPC readiness has not been proven yet."},
	}

	if tlsConfigured {
		checks[2] = contracts.EndpointCheck{
			Stage:   contracts.EndpointCheckStageTLSHandshake,
			Outcome: contracts.EndpointCheckOutcomeNotProven,
			Message: "TLS handshake has not run yet.",
		}
	}

	connectCtx, cancel := context.WithTimeout(ctx, endpointConnectTimeout(cfg.Endpoint))
	defer cancel()

	host, _, err := splitHostPort(cfg.Endpoint.Target)
	if err != nil {
		diagnostic := &endpointDiagnostic{
			Level:    "error",
			Code:     "validation.endpoint_target_invalid",
			Category: contracts.ErrorCategoryValidation,
			Message:  "Endpoint target must use the host:port format before transport preflight can start.",
			NextStep: "Fix the endpoint target and retry the connection test.",
			Details: map[string]string{
				"target": cfg.Endpoint.Target,
				"cause":  err.Error(),
			},
		}

		checks[0] = contracts.EndpointCheck{
			Stage:   contracts.EndpointCheckStageTargetResolution,
			Outcome: contracts.EndpointCheckOutcomeFailed,
			Message: diagnostic.Message,
			Details: copyDetails(diagnostic.Details),
		}

		return EndpointProbeReport{
			TLSConfigured: tlsConfigured,
			Checks:        checks,
			Diagnostic:    diagnostic,
			Duration:      time.Since(startedAt),
		}
	}

	resolvedAddrs, err := resolveTarget(connectCtx, host)
	if err != nil {
		diagnostic := classifyResolutionError(cfg.Endpoint, err)
		checks[0] = contracts.EndpointCheck{
			Stage:   contracts.EndpointCheckStageTargetResolution,
			Outcome: contracts.EndpointCheckOutcomeFailed,
			Message: diagnostic.Message,
			Details: copyDetails(diagnostic.Details),
		}

		return EndpointProbeReport{
			TLSConfigured: tlsConfigured,
			Checks:        checks,
			Diagnostic:    diagnostic,
			Duration:      time.Since(startedAt),
		}
	}

	checks[0] = contracts.EndpointCheck{
		Stage:   contracts.EndpointCheckStageTargetResolution,
		Outcome: contracts.EndpointCheckOutcomePassed,
		Message: "Endpoint target resolved successfully.",
		Details: map[string]string{
			"host":          host,
			"resolvedCount": fmt.Sprintf("%d", len(resolvedAddrs)),
			"resolvedFirst": resolvedAddrs[0],
		},
	}

	dialer := &net.Dialer{}
	rawConn, err := dialer.DialContext(connectCtx, "tcp", cfg.Endpoint.Target)
	if err != nil {
		diagnostic := classifyTCPError(cfg.Endpoint, err)
		checks[1] = contracts.EndpointCheck{
			Stage:   contracts.EndpointCheckStageTCPConnect,
			Outcome: contracts.EndpointCheckOutcomeFailed,
			Message: diagnostic.Message,
			Details: copyDetails(diagnostic.Details),
		}

		return EndpointProbeReport{
			TLSConfigured: tlsConfigured,
			Checks:        checks,
			Diagnostic:    diagnostic,
			Duration:      time.Since(startedAt),
		}
	}

	checks[1] = contracts.EndpointCheck{
		Stage:   contracts.EndpointCheckStageTCPConnect,
		Outcome: contracts.EndpointCheckOutcomePassed,
		Message: "TCP connection to the endpoint target succeeded.",
		Details: map[string]string{
			"target": cfg.Endpoint.Target,
		},
	}

	systemPool, tlsConfig, tlsErr := a.buildClientTLSConfig(cfg)
	if tlsErr != nil {
		_ = rawConn.Close()
		checks[2] = contracts.EndpointCheck{
			Stage:   contracts.EndpointCheckStageTLSHandshake,
			Outcome: contracts.EndpointCheckOutcomeFailed,
			Message: tlsErr.Envelope.Message,
			Details: copyDetails(tlsErr.Envelope.Details),
		}

		return EndpointProbeReport{
			TLSConfigured: tlsConfigured,
			Checks:        checks,
			Diagnostic: &endpointDiagnostic{
				Level:    "error",
				Code:     tlsErr.Envelope.Code,
				Category: tlsErr.Envelope.Category,
				Message:  tlsErr.Envelope.Message,
				NextStep: tlsErr.NextStep,
				Details:  copyDetails(tlsErr.Envelope.Details),
			},
			Duration: time.Since(startedAt),
		}
	}

	tlsOK := !tlsConfigured
	if tlsConfigured {
		tlsConn := tls.Client(rawConn, tlsConfig.Clone())
		if err := tlsConn.HandshakeContext(connectCtx); err != nil {
			_ = rawConn.Close()
			diagnostic := classifyTLSError(cfg.Endpoint, err)
			checks[2] = contracts.EndpointCheck{
				Stage:   contracts.EndpointCheckStageTLSHandshake,
				Outcome: contracts.EndpointCheckOutcomeFailed,
				Message: diagnostic.Message,
				Details: copyDetails(diagnostic.Details),
			}

			return EndpointProbeReport{
				TransportReachable: true,
				TLSConfigured:      true,
				TLSOK:              false,
				Checks:             checks,
				Diagnostic:         diagnostic,
				Duration:           time.Since(startedAt),
			}
		}

		state := tlsConn.ConnectionState()
		if state.NegotiatedProtocol != "h2" {
			_ = tlsConn.Close()
			diagnostic := &endpointDiagnostic{
				Level:    "error",
				Code:     "transport.tls_alpn_missing",
				Category: contracts.ErrorCategoryTransport,
				Message:  "TLS handshake succeeded but the server did not negotiate HTTP/2 for gRPC.",
				NextStep: "Check whether the endpoint actually exposes gRPC over HTTP/2 and that intermediate proxies preserve ALPN negotiation.",
				Details: map[string]string{
					"negotiatedProtocol": state.NegotiatedProtocol,
					"serverName":         tlsConfig.ServerName,
				},
			}
			checks[2] = contracts.EndpointCheck{
				Stage:   contracts.EndpointCheckStageTLSHandshake,
				Outcome: contracts.EndpointCheckOutcomeFailed,
				Message: diagnostic.Message,
				Details: copyDetails(diagnostic.Details),
			}

			return EndpointProbeReport{
				TransportReachable: true,
				TLSConfigured:      true,
				TLSOK:              false,
				Checks:             checks,
				Diagnostic:         diagnostic,
				Duration:           time.Since(startedAt),
			}
		}

		tlsOK = true
		checks[2] = contracts.EndpointCheck{
			Stage:   contracts.EndpointCheckStageTLSHandshake,
			Outcome: contracts.EndpointCheckOutcomePassed,
			Message: "TLS handshake and HTTP/2 ALPN negotiation succeeded.",
			Details: map[string]string{
				"serverName":         tlsConfig.ServerName,
				"negotiatedProtocol": state.NegotiatedProtocol,
				"cipherSuite":        tls.CipherSuiteName(state.CipherSuite),
			},
		}
		_ = tlsConn.Close()
	} else {
		_ = rawConn.Close()
	}

	dialOptions := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if cfg.Endpoint.Authority != "" {
		dialOptions = append(dialOptions, grpc.WithAuthority(cfg.Endpoint.Authority))
	}

	if tlsConfigured {
		creds := newSplitAuthorityCredentials(grpccredentials.NewTLS(buildDialTLSConfig(tlsConfig, systemPool)), tlsConfig.ServerName)
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(creds))
	} else {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	clientConn, err := grpc.DialContext(connectCtx, cfg.Endpoint.Target, dialOptions...)
	if err != nil {
		diagnostic := classifyGRPCError(cfg.Endpoint, err)
		checks[3] = contracts.EndpointCheck{
			Stage:   contracts.EndpointCheckStageGRPCReadiness,
			Outcome: contracts.EndpointCheckOutcomeFailed,
			Message: diagnostic.Message,
			Details: copyDetails(diagnostic.Details),
		}

		return EndpointProbeReport{
			TransportReachable: true,
			TLSConfigured:      tlsConfigured,
			TLSOK:              tlsOK,
			GRPCReady:          false,
			GRPCReadyProven:    false,
			Checks:             checks,
			Diagnostic:         diagnostic,
			Duration:           time.Since(startedAt),
		}
	}
	defer func() {
		_ = clientConn.Close()
	}()

	state := clientConn.GetState()
	if state != connectivity.Ready {
		diagnostic := &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.grpc_not_ready",
			Category: contracts.ErrorCategoryTransport,
			Message:  "The gRPC client connection did not reach the READY state within the configured connect timeout.",
			NextStep: "Increase the connect timeout or verify that the endpoint accepts HTTP/2 gRPC connections without an intermediate protocol mismatch.",
			Details: map[string]string{
				"state": state.String(),
			},
		}
		checks[3] = contracts.EndpointCheck{
			Stage:   contracts.EndpointCheckStageGRPCReadiness,
			Outcome: contracts.EndpointCheckOutcomeFailed,
			Message: diagnostic.Message,
			Details: copyDetails(diagnostic.Details),
		}

		return EndpointProbeReport{
			TransportReachable: true,
			TLSConfigured:      tlsConfigured,
			TLSOK:              tlsOK,
			GRPCReady:          false,
			GRPCReadyProven:    false,
			Checks:             checks,
			Diagnostic:         diagnostic,
			Duration:           time.Since(startedAt),
		}
	}

	checks[3] = contracts.EndpointCheck{
		Stage:   contracts.EndpointCheckStageGRPCReadiness,
		Outcome: contracts.EndpointCheckOutcomePassed,
		Message: "The gRPC channel reached READY without executing a user RPC.",
		Details: map[string]string{
			"state": state.String(),
		},
	}

	return EndpointProbeReport{
		TransportReachable: true,
		TLSConfigured:      tlsConfigured,
		TLSOK:              tlsOK,
		GRPCReady:          true,
		GRPCReadyProven:    true,
		Checks:             checks,
		Diagnostic: &endpointDiagnostic{
			Level:    "info",
			Code:     "transport.endpoint_ready",
			Category: contracts.ErrorCategoryTransport,
			Message:  "Endpoint preflight passed target resolution, TCP connect, TLS and gRPC readiness checks.",
			NextStep: "Proceed with reflection or a direct method invocation on this endpoint.",
			Details: map[string]string{
				"target":    cfg.Endpoint.Target,
				"tlsMode":   string(cfg.Endpoint.TLS.Mode),
				"authority": cfg.Endpoint.Authority,
			},
		},
		Duration: time.Since(startedAt),
	}
}

func (a *grpcTransportAdapter) buildClientTLSConfig(cfg EndpointRuntimeConfig) (*x509.CertPool, *tls.Config, *classifiedError) {
	if cfg.Endpoint.TLS.Mode == contracts.TLSModePlaintext {
		return nil, nil, nil
	}

	systemPool, err := a.systemCertPool()
	if err != nil {
		return nil, nil, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "application.system_ca_unavailable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The runtime could not load the operating system certificate pool.",
				Details: map[string]string{
					"cause": err.Error(),
				},
			},
			NextStep: "Retry on a machine with a readable system certificate store or switch the endpoint to a custom CA bundle.",
		}
	}

	tlsConfig, classifiedErr := buildTLSConfig(cfg.Endpoint, systemPool, cfg.CACertPEM, cfg.ClientCertPEM, cfg.ClientKeyPEM)
	return systemPool, tlsConfig, classifiedErr
}

func resolveTarget(ctx context.Context, host string) ([]string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []string{ip.String()}, nil
	}

	results, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	addrs := make([]string, 0, len(results))
	for _, result := range results {
		addrs = append(addrs, result.IP.String())
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no IP addresses resolved for %s", host)
	}

	return addrs, nil
}

func classifyResolutionError(endpointPreset contracts.EndpointPreset, err error) *endpointDiagnostic {
	details := map[string]string{
		"target": endpointPreset.Target,
		"cause":  err.Error(),
	}

	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr):
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.target_resolution_failed",
			Category: contracts.ErrorCategoryTransport,
			Message:  "The endpoint host could not be resolved.",
			NextStep: "Check the hostname, local DNS configuration and whether the endpoint should use a different target value.",
			Details:  details,
		}
	case errors.Is(err, context.DeadlineExceeded):
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.target_resolution_timeout",
			Category: contracts.ErrorCategoryTransport,
			Message:  "Target resolution exceeded the configured connect timeout.",
			NextStep: "Increase the connect timeout or check whether local DNS resolution is blocked.",
			Details:  details,
		}
	default:
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.target_resolution_failed",
			Category: contracts.ErrorCategoryTransport,
			Message:  "The endpoint host could not be resolved.",
			NextStep: "Check the target hostname and retry the endpoint test.",
			Details:  details,
		}
	}
}

func classifyTCPError(endpointPreset contracts.EndpointPreset, err error) *endpointDiagnostic {
	details := map[string]string{
		"target": endpointPreset.Target,
		"cause":  err.Error(),
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.connect_timeout",
			Category: contracts.ErrorCategoryTransport,
			Message:  "TCP connect did not complete within the configured connect timeout.",
			NextStep: "Increase the connect timeout or confirm the endpoint is reachable on the expected port.",
			Details:  details,
		}
	case errors.Is(err, syscall.ECONNREFUSED):
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.connection_refused",
			Category: contracts.ErrorCategoryTransport,
			Message:  "The remote endpoint refused the TCP connection.",
			NextStep: "Check whether the service is listening on the expected port and that local firewall rules allow the connection.",
			Details:  details,
		}
	default:
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.tcp_connect_failed",
			Category: contracts.ErrorCategoryTransport,
			Message:  "TCP connect to the endpoint failed.",
			NextStep: "Check network reachability, endpoint target and firewall or proxy configuration.",
			Details:  details,
		}
	}
}

func classifyTLSError(endpointPreset contracts.EndpointPreset, err error) *endpointDiagnostic {
	details := map[string]string{
		"target":             endpointPreset.Target,
		"cause":              err.Error(),
		"tlsMode":            string(endpointPreset.TLS.Mode),
		"serverNameOverride": endpointPreset.TLS.ServerNameOverride,
	}

	var hostnameErr x509.HostnameError
	var unknownAuthorityErr x509.UnknownAuthorityError

	switch {
	case errors.As(err, &hostnameErr):
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.tls_hostname_mismatch",
			Category: contracts.ErrorCategoryTransport,
			Message:  "TLS handshake failed because the server certificate does not match the configured host name.",
			NextStep: "Check serverNameOverride, the endpoint target host and the SAN entries on the server certificate.",
			Details:  details,
		}
	case errors.As(err, &unknownAuthorityErr):
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.tls_unknown_authority",
			Category: contracts.ErrorCategoryTransport,
			Message:  "TLS handshake failed because the server certificate chain is not trusted.",
			NextStep: "Check whether the endpoint should use system CA or a custom CA bundle and verify that the correct CA file is registered.",
			Details:  details,
		}
	case errors.Is(err, context.DeadlineExceeded):
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.tls_handshake_timeout",
			Category: contracts.ErrorCategoryTransport,
			Message:  "TLS handshake did not complete within the configured connect timeout.",
			NextStep: "Increase the connect timeout or inspect whether a proxy or load balancer is stalling the TLS handshake.",
			Details:  details,
		}
	case strings.Contains(strings.ToLower(err.Error()), "certificate required"):
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.tls_client_certificate_required",
			Category: contracts.ErrorCategoryTransport,
			Message:  "TLS handshake failed because the server requires a client certificate.",
			NextStep: "Switch the endpoint to mTLS mode and register a client certificate and private key for it.",
			Details:  details,
		}
	default:
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.tls_handshake_failed",
			Category: contracts.ErrorCategoryTransport,
			Message:  "TLS handshake failed.",
			NextStep: "Check CA trust, serverNameOverride, client certificate material and whether the endpoint is actually speaking TLS.",
			Details:  details,
		}
	}
}

func classifyGRPCError(endpointPreset contracts.EndpointPreset, err error) *endpointDiagnostic {
	details := map[string]string{
		"target":    endpointPreset.Target,
		"cause":     err.Error(),
		"authority": endpointPreset.Authority,
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.grpc_connect_timeout",
			Category: contracts.ErrorCategoryTransport,
			Message:  "The gRPC channel did not reach READY before the connect timeout elapsed.",
			NextStep: "Verify the service accepts gRPC over HTTP/2 and increase the connect timeout if the network path is slow.",
			Details:  details,
		}
	default:
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "transport.grpc_not_ready",
			Category: contracts.ErrorCategoryTransport,
			Message:  "The endpoint accepted the network connection but gRPC channel readiness could not be proven.",
			NextStep: "Check for HTTP/2, TLS termination or proxy issues and confirm the target is a gRPC server rather than a generic HTTPS listener.",
			Details:  details,
		}
	}
}

func buildDialTLSConfig(source *tls.Config, systemPool *x509.CertPool) *tls.Config {
	cloned := source.Clone()
	if cloned.RootCAs == nil {
		cloned.RootCAs = cloneCertPool(systemPool)
	}

	return cloned
}

func endpointConnectTimeout(endpointPreset contracts.EndpointPreset) time.Duration {
	return time.Duration(endpointPreset.ConnectTimeoutMs) * time.Millisecond
}

type splitAuthorityCredentials struct {
	base                grpccredentials.TransportCredentials
	handshakeServerName string
}

func newSplitAuthorityCredentials(base grpccredentials.TransportCredentials, handshakeServerName string) grpccredentials.TransportCredentials {
	if handshakeServerName == "" {
		return base
	}

	return &splitAuthorityCredentials{
		base:                base,
		handshakeServerName: handshakeServerName,
	}
}

func (c *splitAuthorityCredentials) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, grpccredentials.AuthInfo, error) {
	handshakeAuthority := authority
	if c.handshakeServerName != "" {
		handshakeAuthority = c.handshakeServerName
	}

	conn, authInfo, err := c.base.ClientHandshake(ctx, handshakeAuthority, rawConn)
	if err != nil {
		return nil, nil, err
	}

	if tlsInfo, ok := authInfo.(grpccredentials.TLSInfo); ok {
		return conn, tlsAuthorityPassthroughInfo{TLSInfo: tlsInfo}, nil
	}

	return conn, authInfo, nil
}

func (c *splitAuthorityCredentials) ServerHandshake(rawConn net.Conn) (net.Conn, grpccredentials.AuthInfo, error) {
	return c.base.ServerHandshake(rawConn)
}

func (c *splitAuthorityCredentials) Info() grpccredentials.ProtocolInfo {
	info := c.base.Info()
	info.ServerName = ""
	return info
}

func (c *splitAuthorityCredentials) Clone() grpccredentials.TransportCredentials {
	return &splitAuthorityCredentials{
		base:                c.base.Clone(),
		handshakeServerName: c.handshakeServerName,
	}
}

func (c *splitAuthorityCredentials) OverrideServerName(serverNameOverride string) error {
	c.handshakeServerName = serverNameOverride
	return nil
}

type tlsAuthorityPassthroughInfo struct {
	grpccredentials.TLSInfo
}

func (tlsAuthorityPassthroughInfo) ValidateAuthority(string) error {
	return nil
}
