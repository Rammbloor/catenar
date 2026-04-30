package endpoint

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"

	"tether/internal/contracts"
)

func TestBuildUnaryRequestTemplateFromReflectionUsesSelectedCatalogMethod(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{})
	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	selectedMethod := findCatalogMethod(t, catalogResponse.Data.Services, "tether.demo.v1.ReflectionDemo.Ping")
	response := service.BuildUnaryRequestTemplateFromReflection(context.Background(), ReflectionUnaryTemplateInput{
		Endpoint: catalogResponse.Data.Endpoint,
		Method:   selectedMethod,
	})

	if !response.Ok || response.Data == nil {
		t.Fatalf("expected unary request template, got %+v", response.Error)
	}

	body, ok := response.Data.Body.(string)
	if !ok {
		t.Fatalf("expected timestamp request template to be a string, got %T", response.Data.Body)
	}

	if body != "" {
		t.Fatalf("expected empty timestamp template, got %q", body)
	}
}

func TestInvokeUnaryFromReflectionUsesSelectedCatalogMethod(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{})
	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	selectedMethod := findCatalogMethod(t, catalogResponse.Data.Services, "tether.demo.v1.ReflectionDemo.Ping")
	response := service.InvokeUnaryFromReflection(context.Background(), ReflectionUnaryInvokeInput{
		Endpoint: catalogResponse.Data.Endpoint,
		Method:   selectedMethod,
		Body:     "2026-04-27T10:15:32Z",
	})

	if !response.Ok || response.Data == nil {
		t.Fatalf("expected unary invoke response, got %+v", response.Error)
	}

	if response.Data.Status.Code != codes.OK.String() {
		t.Fatalf("expected OK status, got %+v", response.Data.Status)
	}

	body, ok := response.Data.ResponseBody.(map[string]any)
	if !ok {
		t.Fatalf("expected struct response body, got %T", response.Data.ResponseBody)
	}

	if body["echoSeconds"] != float64(1777284932) {
		t.Fatalf("expected echoed timestamp seconds, got %+v", body)
	}

	if response.Data.Headers["x-reflection-demo"][0] != "ping" {
		t.Fatalf("expected demo header, got %+v", response.Data.Headers)
	}

	if response.Data.Trailers["x-reflection-demo-trailer"][0] != "ok" {
		t.Fatalf("expected demo trailer, got %+v", response.Data.Trailers)
	}
}

func TestCatalogLoadFromReflectionProvidesEndpointIdentityAndRequestTemplates(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})

	response := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !response.Ok || response.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", response.Error)
	}

	if response.Data.Endpoint.ID == "" {
		t.Fatalf("expected transient endpoint identity to be assigned")
	}

	template, ok := response.Data.RequestTemplates["tether.demo.v1.ReflectionDemo.Ping"]
	if !ok {
		t.Fatalf("expected unary request template to be present")
	}

	templateValue, ok := template.(string)
	if !ok || templateValue != "" {
		t.Fatalf("expected timestamp request template to be an empty string, got %#v", template)
	}
}

