package endpoint

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"

	"tether/internal/contracts"
)

type ReflectionUnaryTemplateInput struct {
	Endpoint contracts.EndpointPreset `json:"endpoint"`
	Method   contracts.CatalogMethod  `json:"method"`
}

type ReflectionUnaryTemplateResult struct {
	Endpoint    contracts.EndpointPreset `json:"endpoint"`
	Method      contracts.CatalogMethod  `json:"method"`
	Body        any                      `json:"body"`
	GeneratedAt string                   `json:"generatedAt"`
	DurationMs  int64                    `json:"durationMs"`
}

type ReflectionUnaryTemplateResponse struct {
	Ok    bool                           `json:"ok"`
	Data  *ReflectionUnaryTemplateResult `json:"data,omitempty"`
	Error *contracts.ErrorEnvelope       `json:"error,omitempty"`
}

type ReflectionUnaryInvokeInput struct {
	Endpoint    contracts.EndpointPreset `json:"endpoint"`
	Method      contracts.CatalogMethod  `json:"method"`
	Metadata    map[string]string        `json:"metadata,omitempty"`
	Body        any                      `json:"body"`
	CallOptions contracts.CallOptions    `json:"callOptions,omitempty"`
}

type ReflectionUnaryInvokeResult struct {
	Endpoint     contracts.EndpointPreset `json:"endpoint"`
	Method       contracts.CatalogMethod  `json:"method"`
	RequestBody  any                      `json:"requestBody"`
	ResponseBody any                      `json:"responseBody"`
	Headers      map[string][]string      `json:"headers,omitempty"`
	Trailers     map[string][]string      `json:"trailers,omitempty"`
	Status       contracts.StreamStatus   `json:"status"`
	StartedAt    string                   `json:"startedAt"`
	FinishedAt   string                   `json:"finishedAt"`
	DurationMs   int64                    `json:"durationMs"`
}

type ReflectionUnaryInvokeResponse struct {
	Ok    bool                         `json:"ok"`
	Data  *ReflectionUnaryInvokeResult `json:"data,omitempty"`
	Error *contracts.ErrorEnvelope     `json:"error,omitempty"`
}

type unaryReflectionSelection struct {
	Endpoint         contracts.EndpointPreset
	Connection       GRPCClientConn
	MethodDescriptor protoreflect.MethodDescriptor
}

func (s *Service) BuildUnaryRequestTemplateFromReflection(ctx context.Context, input ReflectionUnaryTemplateInput) ReflectionUnaryTemplateResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	startedAt := s.now().UTC()
	selection, errorEnvelope := s.resolveUnaryReflectionSelection(ctx, input.Endpoint, input.Method, startedAt, "unary-template")
	if errorEnvelope != nil {
		return ReflectionUnaryTemplateResponse{
			Ok:    false,
			Error: errorEnvelope,
		}
	}
	defer func() {
		_ = selection.Connection.Close()
	}()

	return ReflectionUnaryTemplateResponse{
		Ok: true,
		Data: &ReflectionUnaryTemplateResult{
			Endpoint:    selection.Endpoint,
			Method:      input.Method,
			Body:        buildStarterJSONValue(selection.MethodDescriptor.Input()),
			GeneratedAt: startedAt.Format(time.RFC3339Nano),
			DurationMs:  time.Since(startedAt).Milliseconds(),
		},
	}
}

func (s *Service) InvokeUnaryFromReflection(ctx context.Context, input ReflectionUnaryInvokeInput) ReflectionUnaryInvokeResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	if input.Method.RPCType != contracts.RPCTypeUnary {
		return ReflectionUnaryInvokeResponse{
			Ok: false,
			Error: &contracts.ErrorEnvelope{
				Code:     "validation.method_rpc_type_invalid",
				Category: contracts.ErrorCategoryValidation,
				Message:  "The selected catalog method is not unary and cannot be executed in the unary flow.",
				Details: map[string]string{
					"method":  input.Method.FullName,
					"rpcType": string(input.Method.RPCType),
				},
			},
		}
	}

	startedAt := s.now().UTC()
	selection, errorEnvelope := s.resolveUnaryReflectionSelection(ctx, input.Endpoint, input.Method, startedAt, "unary-invoke")
	if errorEnvelope != nil {
		return ReflectionUnaryInvokeResponse{
			Ok:    false,
			Error: errorEnvelope,
		}
	}
	defer func() {
		_ = selection.Connection.Close()
	}()

	invokeResult, invokeDiagnostic := s.grpcRuntime.InvokeUnary(ctx, selection.Connection, UnaryInvokeRequest{
		Method:         input.Method,
		Descriptor:     selection.MethodDescriptor,
		Metadata:       mergeInvokeMetadata(selection.Endpoint.MetadataDefaults, input.Metadata),
		Body:           input.Body,
		RequestTimeout: resolveUnaryRequestTimeout(selection.Endpoint, input.CallOptions),
	})
	if invokeDiagnostic != nil {
		s.emitDiagnostic("unary-invoke", startedAt, invokeDiagnostic)
		return ReflectionUnaryInvokeResponse{
			Ok:    false,
			Error: errorEnvelopeFromDiagnostic(invokeDiagnostic, "application.unary_invoke_failed", "The unary call could not be completed."),
		}
	}

	finishedAt := startedAt.Add(invokeResult.Duration)
	return ReflectionUnaryInvokeResponse{
		Ok: true,
		Data: &ReflectionUnaryInvokeResult{
			Endpoint:     selection.Endpoint,
			Method:       input.Method,
			RequestBody:  input.Body,
			ResponseBody: invokeResult.ResponseBody,
			Headers:      invokeResult.Headers,
			Trailers:     invokeResult.Trailers,
			Status:       invokeResult.Status,
			StartedAt:    startedAt.Format(time.RFC3339Nano),
			FinishedAt:   finishedAt.Format(time.RFC3339Nano),
			DurationMs:   invokeResult.Duration.Milliseconds(),
		},
	}
}

