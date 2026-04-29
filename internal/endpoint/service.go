package endpoint

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"

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
	InvokeUnary(ctx context.Context, conn GRPCClientConn, request UnaryInvokeRequest) (UnaryInvokeResult, *endpointDiagnostic)
	StartServerStream(ctx context.Context, conn GRPCClientConn, request ServerStreamStartRequest) (ServerStreamStartResult, *endpointDiagnostic)
	ConsumeServerStream(request ServerStreamConsumeRequest) (ServerStreamConsumeResult, *endpointDiagnostic)
}

type MethodCatalog struct {
	Services       []contracts.CatalogService
	WellKnownTypes []contracts.CatalogMessageRef
	methods        map[string]protoreflect.MethodDescriptor
}

type ReflectionCatalog = MethodCatalog

type ReflectionClient interface {
	LoadCatalog(ctx context.Context, conn GRPCClientConn, endpointPreset contracts.EndpointPreset) (MethodCatalog, *endpointDiagnostic)
}

type ProtoLoader interface {
	LoadCatalog(ctx context.Context, input ProtoLoaderInput) (MethodCatalog, *endpointDiagnostic)
}

type cachedMethodCatalog struct {
	source           contracts.CatalogSourceKind
	scope            WorkspaceContext
	endpoint         contracts.EndpointPreset
	catalog          MethodCatalog
	requestTemplates map[string]any
}

type ServiceDependencies struct {
	AppDataDir       string
	WorkspaceManager WorkspaceManager
	SecretStore      SecretStore
	TransportAdapter TransportAdapter
	GRPCRuntime      GRPCRuntime
	ReflectionClient ReflectionClient
	ProtoLoader      ProtoLoader
	HistoryStore     HistoryStore
	EventLog         EventLog
	Now              func() time.Time
}

type Service struct {
	workspaceManager  WorkspaceManager
	secretStore       SecretStore
	transportAdapter  TransportAdapter
	grpcRuntime       GRPCRuntime
	reflectionClient  ReflectionClient
	protoLoader       ProtoLoader
	historyStore      HistoryStore
	eventLog          EventLog
	emitter           EventEmitter
	catalogCacheMu    sync.RWMutex
	catalogCache      map[string]cachedMethodCatalog
	streamSessionsMu  sync.Mutex
	streamSessions    map[string]*activeStreamSession
	initializationErr error
	now               func() time.Time
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

	protoLoader := deps.ProtoLoader
	if protoLoader == nil {
		protoLoader = newProtoLoader()
	}

	baseDir := appDataBaseDir(deps.AppDataDir)
	historyStore := deps.HistoryStore
	var initializationErr error
	if historyStore == nil {
		store, err := newSQLiteHistoryStore(baseDir)
		if err != nil {
			initializationErr = err
		} else {
			historyStore = store
		}
	}

	eventLog := deps.EventLog
	if eventLog == nil {
		eventLog = newFileEventLog(baseDir)
	}

	return &Service{
		workspaceManager:  manager,
		secretStore:       store,
		transportAdapter:  adapter,
		grpcRuntime:       runtime,
		reflectionClient:  reflectionClient,
		protoLoader:       protoLoader,
		historyStore:      historyStore,
		eventLog:          eventLog,
		catalogCache:      make(map[string]cachedMethodCatalog),
		streamSessions:    make(map[string]*activeStreamSession),
		initializationErr: initializationErr,
		now:               now,
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

	requestTemplates := buildRequestTemplates(catalog)
	s.storeMethodCatalog(endpointPreset.ID, contracts.CatalogSourceReflection, cachedMethodCatalog{
		source:           contracts.CatalogSourceReflection,
		scope:            scope,
		endpoint:         endpointPreset,
		catalog:          catalog,
		requestTemplates: requestTemplates,
	})

	result := contracts.ReflectionCatalogResult{
		Endpoint:         endpointPreset,
		Services:         catalog.Services,
		WellKnownTypes:   catalog.WellKnownTypes,
		RequestTemplates: requestTemplates,
		Diagnostic:       event,
		LoadedAt:         startedAt.Format(time.RFC3339Nano),
		DurationMs:       time.Since(startedAt).Milliseconds(),
	}

	return contracts.CatalogLoadFromReflectionResponse{
		Ok:   true,
		Data: &result,
	}
}

func (s *Service) CatalogLoadFromProtoSources(ctx context.Context, input contracts.CatalogLoadFromProtoSourcesInput) contracts.CatalogLoadFromProtoSourcesResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	issues := ValidateEndpointPreset(input.Endpoint)
	if len(issues) > 0 {
		first := issues[0]
		return contracts.CatalogLoadFromProtoSourcesResponse{
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

	if len(input.ProtoSources) == 0 {
		return contracts.CatalogLoadFromProtoSourcesResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.proto_sources_required",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Add at least one proto file or directory before loading the proto catalog.",
			},
		}
	}

	scope, endpointPreset, err := s.workspaceManager.PrepareEndpointTest(ctx, contracts.EndpointTestInput{Endpoint: input.Endpoint})
	if err != nil {
		return contracts.CatalogLoadFromProtoSourcesResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.workspace_context_unavailable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The runtime could not prepare workspace context for the proto catalog load.",
				Details: map[string]string{
					"cause": err.Error(),
				},
			},
		}
	}

	startedAt := s.now().UTC()
	catalog, protoDiagnostic := s.protoLoader.LoadCatalog(ctx, ProtoLoaderInput{
		ProtoSources: input.ProtoSources,
		ImportPaths:  input.ImportPaths,
	})
	event := s.emitDiagnostic("catalog-proto", startedAt, protoDiagnostic)
	if protoDiagnostic != nil && protoDiagnostic.Level == "error" {
		return contracts.CatalogLoadFromProtoSourcesResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     protoDiagnostic.Code,
				Category: protoDiagnostic.Category,
				Message:  protoDiagnostic.Message,
				Details:  copyDetails(protoDiagnostic.Details),
			},
		}
	}

	requestTemplates := buildRequestTemplates(catalog)
	s.storeMethodCatalog(endpointPreset.ID, contracts.CatalogSourceProto, cachedMethodCatalog{
		source:           contracts.CatalogSourceProto,
		scope:            scope,
		endpoint:         endpointPreset,
		catalog:          catalog,
		requestTemplates: requestTemplates,
	})

	return contracts.CatalogLoadFromProtoSourcesResponse{
		Ok: true,
		Data: &contracts.ProtoCatalogResult{
			Endpoint:         endpointPreset,
			ProtoSources:     append([]contracts.ProtoSource(nil), input.ProtoSources...),
			ImportPaths:      append([]string(nil), input.ImportPaths...),
			Services:         catalog.Services,
			WellKnownTypes:   catalog.WellKnownTypes,
			RequestTemplates: requestTemplates,
			Diagnostic:       event,
			LoadedAt:         startedAt.Format(time.RFC3339Nano),
			DurationMs:       time.Since(startedAt).Milliseconds(),
		},
	}
}

