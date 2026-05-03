package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"

	"tether/internal/contracts"
	"tether/internal/endpoint"
)

const (
	CurrentManifestVersion = 1
	manifestFileName       = "workspace.yaml"
)

type ManagerOptions struct {
	Now func() time.Time
}

type Manager struct {
	mu     sync.RWMutex
	active *activeWorkspace
	now    func() time.Time
}

type activeWorkspace struct {
	id            string
	root          string
	manifestPath  string
	manifest      manifestFile
	savedRequests []savedRequestFile
	backupPaths   []string
}

type manifestFile struct {
	Version       int               `yaml:"version"`
	Name          string            `yaml:"name"`
	Endpoints     []endpointFile    `yaml:"endpoints"`
	ProtoSources  []protoSourceFile `yaml:"protoSources,omitempty"`
	ImportPaths   []string          `yaml:"importPaths,omitempty"`
	SavedRequests []string          `yaml:"savedRequests,omitempty"`
	Settings      *settingsFile     `yaml:"settings,omitempty"`
}

type settingsFile struct {
	RedactDefaults bool                `yaml:"redactDefaults,omitempty"`
	EventRetention *eventRetentionFile `yaml:"eventRetention,omitempty"`
}

type eventRetentionFile struct {
	MaxEventsPerCall int `yaml:"maxEventsPerCall,omitempty"`
	MaxBytesPerCall  int `yaml:"maxBytesPerCall,omitempty"`
}

type endpointFile struct {
	ID                  string            `yaml:"id,omitempty"`
	Name                string            `yaml:"name,omitempty"`
	Target              string            `yaml:"target"`
	Authority           string            `yaml:"authority,omitempty"`
	TLS                 tlsFile           `yaml:"tls"`
	MetadataDefaults    map[string]string `yaml:"metadataDefaults,omitempty"`
	ConnectTimeoutMs    int               `yaml:"connectTimeoutMs,omitempty"`
	RequestTimeoutMs    int               `yaml:"requestTimeoutMs,omitempty"`
	StreamIdleTimeoutMs int               `yaml:"streamIdleTimeoutMs,omitempty"`
}

type tlsFile struct {
	Mode               contracts.TLSMode `yaml:"mode"`
	ServerNameOverride string            `yaml:"serverNameOverride,omitempty"`
	CACert             string            `yaml:"caCert,omitempty"`
	ClientCert         string            `yaml:"clientCert,omitempty"`
	ClientKey          string            `yaml:"clientKey,omitempty"`
}

type protoSourceFile struct {
	Type contracts.ProtoSourceType `yaml:"type"`
	Path string                    `yaml:"path"`
}

type savedRequestFile struct {
	ID               string               `yaml:"id"`
	Method           string               `yaml:"method"`
	RPCType          contracts.RPCType    `yaml:"rpcType"`
	EndpointRef      string               `yaml:"endpointRef"`
	EnvironmentRef   string               `yaml:"environmentRef,omitempty"`
	MetadataTemplate map[string]string    `yaml:"metadataTemplate,omitempty"`
	CallOptions      callOptionsFile      `yaml:"callOptions,omitempty"`
	RequestSpec      savedRequestSpecFile `yaml:"requestSpec"`
	path             string
}

type callOptionsFile struct {
	RequestTimeoutMs    int `yaml:"requestTimeoutMs,omitempty"`
	StreamIdleTimeoutMs int `yaml:"streamIdleTimeoutMs,omitempty"`
}

type savedRequestSpecFile struct {
	Mode     string                    `yaml:"mode"`
	Body     any                       `yaml:"body,omitempty"`
	Messages []contracts.StreamMessage `yaml:"messages,omitempty"`
}

type workspaceIssue struct {
	field    string
	code     string
	category contracts.ErrorCategory
	message  string
	path     string
}

func NewManager(options ManagerOptions) *Manager {
	now := options.Now
	if now == nil {
		now = time.Now
	}

	return &Manager{now: now}
}

