package endpoint

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"tether/internal/contracts"
)

func ValidateEndpointPreset(endpointPreset contracts.EndpointPreset) []contracts.EndpointValidationIssue {
	var issues []contracts.EndpointValidationIssue

	host, port, targetErr := splitHostPort(endpointPreset.Target)
	switch {
	case strings.TrimSpace(endpointPreset.Target) == "":
		issues = append(issues, newValidationIssue("target", "validation.endpoint_target_required", "Endpoint target is required."))
	case targetErr != nil:
		issues = append(issues, newValidationIssue("target", "validation.endpoint_target_invalid", "Endpoint target must use the host:port format."))
	case host == "":
		issues = append(issues, newValidationIssue("target", "validation.endpoint_target_invalid", "Endpoint target must include a host name or IP address."))
	case port == "":
		issues = append(issues, newValidationIssue("target", "validation.endpoint_target_invalid", "Endpoint target must include a port."))
	default:
		portValue, err := strconv.Atoi(port)
		if err != nil || portValue < 1 || portValue > 65535 {
			issues = append(issues, newValidationIssue("target", "validation.endpoint_target_port_invalid", "Endpoint port must be between 1 and 65535."))
		}
	}

	if endpointPreset.ConnectTimeoutMs <= 0 {
		issues = append(issues, newValidationIssue("connectTimeoutMs", "validation.endpoint_connect_timeout_invalid", "Connect timeout must be greater than zero milliseconds."))
	}

	if endpointPreset.RequestTimeoutMs < 0 {
		issues = append(issues, newValidationIssue("requestTimeoutMs", "validation.endpoint_request_timeout_invalid", "Default request timeout cannot be negative."))
	}

	if endpointPreset.StreamIdleTimeoutMs < 0 {
		issues = append(issues, newValidationIssue("streamIdleTimeoutMs", "validation.endpoint_stream_idle_timeout_invalid", "Stream idle timeout cannot be negative."))
	}

	if endpointPreset.Authority != "" && !isValidAuthority(endpointPreset.Authority) {
		issues = append(issues, newValidationIssue("authority", "validation.endpoint_authority_invalid", "Authority must be a valid host or host:port authority value without path segments."))
	}

	if endpointPreset.TLS.ServerNameOverride != "" && !isValidServerName(endpointPreset.TLS.ServerNameOverride) {
		issues = append(issues, newValidationIssue("tls.serverNameOverride", "validation.endpoint_server_name_override_invalid", "Server name override must be a valid hostname or IP literal."))
	}

	switch endpointPreset.TLS.Mode {
	case contracts.TLSModePlaintext:
		if endpointPreset.TLS.ServerNameOverride != "" {
			issues = append(issues, newValidationIssue("tls.serverNameOverride", "validation.endpoint_server_name_override_requires_tls", "Server name override is only valid for TLS-enabled endpoints."))
		}
		if endpointPreset.TLS.CACert != "" {
			issues = append(issues, newValidationIssue("tls.caCert", "validation.endpoint_tls_ca_not_allowed", "Plaintext endpoints cannot reference a CA certificate."))
		}
		if endpointPreset.TLS.ClientCert != "" || endpointPreset.TLS.ClientKey != "" {
			issues = append(issues, newValidationIssue("tls.clientCert", "validation.endpoint_tls_client_material_not_allowed", "Plaintext endpoints cannot reference a client certificate or private key."))
		}
	case contracts.TLSModeSystemCA:
		if endpointPreset.TLS.CACert != "" {
			issues = append(issues, newValidationIssue("tls.caCert", "validation.endpoint_tls_ca_not_allowed", "System CA mode must not include a custom CA certificate reference."))
		}
		if endpointPreset.TLS.ClientCert != "" || endpointPreset.TLS.ClientKey != "" {
			issues = append(issues, newValidationIssue("tls.clientCert", "validation.endpoint_tls_client_material_not_allowed", "System CA mode must not include a client certificate or private key."))
		}
	case contracts.TLSModeCustomCA:
		if endpointPreset.TLS.CACert == "" {
			issues = append(issues, newValidationIssue("tls.caCert", "validation.endpoint_tls_ca_required", "Custom CA mode requires a CA certificate reference."))
		}
		if endpointPreset.TLS.ClientCert != "" || endpointPreset.TLS.ClientKey != "" {
			issues = append(issues, newValidationIssue("tls.clientCert", "validation.endpoint_tls_client_material_not_allowed", "Custom CA mode must not include a client certificate or private key."))
		}
	case contracts.TLSModeMTLS:
		if endpointPreset.TLS.ClientCert == "" || endpointPreset.TLS.ClientKey == "" {
			issues = append(issues, newValidationIssue("tls.clientCert", "validation.endpoint_tls_client_material_required", "mTLS mode requires both a client certificate and a private key reference."))
		}
	default:
		issues = append(issues, newValidationIssue("tls.mode", "validation.endpoint_tls_mode_invalid", "TLS mode must be one of plaintext, system_ca, custom_ca or mtls."))
	}

	normalizedKeys := make(map[string]string, len(endpointPreset.MetadataDefaults))
	for key := range endpointPreset.MetadataDefaults {
		if strings.TrimSpace(key) == "" {
			issues = append(issues, newValidationIssue("metadataDefaults", "validation.endpoint_metadata_key_required", "Metadata keys must not be empty."))
			continue
		}

		lower := strings.ToLower(key)
		for _, r := range key {
			switch {
			case r >= 'a' && r <= 'z':
			case r >= 'A' && r <= 'Z':
			case r >= '0' && r <= '9':
			case r == '-' || r == '_' || r == '.':
			default:
				issues = append(issues, newValidationIssue(fmt.Sprintf("metadataDefaults.%s", key), "validation.endpoint_metadata_key_invalid", "Metadata keys may only use letters, digits, hyphen, underscore or dot."))
				goto nextKey
			}
		}

		if strings.HasPrefix(lower, "grpc-") {
			issues = append(issues, newValidationIssue(fmt.Sprintf("metadataDefaults.%s", key), "validation.endpoint_metadata_key_reserved", "Metadata keys starting with grpc- are reserved for gRPC internals."))
		}

		if previous, exists := normalizedKeys[lower]; exists && previous != key {
			issues = append(issues, newValidationIssue("metadataDefaults", "validation.endpoint_metadata_key_duplicate", "Metadata keys must stay unique after lowercasing."))
		} else {
			normalizedKeys[lower] = key
		}

	nextKey:
	}

	return issues
}

