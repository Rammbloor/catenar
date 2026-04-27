package endpoint

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"

	"tether/internal/contracts"
)

type EventEmitter interface {
	Emit(eventName contracts.EventName, payload any) error
}

type WorkspaceContext struct {
	ID   string
	Kind string
}

type WorkspaceManager interface {
	PrepareEndpointTest(ctx context.Context, input contracts.EndpointTestInput) (WorkspaceContext, contracts.EndpointPreset, error)
}

type SecretUsageIntent string

const (
	SecretUsageTLSCA         SecretUsageIntent = "tls_ca"
	SecretUsageTLSClientCert SecretUsageIntent = "tls_client_cert"
	SecretUsageTLSClientKey  SecretUsageIntent = "tls_client_key"
)

type ResolvedMaterial struct {
	Ref       ParsedSecretRef
	Kind      string
	Path      string
	Bytes     []byte
	UpdatedAt string
}

type SecretStore interface {
	Resolve(ctx context.Context, scope WorkspaceContext, ref string, usage SecretUsageIntent) (ResolvedMaterial, error)
}

type EndpointRuntimeConfig struct {
	Endpoint      contracts.EndpointPreset
	CACertPEM     []byte
	ClientCertPEM []byte
	ClientKeyPEM  []byte
}

type endpointDiagnostic struct {
	Level    string
	Code     string
	Category contracts.ErrorCategory
	Message  string
	NextStep string
	Details  map[string]string
}

type EndpointProbeReport struct {
	TransportReachable bool
	TLSConfigured      bool
	TLSOK              bool
	GRPCReady          bool
	GRPCReadyProven    bool
	Checks             []contracts.EndpointCheck
	Diagnostic         *endpointDiagnostic
	Duration           time.Duration
}

type TransportAdapter interface {
	TestEndpoint(ctx context.Context, cfg EndpointRuntimeConfig) EndpointProbeReport
}

type GRPCClientConn interface {
	grpc.ClientConnInterface
	Close() error
}

type GRPCRuntime interface {
	Dial(ctx context.Context, cfg EndpointRuntimeConfig) (GRPCClientConn, *endpointDiagnostic)
}

type ReflectionCatalog struct {
	Services       []contracts.CatalogService
	WellKnownTypes []contracts.CatalogMessageRef
}

type ReflectionClient interface {
	LoadCatalog(ctx context.Context, conn GRPCClientConn, endpointPreset contracts.EndpointPreset) (ReflectionCatalog, *endpointDiagnostic)
}

type ServiceDependencies struct {
	AppDataDir       string
	WorkspaceManager WorkspaceManager
	SecretStore      SecretStore
	TransportAdapter TransportAdapter
	GRPCRuntime      GRPCRuntime
	ReflectionClient ReflectionClient
	Now              func() time.Time
}

type Service struct {
	workspaceManager WorkspaceManager
	secretStore      SecretStore
	transportAdapter TransportAdapter
	grpcRuntime      GRPCRuntime
	reflectionClient ReflectionClient
	emitter          EventEmitter
	now              func() time.Time
}

func NewService(deps ServiceDependencies) *Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}

	manager := deps.WorkspaceManager
	if manager == nil {
		manager = transientWorkspaceManager{}
	}

	store := deps.SecretStore
	if store == nil {
		store = NewFileSecretStore(newJSONMaterialIndex(materialIndexPath(deps.AppDataDir)))
	}

	adapter := deps.TransportAdapter
	if adapter == nil {
		adapter = newGRPCTransportAdapter(grpcTransportAdapterOptions{
			systemCertPool: x509.SystemCertPool,
		})
	}

	runtime := deps.GRPCRuntime
	if runtime == nil {
		runtime = newGRPCRuntime(grpcTransportAdapterOptions{
			systemCertPool: x509.SystemCertPool,
		})
	}

	reflectionClient := deps.ReflectionClient
	if reflectionClient == nil {
		reflectionClient = newServerReflectionClient()
	}

	return &Service{
		workspaceManager: manager,
		secretStore:      store,
		transportAdapter: adapter,
		grpcRuntime:      runtime,
		reflectionClient: reflectionClient,
		now:              now,
	}
}

func (s *Service) SetEmitter(emitter EventEmitter) {
	s.emitter = emitter
}

