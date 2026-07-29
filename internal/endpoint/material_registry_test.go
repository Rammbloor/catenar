package endpoint

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"catenar/internal/contracts"
)

func TestMaterialRegisterFileStoresLogicalSecretRef(t *testing.T) {
	appDataDir := t.TempDir()
	materialPath := writeFile(t, t.TempDir(), "ca.pem", []byte("certificate-data"))
	now := time.Date(2026, 5, 9, 9, 30, 0, 0, time.UTC)

	service := NewService(ServiceDependencies{
		AppDataDir: appDataDir,
		Now:        func() time.Time { return now },
	})

	response := service.MaterialRegisterFile(context.Background(), contracts.MaterialRegisterFileInput{
		Namespace: "tls",
		Key:       "ca.pem",
		Path:      materialPath,
		Kind:      string(contracts.MaterialKindTLSCA),
	})
	if !response.Ok {
		t.Fatalf("expected material registration to succeed, got %+v", response.Error)
	}

	if response.Data.Ref != "secret-ref:file/tls/ca.pem" {
		t.Fatalf("unexpected material ref %q", response.Data.Ref)
	}
	if response.Data.Record.Path != materialPath {
		t.Fatalf("expected absolute material path %q, got %q", materialPath, response.Data.Record.Path)
	}
	if response.Data.Record.CreatedAt != now.Format(time.RFC3339Nano) ||
		response.Data.Record.UpdatedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected timestamps: %+v", response.Data.Record)
	}

	resolved, err := service.secretStore.Resolve(context.Background(), WorkspaceContext{}, response.Data.Ref, SecretUsageTLSCA)
	if err != nil {
		t.Fatalf("resolve registered material: %v", err)
	}
	if !bytes.Equal(resolved.Bytes, []byte("certificate-data")) {
		t.Fatalf("unexpected resolved material bytes %q", string(resolved.Bytes))
	}
}

func TestMaterialRegisterFileUpsertsExistingHandle(t *testing.T) {
	appDataDir := t.TempDir()
	firstPath := writeFile(t, t.TempDir(), "first.txt", []byte("first"))
	secondPath := writeFile(t, t.TempDir(), "second.txt", []byte("second"))
	times := []time.Time{
		time.Date(2026, 5, 9, 9, 30, 0, 0, time.UTC),
		time.Date(2026, 5, 9, 9, 45, 0, 0, time.UTC),
	}
	index := 0

	service := NewService(ServiceDependencies{
		AppDataDir: appDataDir,
		Now: func() time.Time {
			current := times[index]
			if index < len(times)-1 {
				index++
			}
			return current
		},
	})

	firstResponse := service.MaterialRegisterFile(context.Background(), contracts.MaterialRegisterFileInput{
		Namespace: "env",
		Key:       "token",
		Path:      firstPath,
		Kind:      string(contracts.MaterialKindMetadata),
	})
	if !firstResponse.Ok {
		t.Fatalf("expected first registration to succeed, got %+v", firstResponse.Error)
	}

	secondResponse := service.MaterialRegisterFile(context.Background(), contracts.MaterialRegisterFileInput{
		Namespace: "env",
		Key:       "token",
		Path:      secondPath,
		Kind:      string(contracts.MaterialKindMetadata),
	})
	if !secondResponse.Ok {
		t.Fatalf("expected second registration to succeed, got %+v", secondResponse.Error)
	}

	content, err := os.ReadFile(materialIndexPath(appDataDir))
	if err != nil {
		t.Fatalf("read material index: %v", err)
	}
	records, err := decodeMaterialRecords(content)
	if err != nil {
		t.Fatalf("decode material index: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one upserted record, got %+v", records)
	}
	if records[0].Path != secondPath {
		t.Fatalf("expected updated path %q, got %q", secondPath, records[0].Path)
	}
	if records[0].CreatedAt != times[0].Format(time.RFC3339Nano) ||
		records[0].UpdatedAt != times[1].Format(time.RFC3339Nano) {
		t.Fatalf("expected createdAt to be preserved and updatedAt to move forward, got %+v", records[0])
	}
}

func TestMaterialRegisterFileRejectsMissingFile(t *testing.T) {
	service := NewService(ServiceDependencies{AppDataDir: t.TempDir()})

	response := service.MaterialRegisterFile(context.Background(), contracts.MaterialRegisterFileInput{
		Namespace: "tls",
		Key:       "ca.pem",
		Path:      "/path/that/does/not/exist.pem",
		Kind:      string(contracts.MaterialKindTLSCA),
	})
	if response.Ok || response.Error == nil {
		t.Fatalf("expected missing file validation error, got %+v", response)
	}
	if response.Error.Code != "validation.material_path_missing" {
		t.Fatalf("expected validation.material_path_missing, got %+v", response.Error)
	}
}
