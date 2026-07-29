package endpoint

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"catenar/internal/contracts"
)

var envTemplateExpression = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

func (s *Service) resolveInvokeMetadata(
	ctx context.Context,
	scope WorkspaceContext,
	endpointPreset contracts.EndpointPreset,
	environmentRef string,
	overrides map[string]string,
) (map[string]string, *endpointDiagnostic) {
	if ctx == nil {
		ctx = context.Background()
	}

	merged := mergeInvokeMetadata(endpointPreset.MetadataDefaults, overrides)
	trimmedRef := strings.TrimSpace(environmentRef)
	hasTemplate := metadataHasTemplate(merged)

	if trimmedRef == "" {
		if !hasTemplate {
			return merged, nil
		}
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     "validation.environment_ref_required",
			Category: contracts.ErrorCategoryValidation,
			Message:  "Metadata templates that reference env values require an environmentRef.",
			NextStep: "Select a workspace environment or remove {{ env.* }} placeholders from metadata.",
		}
	}

	values, ok := scope.Environments[trimmedRef]
	if !ok {
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     "validation.environment_ref_invalid",
			Category: contracts.ErrorCategoryValidation,
			Message:  "The selected environment is not available in the active workspace.",
			NextStep: "Re-open the workspace or choose an environment that exists in workspace.yaml.",
			Details: map[string]string{
				"environmentRef": trimmedRef,
			},
		}
	}

	if !hasTemplate {
		return merged, nil
	}

	resolved := make(map[string]string, len(merged))
	for key, value := range merged {
		nextValue, diagnostic := s.interpolateEnvTemplates(ctx, scope, key, value, trimmedRef, values)
		if diagnostic != nil {
			return nil, diagnostic
		}
		resolved[key] = nextValue
	}

	return resolved, nil
}

func metadataHasTemplate(metadata map[string]string) bool {
	for _, value := range metadata {
		if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return true
		}
	}
	return false
}

func (s *Service) interpolateEnvTemplates(
	ctx context.Context,
	scope WorkspaceContext,
	metadataKey,
	value,
	environmentRef string,
	values map[string]string,
) (string, *endpointDiagnostic) {
	if !strings.Contains(value, "{{") && !strings.Contains(value, "}}") {
		return value, nil
	}

	unmatched := envTemplateExpression.ReplaceAllString(value, "")
	if strings.Contains(unmatched, "{{") || strings.Contains(unmatched, "}}") {
		return "", &endpointDiagnostic{
			Level:    "error",
			Code:     "validation.template_expression_invalid",
			Category: contracts.ErrorCategoryValidation,
			Message:  "Metadata contains an invalid template expression.",
			NextStep: "Use placeholders in the form {{ env.key }}.",
			Details: map[string]string{
				"metadataKey": metadataKey,
			},
		}
	}

	resolved := value
	for _, match := range envTemplateExpression.FindAllStringSubmatch(value, -1) {
		raw := match[0]
		expression := strings.TrimSpace(match[1])
		if !strings.HasPrefix(expression, "env.") || strings.TrimSpace(strings.TrimPrefix(expression, "env.")) == "" {
			return "", &endpointDiagnostic{
				Level:    "error",
				Code:     "validation.template_expression_unsupported",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Only env templates are supported in request metadata.",
				NextStep: "Use placeholders in the form {{ env.key }}.",
				Details: map[string]string{
					"metadataKey": metadataKey,
					"expression":  expression,
				},
			}
		}

		envKey := strings.TrimSpace(strings.TrimPrefix(expression, "env."))
		envValue, ok := values[envKey]
		if !ok {
			return "", &endpointDiagnostic{
				Level:    "error",
				Code:     "validation.template_env_key_missing",
				Category: contracts.ErrorCategoryValidation,
				Message:  fmt.Sprintf("Environment %q does not define %q.", environmentRef, envKey),
				NextStep: "Add the missing value to the selected workspace environment or update the metadata template.",
				Details: map[string]string{
					"metadataKey":    metadataKey,
					"environmentRef": environmentRef,
					"envKey":         envKey,
				},
			}
		}
		if strings.HasPrefix(strings.TrimSpace(envValue), "secret-ref:") {
			resolvedSecret, diagnostic := s.resolveEnvironmentMetadataSecret(ctx, scope, envValue, metadataKey, environmentRef, envKey)
			if diagnostic != nil {
				return "", diagnostic
			}
			envValue = resolvedSecret
		}

		resolved = strings.ReplaceAll(resolved, raw, envValue)
	}

	return resolved, nil
}

func (s *Service) resolveEnvironmentMetadataSecret(
	ctx context.Context,
	scope WorkspaceContext,
	ref,
	metadataKey,
	environmentRef,
	envKey string,
) (string, *endpointDiagnostic) {
	material, err := s.secretStore.Resolve(ctx, scope, ref, SecretUsageMetadata)
	if err == nil {
		return strings.TrimRight(string(material.Bytes), "\r\n"), nil
	}

	if classified, ok := err.(*classifiedError); ok {
		details := copyDetails(classified.Envelope.Details)
		details["metadataKey"] = metadataKey
		details["environmentRef"] = environmentRef
		details["envKey"] = envKey

		return "", &endpointDiagnostic{
			Level:    "error",
			Code:     classified.Envelope.Code,
			Category: classified.Envelope.Category,
			Message:  classified.Envelope.Message,
			NextStep: classified.NextStep,
			Details:  details,
		}
	}

	return "", &endpointDiagnostic{
		Level:    "error",
		Code:     "application.environment_secret_resolve_failed",
		Category: contracts.ErrorCategoryApplication,
		Message:  "The runtime could not resolve an environment secret reference.",
		NextStep: "Check the material store and retry the call.",
		Details: map[string]string{
			"metadataKey":    metadataKey,
			"environmentRef": environmentRef,
			"envKey":         envKey,
			"cause":          err.Error(),
		},
	}
}