func (s *Service) EndpointTest(ctx context.Context, input contracts.EndpointTestInput) contracts.EndpointTestResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	issues := ValidateEndpointPreset(input.Endpoint)
	if len(issues) > 0 {
		first := issues[0]
		return contracts.EndpointTestResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     first.Code,
				Category: contracts.ErrorCategoryValidation,
				Message:  first.Message,
				Details: map[string]string{
					"field":      first.Field,
					"issueCount": fmt.Sprintf("%d", len(issues)),
				},
			},
		}
	}

	scope, endpointPreset, err := s.workspaceManager.PrepareEndpointTest(ctx, input)
	if err != nil {
		return contracts.EndpointTestResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.workspace_context_unavailable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The runtime could not prepare workspace context for the endpoint test.",
				Details: map[string]string{
					"cause": err.Error(),
				},
			},
		}
	}

	testedAt := s.now().UTC()
	runtimeCfg, prepDiagnostic := s.resolveRuntimeConfig(ctx, scope, endpointPreset)
	if prepDiagnostic != nil {
		event := s.emitDiagnostic("endpoint-test", testedAt, prepDiagnostic)
		result := contracts.EndpointTestResult{
			Endpoint:           endpointPreset,
			TransportReachable: false,
			TLSConfigured:      endpointPreset.TLS.Mode != contracts.TLSModePlaintext,
			TLSOK:              false,
			GRPCReady:          false,
			GRPCReadyProven:    false,
			Checks: []contracts.EndpointCheck{
				{
					Stage:   contracts.EndpointCheckStageTargetResolution,
					Outcome: contracts.EndpointCheckOutcomeNotProven,
					Message: "Target resolution was not attempted because endpoint preparation failed.",
				},
				{
					Stage:   contracts.EndpointCheckStageTCPConnect,
					Outcome: contracts.EndpointCheckOutcomeNotProven,
					Message: "TCP connect was not attempted because endpoint preparation failed.",
				},
				{
					Stage:   contracts.EndpointCheckStageTLSHandshake,
					Outcome: contracts.EndpointCheckOutcomeFailed,
					Message: prepDiagnostic.Message,
					Details: copyDetails(prepDiagnostic.Details),
				},
				{
					Stage:   contracts.EndpointCheckStageGRPCReadiness,
					Outcome: contracts.EndpointCheckOutcomeNotProven,
					Message: "gRPC readiness could not be proven because transport preparation failed.",
				},
			},
			Diagnostic: event,
			TestedAt:   testedAt.Format(time.RFC3339Nano),
			DurationMs: 0,
		}

		return contracts.EndpointTestResponse{
			Ok:   true,
			Data: &result,
		}
	}

	report := s.transportAdapter.TestEndpoint(ctx, runtimeCfg)
	event := s.emitDiagnostic("endpoint-test", testedAt, report.Diagnostic)
	result := contracts.EndpointTestResult{
		Endpoint:           endpointPreset,
		TransportReachable: report.TransportReachable,
		TLSConfigured:      report.TLSConfigured,
		TLSOK:              report.TLSOK,
		GRPCReady:          report.GRPCReady,
		GRPCReadyProven:    report.GRPCReadyProven,
		Checks:             report.Checks,
		Diagnostic:         event,
		TestedAt:           testedAt.Format(time.RFC3339Nano),
		DurationMs:         report.Duration.Milliseconds(),
	}

	return contracts.EndpointTestResponse{
		Ok:   true,
		Data: &result,
	}
}

func (s *Service) CatalogLoadFromReflection(ctx context.Context, input contracts.CatalogLoadFromReflectionInput) contracts.CatalogLoadFromReflectionResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	issues := ValidateEndpointPreset(input.Endpoint)
	if len(issues) > 0 {
		first := issues[0]
		return contracts.CatalogLoadFromReflectionResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     first.Code,
				Category: contracts.ErrorCategoryValidation,
				Message:  first.Message,
				Details: map[string]string{
					"field":      first.Field,
					"issueCount": fmt.Sprintf("%d", len(issues)),
				},
			},
		}
	}

	scope, endpointPreset, err := s.workspaceManager.PrepareEndpointTest(ctx, contracts.EndpointTestInput{Endpoint: input.Endpoint})
	if err != nil {
		return contracts.CatalogLoadFromReflectionResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.workspace_context_unavailable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The runtime could not prepare workspace context for the reflection load.",
				Details: map[string]string{
					"cause": err.Error(),
				},
			},
		}
	}

	startedAt := s.now().UTC()
	runtimeCfg, prepDiagnostic := s.resolveRuntimeConfig(ctx, scope, endpointPreset)
	if prepDiagnostic != nil {
		s.emitDiagnostic("catalog-reflection", startedAt, prepDiagnostic)
		return failureCatalogResponse(prepDiagnostic)
	}

	probeReport := s.transportAdapter.TestEndpoint(ctx, runtimeCfg)
	if !probeReport.GRPCReady || !probeReport.GRPCReadyProven {
		s.emitDiagnostic("catalog-reflection", startedAt, probeReport.Diagnostic)
		return failureCatalogResponse(probeReport.Diagnostic)
	}

	conn, runtimeDiagnostic := s.grpcRuntime.Dial(ctx, runtimeCfg)
	if runtimeDiagnostic != nil {
		s.emitDiagnostic("catalog-reflection", startedAt, runtimeDiagnostic)
		return failureCatalogResponse(runtimeDiagnostic)
	}
	defer func() {
		_ = conn.Close()
	}()

	catalog, reflectionDiagnostic := s.reflectionClient.LoadCatalog(ctx, conn, endpointPreset)
	event := s.emitDiagnostic("catalog-reflection", startedAt, reflectionDiagnostic)
	if reflectionDiagnostic != nil && reflectionDiagnostic.Level == "error" {
		return failureCatalogResponse(reflectionDiagnostic)
	}

	result := contracts.ReflectionCatalogResult{
		Endpoint:       endpointPreset,
		Services:       catalog.Services,
		WellKnownTypes: catalog.WellKnownTypes,
		Diagnostic:     event,
		LoadedAt:       startedAt.Format(time.RFC3339Nano),
		DurationMs:     time.Since(startedAt).Milliseconds(),
	}

	return contracts.CatalogLoadFromReflectionResponse{
		Ok:   true,
		Data: &result,
	}
}