func (m *Manager) Create(_ context.Context, input contracts.WorkspaceCreateInput) contracts.WorkspaceResponse {
	root, manifestPath, err := resolveWorkspacePath(input.Path)
	if err != nil {
		return workspaceFailure(workspacePathIssue(err))
	}

	if _, err := os.Stat(manifestPath); err == nil {
		return workspaceFailure(workspaceIssue{
			field:    "path",
			code:     "workspace.manifest_already_exists",
			category: contracts.ErrorCategoryWorkspace,
			message:  "A workspace manifest already exists at the requested path.",
			path:     manifestPath,
		})
	} else if !errors.Is(err, os.ErrNotExist) {
		return workspaceFailure(workspaceIssue{
			field:    "path",
			code:     "workspace.manifest_unreadable",
			category: contracts.ErrorCategoryWorkspace,
			message:  "The workspace manifest path could not be checked.",
			path:     manifestPath,
		})
	}

	manifest := manifestFile{
		Version:      CurrentManifestVersion,
		Name:         strings.TrimSpace(input.Name),
		Endpoints:    endpointsToFile(input.Endpoints),
		ProtoSources: protoSourcesToFile(input.ProtoSources),
		ImportPaths:  cleanStringList(input.ImportPaths),
	}
	if strings.TrimSpace(manifest.Name) == "" {
		manifest.Name = filepath.Base(root)
	}
	manifest.Endpoints = ensureEndpointFileIdentities(manifest.Endpoints)

	if issues := validateManifest(root, manifest, nil); len(issues) > 0 {
		return workspaceFailure(issues[0])
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return workspaceFailure(workspaceIssue{
			field:    "path",
			code:     "workspace.directory_create_failed",
			category: contracts.ErrorCategoryWorkspace,
			message:  "The workspace directory could not be created.",
			path:     root,
		})
	}
	if err := writeManifestFile(manifestPath, manifest); err != nil {
		return workspaceFailure(workspaceIssue{
			field:    "path",
			code:     "workspace.manifest_write_failed",
			category: contracts.ErrorCategoryWorkspace,
			message:  "The workspace manifest could not be written.",
			path:     manifestPath,
		})
	}

	active := &activeWorkspace{
		id:           workspaceID(manifestPath),
		root:         root,
		manifestPath: manifestPath,
		manifest:     manifest,
	}
	m.setActive(active)

	return workspaceSuccess(active.snapshot())
}

func (m *Manager) Open(_ context.Context, path string) contracts.WorkspaceResponse {
	root, manifestPath, err := resolveWorkspacePath(path)
	if err != nil {
		return workspaceFailure(workspacePathIssue(err))
	}

	content, err := os.ReadFile(manifestPath)
	if err != nil {
		code := "workspace.manifest_unreadable"
		message := "The workspace manifest could not be read."
		if errors.Is(err, os.ErrNotExist) {
			code = "workspace.manifest_not_found"
			message = "No workspace manifest exists at the requested path."
		}
		return workspaceFailure(workspaceIssue{
			field:    "path",
			code:     code,
			category: contracts.ErrorCategoryWorkspace,
			message:  message,
			path:     manifestPath,
		})
	}

	manifest, versionPresent, err := decodeManifest(content)
	if err != nil {
		return workspaceFailure(workspaceIssue{
			field:    "manifest",
			code:     "workspace.manifest_invalid",
			category: contracts.ErrorCategoryWorkspace,
			message:  "The workspace manifest is not valid YAML for the v1 schema.",
			path:     manifestPath,
		})
	}
	if !versionPresent {
		return workspaceFailure(workspaceIssue{
			field:    "version",
			code:     "workspace.version_required",
			category: contracts.ErrorCategoryWorkspace,
			message:  "Workspace manifest version is required.",
			path:     manifestPath,
		})
	}
	if manifest.Version > CurrentManifestVersion {
		return workspaceFailure(workspaceIssue{
			field:    "version",
			code:     "workspace.version_unsupported",
			category: contracts.ErrorCategoryWorkspace,
			message:  "This workspace manifest version is newer than this runtime supports.",
			path:     manifestPath,
		})
	}

	var backupPaths []string
	if manifest.Version < CurrentManifestVersion {
		backupPath, err := m.backupManifest(manifestPath)
		if err != nil {
			return workspaceFailure(workspaceIssue{
				field:    "version",
				code:     "workspace.migration_backup_failed",
				category: contracts.ErrorCategoryWorkspace,
				message:  "The workspace manifest could not be backed up before migration.",
				path:     manifestPath,
			})
		}
		backupPaths = append(backupPaths, backupPath)

		manifest = migrateManifest(manifest, root)
		if err := writeManifestFile(manifestPath, manifest); err != nil {
			return workspaceFailure(workspaceIssue{
				field:    "version",
				code:     "workspace.migration_write_failed",
				category: contracts.ErrorCategoryWorkspace,
				message:  "The migrated workspace manifest could not be written.",
				path:     manifestPath,
			})
		}
	}

	savedRequests, issues := loadSavedRequests(root, manifest.SavedRequests)
	if len(issues) > 0 {
		return workspaceFailure(issues[0])
	}
	if issues := validateManifest(root, manifest, savedRequests); len(issues) > 0 {
		return workspaceFailure(issues[0])
	}

	active := &activeWorkspace{
		id:            workspaceID(manifestPath),
		root:          root,
		manifestPath:  manifestPath,
		manifest:      manifest,
		savedRequests: savedRequests,
		backupPaths:   backupPaths,
	}
	m.setActive(active)

	return workspaceSuccess(active.snapshot())
}

