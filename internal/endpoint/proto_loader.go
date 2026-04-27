package endpoint

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/reporter"
	"google.golang.org/protobuf/reflect/protoreflect"

	"tether/internal/contracts"
)

type ProtoLoaderInput struct {
	ProtoSources []contracts.ProtoSource
	ImportPaths  []string
}

type protoLoader struct{}

type protoCompilePlan struct {
	entryNames  []string
	importRoots []string
	sourceCount int
}

type protoImportNotFoundError struct {
	Import      string
	ImportRoots []string
}

func (e *protoImportNotFoundError) Error() string {
	if len(e.ImportRoots) == 0 {
		return fmt.Sprintf("proto import %s was not found", e.Import)
	}

	return fmt.Sprintf("proto import %s was not found in %s", e.Import, strings.Join(e.ImportRoots, ", "))
}

func (e *protoImportNotFoundError) Unwrap() error {
	return fs.ErrNotExist
}

type protoSourceResolver struct {
	importRoots []string
}

type protoCompileReporter struct {
	first reporter.ErrorWithPos
}

func newProtoLoader() ProtoLoader {
	return &protoLoader{}
}

func (r *protoSourceResolver) FindFileByPath(path string) (protocompile.SearchResult, error) {
	if len(r.importRoots) == 0 {
		reader, err := os.Open(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return protocompile.SearchResult{}, &protoImportNotFoundError{Import: filepath.ToSlash(path)}
			}
			return protocompile.SearchResult{}, err
		}
		return protocompile.SearchResult{Source: reader}, nil
	}

	for _, root := range r.importRoots {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		reader, err := os.Open(fullPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return protocompile.SearchResult{}, err
		}
		return protocompile.SearchResult{Source: reader}, nil
	}

	return protocompile.SearchResult{}, &protoImportNotFoundError{
		Import:      filepath.ToSlash(path),
		ImportRoots: append([]string(nil), r.importRoots...),
	}
}

func (r *protoCompileReporter) Error(err reporter.ErrorWithPos) error {
	if r.first == nil {
		r.first = err
	}

	return err
}

func (r *protoCompileReporter) Warning(reporter.ErrorWithPos) {}

func (l *protoLoader) LoadCatalog(ctx context.Context, input ProtoLoaderInput) (MethodCatalog, *endpointDiagnostic) {
	plan, diagnostic := buildProtoCompilePlan(input)
	if diagnostic != nil {
		return MethodCatalog{}, diagnostic
	}

	reporter := &protoCompileReporter{}
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protoSourceResolver{
			importRoots: plan.importRoots,
		}),
		Reporter: reporter,
	}

	files, err := compiler.Compile(ctx, plan.entryNames...)
	if err != nil {
		return MethodCatalog{}, classifyProtoCompileError(err, reporter.first)
	}

	catalog := buildMethodCatalogFromFiles(files)
	return catalog, successProtoDiagnostic(plan, catalog)
}