func normalizeEndpointPreset(endpointPreset contracts.EndpointPreset) contracts.EndpointPreset {
	normalized := endpointPreset
	normalized.Target = strings.TrimSpace(endpointPreset.Target)
	normalized.Authority = strings.TrimSpace(endpointPreset.Authority)
	normalized.TLS.ServerNameOverride = strings.TrimSpace(endpointPreset.TLS.ServerNameOverride)
	normalized.TLS.CACert = strings.TrimSpace(endpointPreset.TLS.CACert)
	normalized.TLS.ClientCert = strings.TrimSpace(endpointPreset.TLS.ClientCert)
	normalized.TLS.ClientKey = strings.TrimSpace(endpointPreset.TLS.ClientKey)

	if len(endpointPreset.MetadataDefaults) > 0 {
		normalized.MetadataDefaults = make(map[string]string, len(endpointPreset.MetadataDefaults))
		for key, value := range endpointPreset.MetadataDefaults {
			normalized.MetadataDefaults[strings.ToLower(strings.TrimSpace(key))] = value
		}
	}

	return normalized
}

func NormalizeEndpointPreset(endpointPreset contracts.EndpointPreset) contracts.EndpointPreset {
	return normalizeEndpointPreset(endpointPreset)
}

func newValidationIssue(field, code, message string) contracts.EndpointValidationIssue {
	return contracts.EndpointValidationIssue{
		Field:   field,
		Code:    code,
		Message: message,
	}
}

func splitHostPort(target string) (string, string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(target))
	if err == nil {
		return host, port, nil
	}

	if addrErr, ok := err.(*net.AddrError); ok && strings.Contains(addrErr.Err, "missing port in address") {
		return "", "", err
	}

	return "", "", err
}

func isValidAuthority(value string) bool {
	parsed, err := url.Parse("//" + value)
	if err != nil {
		return false
	}

	if parsed.Host == "" || parsed.Host != value {
		return false
	}

	return !strings.ContainsAny(value, " /\t\r\n")
}

func isValidServerName(value string) bool {
	if strings.ContainsAny(value, " /\t\r\n") {
		return false
	}

	if strings.Contains(value, ":") && net.ParseIP(strings.Trim(value, "[]")) == nil {
		return false
	}

	return true
}
