package endpoint

import (
	"context"
	"testing"

	"catenar/internal/contracts"
)

func TestResolveInvokeMetadataInterpolatesWorkspaceEnvironmentValues(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceDependencies{AppDataDir: t.TempDir()})
	metadata, diagnostic := service.resolveInvokeMetadata(
		context.Background(),
		WorkspaceContext{
			Environments: map[string]map[string]string{
				"local": {
					"token":  "dev-token",
					"tenant": "dev",
				},
			},
		},
		contracts.EndpointPreset{
			MetadataDefaults: map[string]string{
				"authorization": "Bearer {{ env.token }}",
			},
		},
		"local",
		map[string]string{
			"x-tenant": "{{ env.tenant }}",
		},
	)

	if diagnostic != nil {
		t.Fatalf("expected metadata interpolation to succeed, got %+v", diagnostic)
	}
	if metadata["authorization"] != "Bearer dev-token" {
		t.Fatalf("expected resolved authorization metadata, got %+v", metadata)
	}
	if metadata["x-tenant"] != "dev" {
		t.Fatalf("expected resolved tenant metadata, got %+v", metadata)
	}
}

func TestResolveInvokeMetadataRequiresEnvironmentForTemplates(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceDependencies{AppDataDir: t.TempDir()})
	_, diagnostic := service.resolveInvokeMetadata(
		context.Background(),
		WorkspaceContext{},
		contracts.EndpointPreset{
			MetadataDefaults: map[string]string{
				"authorization": "Bearer {{ env.token }}",
			},
		},
		"",
		nil,
	)

	if diagnostic == nil || diagnostic.Code != "validation.environment_ref_required" {
		t.Fatalf("expected environmentRef required diagnostic, got %+v", diagnostic)
	}
}

func TestResolveInvokeMetadataRejectsMissingEnvironmentKey(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceDependencies{AppDataDir: t.TempDir()})
	_, diagnostic := service.resolveInvokeMetadata(
		context.Background(),
		WorkspaceContext{
			Environments: map[string]map[string]string{
				"local": {},
			},
		},
		contracts.EndpointPreset{
			MetadataDefaults: map[string]string{
				"authorization": "Bearer {{ env.token }}",
			},
		},
		"local",
		nil,
	)

	if diagnostic == nil || diagnostic.Code != "validation.template_env_key_missing" {
		t.Fatalf("expected missing env key diagnostic, got %+v", diagnostic)
	}
}
