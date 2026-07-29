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
	"time"

	"catenar/internal/contracts"
)

func (s *Service) MaterialRegisterFile(ctx context.Context, input contracts.MaterialRegisterFileInput) contracts.MaterialRegisterFileResponse {
	if ctx == nil {
		ctx = context.Background()
	}

	record, errEnvelope := s.materialRecordFromRegisterInput(input)
	if errEnvelope != nil {
		return contracts.MaterialRegisterFileResponse{Ok: false, Error: errEnvelope}
	}

	saved, err := s.materialIndex.Upsert(ctx, record)
	if err != nil {
		return contracts.MaterialRegisterFileResponse{Ok: false, Error: materialRegisterErrorEnvelope(err)}
	}

	return contracts.MaterialRegisterFileResponse{
		Ok: true,
		Data: &contracts.MaterialRegisterFileResult{
			Ref:    secretRefForMaterial(saved),
			Record: materialRecordToContract(saved),
		},
	}
}

func (s *Service) materialRecordFromRegisterInput(input contracts.MaterialRegisterFileInput) (MaterialRecord, *contracts.ErrorEnvelope) {
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		return MaterialRecord{}, materialValidationError(
			"validation.material_namespace_required",
			"Material namespace is required.",
			map[string]string{"field": "namespace"},
		)
	}

	key := strings.TrimSpace(input.Key)
	if key == "" {
		return MaterialRecord{}, materialValidationError(
			"validation.material_key_required",
			"Material key is required.",
			map[string]string{"field": "key"},
		)
	}

	kind := strings.TrimSpace(input.Kind)
	if !isSupportedMaterialKind(kind) {
		return MaterialRecord{}, materialValidationError(
			"validation.material_kind_invalid",
			"Material kind is not supported.",
			map[string]string{
				"field": "kind",
				"kind":  kind,
			},
		)
	}

	path := strings.TrimSpace(input.Path)
	if path == "" {
		return MaterialRecord{}, materialValidationError(
			"validation.material_path_required",
			"Material file path is required.",
			map[string]string{"field": "path"},
		)
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return MaterialRecord{}, materialValidationError(
			"validation.material_path_invalid",
			"Material file path could not be resolved.",
			map[string]string{
				"field": "path",
				"cause": err.Error(),
			},
		)
	}
	absolutePath = filepath.Clean(absolutePath)

	stat, err := os.Stat(absolutePath)
	if err != nil {
		return MaterialRecord{}, materialValidationError(
			"validation.material_path_missing",
			"Material file does not exist.",
			map[string]string{
				"field": "path",
				"path":  absolutePath,
				"cause": err.Error(),
			},
		)
	}
	if stat.IsDir() {
		return MaterialRecord{}, materialValidationError(
			"validation.material_path_not_file",
			"Material path must point to a file.",
			map[string]string{
				"field": "path",
				"path":  absolutePath,
			},
		)
	}

	now := s.now().UTC().Format(time.RFC3339Nano)
	return MaterialRecord{
		Backend:   "file",
		Namespace: namespace,
		Key:       key,
		Path:      absolutePath,
		Kind:      kind,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (i *jsonMaterialIndex) Upsert(ctx context.Context, record MaterialRecord) (MaterialRecord, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return MaterialRecord{}, err
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	records, err := i.readRecordsForWriteLocked()
	if err != nil {
		return MaterialRecord{}, err
	}

	for index, existing := range records {
		if existing.Backend == record.Backend && existing.Namespace == record.Namespace && existing.Key == record.Key {
			if existing.CreatedAt != "" {
				record.CreatedAt = existing.CreatedAt
			}
			records[index] = record
			return record, i.writeRecordsLocked(records)
		}
	}

	records = append(records, record)
	return record, i.writeRecordsLocked(records)
}

func (i *jsonMaterialIndex) readRecordsForWriteLocked() ([]MaterialRecord, error) {
	content, err := os.ReadFile(i.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "application.secret_material_index_unreadable",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The TLS material index could not be read from disk.",
				Details: map[string]string{
					"path":  i.path,
					"cause": err.Error(),
				},
			},
			NextStep: "Check application data directory permissions and register the file material again.",
		}
	}

	records, err := decodeMaterialRecords(content)
	if err != nil {
		return nil, &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "workspace.secret_material_index_invalid",
				Category: contracts.ErrorCategoryWorkspace,
				Message:  "The TLS material index file is malformed.",
				Details: map[string]string{
					"path":  i.path,
					"cause": err.Error(),
				},
			},
			NextStep: "Repair or recreate the material index before registering new file-backed material.",
		}
	}

	return records, nil
}