func (s *Service) resolveUnaryReflectionSelection(
	ctx context.Context,
	endpointPreset contracts.EndpointPreset,
	selectedMethod contracts.CatalogMethod,
	startedAt time.Time,
	diagnosticSource string,
) (*unaryReflectionSelection, *contracts.ErrorEnvelope) {
	issues := ValidateEndpointPreset(endpointPreset)
	if len(issues) > 0 {
		first := issues[0]
		return nil, &contracts.ErrorEnvelope{
			Code:     first.Code,
			Category: contracts.ErrorCategoryValidation,
			Message:  first.Message,
			Details: map[string]string{
				"field":      first.Field,
				"issueCount": fmt.Sprintf("%d", len(issues)),
			},
		}
	}

	scope, normalizedEndpoint, err := s.workspaceManager.PrepareEndpointTest(ctx, contracts.EndpointTestInput{Endpoint: endpointPreset})
	if err != nil {
		return nil, &contracts.ErrorEnvelope{
			Code:     "application.workspace_context_unavailable",
			Category: contracts.ErrorCategoryApplication,
			Message:  "The runtime could not prepare workspace context for the reflection-selected unary flow.",
			Details: map[string]string{
				"cause": err.Error(),
			},
		}
	}

	runtimeCfg, prepDiagnostic := s.resolveRuntimeConfig(ctx, scope, normalizedEndpoint)
	if prepDiagnostic != nil {
		s.emitDiagnostic(diagnosticSource, startedAt, prepDiagnostic)
		return nil, errorEnvelopeFromDiagnostic(prepDiagnostic, "application.endpoint_preparation_failed", "The endpoint could not be prepared.")
	}

	conn, runtimeDiagnostic := s.grpcRuntime.Dial(ctx, runtimeCfg)
	if runtimeDiagnostic != nil {
		s.emitDiagnostic(diagnosticSource, startedAt, runtimeDiagnostic)
		return nil, errorEnvelopeFromDiagnostic(runtimeDiagnostic, "transport.grpc_not_ready", "The endpoint could not establish a ready gRPC channel.")
	}

	catalog, reflectionDiagnostic := s.reflectionClient.LoadCatalog(ctx, conn, normalizedEndpoint)
	if reflectionDiagnostic != nil && reflectionDiagnostic.Level == "error" {
		s.emitDiagnostic(diagnosticSource, startedAt, reflectionDiagnostic)
		_ = conn.Close()
		return nil, errorEnvelopeFromDiagnostic(reflectionDiagnostic, "application.reflection_load_failed", "The reflection catalog could not be loaded.")
	}

	methodDescriptor, found, matches := catalog.resolveMethodSelection(selectedMethod)
	if !found {
		_ = conn.Close()
		return nil, &contracts.ErrorEnvelope{
			Code:     "reflection.method_not_found",
			Category: contracts.ErrorCategoryReflection,
			Message:  "The selected method is no longer present in the reflection catalog for this endpoint.",
			Details: map[string]string{
				"method": selectedMethod.FullName,
				"target": normalizedEndpoint.Target,
			},
		}
	}

	if !matches {
		_ = conn.Close()
		return nil, &contracts.ErrorEnvelope{
			Code:     "reflection.method_signature_mismatch",
			Category: contracts.ErrorCategoryReflection,
			Message:  "The selected method no longer matches the reflection catalog contract for this endpoint.",
			Details: map[string]string{
				"method": selectedMethod.FullName,
				"target": normalizedEndpoint.Target,
			},
		}
	}

	return &unaryReflectionSelection{
		Endpoint:         normalizedEndpoint,
		Connection:       conn,
		MethodDescriptor: methodDescriptor,
	}, nil
}