func (m *Manager) Save(_ context.Context, input contracts.WorkspaceSaveInput) contracts.WorkspaceResponse {
	active := m.activeClone()
	if active == nil {
		return workspaceFailure(noActiveWorkspaceIssue())
	}

	next := active.manifest
	if strings.TrimSpace(input.Name) != "" {
		next.Name = strings.TrimSpace(input.Name)
	}
	if input.Endpoints != nil {
		next.Endpoints = ensureEndpointFileIdentities(endpointsToFile(input.Endpoints))
	}
	if input.ProtoSources != nil {
		next.ProtoSources = protoSourcesToFile(input.ProtoSources)
	}
	if input.ImportPaths != nil {
		next.ImportPaths = cleanStringList(input.ImportPaths)
	}
	next.Version = CurrentManifestVersion
	next.SavedRequests = cleanStringList(next.SavedRequests)

	if issues := validateManifest(active.root, next, active.savedRequests); len(issues) > 0 {
		return workspaceFailure(issues[0])
	}
	if err := writeManifestFile(active.manifestPath, next); err != nil {
		return workspaceFailure(workspaceIssue{
			field:    "path",
			code:     "workspace.manifest_write_failed",
			category: contracts.ErrorCategoryWorkspace,
			message:  "The workspace manifest could not be written.",
			path:     active.manifestPath,
		})
	}

	active.manifest = next
	m.setActive(active)

	return workspaceSuccess(active.snapshot())
}

func (m *Manager) Validate(_ context.Context, input contracts.WorkspaceValidateInput) contracts.WorkspaceValidateResponse {
	active := m.activeClone()
	if active == nil {
		return contracts.WorkspaceValidateResponse{
			Ok:    false,
			Error: errorEnvelopeFromIssue(noActiveWorkspaceIssue()),
		}
	}

	next := active.manifest
	if strings.TrimSpace(input.Name) != "" {
		next.Name = strings.TrimSpace(input.Name)
	}
	if input.Endpoints != nil {
		next.Endpoints = ensureEndpointFileIdentities(endpointsToFile(input.Endpoints))
	}
	if input.ProtoSources != nil {
		next.ProtoSources = protoSourcesToFile(input.ProtoSources)
	}
	if input.ImportPaths != nil {
		next.ImportPaths = cleanStringList(input.ImportPaths)
	}
	next.Version = CurrentManifestVersion

	issues := validateManifest(active.root, next, active.savedRequests)
	snapshot := active.snapshot()
	return contracts.WorkspaceValidateResponse{
		Ok: true,
		Data: &contracts.WorkspaceValidateResult{
			Workspace: &snapshot,
			Issues:    issuesToContract(issues),
		},
	}
}

func (m *Manager) RequestSave(_ context.Context, input contracts.RequestSaveInput) contracts.RequestSaveResponse {
	active := m.activeClone()
	if active == nil {
		return contracts.RequestSaveResponse{
			Ok:    false,
			Error: errorEnvelopeFromIssue(noActiveWorkspaceIssue()),
		}
	}

	request, issue := requestFileFromInput(input)
	if issue != nil {
		return requestFailure(*issue)
	}
	request.path = requestRelativePath(request.ID)

	requests := replaceSavedRequest(active.savedRequests, request)
	if issues := validateManifest(active.root, active.manifest, requests); len(issues) > 0 {
		return requestFailure(issues[0])
	}

	absolutePath, issue := workspaceRelativePath(active.root, request.path, "savedRequests")
	if issue != nil {
		return requestFailure(*issue)
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return requestFailure(workspaceIssue{
			field:    "savedRequests",
			code:     "workspace.saved_request_write_failed",
			category: contracts.ErrorCategoryWorkspace,
			message:  "The saved request directory could not be created.",
			path:     filepath.Dir(absolutePath),
		})
	}
	if err := writeYAMLFile(absolutePath, request); err != nil {
		return requestFailure(workspaceIssue{
			field:    "savedRequests",
			code:     "workspace.saved_request_write_failed",
			category: contracts.ErrorCategoryWorkspace,
			message:  "The saved request file could not be written.",
			path:     absolutePath,
		})
	}

	nextManifest := active.manifest
	nextManifest.SavedRequests = addUniqueString(nextManifest.SavedRequests, request.path)
	if err := writeManifestFile(active.manifestPath, nextManifest); err != nil {
		return requestFailure(workspaceIssue{
			field:    "savedRequests",
			code:     "workspace.manifest_write_failed",
			category: contracts.ErrorCategoryWorkspace,
			message:  "The workspace manifest could not be updated with the saved request reference.",
			path:     active.manifestPath,
		})
	}

	active.manifest = nextManifest
	active.savedRequests = requests
	m.setActive(active)
	snapshot := active.snapshot()

	return contracts.RequestSaveResponse{
		Ok: true,
		Data: &contracts.RequestSaveResult{
			Workspace:    snapshot,
			SavedRequest: request.summary(),
		},
	}
}

func (m *Manager) PrepareEndpointTest(_ context.Context, input contracts.EndpointTestInput) (endpoint.WorkspaceContext, contracts.EndpointPreset, error) {
	normalized := endpoint.EnsureEndpointIdentity(endpoint.NormalizeEndpointPreset(input.Endpoint))

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active == nil {
		return endpoint.WorkspaceContext{
			ID:   "transient",
			Kind: "editor-session",
		}, normalized, nil
	}

	m.active.manifest.Endpoints = upsertEndpointFile(m.active.manifest.Endpoints, endpointToFile(normalized))
	return endpoint.WorkspaceContext{
		ID:             m.active.id,
		Kind:           "file-workspace",
		EventRetention: eventRetentionFromSettings(m.active.manifest.Settings),
	}, normalized, nil
}