func TestCallInvokeUnaryUsesCachedReflectionCatalogAndPersistsHistory(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	invokeResponse := service.CallInvokeUnary(context.Background(), contracts.CallInvokeUnaryInput{
		EndpointID: catalogResponse.Data.Endpoint.ID,
		Method:     "tether.demo.v1.ReflectionDemo.Ping",
		Metadata: map[string]string{
			"authorization": "Bearer very-secret",
			"x-auth-token":  "reflection-auth-token-secret",
			"x-request-id":  "req-123",
		},
		Body: "2026-04-27T10:15:32Z",
	})

	if !invokeResponse.Ok || invokeResponse.Data == nil {
		t.Fatalf("expected unary invoke response, got %+v", invokeResponse.Error)
	}

	if invokeResponse.Data.FinalState != contracts.StreamStateClosed {
		t.Fatalf("expected closed final state, got %+v", invokeResponse.Data)
	}

	if invokeResponse.Data.Status.Code != codes.OK.String() {
		t.Fatalf("expected OK status, got %+v", invokeResponse.Data.Status)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil {
		t.Fatalf("expected history list, got %+v", historyList.Error)
	}

	if len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected one history summary, got %+v", historyList.Data.Calls)
	}

	summary := historyList.Data.Calls[0]
	if summary.CallID != invokeResponse.Data.CallID || summary.GRPCStatusCode != codes.OK.String() {
		t.Fatalf("unexpected call summary: %+v", summary)
	}

	historyGet := service.HistoryGet(context.Background(), invokeResponse.Data.CallID)
	if !historyGet.Ok || historyGet.Data == nil {
		t.Fatalf("expected history detail, got %+v", historyGet.Error)
	}

	eventKinds := make([]string, 0, len(historyGet.Data.Events))
	for _, event := range historyGet.Data.Events {
		eventKinds = append(eventKinds, event.Kind)
	}

	expectedKinds := []string{
		"call_started",
		"message_sent",
		"headers_received",
		"message_received",
		"trailers_received",
		"call_finished",
	}
	for _, expectedKind := range expectedKinds {
		if !slices.Contains(eventKinds, expectedKind) {
			t.Fatalf("expected history event %q, got %+v", expectedKind, eventKinds)
		}
	}

	startedEvent := historyGet.Data.Events[0]
	if startedEvent.GRPC == nil || startedEvent.GRPC.Metadata == nil {
		t.Fatalf("expected request metadata in stored event, got %+v", startedEvent)
	}
	if startedEvent.GRPC.Metadata["authorization"][0] != "[REDACTED]" {
		t.Fatalf("expected authorization metadata to be redacted, got %+v", startedEvent.GRPC.Metadata)
	}
	if startedEvent.GRPC.Metadata["x-auth-token"][0] != "[REDACTED]" {
		t.Fatalf("expected token-like metadata to be redacted, got %+v", startedEvent.GRPC.Metadata)
	}
	if startedEvent.GRPC.Metadata["x-request-id"][0] != "req-123" {
		t.Fatalf("expected non-secret metadata to be preserved, got %+v", startedEvent.GRPC.Metadata)
	}
	if historyGet.Data.Headers["set-cookie"][0] != "[REDACTED]" {
		t.Fatalf("expected persisted headers to redact secrets, got %+v", historyGet.Data.Headers)
	}
	if historyGet.Data.Trailers["set-cookie"][0] != "[REDACTED]" {
		t.Fatalf("expected persisted trailers to redact secrets, got %+v", historyGet.Data.Trailers)
	}

	summaryPayload, err := os.ReadFile(summary.SummaryPath)
	if err != nil {
		t.Fatalf("read summary payload: %v", err)
	}
	if strings.Contains(string(summaryPayload), "reflection-secret") ||
		strings.Contains(string(summaryPayload), "reflection-auth-token-secret") ||
		strings.Contains(string(summaryPayload), "reflection-token-secret") ||
		strings.Contains(string(summaryPayload), "reflection-refresh-token-secret") {
		t.Fatalf("expected persisted summary artifact to omit raw secret metadata, got %s", summaryPayload)
	}
}

func TestCallInvokeUnaryReturnsCompletedResultForGRPCStatusFailureAndPersistsHistory(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	invokeResponse := service.CallInvokeUnary(context.Background(), contracts.CallInvokeUnaryInput{
		EndpointID: catalogResponse.Data.Endpoint.ID,
		Method:     "tether.demo.v1.ReflectionDemo.Ping",
		Body:       "1969-12-31T23:59:59Z",
	})

	if !invokeResponse.Ok || invokeResponse.Data == nil {
		t.Fatalf("expected grpc status result, got %+v", invokeResponse.Error)
	}

	if invokeResponse.Data.FinalState != contracts.StreamStateError {
		t.Fatalf("expected error final state, got %+v", invokeResponse.Data)
	}

	if invokeResponse.Data.Status.Code != codes.InvalidArgument.String() {
		t.Fatalf("expected INVALID_ARGUMENT status, got %+v", invokeResponse.Data.Status)
	}

	if invokeResponse.Data.Diagnostic == nil || invokeResponse.Data.Diagnostic.Code != "grpc_status.invalid_argument" {
		t.Fatalf("expected grpc status diagnostic, got %+v", invokeResponse.Data.Diagnostic)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected one history summary, got %+v", historyList)
	}

	summary := historyList.Data.Calls[0]
	if summary.ErrorCategory != contracts.ErrorCategoryGRPCStatus || summary.ErrorCode != "grpc_status.invalid_argument" {
		t.Fatalf("expected grpc_status summary classification, got %+v", summary)
	}
}

