package endpoint

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/codes"

	"tether/internal/contracts"
)

func TestCatalogLoadFromProtoSourcesSupportsMultiDirectoryImportsAndBuildsRequestTemplates(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		disableReflection: true,
	})
	defer stop()

	serviceRoot, importRoot := writeProtoCatalogFixture(t)
	service := NewService(ServiceDependencies{})

	response := service.CatalogLoadFromProtoSources(context.Background(), contracts.CatalogLoadFromProtoSourcesInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
		ProtoSources: []contracts.ProtoSource{
			{Type: contracts.ProtoSourceTypeDirectory, Path: serviceRoot},
		},
		ImportPaths: []string{importRoot},
	})

	if !response.Ok || response.Data == nil {
		t.Fatalf("expected proto catalog, got %+v", response.Error)
	}

	if response.Data.Endpoint.ID == "" {
		t.Fatalf("expected transient endpoint identity to be assigned")
	}

	if response.Data.Diagnostic == nil || response.Data.Diagnostic.Code != "proto.catalog_loaded" {
		t.Fatalf("expected proto.catalog_loaded diagnostic, got %+v", response.Data.Diagnostic)
	}

	selectedMethod := findCatalogMethod(t, response.Data.Services, "tether.demo.v1.ReflectionDemo.Ping")
	if selectedMethod.RPCType != contracts.RPCTypeUnary {
		t.Fatalf("expected unary proto-loaded method, got %+v", selectedMethod)
	}

	template, ok := response.Data.RequestTemplates[selectedMethod.FullName]
	if !ok {
		t.Fatalf("expected unary request template to be present")
	}

	templateValue, ok := template.(string)
	if !ok || templateValue != "" {
		t.Fatalf("expected timestamp request template to be an empty string, got %#v", template)
	}
}

func TestCatalogLoadFromProtoSourcesPrefersSelectedSourceRootsOverImportPaths(t *testing.T) {
	t.Parallel()

	serviceRoot, importRoot := writeOverlappingProtoCatalogFixture(t)
	service := NewService(ServiceDependencies{})

	response := service.CatalogLoadFromProtoSources(context.Background(), contracts.CatalogLoadFromProtoSourcesInput{
		Endpoint: contracts.EndpointPreset{
			Target:              "127.0.0.1:50051",
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
		ProtoSources: []contracts.ProtoSource{
			{Type: contracts.ProtoSourceTypeDirectory, Path: serviceRoot},
		},
		ImportPaths: []string{importRoot},
	})

	if !response.Ok || response.Data == nil {
		t.Fatalf("expected proto catalog, got %+v", response.Error)
	}

	template, ok := response.Data.RequestTemplates["tether.demo.v1.ReflectionDemo.UseSource"]
	if !ok {
		t.Fatalf("expected request template for imported source-backed method")
	}

	templateBody, ok := template.(map[string]any)
	if !ok {
		t.Fatalf("expected object request template, got %T", template)
	}

	if _, ok := templateBody["preferredSource"]; !ok {
		t.Fatalf("expected source-root descriptor fields to win, got %+v", templateBody)
	}
	if _, ok := templateBody["wrongImportRoot"]; ok {
		t.Fatalf("expected conflicting import-root descriptor to be ignored, got %+v", templateBody)
	}
}

func TestCallInvokeUnaryUsesCachedProtoCatalogWithoutReflectionAndPersistsHistory(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		disableReflection: true,
	})
	defer stop()

	serviceRoot, importRoot := writeProtoCatalogFixture(t)
	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})

	catalogResponse := service.CatalogLoadFromProtoSources(context.Background(), contracts.CatalogLoadFromProtoSourcesInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
		ProtoSources: []contracts.ProtoSource{
			{Type: contracts.ProtoSourceTypeDirectory, Path: serviceRoot},
		},
		ImportPaths: []string{importRoot},
	})

	if !catalogResponse.Ok || catalogResponse.Data == nil {
		t.Fatalf("expected proto catalog, got %+v", catalogResponse.Error)
	}

	invokeResponse := service.CallInvokeUnary(context.Background(), contracts.CallInvokeUnaryInput{
		CatalogSource: contracts.CatalogSourceProto,
		EndpointID:    catalogResponse.Data.Endpoint.ID,
		Method:        "tether.demo.v1.ReflectionDemo.Ping",
		Metadata: map[string]string{
			"authorization": "Bearer proto-secret",
			"x-request-id":  "req-proto-123",
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

	body, ok := invokeResponse.Data.ResponseBody.(map[string]any)
	if !ok {
		t.Fatalf("expected struct response body, got %T", invokeResponse.Data.ResponseBody)
	}
	if body["echoSeconds"] != float64(1777284932) {
		t.Fatalf("expected echoed timestamp seconds, got %+v", body)
	}

	historyList := service.HistoryList(context.Background(), contracts.HistoryListInput{Limit: 5})
	if !historyList.Ok || historyList.Data == nil || len(historyList.Data.Calls) != 1 {
		t.Fatalf("expected one history summary, got %+v", historyList)
	}

	summary := historyList.Data.Calls[0]
	if summary.CallID != invokeResponse.Data.CallID || summary.GRPCStatusCode != codes.OK.String() {
		t.Fatalf("unexpected call summary: %+v", summary)
	}

	historyGet := service.HistoryGet(context.Background(), invokeResponse.Data.CallID)
	if !historyGet.Ok || historyGet.Data == nil {
		t.Fatalf("expected history detail, got %+v", historyGet.Error)
	}

	startedEvent := historyGet.Data.Events[0]
	if startedEvent.GRPC == nil || startedEvent.GRPC.Metadata == nil {
		t.Fatalf("expected request metadata in stored event, got %+v", startedEvent)
	}
	if startedEvent.GRPC.Metadata["authorization"][0] != "[REDACTED]" {
		t.Fatalf("expected authorization metadata to be redacted, got %+v", startedEvent.GRPC.Metadata)
	}
	if startedEvent.GRPC.Metadata["x-request-id"][0] != "req-proto-123" {
		t.Fatalf("expected non-secret metadata to be preserved, got %+v", startedEvent.GRPC.Metadata)
	}
}

func TestCatalogLoadFromProtoSourcesClassifiesMissingImportAsProto(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		disableReflection: true,
	})
	defer stop()

	serviceRoot, _ := writeProtoCatalogFixture(t)
	service := NewService(ServiceDependencies{})

	response := service.CatalogLoadFromProtoSources(context.Background(), contracts.CatalogLoadFromProtoSourcesInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
		ProtoSources: []contracts.ProtoSource{
			{Type: contracts.ProtoSourceTypeDirectory, Path: serviceRoot},
		},
	})

	if response.Ok {
		t.Fatalf("expected proto load to fail when imports are missing")
	}

	if response.Error == nil || response.Error.Code != "proto.missing_import" || response.Error.Category != contracts.ErrorCategoryProto {
		t.Fatalf("expected proto.missing_import error, got %+v", response.Error)
	}

	if response.Error.Details["import"] != "shared/v1/proto_marker.proto" {
		t.Fatalf("expected missing import details, got %+v", response.Error.Details)
	}
	if response.Error.Details["file"] != "demo/v1/reflection_demo.proto" {
		t.Fatalf("expected source file details, got %+v", response.Error.Details)
	}
}