func (m *Manager) backupManifest(manifestPath string) (string, error) {
	backupPath := fmt.Sprintf("%s.bak.%s", manifestPath, m.now().UTC().Format("20060102T150405Z"))

	source, err := os.Open(manifestPath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = source.Close()
	}()

	destination, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = destination.Close()
	}()

	if _, err := io.Copy(destination, source); err != nil {
		return "", err
	}

	return backupPath, nil
}

func (m *Manager) setActive(active *activeWorkspace) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = active
}

func (m *Manager) activeClone() *activeWorkspace {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.active == nil {
		return nil
	}

	cloned := *m.active
	cloned.manifest.Endpoints = append([]endpointFile(nil), m.active.manifest.Endpoints...)
	cloned.manifest.ProtoSources = append([]protoSourceFile(nil), m.active.manifest.ProtoSources...)
	cloned.manifest.ImportPaths = append([]string(nil), m.active.manifest.ImportPaths...)
	cloned.manifest.SavedRequests = append([]string(nil), m.active.manifest.SavedRequests...)
	cloned.savedRequests = append([]savedRequestFile(nil), m.active.savedRequests...)
	cloned.backupPaths = append([]string(nil), m.active.backupPaths...)
	return &cloned
}

func (a *activeWorkspace) snapshot() contracts.WorkspaceSnapshot {
	return contracts.WorkspaceSnapshot{
		ID:            a.id,
		Version:       a.manifest.Version,
		Name:          a.manifest.Name,
		Path:          a.root,
		ManifestPath:  a.manifestPath,
		Endpoints:     endpointsFromFile(a.manifest.Endpoints),
		ProtoSources:  protoSourcesFromFile(a.manifest.ProtoSources),
		ImportPaths:   append([]string(nil), a.manifest.ImportPaths...),
		SavedRequests: savedRequestSummaries(a.savedRequests),
		BackupPaths:   append([]string(nil), a.backupPaths...),
	}
}

func eventRetentionFromSettings(settings *settingsFile) endpoint.EventRetentionPolicy {
	if settings == nil || settings.EventRetention == nil {
		return endpoint.EventRetentionPolicy{}
	}

	return endpoint.EventRetentionPolicy{
		MaxEventsPerCall: settings.EventRetention.MaxEventsPerCall,
		MaxBytesPerCall:  int64(settings.EventRetention.MaxBytesPerCall),
	}
}

func resolveWorkspacePath(path string) (string, string, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return "", "", errors.New("workspace path is required")
	}

	absolute, err := filepath.Abs(cleaned)
	if err != nil {
		return "", "", err
	}
	absolute = filepath.Clean(absolute)

	extension := strings.ToLower(filepath.Ext(absolute))
	if extension == ".yaml" || extension == ".yml" {
		return filepath.Dir(absolute), absolute, nil
	}

	return absolute, filepath.Join(absolute, manifestFileName), nil
}

func workspacePathIssue(err error) workspaceIssue {
	return workspaceIssue{
		field:    "path",
		code:     "workspace.path_required",
		category: contracts.ErrorCategoryWorkspace,
		message:  err.Error(),
	}
}

func decodeManifest(content []byte) (manifestFile, bool, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(content, &node); err != nil {
		return manifestFile{}, false, err
	}

	versionPresent := mappingNodeHasKey(&node, "version")
	var manifest manifestFile
	if err := node.Decode(&manifest); err != nil {
		return manifestFile{}, versionPresent, err
	}

	return manifest, versionPresent, nil
}

func mappingNodeHasKey(node *yaml.Node, key string) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return mappingNodeHasKey(node.Content[0], key)
	}
	if node.Kind != yaml.MappingNode {
		return false
	}

	for index := 0; index < len(node.Content)-1; index += 2 {
		if node.Content[index].Value == key {
			return true
		}
	}

	return false
}

func migrateManifest(manifest manifestFile, root string) manifestFile {
	migrated := manifest
	migrated.Version = CurrentManifestVersion
	if strings.TrimSpace(migrated.Name) == "" {
		migrated.Name = filepath.Base(root)
	}
	migrated.Endpoints = ensureEndpointFileIdentities(migrated.Endpoints)
	return migrated
}

func loadSavedRequests(root string, refs []string) ([]savedRequestFile, []workspaceIssue) {
	requests := make([]savedRequestFile, 0, len(refs))
	for index, ref := range refs {
		path, issue := workspaceRelativePath(root, ref, fmt.Sprintf("savedRequests[%d]", index))
		if issue != nil {
			return nil, []workspaceIssue{*issue}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil, []workspaceIssue{{
				field:    fmt.Sprintf("savedRequests[%d]", index),
				code:     "workspace.saved_request_missing",
				category: contracts.ErrorCategoryWorkspace,
				message:  "A saved request referenced by the workspace manifest could not be read.",
				path:     path,
			}}
		}

		var request savedRequestFile
		if err := yaml.Unmarshal(content, &request); err != nil {
			return nil, []workspaceIssue{{
				field:    fmt.Sprintf("savedRequests[%d]", index),
				code:     "workspace.saved_request_parse_failed",
				category: contracts.ErrorCategoryWorkspace,
				message:  "A saved request file is not valid YAML for the v1 schema.",
				path:     path,
			}}
		}
		request.path = filepath.ToSlash(filepath.Clean(ref))
		requests = append(requests, request)
	}

	return requests, nil
}