func buildStarterJSONValue(messageDescriptor protoreflect.MessageDescriptor) any {
	return buildStarterMessageValue(messageDescriptor, map[protoreflect.FullName]struct{}{})
}

func buildStarterMessageValue(messageDescriptor protoreflect.MessageDescriptor, visited map[protoreflect.FullName]struct{}) any {
	if messageDescriptor == nil {
		return map[string]any{}
	}

	if value, ok := buildWellKnownStarterValue(messageDescriptor); ok {
		return value
	}

	if _, seen := visited[messageDescriptor.FullName()]; seen {
		return map[string]any{}
	}
	visited[messageDescriptor.FullName()] = struct{}{}
	defer delete(visited, messageDescriptor.FullName())

	body := make(map[string]any, messageDescriptor.Fields().Len())
	fields := messageDescriptor.Fields()
	for index := 0; index < fields.Len(); index++ {
		fieldDescriptor := fields.Get(index)
		if oneof := fieldDescriptor.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() {
			continue
		}

		body[fieldDescriptor.JSONName()] = buildStarterFieldValue(fieldDescriptor, visited)
	}

	return body
}

func buildStarterFieldValue(fieldDescriptor protoreflect.FieldDescriptor, visited map[protoreflect.FullName]struct{}) any {
	if fieldDescriptor.IsList() {
		return []any{}
	}

	if fieldDescriptor.IsMap() {
		return map[string]any{}
	}

	switch fieldDescriptor.Kind() {
	case protoreflect.BoolKind:
		return false
	case protoreflect.StringKind, protoreflect.BytesKind:
		return ""
	case protoreflect.EnumKind:
		if fieldDescriptor.Enum().Values().Len() == 0 {
			return ""
		}
		return string(fieldDescriptor.Enum().Values().Get(0).Name())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind:
		return float64(0)
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return buildStarterMessageValue(fieldDescriptor.Message(), visited)
	default:
		return nil
	}
}

func buildWellKnownStarterValue(messageDescriptor protoreflect.MessageDescriptor) (any, bool) {
	switch string(messageDescriptor.FullName()) {
	case "google.protobuf.Timestamp", "google.protobuf.Duration", "google.protobuf.FieldMask":
		return "", true
	case "google.protobuf.Struct", "google.protobuf.Empty":
		return map[string]any{}, true
	case "google.protobuf.ListValue":
		return []any{}, true
	case "google.protobuf.Value":
		return nil, true
	case "google.protobuf.BoolValue":
		return false, true
	case "google.protobuf.StringValue", "google.protobuf.BytesValue":
		return "", true
	case "google.protobuf.DoubleValue", "google.protobuf.FloatValue",
		"google.protobuf.Int32Value", "google.protobuf.Int64Value",
		"google.protobuf.UInt32Value", "google.protobuf.UInt64Value":
		return float64(0), true
	case "google.protobuf.Any":
		return map[string]any{
			"@type": "",
			"value": map[string]any{},
		}, true
	default:
		return nil, false
	}
}

func mergeInvokeMetadata(defaults, overrides map[string]string) map[string]string {
	if len(defaults) == 0 && len(overrides) == 0 {
		return nil
	}

	merged := make(map[string]string, len(defaults)+len(overrides))
	for key, value := range defaults {
		merged[strings.ToLower(strings.TrimSpace(key))] = value
	}
	for key, value := range overrides {
		merged[strings.ToLower(strings.TrimSpace(key))] = value
	}

	return merged
}

func resolveUnaryRequestTimeout(endpointPreset contracts.EndpointPreset, callOptions contracts.CallOptions) time.Duration {
	timeoutMs := endpointPreset.RequestTimeoutMs
	if callOptions.RequestTimeoutMs > 0 {
		timeoutMs = callOptions.RequestTimeoutMs
	}

	if timeoutMs <= 0 {
		return 0
	}

	return time.Duration(timeoutMs) * time.Millisecond
}

func errorEnvelopeFromDiagnostic(diagnostic *endpointDiagnostic, fallbackCode, fallbackMessage string) *contracts.ErrorEnvelope {
	if diagnostic == nil {
		return &contracts.ErrorEnvelope{
			Code:     fallbackCode,
			Category: contracts.ErrorCategoryApplication,
			Message:  fallbackMessage,
		}
	}

	return &contracts.ErrorEnvelope{
		Code:     diagnostic.Code,
		Category: diagnostic.Category,
		Message:  diagnostic.Message,
		Details:  copyDetails(diagnostic.Details),
	}
}