func TestCallInvokeUnaryRejectsInvalidBodyBeforeSendAndSkipsHistoryPersistence(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})

	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	invokeResponse := service.CallInvokeUnary(context.Background(), contracts.CallInvokeUnaryInput{
		EndpointID: catalogResponse.Data.Endpoint.ID,
		Method:     "tether.demo.v1.ReflectionDemo.Ping",
		Body:       map[string]any{"seconds": "oops"},
	})

	if invokeResponse.Ok {
		t.Fatalf("expected invalid unary body to fail")
	}

	if invokeResponse.Error == nil || invokeResponse.Error.Code != "validation.request_body_invalid" {
		t.Fatalf("expected validation.request_body_invalid error, got %+v", invokeResponse.Error)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil {
		t.Fatalf("expected history list after validation failure, got %+v", historyList.Error)
	}
	if len(historyList.Data.Calls) != 0 {
		t.Fatalf("expected no persisted history rows for pre-dispatch validation failure, got %+v", historyList.Data.Calls)
	}
}

func TestInvokeUnaryFromReflectionRejectsNonUnaryCatalogMethod(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{})
	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	selectedMethod := findCatalogMethod(t, catalogResponse.Data.Services, "tether.demo.v1.ReflectionDemo.Watch")
	response := service.InvokeUnaryFromReflection(context.Background(), ReflectionUnaryInvokeInput{
		Endpoint: catalogResponse.Data.Endpoint,
		Method:   selectedMethod,
		Body:     map[string]any{},
	})

	if response.Ok {
		t.Fatalf("expected unary invoke to reject a server-streaming method")
	}

	if response.Error == nil || response.Error.Code != "validation.method_rpc_type_invalid" {
		t.Fatalf("expected validation.method_rpc_type_invalid error, got %+v", response.Error)
	}
}

func TestInvokeUnaryFromReflectionRejectsInvalidBodyBeforeDispatch(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{})
	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	selectedMethod := findCatalogMethod(t, catalogResponse.Data.Services, "tether.demo.v1.ReflectionDemo.Ping")
	response := service.InvokeUnaryFromReflection(context.Background(), ReflectionUnaryInvokeInput{
		Endpoint: catalogResponse.Data.Endpoint,
		Method:   selectedMethod,
		Body:     map[string]any{"seconds": "oops"},
	})

	if response.Ok {
		t.Fatalf("expected unary invoke to reject an invalid request body")
	}

	if response.Error == nil || response.Error.Code != "validation.request_body_invalid" || response.Error.Category != contracts.ErrorCategoryValidation {
		t.Fatalf("expected validation.request_body_invalid error, got %+v", response.Error)
	}
}

func TestInvokeUnaryFromReflectionSurfacesGRPCStatusErrors(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{})
	catalogResponse := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", catalogResponse.Error)
	}

	selectedMethod := findCatalogMethod(t, catalogResponse.Data.Services, "tether.demo.v1.ReflectionDemo.Ping")
	response := service.InvokeUnaryFromReflection(context.Background(), ReflectionUnaryInvokeInput{
		Endpoint: catalogResponse.Data.Endpoint,
		Method:   selectedMethod,
		Body:     "1969-12-31T23:59:59Z",
	})

	if response.Ok {
		t.Fatalf("expected unary invoke to surface gRPC status failure")
	}

	if response.Error == nil || response.Error.Code != "grpc_status.invalid_argument" || response.Error.Category != contracts.ErrorCategoryGRPCStatus {
		t.Fatalf("expected grpc_status.invalid_argument error, got %+v", response.Error)
	}
}

func findCatalogMethod(t *testing.T, services []contracts.CatalogService, fullName string) contracts.CatalogMethod {
	t.Helper()

	for _, service := range services {
		for _, method := range service.Methods {
			if method.FullName == fullName {
				return method
			}
		}
	}

	t.Fatalf("catalog method %s not found", fullName)
	return contracts.CatalogMethod{}
}