func validateManifest(root string, manifest manifestFile, savedRequests []savedRequestFile) []workspaceIssue {
	var issues []workspaceIssue
	if manifest.Version != CurrentManifestVersion {
		issues = append(issues, workspaceIssue{
			field:    "version",
			code:     "workspace.version_unsupported",
			category: contracts.ErrorCategoryWorkspace,
			message:  "Workspace manifest must use version 1.",
		})
	}
	if strings.TrimSpace(manifest.Name) == "" {
		issues = append(issues, workspaceIssue{
			field:    "name",
			code:     "validation.workspace_name_required",
			category: contracts.ErrorCategoryValidation,
			message:  "Workspace name is required.",
		})
	}

	endpointIDs := make(map[string]struct{}, len(manifest.Endpoints))
	for index, endpointFile := range manifest.Endpoints {
		endpointPreset := endpointFile.toContract()
		fieldPrefix := fmt.Sprintf("endpoints[%d]", index)
		if strings.TrimSpace(endpointPreset.ID) == "" {
			issues = append(issues, workspaceIssue{
				field:    fieldPrefix + ".id",
				code:     "validation.endpoint_id_required",
				category: contracts.ErrorCategoryValidation,
				message:  "Workspace endpoint id is required.",
			})
		}
		if _, exists := endpointIDs[endpointPreset.ID]; exists {
			issues = append(issues, workspaceIssue{
				field:    fieldPrefix + ".id",
				code:     "validation.workspace_endpoint_id_duplicate",
				category: contracts.ErrorCategoryValidation,
				message:  "Workspace endpoint ids must be unique.",
			})
		}
		endpointIDs[endpointPreset.ID] = struct{}{}

		for _, endpointIssue := range endpoint.ValidateEndpointPreset(endpointPreset) {
			issues = append(issues, workspaceIssue{
				field:    fieldPrefix + "." + endpointIssue.Field,
				code:     endpointIssue.Code,
				category: contracts.ErrorCategoryValidation,
				message:  endpointIssue.Message,
			})
		}
		issues = append(issues, validateEndpointSecretRefs(fieldPrefix, endpointPreset)...)
		issues = append(issues, validateMetadataSecretRefs(fieldPrefix+".metadataDefaults", endpointPreset.MetadataDefaults)...)
	}

	for index, source := range manifest.ProtoSources {
		fieldPrefix := fmt.Sprintf("protoSources[%d]", index)
		switch source.Type {
		case contracts.ProtoSourceTypeDirectory, contracts.ProtoSourceTypeFile:
		default:
			issues = append(issues, workspaceIssue{
				field:    fieldPrefix + ".type",
				code:     "validation.proto_source_type_invalid",
				category: contracts.ErrorCategoryValidation,
				message:  "Proto source type must be directory or file.",
			})
		}
		if strings.TrimSpace(source.Path) == "" {
			issues = append(issues, workspaceIssue{
				field:    fieldPrefix + ".path",
				code:     "validation.proto_source_path_required",
				category: contracts.ErrorCategoryValidation,
				message:  "Proto source path is required.",
			})
		}
	}

	for index, importPath := range manifest.ImportPaths {
		if strings.TrimSpace(importPath) == "" {
			issues = append(issues, workspaceIssue{
				field:    fmt.Sprintf("importPaths[%d]", index),
				code:     "validation.import_path_required",
				category: contracts.ErrorCategoryValidation,
				message:  "Import paths must not contain empty entries.",
			})
		}
	}

	for index, ref := range manifest.SavedRequests {
		if _, issue := workspaceRelativePath(root, ref, fmt.Sprintf("savedRequests[%d]", index)); issue != nil {
			issues = append(issues, *issue)
		}
	}
	for index, request := range savedRequests {
		issues = append(issues, validateSavedRequest(index, request, endpointIDs)...)
	}

	return issues
}

func validateEndpointSecretRefs(fieldPrefix string, endpointPreset contracts.EndpointPreset) []workspaceIssue {
	var issues []workspaceIssue
	secretFields := []struct {
		field string
		value string
	}{
		{field: "tls.caCert", value: endpointPreset.TLS.CACert},
		{field: "tls.clientCert", value: endpointPreset.TLS.ClientCert},
		{field: "tls.clientKey", value: endpointPreset.TLS.ClientKey},
	}

	for _, secretField := range secretFields {
		if strings.TrimSpace(secretField.value) == "" {
			continue
		}
		if _, err := endpoint.ParseSecretRef(secretField.value); err != nil {
			issues = append(issues, secretRefIssueFromError(fieldPrefix+"."+secretField.field, err))
		}
	}

	return issues
}