func (s *Service) resolveRuntimeConfig(ctx context.Context, scope WorkspaceContext, endpointPreset contracts.EndpointPreset) (EndpointRuntimeConfig, *endpointDiagnostic) {
	cfg := EndpointRuntimeConfig{
		Endpoint: endpointPreset,
	}

	if endpointPreset.TLS.Mode == contracts.TLSModePlaintext {
		return cfg, nil
	}

	if endpointPreset.TLS.CACert != "" {
		material, err := s.secretStore.Resolve(ctx, scope, endpointPreset.TLS.CACert, SecretUsageTLSCA)
		if err != nil {
			return EndpointRuntimeConfig{}, classifyPreparationError(err, endpointPreset, "caCert")
		}
		cfg.CACertPEM = material.Bytes
	}

	if endpointPreset.TLS.ClientCert != "" {
		material, err := s.secretStore.Resolve(ctx, scope, endpointPreset.TLS.ClientCert, SecretUsageTLSClientCert)
		if err != nil {
			return EndpointRuntimeConfig{}, classifyPreparationError(err, endpointPreset, "clientCert")
		}
		cfg.ClientCertPEM = material.Bytes
	}

	if endpointPreset.TLS.ClientKey != "" {
		material, err := s.secretStore.Resolve(ctx, scope, endpointPreset.TLS.ClientKey, SecretUsageTLSClientKey)
		if err != nil {
			return EndpointRuntimeConfig{}, classifyPreparationError(err, endpointPreset, "clientKey")
		}
		cfg.ClientKeyPEM = material.Bytes
	}

	return cfg, nil
}

func (s *Service) emitDiagnostic(source string, ts time.Time, diagnostic *endpointDiagnostic) *contracts.DiagnosticsUpdateEvent {
	if diagnostic == nil {
		return nil
	}

	event := &contracts.DiagnosticsUpdateEvent{
		ID:        fmt.Sprintf("diag_%d", ts.UnixNano()),
		Source:    source,
		Level:     diagnostic.Level,
		Code:      diagnostic.Code,
		Category:  diagnostic.Category,
		Message:   diagnostic.Message,
		NextStep:  diagnostic.NextStep,
		Details:   copyDetails(diagnostic.Details),
		Timestamp: ts.Format(time.RFC3339Nano),
	}

	if s.emitter != nil {
		_ = s.emitter.Emit(contracts.EventDiagnosticsUpdate, *event)
	}

	return event
}

func materialIndexPath(appDataDir string) string {
	base := appDataDir
	if base == "" {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			base = filepath.Join(os.TempDir(), "tether")
		} else {
			base = filepath.Join(userConfigDir, "tether")
		}
	}

	return filepath.Join(base, "materials", "index.json")
}

func classifyPreparationError(err error, endpointPreset contracts.EndpointPreset, field string) *endpointDiagnostic {
	if classified, ok := err.(*classifiedError); ok {
		details := copyDetails(classified.Envelope.Details)
		details["field"] = field
		details["tlsMode"] = string(endpointPreset.TLS.Mode)

		return &endpointDiagnostic{
			Level:    "error",
			Code:     classified.Envelope.Code,
			Category: classified.Envelope.Category,
			Message:  classified.Envelope.Message,
			NextStep: classified.NextStep,
			Details:  details,
		}
	}

	return &endpointDiagnostic{
		Level:    "error",
		Code:     "application.endpoint_preparation_failed",
		Category: contracts.ErrorCategoryApplication,
		Message:  "The runtime could not prepare TLS material for the endpoint test.",
		NextStep: "Retry after fixing the registered certificate material or re-opening the workspace.",
		Details: map[string]string{
			"cause":   err.Error(),
			"field":   field,
			"tlsMode": string(endpointPreset.TLS.Mode),
		},
	}
}

