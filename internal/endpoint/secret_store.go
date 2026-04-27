package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"tether/internal/contracts"
)

type ParsedSecretRef struct {
	Backend   string
	Namespace string
	Key       string
}

type classifiedError struct {
	Envelope contracts.ErrorEnvelope
	NextStep string
}

func (e *classifiedError) Error() string {
	return e.Envelope.Message
}

type MaterialRecord struct {
	Backend   string `json:"backend"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type MaterialIndex interface {
	Lookup(ctx context.Context, ref ParsedSecretRef) (MaterialRecord, error)
}

type fileSecretStore struct {
	index MaterialIndex
}

func NewFileSecretStore(index MaterialIndex) SecretStore {
	return &fileSecretStore{index: index}
}

func (s *fileSecretStore) Resolve(ctx context.Context, _ WorkspaceContext, ref string, usage SecretUsageIntent) (ResolvedMaterial, error) {
	parsed, err := ParseSecretRef(ref)
	if err != nil {
		return ResolvedMaterial{}, err
	}

	switch parsed.Backend {
	case "file":
	default:
		return ResolvedMaterial{}, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "application.secret_store_backend_unavailable",
				Category: contracts.ErrorCategoryApplication,
				Message:  fmt.Sprintf("Secret backend %q is not available in the current runtime path.", parsed.Backend),
				Details: map[string]string{
					"backend": parsed.Backend,
				},
			},
			NextStep: "Re-register the TLS material through the file-backed material store for this MVP runtime path.",
		}
	}

	record, err := s.index.Lookup(ctx, parsed)
	if err != nil {
		return ResolvedMaterial{}, err
	}

	expectedKind := string(usage)
	if record.Kind != "" && record.Kind != expectedKind {
		return ResolvedMaterial{}, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "validation.secret_material_kind_mismatch",
				Category: contracts.ErrorCategoryValidation,
				Message:  "The registered file material kind does not match the TLS field that references it.",
				Details: map[string]string{
					"expectedKind": expectedKind,
					"actualKind":   record.Kind,
					"namespace":    parsed.Namespace,
					"key":          parsed.Key,
				},
			},
			NextStep: "Re-register the file under the correct TLS material field and try the endpoint test again.",
		}
	}

	if !filepath.IsAbs(record.Path) {
		return ResolvedMaterial{}, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "validation.secret_material_path_invalid",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Registered file material must resolve to an absolute path.",
				Details: map[string]string{
					"path": record.Path,
				},
			},
			NextStep: "Register the file again so the runtime can store an absolute path in the material index.",
		}
	}

	content, err := os.ReadFile(record.Path)
	if err != nil {
		return ResolvedMaterial{}, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "validation.secret_material_missing",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Registered TLS material could not be read from disk.",
				Details: map[string]string{
					"path":      record.Path,
					"namespace": parsed.Namespace,
					"key":       parsed.Key,
					"cause":     err.Error(),
				},
			},
			NextStep: "Restore the referenced file or re-register the TLS material path before testing the endpoint again.",
		}
	}

	return ResolvedMaterial{
		Ref:       parsed,
		Kind:      record.Kind,
		Path:      record.Path,
		Bytes:     content,
		UpdatedAt: record.UpdatedAt,
	}, nil
}

func ParseSecretRef(value string) (ParsedSecretRef, error) {
	const prefix = "secret-ref:"

	if !strings.HasPrefix(value, prefix) {
		return ParsedSecretRef{}, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "validation.secret_ref_malformed",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Secret reference must start with secret-ref:.",
				Details: map[string]string{
					"value": value,
				},
			},
			NextStep: "Re-save the endpoint with a valid secret reference identifier.",
		}
	}

	parts := strings.Split(strings.TrimPrefix(value, prefix), "/")
	if len(parts) != 3 {
		return ParsedSecretRef{}, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "validation.secret_ref_malformed",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Secret reference must use the secret-ref:<backend>/<namespace>/<key> format.",
				Details: map[string]string{
					"value": value,
				},
			},
			NextStep: "Re-save the endpoint with a fully qualified secret reference.",
		}
	}

	decoded := make([]string, 0, len(parts))
	for _, part := range parts {
		unescaped, err := url.PathUnescape(part)
		if err != nil || unescaped == "" {
			return ParsedSecretRef{}, &classifiedError{
				Envelope: contracts.ErrorEnvelope{
					Code:     "validation.secret_ref_malformed",
					Category: contracts.ErrorCategoryValidation,
					Message:  "Secret reference contains an invalid percent-encoded segment.",
					Details: map[string]string{
						"value": value,
					},
				},
				NextStep: "Re-save the endpoint with a valid secret reference identifier.",
			}
		}
		decoded = append(decoded, unescaped)
	}

	return ParsedSecretRef{
		Backend:   decoded[0],
		Namespace: decoded[1],
		Key:       decoded[2],
	}, nil
}

type jsonMaterialIndex struct {
	path string
}

func newJSONMaterialIndex(path string) MaterialIndex {
	return jsonMaterialIndex{path: path}
}

func (i jsonMaterialIndex) Lookup(_ context.Context, ref ParsedSecretRef) (MaterialRecord, error) {
	content, err := os.ReadFile(i.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MaterialRecord{}, &classifiedError{
				Envelope: contracts.ErrorEnvelope{
					Code:     "validation.secret_material_not_registered",
					Category: contracts.ErrorCategoryValidation,
					Message:  "No file-backed TLS material has been registered for this endpoint yet.",
					Details: map[string]string{
						"backend":   ref.Backend,
						"namespace": ref.Namespace,
						"key":       ref.Key,
					},
				},
				NextStep: "Register the CA or client certificate file before testing this endpoint.",
			}
		}

		return MaterialRecord{}, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "application.secret_material_index_unreadable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The TLS material index could not be read from disk.",
				Details: map[string]string{
					"path":  i.path,
					"cause": err.Error(),
				},
			},
			NextStep: "Check application data directory permissions and try the endpoint test again.",
		}
	}

	records, err := decodeMaterialRecords(content)
	if err != nil {
		return MaterialRecord{}, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "workspace.secret_material_index_invalid",
				Category: contracts.ErrorCategoryWorkspace,
				Message:  "The TLS material index file is malformed.",
				Details: map[string]string{
					"path":  i.path,
					"cause": err.Error(),
				},
			},
			NextStep: "Repair or recreate the material index before testing endpoints that depend on file-backed TLS material.",
		}
	}

	for _, record := range records {
		if record.Backend == ref.Backend && record.Namespace == ref.Namespace && record.Key == ref.Key {
			return record, nil
		}
	}

	return MaterialRecord{}, &classifiedError{
		Envelope: contracts.ErrorEnvelope{
			Code:     "validation.secret_material_not_registered",
			Category: contracts.ErrorCategoryValidation,
			Message:  "The endpoint references a TLS material handle that is not registered in the local material index.",
			Details: map[string]string{
				"backend":   ref.Backend,
				"namespace": ref.Namespace,
				"key":       ref.Key,
			},
		},
		NextStep: "Register the missing CA, client certificate or private key file before testing the endpoint again.",
	}
}

func decodeMaterialRecords(content []byte) ([]MaterialRecord, error) {
	var container struct {
		Records []MaterialRecord `json:"records"`
	}

	if err := json.Unmarshal(content, &container); err == nil && container.Records != nil {
		return container.Records, nil
	}

	var records []MaterialRecord
	if err := json.Unmarshal(content, &records); err != nil {
		return nil, err
	}

	return records, nil
}