func (s *Service) CallInvokeUnary(ctx context.Context, input contracts.CallInvokeUnaryInput) contracts.CallInvokeUnaryResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.initializationErr != nil || s.historyStore == nil || s.eventLog == nil {
		return contracts.CallInvokeUnaryResponse{
			Ok:    false,
			Error: wrapHistoryStoreError("initialize", s.initializationErr),
		}
	}

	if strings.TrimSpace(input.EndpointID) == "" {
		return contracts.CallInvokeUnaryResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.endpoint_id_required",
				Category: contracts.ErrorCategoryValidation,
				Message:  "A loaded endpoint id is required before invoking a unary method.",
			},
		}
	}

	if strings.TrimSpace(input.Method) == "" {
		return contracts.CallInvokeUnaryResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.method_required",
				Category: contracts.ErrorCategoryValidation,
				Message:  "A method full name is required before invoking a unary call.",
			},
		}
	}

	catalogSource := normalizeCatalogSource(input.CatalogSource)
	cachedCatalog, ok := s.loadMethodCatalog(input.EndpointID, catalogSource)
	if !ok {
		return contracts.CallInvokeUnaryResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     catalogCacheStateCode("catalog_not_loaded"),
				Category: contracts.ErrorCategoryApplication,
				Message:  fmt.Sprintf("Load the %s catalog for this endpoint again before invoking a unary method.", catalogSourceLabel(catalogSource)),
				Details: map[string]string{
					"endpointId":    input.EndpointID,
					"catalogSource": string(catalogSource),
				},
			},
		}
	}

	methodDescriptor, found := cachedCatalog.catalog.methods[input.Method]
	if !found {
		return contracts.CallInvokeUnaryResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     catalogCacheStateCode("method_not_found"),
				Category: contracts.ErrorCategoryApplication,
				Message:  fmt.Sprintf("The selected method is no longer present in the cached %s catalog.", catalogSourceLabel(catalogSource)),
				Details: map[string]string{
					"endpointId":    input.EndpointID,
					"method":        input.Method,
					"catalogSource": string(catalogSource),
				},
			},
		}
	}

	selectedMethod := buildCatalogMethod(methodDescriptor)
	if selectedMethod.RPCType != contracts.RPCTypeUnary {
		return contracts.CallInvokeUnaryResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.method_rpc_type_invalid",
				Category: contracts.ErrorCategoryValidation,
				Message:  fmt.Sprintf("The selected %s method is not unary and cannot be executed in the unary flow.", catalogSourceLabel(catalogSource)),
				Details: map[string]string{
					"method":  selectedMethod.FullName,
					"rpcType": string(selectedMethod.RPCType),
				},
			},
		}
	}

	runtimeCfg, prepDiagnostic := s.resolveRuntimeConfig(ctx, cachedCatalog.scope, cachedCatalog.endpoint)
	if prepDiagnostic != nil {
		s.emitDiagnostic("unary-invoke", s.now().UTC(), prepDiagnostic)
		return contracts.CallInvokeUnaryResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(prepDiagnostic, "application.endpoint_preparation_failed", "The endpoint could not be prepared."),
		}
	}

	startedAt := s.now().UTC()
	conn, runtimeDiagnostic := s.grpcRuntime.Dial(ctx, runtimeCfg)
	if runtimeDiagnostic != nil {
		s.emitDiagnostic("unary-invoke", startedAt, runtimeDiagnostic)
		return contracts.CallInvokeUnaryResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(runtimeDiagnostic, "transport.grpc_not_ready", "The endpoint could not establish a ready gRPC channel."),
		}
	}
	defer func() {
		_ = conn.Close()
	}()

	mergedMetadata := mergeInvokeMetadata(cachedCatalog.endpoint.MetadataDefaults, input.Metadata)
	callID, sessionID := newCallIdentity(startedAt)
	invokeResult, invokeDiagnostic := s.grpcRuntime.InvokeUnary(ctx, conn, UnaryInvokeRequest{
		Method:         selectedMethod,
		Descriptor:     methodDescriptor,
		Metadata:       mergedMetadata,
		Body:           input.Body,
		RequestTimeout: resolveUnaryRequestTimeout(cachedCatalog.endpoint, input.CallOptions),
	})

	if invokeDiagnostic != nil && invokeDiagnostic.Category == contracts.ErrorCategoryValidation {
		s.emitDiagnostic("unary-invoke", startedAt, invokeDiagnostic)
		return contracts.CallInvokeUnaryResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(invokeDiagnostic, "validation.request_body_invalid", "The unary request body is invalid."),
		}
	}

	finishedAt := startedAt.Add(invokeResult.Duration)
	diagnosticEvent := s.emitDiagnostic("unary-invoke", startedAt, invokeDiagnostic)
	finalState := contracts.StreamStateClosed
	if invokeDiagnostic != nil {
		finalState = contracts.StreamStateError
	}

	artifacts, artifactErr := s.eventLog.WriteUnaryCall(ctx, UnaryEventLogRecord{
		CallID:          callID,
		SessionID:       sessionID,
		EndpointID:      cachedCatalog.endpoint.ID,
		WorkspaceID:     cachedCatalog.scope.ID,
		Method:          selectedMethod.FullName,
		RPCType:         contracts.RPCTypeUnary,
		FinalState:      finalState,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		Duration:        invokeResult.Duration,
		RequestBody:     input.Body,
		ResponseBody:    invokeResult.ResponseBody,
		RequestMetadata: mergedMetadata,
		Headers:         invokeResult.Headers,
		Trailers:        invokeResult.Trailers,
		Status:          invokeResult.Status,
		ErrorCategory:   invokeDiagnosticCategory(invokeDiagnostic),
		ErrorCode:       invokeDiagnosticCode(invokeDiagnostic),
	})
	if artifactErr != nil {
		return contracts.CallInvokeUnaryResponse{
			Ok:    false,
			Error: wrapHistoryStoreError("write_artifacts", artifactErr),
		}
	}

	summary := contracts.HistoryCallSummary{
		CallID:         callID,
		SessionID:      sessionID,
		WorkspaceID:    cachedCatalog.scope.ID,
		Method:         selectedMethod.FullName,
		RPCType:        contracts.RPCTypeUnary,
		EndpointID:     cachedCatalog.endpoint.ID,
		State:          finalState,
		GRPCStatusCode: invokeResult.Status.Code,
		StartedAt:      startedAt.Format(time.RFC3339Nano),
		FinishedAt:     finishedAt.Format(time.RFC3339Nano),
		DurationMs:     invokeResult.Duration.Milliseconds(),
		RequestCount:   1,
		ResponseCount:  boolToCount(invokeResult.ResponseBody != nil),
		Truncated:      false,
		ErrorCategory:  invokeDiagnosticCategory(invokeDiagnostic),
		ErrorCode:      invokeDiagnosticCode(invokeDiagnostic),
		SummaryPath:    artifacts.SummaryPath,
		SessionLogPath: artifacts.SessionLogPath,
	}

	if err := s.historyStore.SaveCallSummary(ctx, summary); err != nil {
		return contracts.CallInvokeUnaryResponse{
			Ok:    false,
			Error: wrapHistoryStoreError("save", err),
		}
	}

	if invokeDiagnostic != nil && invokeDiagnostic.Category != contracts.ErrorCategoryGRPCStatus {
		errorEnvelope := errorEnvelopeFromDiagnostic(invokeDiagnostic, "application.unary_invoke_failed", "The unary call could not be completed.")
		if errorEnvelope.Details == nil {
			errorEnvelope.Details = map[string]string{}
		}
		errorEnvelope.Details["callId"] = callID
		return contracts.CallInvokeUnaryResponse{
			Ok:    false,
			Error: errorEnvelope,
		}
	}

	return contracts.CallInvokeUnaryResponse{
		Ok: true,
		Data: &contracts.CallInvokeUnaryResult{
			CallID:       callID,
			SessionID:    sessionID,
			EndpointID:   cachedCatalog.endpoint.ID,
			Method:       selectedMethod.FullName,
			RPCType:      contracts.RPCTypeUnary,
			FinalState:   finalState,
			RequestBody:  input.Body,
			ResponseBody: invokeResult.ResponseBody,
			Headers:      invokeResult.Headers,
			Trailers:     invokeResult.Trailers,
			Status:       invokeResult.Status,
			Diagnostic:   diagnosticEvent,
			StartedAt:    startedAt.Format(time.RFC3339Nano),
			FinishedAt:   finishedAt.Format(time.RFC3339Nano),
			DurationMs:   invokeResult.Duration.Milliseconds(),
		},
	}
}