func defaultTLSServerName(endpointPreset contracts.EndpointPreset) string {
	if endpointPreset.TLS.ServerNameOverride != "" {
		return endpointPreset.TLS.ServerNameOverride
	}

	host, _, err := splitHostPort(endpointPreset.Target)
	if err != nil {
		return ""
	}

	return host
}

func buildTLSConfig(endpointPreset contracts.EndpointPreset, systemPool *x509.CertPool, caCertPEM, clientCertPEM, clientKeyPEM []byte) (*tls.Config, *classifiedError) {
	serverName := defaultTLSServerName(endpointPreset)
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2"},
		ServerName: serverName,
	}

	switch endpointPreset.TLS.Mode {
	case contracts.TLSModeSystemCA:
		config.RootCAs = cloneCertPool(systemPool)
	case contracts.TLSModeCustomCA:
		rootPool := x509.NewCertPool()
		if ok := rootPool.AppendCertsFromPEM(caCertPEM); !ok {
			return nil, &classifiedError{
				Envelope: contracts.ErrorEnvelope{
					Code:     "validation.endpoint_tls_ca_invalid",
					Category: contracts.ErrorCategoryValidation,
					Message:  "The configured CA certificate could not be parsed as PEM.",
				},
				NextStep: "Re-register the CA file and make sure it contains a valid PEM certificate bundle.",
			}
		}
		config.RootCAs = rootPool
	case contracts.TLSModeMTLS:
		rootPool := cloneCertPool(systemPool)
		if len(caCertPEM) > 0 {
			rootPool = x509.NewCertPool()
			if ok := rootPool.AppendCertsFromPEM(caCertPEM); !ok {
				return nil, &classifiedError{
					Envelope: contracts.ErrorEnvelope{
						Code:     "validation.endpoint_tls_ca_invalid",
						Category: contracts.ErrorCategoryValidation,
						Message:  "The configured mTLS CA certificate could not be parsed as PEM.",
					},
					NextStep: "Re-register the CA file and make sure the endpoint points to the expected root bundle.",
				}
			}
		}
		config.RootCAs = rootPool

		certificate, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
		if err != nil {
			return nil, &classifiedError{
				Envelope: contracts.ErrorEnvelope{
					Code:     "validation.endpoint_tls_client_material_invalid",
					Category: contracts.ErrorCategoryValidation,
					Message:  "The configured client certificate and key could not be loaded as a matching key pair.",
					Details: map[string]string{
						"cause": err.Error(),
					},
				},
				NextStep: "Re-register the client certificate and private key, then test the endpoint again.",
			}
		}
		config.Certificates = []tls.Certificate{certificate}
	default:
		return nil, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "validation.endpoint_tls_mode_invalid",
				Category: contracts.ErrorCategoryValidation,
				Message:  "The endpoint uses an unknown TLS mode.",
				Details: map[string]string{
					"mode": string(endpointPreset.TLS.Mode),
				},
			},
			NextStep: "Choose one of plaintext, system_ca, custom_ca or mtls.",
		}
	}

	return config, nil
}

func cloneCertPool(pool *x509.CertPool) *x509.CertPool {
	if pool == nil {
		return nil
	}

	return pool.Clone()
}

func copyDetails(details map[string]string) map[string]string {
	if len(details) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(details))
	for key, value := range details {
		cloned[key] = value
	}

	return cloned
}

func failureCatalogResponse(diagnostic *endpointDiagnostic) contracts.CatalogLoadFromReflectionResponse {
	if diagnostic == nil {
		return contracts.CatalogLoadFromReflectionResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.reflection_load_failed",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The reflection catalog could not be loaded.",
			},
		}
	}

	return contracts.CatalogLoadFromReflectionResponse{
		Ok: false,
		Error: &contracts.ErrorEnvelope{
			Code:     diagnostic.Code,
			Category: diagnostic.Category,
			Message:  diagnostic.Message,
			Details:  copyDetails(diagnostic.Details),
		},
	}
}

type transientWorkspaceManager struct{}

func (transientWorkspaceManager) PrepareEndpointTest(_ context.Context, input contracts.EndpointTestInput) (WorkspaceContext, contracts.EndpointPreset, error) {
	return WorkspaceContext{
		ID:   "transient",
		Kind: "editor-session",
	}, normalizeEndpointPreset(input.Endpoint), nil
}