func (i *jsonMaterialIndex) writeRecordsLocked(records []MaterialRecord) error {
	if err := os.MkdirAll(filepath.Dir(i.path), 0o700); err != nil {
		return &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "application.secret_material_index_directory_failed",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The material index directory could not be created.",
				Details: map[string]string{
					"path":  filepath.Dir(i.path),
					"cause": err.Error(),
				},
			},
			NextStep: "Check application data directory permissions and register the file material again.",
		}
	}

	content, err := json.MarshalIndent(struct {
		Records []MaterialRecord `json:"records"`
	}{
		Records: records,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal material index: %w", err)
	}
	content = append(content, '\n')

	tempPath := i.path + ".tmp"
	if err := os.WriteFile(tempPath, content, 0o600); err != nil {
		return &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "application.secret_material_index_write_failed",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The material index file could not be written.",
				Details: map[string]string{
					"path":  i.path,
					"cause": err.Error(),
				},
			},
			NextStep: "Check application data directory permissions and register the file material again.",
		}
	}

	if err := os.Rename(tempPath, i.path); err != nil {
		_ = os.Remove(tempPath)
		return &classifiedError{
			Envelope: contracts.ErrorEnvelope{
				Code:     "application.secret_material_index_write_failed",
				Category: contracts.ErrorCategoryApplication,
				Message:  "The material index file could not be finalized.",
				Details: map[string]string{
					"path":  i.path,
					"cause": err.Error(),
				},
			},
			NextStep: "Check application data directory permissions and register the file material again.",
		}
	}

	return nil
}

func isSupportedMaterialKind(kind string) bool {
	switch contracts.MaterialKind(kind) {
	case contracts.MaterialKindTLSCA,
		contracts.MaterialKindTLSClientCert,
		contracts.MaterialKindTLSClientKey,
		contracts.MaterialKindMetadata:
		return true
	default:
		return false
	}
}

func secretRefForMaterial(record MaterialRecord) string {
	return "secret-ref:" + url.PathEscape(record.Backend) + "/" + url.PathEscape(record.Namespace) + "/" + url.PathEscape(record.Key)
}

func materialRecordToContract(record MaterialRecord) contracts.MaterialFileRecord {
	return contracts.MaterialFileRecord{
		Backend:   record.Backend,
		Namespace: record.Namespace,
		Key:       record.Key,
		Path:      record.Path,
		Kind:      record.Kind,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func materialValidationError(code, message string, details map[string]string) *contracts.ErrorEnvelope {
	return &contracts.ErrorEnvelope{
		Code:     code,
		Category: contracts.ErrorCategoryValidation,
		Message:  message,
		Details:  details,
	}
}

func materialRegisterErrorEnvelope(err error) *contracts.ErrorEnvelope {
	if classified, ok := err.(*classifiedError); ok {
		return &contracts.ErrorEnvelope{
			Code:     classified.Envelope.Code,
			Category: classified.Envelope.Category,
			Message:  classified.Envelope.Message,
			Details:  copyDetails(classified.Envelope.Details),
		}
	}

	return &contracts.ErrorEnvelope{
		Code:     "application.secret_material_register_failed",
		Category: contracts.ErrorCategoryApplication,
		Message:  "The file material could not be registered.",
		Details: map[string]string{
			"cause": err.Error(),
		},
	}
}