func (s *Service) HistoryList(ctx context.Context, input contracts.HistoryListInput) contracts.HistoryListResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.initializationErr != nil || s.historyStore == nil {
		return contracts.HistoryListResponse{
			Ok:    false,
			Error: wrapHistoryStoreError("initialize", s.initializationErr),
		}
	}

	calls, err := s.historyStore.ListCalls(ctx, input)
	if err != nil {
		return contracts.HistoryListResponse{
			Ok:    false,
			Error: wrapHistoryStoreError("list", err),
		}
	}

	return contracts.HistoryListResponse{
		Ok: true,
		Data: &contracts.HistoryListResult{
			Calls: calls,
		},
	}
}

func (s *Service) HistoryGet(ctx context.Context, callID string) contracts.HistoryGetResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	if s.initializationErr != nil || s.historyStore == nil {
		return contracts.HistoryGetResponse{
			Ok:    false,
			Error: wrapHistoryStoreError("initialize", s.initializationErr),
		}
	}

	if strings.TrimSpace(callID) == "" {
		return contracts.HistoryGetResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.call_id_required",
				Category: contracts.ErrorCategoryValidation,
				Message:  "A call id is required to load a persisted unary history entry.",
			},
		}
	}

	summary, err := s.historyStore.GetCallSummary(ctx, callID)
	if err != nil {
		return contracts.HistoryGetResponse{
			Ok:    false,
			Error: wrapHistoryStoreError("get", err),
		}
	}

	detail, err := readStoredUnaryHistoryDetail(summary.SummaryPath)
	if err != nil {
		return contracts.HistoryGetResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "application.history_read_failed",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The runtime could not read the persisted unary history artifact.",
				Details: map[string]string{
					"callId": callID,
					"cause":  err.Error(),
				},
			},
		}
	}

	return contracts.HistoryGetResponse{
		Ok: true,
		Data: &contracts.HistoryGetResult{
			Summary:      summary,
			RequestBody:  detail.RequestBody,
			ResponseBody: detail.ResponseBody,
			Headers:      detail.Headers,
			Trailers:     detail.Trailers,
			Status:       detail.Status,
			Events:       detail.Events,
		},
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

func (s *Service) storeMethodCatalog(endpointID string, source contracts.CatalogSourceKind, snapshot cachedMethodCatalog) {
	if endpointID == "" {
		return
	}

	s.catalogCacheMu.Lock()
	defer s.catalogCacheMu.Unlock()
	s.catalogCache[catalogCacheKey(endpointID, source)] = snapshot
}

func (s *Service) loadMethodCatalog(endpointID string, source contracts.CatalogSourceKind) (cachedMethodCatalog, bool) {
	s.catalogCacheMu.RLock()
	defer s.catalogCacheMu.RUnlock()
	snapshot, ok := s.catalogCache[catalogCacheKey(endpointID, source)]
	return snapshot, ok
}

func appDataBaseDir(appDataDir string) string {
	if strings.TrimSpace(appDataDir) != "" {
		return appDataDir
	}

	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "tether")
	}

	return filepath.Join(userConfigDir, "tether")
}