func secretRefIssueFromError(field string, err error) workspaceIssue {
	if classified, ok := err.(*endpoint.ClassifiedError); ok {
		return workspaceIssue{
			field:    field,
			code:     classified.Envelope.Code,
			category: classified.Envelope.Category,
			message:  classified.Envelope.Message,
		}
	}

	return workspaceIssue{
		field:    field,
		code:     "validation.secret_ref_malformed",
		category: contracts.ErrorCategoryValidation,
		message:  "Secret reference must use the secret-ref:<backend>/<namespace>/<key> format.",
	}
}

func validateSavedRequest(index int, request savedRequestFile, endpointIDs map[string]struct{}) []workspaceIssue {
	fieldPrefix := fmt.Sprintf("savedRequests[%d]", index)
	if request.path != "" {
		fieldPrefix = request.path
	}

	var issues []workspaceIssue
	if strings.TrimSpace(request.ID) == "" {
		issues = append(issues, workspaceIssue{
			field:    fieldPrefix + ".id",
			code:     "validation.saved_request_id_required",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request id is required.",
			path:     request.path,
		})
	}
	if strings.TrimSpace(request.Method) == "" {
		issues = append(issues, workspaceIssue{
			field:    fieldPrefix + ".method",
			code:     "validation.saved_request_method_required",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request method is required.",
			path:     request.path,
		})
	}
	if request.RPCType != contracts.RPCTypeUnary &&
		request.RPCType != contracts.RPCTypeServerStream &&
		request.RPCType != contracts.RPCTypeClientStream &&
		request.RPCType != contracts.RPCTypeBidiStream {
		issues = append(issues, workspaceIssue{
			field:    fieldPrefix + ".rpcType",
			code:     "validation.saved_request_rpc_type_invalid",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request rpcType must be unary, server_stream, client_stream or bidi_stream.",
			path:     request.path,
		})
	}
	if strings.TrimSpace(request.EndpointRef) == "" {
		issues = append(issues, workspaceIssue{
			field:    fieldPrefix + ".endpointRef",
			code:     "validation.saved_request_endpoint_ref_required",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request endpointRef is required.",
			path:     request.path,
		})
	} else if _, exists := endpointIDs[request.EndpointRef]; !exists {
		issues = append(issues, workspaceIssue{
			field:    fieldPrefix + ".endpointRef",
			code:     "validation.saved_request_endpoint_ref_invalid",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request endpointRef must point to an endpoint in the workspace manifest.",
			path:     request.path,
		})
	}
	if strings.TrimSpace(request.EnvironmentRef) != "" {
		issues = append(issues, workspaceIssue{
			field:    fieldPrefix + ".environmentRef",
			code:     "validation.saved_request_environment_ref_invalid",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request environmentRef is not supported until workspace environments land.",
			path:     request.path,
		})
	}
	if request.CallOptions.RequestTimeoutMs < 0 {
		issues = append(issues, workspaceIssue{
			field:    fieldPrefix + ".callOptions.requestTimeoutMs",
			code:     "validation.saved_request_request_timeout_invalid",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request timeout cannot be negative.",
			path:     request.path,
		})
	}
	if request.CallOptions.StreamIdleTimeoutMs < 0 {
		issues = append(issues, workspaceIssue{
			field:    fieldPrefix + ".callOptions.streamIdleTimeoutMs",
			code:     "validation.saved_request_stream_idle_timeout_invalid",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request stream idle timeout cannot be negative.",
			path:     request.path,
		})
	}
	if strings.TrimSpace(request.RequestSpec.Mode) == "" {
		issues = append(issues, workspaceIssue{
			field:    fieldPrefix + ".requestSpec.mode",
			code:     "validation.saved_request_spec_mode_required",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request requestSpec.mode is required.",
			path:     request.path,
		})
	}
	issues = append(issues, validateMetadataSecretRefs(fieldPrefix+".metadataTemplate", request.MetadataTemplate)...)

	return issues
}

func validateMetadataSecretRefs(fieldPrefix string, metadata map[string]string) []workspaceIssue {
	var issues []workspaceIssue
	for key, value := range metadata {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		field := fieldPrefix + "." + key
		if strings.HasPrefix(trimmedValue, "secret-ref:") {
			if _, err := endpoint.ParseSecretRef(trimmedValue); err != nil {
				issues = append(issues, secretRefIssueFromError(field, err))
			}
			continue
		}
		if strings.Contains(trimmedValue, "{{") {
			continue
		}
		if isSensitiveMetadataKey(key) {
			issues = append(issues, workspaceIssue{
				field:    field,
				code:     "validation.workspace_metadata_secret_inline",
				category: contracts.ErrorCategoryValidation,
				message:  "Sensitive metadata values must use a template or secret-ref and must not be stored inline in workspace files.",
			})
		}
	}
	return issues
}

func isSensitiveMetadataKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key", "api-key", "apikey":
		return true
	}

	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "credential") ||
		strings.Contains(normalized, "password")
}