func buildProtoCompilePlan(input ProtoLoaderInput) (protoCompilePlan, *endpointDiagnostic) {
	sources := make([]contracts.ProtoSource, 0, len(input.ProtoSources))
	importRoots := make([]string, 0, len(input.ImportPaths)+len(input.ProtoSources))
	seenImportRoots := map[string]struct{}{}

	for _, importPath := range input.ImportPaths {
		normalizedPath := strings.TrimSpace(importPath)
		if normalizedPath == "" {
			continue
		}

		absolutePath, err := filepath.Abs(normalizedPath)
		if err != nil {
			return protoCompilePlan{}, invalidProtoPathDiagnostic("proto.import_path_invalid", normalizedPath, err)
		}

		info, statErr := os.Stat(absolutePath)
		if statErr != nil {
			return protoCompilePlan{}, missingProtoPathDiagnostic("proto.import_path_not_found", normalizedPath, statErr)
		}
		if !info.IsDir() {
			return protoCompilePlan{}, &endpointDiagnostic{
				Level:    "error",
				Code:     "proto.import_path_not_directory",
				Category: contracts.ErrorCategoryProto,
				Message:  "One of the configured proto import paths is not a directory.",
				NextStep: "Point each import path at a directory that contains the imported .proto files.",
				Details: map[string]string{
					"path": normalizedPath,
				},
			}
		}

		if _, seen := seenImportRoots[absolutePath]; !seen {
			importRoots = append(importRoots, absolutePath)
			seenImportRoots[absolutePath] = struct{}{}
		}
	}

	for _, source := range input.ProtoSources {
		normalizedPath := strings.TrimSpace(source.Path)
		if normalizedPath == "" {
			continue
		}

		if source.Type != contracts.ProtoSourceTypeDirectory && source.Type != contracts.ProtoSourceTypeFile {
			return protoCompilePlan{}, &endpointDiagnostic{
				Level:    "error",
				Code:     "validation.proto_source_type_invalid",
				Category: contracts.ErrorCategoryValidation,
				Message:  "Each proto source must be either a directory or a file.",
				NextStep: "Choose one of the supported proto source types before loading the catalog again.",
				Details: map[string]string{
					"path": normalizedPath,
					"type": string(source.Type),
				},
			}
		}

		absolutePath, err := filepath.Abs(normalizedPath)
		if err != nil {
			return protoCompilePlan{}, invalidProtoPathDiagnostic("proto.source_path_invalid", normalizedPath, err)
		}

		info, statErr := os.Stat(absolutePath)
		if statErr != nil {
			return protoCompilePlan{}, missingProtoPathDiagnostic("proto.source_not_found", normalizedPath, statErr)
		}

		if source.Type == contracts.ProtoSourceTypeDirectory {
			if !info.IsDir() {
				return protoCompilePlan{}, &endpointDiagnostic{
					Level:    "error",
					Code:     "proto.source_not_directory",
					Category: contracts.ErrorCategoryProto,
					Message:  "A proto directory source points at a file instead of a directory.",
					NextStep: "Change the proto source type to file or point it at a directory root.",
					Details: map[string]string{
						"path": normalizedPath,
					},
				}
			}

			if _, seen := seenImportRoots[absolutePath]; !seen {
				importRoots = append(importRoots, absolutePath)
				seenImportRoots[absolutePath] = struct{}{}
			}
		} else if info.IsDir() {
			return protoCompilePlan{}, &endpointDiagnostic{
				Level:    "error",
				Code:     "proto.source_not_file",
				Category: contracts.ErrorCategoryProto,
				Message:  "A proto file source points at a directory instead of a single .proto file.",
				NextStep: "Change the proto source type to directory or point it at a specific .proto file.",
				Details: map[string]string{
					"path": normalizedPath,
				},
			}
		}

		sources = append(sources, contracts.ProtoSource{
			Type: source.Type,
			Path: absolutePath,
		})
	}

	entryNames := make([]string, 0)
	seenEntries := map[string]struct{}{}
	for _, source := range sources {
		switch source.Type {
		case contracts.ProtoSourceTypeDirectory:
			directoryEntries, diagnostic := collectDirectoryProtoEntries(source.Path)
			if diagnostic != nil {
				return protoCompilePlan{}, diagnostic
			}

			for _, entryName := range directoryEntries {
				if _, seen := seenEntries[entryName]; seen {
					continue
				}
				entryNames = append(entryNames, entryName)
				seenEntries[entryName] = struct{}{}
			}
		case contracts.ProtoSourceTypeFile:
			entryName, fileImportRoot, diagnostic := buildFileProtoEntry(source.Path, importRoots)
			if diagnostic != nil {
				return protoCompilePlan{}, diagnostic
			}

			if fileImportRoot != "" {
				if _, seen := seenImportRoots[fileImportRoot]; !seen {
					importRoots = append(importRoots, fileImportRoot)
					seenImportRoots[fileImportRoot] = struct{}{}
				}
			}

			if _, seen := seenEntries[entryName]; seen {
				continue
			}
			entryNames = append(entryNames, entryName)
			seenEntries[entryName] = struct{}{}
		}
	}

	if len(entryNames) == 0 {
		return protoCompilePlan{}, &endpointDiagnostic{
			Level:    "error",
			Code:     "validation.proto_sources_required",
			Category: contracts.ErrorCategoryValidation,
			Message:  "Add at least one proto file or directory before loading the proto catalog.",
			NextStep: "Enter a proto directory or file path and then reload the catalog manually.",
		}
	}

	sort.Strings(entryNames)
	return protoCompilePlan{
		entryNames:  entryNames,
		importRoots: importRoots,
		sourceCount: len(sources),
	}, nil
}