func materialIndexPath(appDataDir string) string {
	base := appDataBaseDir(appDataDir)

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
	}, ensureEndpointIdentity(normalizeEndpointPreset(input.Endpoint)), nil
}

func ensureEndpointIdentity(endpointPreset contracts.EndpointPreset) contracts.EndpointPreset {
	normalized := endpointPreset
	if strings.TrimSpace(normalized.Name) == "" {
		normalized.Name = normalized.Target
	}
	if strings.TrimSpace(normalized.ID) != "" {
		return normalized
	}

	fingerprintSource := normalized
	fingerprintSource.ID = ""
	fingerprintSource.Name = ""
	payload, err := json.Marshal(fingerprintSource)
	if err != nil {
		normalized.ID = fmt.Sprintf("ep_%d", time.Now().UnixNano())
		return normalized
	}

	sum := sha256.Sum256(payload)
	normalized.ID = "ep_" + hex.EncodeToString(sum[:6])
	return normalized
}

func EnsureEndpointIdentity(endpointPreset contracts.EndpointPreset) contracts.EndpointPreset {
	return ensureEndpointIdentity(endpointPreset)
}

func buildRequestTemplates(catalog MethodCatalog) map[string]any {
	if len(catalog.methods) == 0 {
		return nil
	}

	templates := make(map[string]any, len(catalog.methods))
	for methodFullName, descriptor := range catalog.methods {
		templates[methodFullName] = buildStarterJSONValue(descriptor.Input())
	}

	return templates
}