func requestFileFromInput(input contracts.RequestSaveInput) (savedRequestFile, *workspaceIssue) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return savedRequestFile{}, &workspaceIssue{
			field:    "id",
			code:     "validation.saved_request_id_required",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request id is required.",
		}
	}
	if strings.TrimSpace(input.RequestSpec.Mode) == "" {
		return savedRequestFile{}, &workspaceIssue{
			field:    "requestSpec.mode",
			code:     "validation.saved_request_spec_mode_required",
			category: contracts.ErrorCategoryValidation,
			message:  "Saved request requestSpec.mode is required.",
		}
	}

	return savedRequestFile{
		ID:               id,
		Method:           strings.TrimSpace(input.Method),
		RPCType:          input.RPCType,
		EndpointRef:      strings.TrimSpace(input.EndpointRef),
		EnvironmentRef:   strings.TrimSpace(input.EnvironmentRef),
		MetadataTemplate: copyStringMap(input.MetadataTemplate),
		CallOptions: callOptionsFile{
			RequestTimeoutMs:    input.CallOptions.RequestTimeoutMs,
			StreamIdleTimeoutMs: input.CallOptions.StreamIdleTimeoutMs,
		},
		RequestSpec: savedRequestSpecFile{
			Mode:     strings.TrimSpace(input.RequestSpec.Mode),
			Body:     input.RequestSpec.Body,
			Messages: append([]contracts.StreamMessage(nil), input.RequestSpec.Messages...),
		},
	}, nil
}

func workspaceRelativePath(root, relativePath, field string) (string, *workspaceIssue) {
	trimmed := strings.TrimSpace(relativePath)
	if trimmed == "" {
		return "", &workspaceIssue{
			field:    field,
			code:     "workspace.saved_request_ref_invalid",
			category: contracts.ErrorCategoryWorkspace,
			message:  "Saved request references must not be empty.",
		}
	}
	if filepath.IsAbs(trimmed) {
		return "", &workspaceIssue{
			field:    field,
			code:     "workspace.saved_request_ref_invalid",
			category: contracts.ErrorCategoryWorkspace,
			message:  "Saved request references must be relative workspace paths.",
			path:     trimmed,
		}
	}

	cleaned := filepath.Clean(filepath.FromSlash(trimmed))
	if cleaned == "." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return "", &workspaceIssue{
			field:    field,
			code:     "workspace.saved_request_ref_invalid",
			category: contracts.ErrorCategoryWorkspace,
			message:  "Saved request references must stay inside the workspace directory.",
			path:     trimmed,
		}
	}

	return filepath.Join(root, cleaned), nil
}

func writeManifestFile(path string, manifest manifestFile) error {
	manifest.Version = CurrentManifestVersion
	return writeYAMLFile(path, manifest)
}

func writeYAMLFile(path string, value any) error {
	content, err := yaml.Marshal(value)
	if err != nil {
		return err
	}

	return os.WriteFile(path, content, 0o644)
}

func endpointsToFile(endpoints []contracts.EndpointPreset) []endpointFile {
	result := make([]endpointFile, 0, len(endpoints))
	for _, endpointPreset := range endpoints {
		result = append(result, endpointToFile(endpoint.EnsureEndpointIdentity(endpoint.NormalizeEndpointPreset(endpointPreset))))
	}
	return result
}

func endpointToFile(endpointPreset contracts.EndpointPreset) endpointFile {
	return endpointFile{
		ID:        strings.TrimSpace(endpointPreset.ID),
		Name:      strings.TrimSpace(endpointPreset.Name),
		Target:    strings.TrimSpace(endpointPreset.Target),
		Authority: strings.TrimSpace(endpointPreset.Authority),
		TLS: tlsFile{
			Mode:               endpointPreset.TLS.Mode,
			ServerNameOverride: strings.TrimSpace(endpointPreset.TLS.ServerNameOverride),
			CACert:             strings.TrimSpace(endpointPreset.TLS.CACert),
			ClientCert:         strings.TrimSpace(endpointPreset.TLS.ClientCert),
			ClientKey:          strings.TrimSpace(endpointPreset.TLS.ClientKey),
		},
		MetadataDefaults:    copyStringMap(endpointPreset.MetadataDefaults),
		ConnectTimeoutMs:    endpointPreset.ConnectTimeoutMs,
		RequestTimeoutMs:    endpointPreset.RequestTimeoutMs,
		StreamIdleTimeoutMs: endpointPreset.StreamIdleTimeoutMs,
	}
}

func endpointsFromFile(endpoints []endpointFile) []contracts.EndpointPreset {
	result := make([]contracts.EndpointPreset, 0, len(endpoints))
	for _, endpointFile := range endpoints {
		result = append(result, endpointFile.toContract())
	}
	return result
}

func (e endpointFile) toContract() contracts.EndpointPreset {
	return contracts.EndpointPreset{
		ID:        strings.TrimSpace(e.ID),
		Name:      strings.TrimSpace(e.Name),
		Target:    strings.TrimSpace(e.Target),
		Authority: strings.TrimSpace(e.Authority),
		TLS: contracts.EndpointTLSSettings{
			Mode:               e.TLS.Mode,
			ServerNameOverride: strings.TrimSpace(e.TLS.ServerNameOverride),
			CACert:             strings.TrimSpace(e.TLS.CACert),
			ClientCert:         strings.TrimSpace(e.TLS.ClientCert),
			ClientKey:          strings.TrimSpace(e.TLS.ClientKey),
		},
		MetadataDefaults:    copyStringMap(e.MetadataDefaults),
		ConnectTimeoutMs:    e.ConnectTimeoutMs,
		RequestTimeoutMs:    e.RequestTimeoutMs,
		StreamIdleTimeoutMs: e.StreamIdleTimeoutMs,
	}
}