func collectDirectoryProtoEntries(root string) ([]string, *endpointDiagnostic) {
	entries := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".proto") {
			relativePath, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			entries = append(entries, filepath.ToSlash(relativePath))
		}
		return nil
	})
	if err != nil {
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     "proto.source_walk_failed",
			Category: contracts.ErrorCategoryProto,
			Message:  "The runtime could not enumerate .proto files from the selected directory.",
			NextStep: "Check filesystem permissions for the proto directory and retry the manual reload.",
			Details: map[string]string{
				"path":  root,
				"cause": err.Error(),
			},
		}
	}

	if len(entries) == 0 {
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     "proto.no_proto_files",
			Category: contracts.ErrorCategoryProto,
			Message:  "The selected proto directory does not contain any .proto files.",
			NextStep: "Choose a directory that contains your service proto files and reload the catalog manually.",
			Details: map[string]string{
				"path": root,
			},
		}
	}

	sort.Strings(entries)
	return entries, nil
}

func buildFileProtoEntry(path string, importRoots []string) (string, string, *endpointDiagnostic) {
	chosenRoot := ""
	chosenDepth := -1
	for _, root := range importRoots {
		relativePath, err := filepath.Rel(root, path)
		if err != nil || relativePath == "." || strings.HasPrefix(relativePath, "..") {
			continue
		}

		depth := strings.Count(root, string(filepath.Separator))
		if depth > chosenDepth {
			chosenRoot = root
			chosenDepth = depth
		}
	}

	if chosenRoot == "" {
		chosenRoot = filepath.Dir(path)
	}

	relativePath, err := filepath.Rel(chosenRoot, path)
	if err != nil {
		return "", "", &endpointDiagnostic{
			Level:    "error",
			Code:     "proto.source_path_invalid",
			Category: contracts.ErrorCategoryProto,
			Message:  "The runtime could not normalize one of the selected proto file paths.",
			NextStep: "Re-enter the proto file path or reload the workspace before trying again.",
			Details: map[string]string{
				"path":  path,
				"cause": err.Error(),
			},
		}
	}

	return filepath.ToSlash(relativePath), chosenRoot, nil
}

func buildMethodCatalogFromFiles(files linker.Files) MethodCatalog {
	services := make([]contracts.CatalogService, 0)
	methodsByFullName := make(map[string]protoreflect.MethodDescriptor)
	wellKnownRefs := map[string]contracts.CatalogMessageRef{}

	for _, file := range files {
		serviceDescriptors := file.Services()
		for serviceIndex := 0; serviceIndex < serviceDescriptors.Len(); serviceIndex++ {
			serviceDescriptor := serviceDescriptors.Get(serviceIndex)
			methods := make([]contracts.CatalogMethod, 0, serviceDescriptor.Methods().Len())
			for methodIndex := 0; methodIndex < serviceDescriptor.Methods().Len(); methodIndex++ {
				methodDescriptor := serviceDescriptor.Methods().Get(methodIndex)
				catalogMethod := buildCatalogMethod(methodDescriptor)
				methods = append(methods, catalogMethod)
				methodsByFullName[catalogMethod.FullName] = methodDescriptor

				collectWellKnownTypes(methodDescriptor.Input(), wellKnownRefs, map[protoreflect.FullName]struct{}{})
				collectWellKnownTypes(methodDescriptor.Output(), wellKnownRefs, map[protoreflect.FullName]struct{}{})
			}

			sort.Slice(methods, func(i, j int) bool {
				return methods[i].Name < methods[j].Name
			})

			services = append(services, contracts.CatalogService{
				Name:     string(serviceDescriptor.Name()),
				FullName: string(serviceDescriptor.FullName()),
				Methods:  methods,
			})
		}
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].FullName < services[j].FullName
	})

	wellKnownTypes := make([]contracts.CatalogMessageRef, 0, len(wellKnownRefs))
	for _, messageRef := range wellKnownRefs {
		wellKnownTypes = append(wellKnownTypes, messageRef)
	}
	sort.Slice(wellKnownTypes, func(i, j int) bool {
		return wellKnownTypes[i].FullName < wellKnownTypes[j].FullName
	})

	return MethodCatalog{
		Services:       services,
		WellKnownTypes: wellKnownTypes,
		methods:        methodsByFullName,
	}
}