func catalogCacheKey(endpointID string, source contracts.CatalogSourceKind) string {
	return endpointID + "::" + string(normalizeCatalogSource(source))
}

func normalizeCatalogSource(source contracts.CatalogSourceKind) contracts.CatalogSourceKind {
	if source == contracts.CatalogSourceProto {
		return source
	}

	return contracts.CatalogSourceReflection
}

func catalogCacheStateCode(suffix string) string {
	return "application." + suffix
}

func catalogSourceLabel(source contracts.CatalogSourceKind) string {
	if normalizeCatalogSource(source) == contracts.CatalogSourceProto {
		return "proto-loaded"
	}

	return "reflection-loaded"
}

func newCallIdentity(startedAt time.Time) (string, string) {
	nanos := startedAt.UnixNano()
	return fmt.Sprintf("call_%d", nanos), fmt.Sprintf("sess_%d", nanos)
}

func invokeDiagnosticCategory(diagnostic *endpointDiagnostic) contracts.ErrorCategory {
	if diagnostic == nil {
		return ""
	}
	return diagnostic.Category
}

func invokeDiagnosticCode(diagnostic *endpointDiagnostic) string {
	if diagnostic == nil {
		return ""
	}
	return diagnostic.Code
}

func boolToCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func readStoredUnaryHistoryDetail(path string) (storedUnaryHistoryDetail, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return storedUnaryHistoryDetail{}, err
	}

	var detail storedUnaryHistoryDetail
	if err := json.Unmarshal(payload, &detail); err != nil {
		return storedUnaryHistoryDetail{}, err
	}

	sanitized := detail
	sanitized.Headers = redactMetadataValues(detail.Headers)
	sanitized.Trailers = redactMetadataValues(detail.Trailers)
	if !metadataValuesEqual(detail.Headers, sanitized.Headers) || !metadataValuesEqual(detail.Trailers, sanitized.Trailers) {
		if err := writeJSONFile(path, sanitized); err != nil {
			return storedUnaryHistoryDetail{}, err
		}
	}

	return sanitized, nil
}