func ensureEndpointFileIdentities(endpoints []endpointFile) []endpointFile {
	result := make([]endpointFile, 0, len(endpoints))
	for _, fileEndpoint := range endpoints {
		result = append(result, endpointToFile(endpoint.EnsureEndpointIdentity(endpoint.NormalizeEndpointPreset(fileEndpoint.toContract()))))
	}
	return result
}

func upsertEndpointFile(endpoints []endpointFile, next endpointFile) []endpointFile {
	for index, existing := range endpoints {
		if existing.ID == next.ID {
			endpoints[index] = next
			return endpoints
		}
	}

	return append(endpoints, next)
}

func protoSourcesToFile(protoSources []contracts.ProtoSource) []protoSourceFile {
	result := make([]protoSourceFile, 0, len(protoSources))
	for _, source := range protoSources {
		result = append(result, protoSourceFile{
			Type: source.Type,
			Path: strings.TrimSpace(source.Path),
		})
	}
	return result
}

func protoSourcesFromFile(protoSources []protoSourceFile) []contracts.ProtoSource {
	result := make([]contracts.ProtoSource, 0, len(protoSources))
	for _, source := range protoSources {
		result = append(result, contracts.ProtoSource{
			Type: source.Type,
			Path: source.Path,
		})
	}
	return result
}

func savedRequestSummaries(requests []savedRequestFile) []contracts.WorkspaceSavedRequestSummary {
	result := make([]contracts.WorkspaceSavedRequestSummary, 0, len(requests))
	for _, request := range requests {
		result = append(result, request.summary())
	}
	return result
}

func (r savedRequestFile) summary() contracts.WorkspaceSavedRequestSummary {
	return contracts.WorkspaceSavedRequestSummary{
		ID:          r.ID,
		Path:        r.path,
		Method:      r.Method,
		RPCType:     r.RPCType,
		EndpointRef: r.EndpointRef,
	}
}

func replaceSavedRequest(requests []savedRequestFile, next savedRequestFile) []savedRequestFile {
	for index, existing := range requests {
		if existing.ID == next.ID || existing.path == next.path {
			requests[index] = next
			return requests
		}
	}

	return append(requests, next)
}

func requestRelativePath(id string) string {
	return filepath.ToSlash(filepath.Join("requests", sanitizeFileStem(id)+".yaml"))
}

func sanitizeFileStem(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		case unicode.IsSpace(r) || r == '/':
			builder.WriteRune('-')
		default:
			builder.WriteRune('-')
		}
	}

	result := strings.Trim(builder.String(), "-.")
	if result == "" {
		return "request"
	}
	return result
}

func workspaceID(manifestPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(manifestPath)))
	return "ws_" + hex.EncodeToString(sum[:6])
}

func cleanStringList(values []string) []string {
	if values == nil {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			result = append(result, "")
			continue
		}
		result = append(result, strings.TrimSpace(value))
	}
	return result
}

func addUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func workspaceSuccess(snapshot contracts.WorkspaceSnapshot) contracts.WorkspaceResponse {
	return contracts.WorkspaceResponse{
		Ok: true,
		Data: &contracts.WorkspaceResult{
			Workspace: snapshot,
		},
	}
}

func workspaceFailure(issue workspaceIssue) contracts.WorkspaceResponse {
	return contracts.WorkspaceResponse{
		Ok:    false,
		Error: errorEnvelopeFromIssue(issue),
	}
}

func requestFailure(issue workspaceIssue) contracts.RequestSaveResponse {
	return contracts.RequestSaveResponse{
		Ok:    false,
		Error: errorEnvelopeFromIssue(issue),
	}
}

func errorEnvelopeFromIssue(issue workspaceIssue) *contracts.ErrorEnvelope {
	details := map[string]string{
		"field": issue.field,
	}
	if issue.path != "" {
		details["path"] = issue.path
	}

	return &contracts.ErrorEnvelope{
		Code:     issue.code,
		Category: issue.category,
		Message:  issue.message,
		Details:  details,
	}
}

func issuesToContract(issues []workspaceIssue) []contracts.WorkspaceValidationIssue {
	result := make([]contracts.WorkspaceValidationIssue, 0, len(issues))
	for _, issue := range issues {
		result = append(result, contracts.WorkspaceValidationIssue{
			Field:    issue.field,
			Code:     issue.code,
			Category: issue.category,
			Message:  issue.message,
			Path:     issue.path,
		})
	}
	return result
}

func noActiveWorkspaceIssue() workspaceIssue {
	return workspaceIssue{
		field:    "workspace",
		code:     "workspace.not_open",
		category: contracts.ErrorCategoryWorkspace,
		message:  "Open or create a workspace before saving workspace data.",
	}
}