func successProtoDiagnostic(plan protoCompilePlan, catalog MethodCatalog) *endpointDiagnostic {
	return &endpointDiagnostic{
		Level:    "info",
		Code:     "proto.catalog_loaded",
		Category: contracts.ErrorCategoryProto,
		Message:  "Proto sources loaded the service and method catalog successfully.",
		NextStep: "Pick a method from the proto catalog and continue with request authoring or invocation.",
		Details: map[string]string{
			"entryCount":      fmt.Sprintf("%d", len(plan.entryNames)),
			"importPathCount": fmt.Sprintf("%d", len(plan.importRoots)),
			"serviceCount":    fmt.Sprintf("%d", len(catalog.Services)),
			"sourceCount":     fmt.Sprintf("%d", plan.sourceCount),
		},
	}
}

func classifyProtoCompileError(err error, compileError reporter.ErrorWithPos) *endpointDiagnostic {
	if compileError == nil {
		_ = errors.As(err, &compileError)
	}

	if compileError != nil {
		var missingImport *protoImportNotFoundError
		if errors.As(compileError.Unwrap(), &missingImport) {
			return &endpointDiagnostic{
				Level:    "error",
				Code:     "proto.missing_import",
				Category: contracts.ErrorCategoryProto,
				Message:  fmt.Sprintf("Import %s could not be resolved from the configured proto roots.", missingImport.Import),
				NextStep: "Add the directory that contains this imported proto file to the import paths and reload the proto catalog manually.",
				Details: map[string]string{
					"file":   filepath.ToSlash(compileError.GetPosition().Filename),
					"line":   fmt.Sprintf("%d", compileError.GetPosition().Line),
					"column": fmt.Sprintf("%d", compileError.GetPosition().Col),
					"import": missingImport.Import,
				},
			}
		}

		return &endpointDiagnostic{
			Level:    "error",
			Code:     "proto.parse_failed",
			Category: contracts.ErrorCategoryProto,
			Message:  "The proto sources could not be parsed and linked into a usable descriptor graph.",
			NextStep: "Fix the reported proto syntax or descriptor issue, then reload the proto catalog manually.",
			Details: map[string]string{
				"file":   filepath.ToSlash(compileError.GetPosition().Filename),
				"line":   fmt.Sprintf("%d", compileError.GetPosition().Line),
				"column": fmt.Sprintf("%d", compileError.GetPosition().Col),
				"cause":  compileError.Unwrap().Error(),
			},
		}
	}

	var missingImport *protoImportNotFoundError
	if errors.As(err, &missingImport) {
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "proto.missing_import",
			Category: contracts.ErrorCategoryProto,
			Message:  fmt.Sprintf("Import %s could not be resolved from the configured proto roots.", missingImport.Import),
			NextStep: "Add the missing import directory and reload the proto catalog manually.",
			Details: map[string]string{
				"import": missingImport.Import,
			},
		}
	}

	return &endpointDiagnostic{
		Level:    "error",
		Code:     "proto.load_failed",
		Category: contracts.ErrorCategoryProto,
		Message:  "The proto sources could not be compiled into a method catalog.",
		NextStep: "Inspect the proto diagnostics, fix the import or descriptor issue, and then reload the catalog manually.",
		Details: map[string]string{
			"cause": err.Error(),
		},
	}
}

func invalidProtoPathDiagnostic(code string, path string, err error) *endpointDiagnostic {
	return &endpointDiagnostic{
		Level:    "error",
		Code:     code,
		Category: contracts.ErrorCategoryProto,
		Message:  "One of the configured proto paths could not be normalized.",
		NextStep: "Check the proto path for invalid characters or workspace path issues and retry.",
		Details: map[string]string{
			"path":  path,
			"cause": err.Error(),
		},
	}
}

func missingProtoPathDiagnostic(code string, path string, err error) *endpointDiagnostic {
	return &endpointDiagnostic{
		Level:    "error",
		Code:     code,
		Category: contracts.ErrorCategoryProto,
		Message:  "One of the configured proto paths does not exist on disk.",
		NextStep: "Update the proto source or import path so it points at an existing file or directory, then reload manually.",
		Details: map[string]string{
			"path":  path,
			"cause": err.Error(),
		},
	}
}