func writeProtoCatalogFixture(t *testing.T) (string, string) {
	t.Helper()

	serviceRoot := filepath.Join(t.TempDir(), "proto-service")
	importRoot := filepath.Join(t.TempDir(), "proto-imports")

	writeProtoFile(
		t,
		serviceRoot,
		filepath.Join("demo", "v1", "reflection_demo.proto"),
		`syntax = "proto3";
package tether.demo.v1;

import "google/protobuf/empty.proto";
import "google/protobuf/struct.proto";
import "google/protobuf/timestamp.proto";
import "shared/v1/proto_marker.proto";

message ProtoImportMarker {
  tether.demo.shared.v1.ImportedMarker marker = 1;
}

service ReflectionDemo {
  rpc Ping(google.protobuf.Timestamp) returns (google.protobuf.Struct);
  rpc Watch(google.protobuf.Empty) returns (stream google.protobuf.Timestamp);
}
`,
	)

	writeProtoFile(
		t,
		importRoot,
		filepath.Join("shared", "v1", "proto_marker.proto"),
		`syntax = "proto3";
package tether.demo.shared.v1;

message ImportedMarker {
  string label = 1;
}
`,
	)

	return serviceRoot, importRoot
}

func writeOverlappingProtoCatalogFixture(t *testing.T) (string, string) {
	t.Helper()

	serviceRoot := filepath.Join(t.TempDir(), "proto-service")
	importRoot := filepath.Join(t.TempDir(), "proto-imports")

	writeProtoFile(
		t,
		serviceRoot,
		filepath.Join("demo", "v1", "reflection_demo.proto"),
		`syntax = "proto3";
package tether.demo.v1;

import "google/protobuf/empty.proto";
import "shared/v1/request.proto";

service ReflectionDemo {
  rpc UseSource(tether.demo.shared.v1.SourceRequest) returns (google.protobuf.Empty);
}
`,
	)

	writeProtoFile(
		t,
		serviceRoot,
		filepath.Join("shared", "v1", "request.proto"),
		`syntax = "proto3";
package tether.demo.shared.v1;

message SourceRequest {
  string preferred_source = 1;
}
`,
	)

	writeProtoFile(
		t,
		importRoot,
		filepath.Join("shared", "v1", "request.proto"),
		`syntax = "proto3";
package tether.demo.shared.v1;

message SourceRequest {
  int32 wrong_import_root = 1;
}
`,
	)

	return serviceRoot, importRoot
}

func writeProtoFile(t *testing.T, root string, relativePath string, contents string) {
	t.Helper()

	fullPath := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir proto path: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write proto file: %v", err)
	}
}
