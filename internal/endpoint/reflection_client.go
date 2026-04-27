package endpoint

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/grpc/codes"
	v1reflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1"
	v1alphareflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"tether/internal/contracts"
)

type serverReflectionClient struct{}

func newServerReflectionClient() ReflectionClient {
	return &serverReflectionClient{}
}

func (c *serverReflectionClient) LoadCatalog(ctx context.Context, conn GRPCClientConn, endpointPreset contracts.EndpointPreset) (ReflectionCatalog, *endpointDiagnostic) {
	catalog, err := c.loadCatalogV1(ctx, conn)
	if err == nil {
		return catalog, successReflectionDiagnostic(endpointPreset, catalog)
	}

	if isReflectionUnavailableStatus(err) {
		catalog, alphaErr := c.loadCatalogV1Alpha(ctx, conn)
		if alphaErr == nil {
			return catalog, successReflectionDiagnostic(endpointPreset, catalog)
		}

		return ReflectionCatalog{}, classifyReflectionError(endpointPreset, alphaErr)
	}

	return ReflectionCatalog{}, classifyReflectionError(endpointPreset, err)
}

func (c *serverReflectionClient) loadCatalogV1(ctx context.Context, conn GRPCClientConn) (ReflectionCatalog, error) {
	client := v1reflectiongrpc.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return ReflectionCatalog{}, err
	}
	defer func() {
		_ = stream.CloseSend()
	}()

	serviceNames, rawFiles, err := loadReflectionPayload(
		sendV1ListServices(stream),
		func(symbol string) error { return sendV1FileContainingSymbol(stream, symbol) },
		func() ([]string, [][]byte, error) { return recvV1(stream) },
	)
	if err != nil {
		return ReflectionCatalog{}, err
	}

	return buildReflectionCatalog(serviceNames, rawFiles)
}

func (c *serverReflectionClient) loadCatalogV1Alpha(ctx context.Context, conn GRPCClientConn) (ReflectionCatalog, error) {
	client := v1alphareflectiongrpc.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return ReflectionCatalog{}, err
	}
	defer func() {
		_ = stream.CloseSend()
	}()

	serviceNames, rawFiles, err := loadReflectionPayload(
		sendV1AlphaListServices(stream),
		func(symbol string) error { return sendV1AlphaFileContainingSymbol(stream, symbol) },
		func() ([]string, [][]byte, error) { return recvV1Alpha(stream) },
	)
	if err != nil {
		return ReflectionCatalog{}, err
	}

	return buildReflectionCatalog(serviceNames, rawFiles)
}

func loadReflectionPayload(
	sendListServices func() error,
	sendFileContainingSymbol func(symbol string) error,
	recv func() ([]string, [][]byte, error),
) ([]string, map[string]*descriptorpb.FileDescriptorProto, error) {
	if err := sendListServices(); err != nil {
		return nil, nil, err
	}

	serviceNames, _, err := recv()
	if err != nil {
		return nil, nil, err
	}

	filteredServices := make([]string, 0, len(serviceNames))
	rawFiles := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, serviceName := range serviceNames {
		if strings.HasPrefix(serviceName, "grpc.reflection.") {
			continue
		}

		filteredServices = append(filteredServices, serviceName)
		if err := sendFileContainingSymbol(serviceName); err != nil {
			return nil, nil, err
		}

		_, descriptorBytes, recvErr := recv()
		if recvErr != nil {
			return nil, nil, recvErr
		}

		for _, raw := range descriptorBytes {
			fileProto := &descriptorpb.FileDescriptorProto{}
			if err := proto.Unmarshal(raw, fileProto); err != nil {
				return nil, nil, fmt.Errorf("unmarshal reflected descriptor: %w", err)
			}
			rawFiles[fileProto.GetName()] = fileProto
		}
	}

	sort.Strings(filteredServices)
	return filteredServices, rawFiles, nil
}

func buildReflectionCatalog(serviceNames []string, rawFiles map[string]*descriptorpb.FileDescriptorProto) (ReflectionCatalog, error) {
	files := new(protoregistry.Files)
	resolver := chainedResolver{
		primary:  files,
		fallback: protoregistry.GlobalFiles,
	}

	remaining := make(map[string]*descriptorpb.FileDescriptorProto, len(rawFiles))
	for name, fileProto := range rawFiles {
		remaining[name] = fileProto
	}

	for len(remaining) > 0 {
		progressed := false
		for name, fileProto := range remaining {
			fileDescriptor, err := protodesc.NewFile(fileProto, resolver)
			if err != nil {
				continue
			}

			if err := files.RegisterFile(fileDescriptor); err != nil {
				return ReflectionCatalog{}, fmt.Errorf("register reflected descriptor %s: %w", name, err)
			}

			delete(remaining, name)
			progressed = true
		}

		if !progressed {
			return ReflectionCatalog{}, fmt.Errorf("incomplete descriptor graph")
		}
	}

	services := make([]contracts.CatalogService, 0, len(serviceNames))
	wellKnownRefs := map[string]contracts.CatalogMessageRef{}
	for _, serviceName := range serviceNames {
		descriptor, err := resolver.FindDescriptorByName(protoreflect.FullName(serviceName))
		if err != nil {
			return ReflectionCatalog{}, fmt.Errorf("resolve reflected service %s: %w", serviceName, err)
		}

		serviceDescriptor, ok := descriptor.(protoreflect.ServiceDescriptor)
		if !ok {
			return ReflectionCatalog{}, fmt.Errorf("descriptor %s is not a service", serviceName)
		}

		methods := make([]contracts.CatalogMethod, 0, serviceDescriptor.Methods().Len())
		for index := 0; index < serviceDescriptor.Methods().Len(); index++ {
			methodDescriptor := serviceDescriptor.Methods().Get(index)
			requestType := buildCatalogMessageRef(methodDescriptor.Input())
			responseType := buildCatalogMessageRef(methodDescriptor.Output())

			methods = append(methods, contracts.CatalogMethod{
				Name:         string(methodDescriptor.Name()),
				FullName:     string(methodDescriptor.FullName()),
				RPCType:      rpcTypeForMethod(methodDescriptor),
				RequestType:  requestType,
				ResponseType: responseType,
			})

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

	return ReflectionCatalog{
		Services:       services,
		WellKnownTypes: wellKnownTypes,
	}, nil
}

func sendV1ListServices(stream v1reflectiongrpc.ServerReflection_ServerReflectionInfoClient) func() error {
	return func() error {
		return stream.Send(&v1reflectiongrpc.ServerReflectionRequest{
			MessageRequest: &v1reflectiongrpc.ServerReflectionRequest_ListServices{
				ListServices: "*",
			},
		})
	}
}

func sendV1FileContainingSymbol(stream v1reflectiongrpc.ServerReflection_ServerReflectionInfoClient, symbol string) error {
	return stream.Send(&v1reflectiongrpc.ServerReflectionRequest{
		MessageRequest: &v1reflectiongrpc.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: symbol,
		},
	})
}

func recvV1(stream v1reflectiongrpc.ServerReflection_ServerReflectionInfoClient) ([]string, [][]byte, error) {
	response, err := stream.Recv()
	if err != nil {
		return nil, nil, err
	}

	if errorResponse := response.GetErrorResponse(); errorResponse != nil {
		return nil, nil, status.Error(codes.Code(errorResponse.GetErrorCode()), errorResponse.GetErrorMessage())
	}

	if listResponse := response.GetListServicesResponse(); listResponse != nil {
		serviceNames := make([]string, 0, len(listResponse.GetService()))
		for _, service := range listResponse.GetService() {
			serviceNames = append(serviceNames, service.GetName())
		}
		return serviceNames, nil, nil
	}

	if fileResponse := response.GetFileDescriptorResponse(); fileResponse != nil {
		return nil, fileResponse.GetFileDescriptorProto(), nil
	}

	return nil, nil, fmt.Errorf("unexpected v1 reflection response")
}

func sendV1AlphaListServices(stream v1alphareflectiongrpc.ServerReflection_ServerReflectionInfoClient) func() error {
	return func() error {
		return stream.Send(&v1alphareflectiongrpc.ServerReflectionRequest{
			MessageRequest: &v1alphareflectiongrpc.ServerReflectionRequest_ListServices{
				ListServices: "*",
			},
		})
	}
}

func sendV1AlphaFileContainingSymbol(stream v1alphareflectiongrpc.ServerReflection_ServerReflectionInfoClient, symbol string) error {
	return stream.Send(&v1alphareflectiongrpc.ServerReflectionRequest{
		MessageRequest: &v1alphareflectiongrpc.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: symbol,
		},
	})
}

func recvV1Alpha(stream v1alphareflectiongrpc.ServerReflection_ServerReflectionInfoClient) ([]string, [][]byte, error) {
	response, err := stream.Recv()
	if err != nil {
		return nil, nil, err
	}

	if errorResponse := response.GetErrorResponse(); errorResponse != nil {
		return nil, nil, status.Error(codes.Code(errorResponse.GetErrorCode()), errorResponse.GetErrorMessage())
	}

	if listResponse := response.GetListServicesResponse(); listResponse != nil {
		serviceNames := make([]string, 0, len(listResponse.GetService()))
		for _, service := range listResponse.GetService() {
			serviceNames = append(serviceNames, service.GetName())
		}
		return serviceNames, nil, nil
	}

	if fileResponse := response.GetFileDescriptorResponse(); fileResponse != nil {
		return nil, fileResponse.GetFileDescriptorProto(), nil
	}

	return nil, nil, fmt.Errorf("unexpected v1alpha reflection response")
}

func buildCatalogMessageRef(messageDescriptor protoreflect.MessageDescriptor) contracts.CatalogMessageRef {
	return contracts.CatalogMessageRef{
		Name:        string(messageDescriptor.Name()),
		FullName:    string(messageDescriptor.FullName()),
		IsWellKnown: isWellKnownDescriptor(messageDescriptor.FullName()),
	}
}

func rpcTypeForMethod(methodDescriptor protoreflect.MethodDescriptor) contracts.RPCType {
	switch {
	case methodDescriptor.IsStreamingClient() && methodDescriptor.IsStreamingServer():
		return contracts.RPCTypeBidiStream
	case methodDescriptor.IsStreamingClient():
		return contracts.RPCTypeClientStream
	case methodDescriptor.IsStreamingServer():
		return contracts.RPCTypeServerStream
	default:
		return contracts.RPCTypeUnary
	}
}

func collectWellKnownTypes(messageDescriptor protoreflect.MessageDescriptor, refs map[string]contracts.CatalogMessageRef, visited map[protoreflect.FullName]struct{}) {
	if messageDescriptor == nil {
		return
	}

	if _, seen := visited[messageDescriptor.FullName()]; seen {
		return
	}
	visited[messageDescriptor.FullName()] = struct{}{}

	if isWellKnownDescriptor(messageDescriptor.FullName()) {
		key := string(messageDescriptor.FullName())
		refs[key] = buildCatalogMessageRef(messageDescriptor)
	}

	fields := messageDescriptor.Fields()
	for index := 0; index < fields.Len(); index++ {
		fieldDescriptor := fields.Get(index)
		if fieldDescriptor.Message() != nil {
			collectWellKnownTypes(fieldDescriptor.Message(), refs, visited)
		}
	}
}

func isWellKnownDescriptor(fullName protoreflect.FullName) bool {
	return strings.HasPrefix(string(fullName), "google.protobuf.")
}

func successReflectionDiagnostic(endpointPreset contracts.EndpointPreset, catalog ReflectionCatalog) *endpointDiagnostic {
	return &endpointDiagnostic{
		Level:    "info",
		Code:     "reflection.catalog_loaded",
		Category: contracts.ErrorCategoryReflection,
		Message:  "Reflection loaded the service and method catalog for this endpoint.",
		NextStep: "Pick a method from the catalog and continue with request authoring or invocation.",
		Details: map[string]string{
			"target":          endpointPreset.Target,
			"serviceCount":    fmt.Sprintf("%d", len(catalog.Services)),
			"wellKnownCount":  fmt.Sprintf("%d", len(catalog.WellKnownTypes)),
			"reflectionLayer": "grpc_server_reflection",
		},
	}
}

func classifyReflectionError(endpointPreset contracts.EndpointPreset, err error) *endpointDiagnostic {
	details := map[string]string{
		"target": endpointPreset.Target,
		"cause":  err.Error(),
	}

	statusCode := status.Code(err)
	switch {
	case statusCode == codes.Unimplemented || statusCode == codes.NotFound:
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "reflection.unavailable",
			Category: contracts.ErrorCategoryReflection,
			Message:  "The endpoint is reachable, but it does not expose gRPC Server Reflection.",
			NextStep: "Use proto import for this service or enable and route the reflection service on the server.",
			Details:  details,
		}
	case statusCode == codes.PermissionDenied || statusCode == codes.Unauthenticated:
		return &endpointDiagnostic{
			Level:    "error",
			Code:     "reflection.permission_denied",
			Category: contracts.ErrorCategoryReflection,
			Message:  "The endpoint denied access to gRPC Server Reflection.",
			NextStep: "Check endpoint metadata, auth requirements and whether reflection is intentionally restricted on this environment.",
			Details:  details,
		}
	default:
		if strings.Contains(err.Error(), "incomplete descriptor graph") || strings.Contains(err.Error(), "resolve reflected service") {
			return &endpointDiagnostic{
				Level:    "error",
				Code:     "reflection.incomplete_descriptors",
				Category: contracts.ErrorCategoryReflection,
				Message:  "Reflection returned an incomplete descriptor graph for this endpoint.",
				NextStep: "Retry against a fully configured reflection server or fall back to importing the proto sources directly.",
				Details:  details,
			}
		}

		return &endpointDiagnostic{
			Level:    "error",
			Code:     "reflection.query_failed",
			Category: contracts.ErrorCategoryReflection,
			Message:  "The reflection request failed after the endpoint connection succeeded.",
			NextStep: "Inspect server-side reflection support, routing and auth requirements, or switch to proto import if reflection is intentionally unavailable.",
			Details:  details,
		}
	}
}

func isReflectionUnavailableStatus(err error) bool {
	return status.Code(err) == codes.Unimplemented || status.Code(err) == codes.NotFound
}

type chainedResolver struct {
	primary  protodesc.Resolver
	fallback protodesc.Resolver
}

func (r chainedResolver) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	if r.primary != nil {
		fileDescriptor, err := r.primary.FindFileByPath(path)
		if err == nil {
			return fileDescriptor, nil
		}
		if !errors.Is(err, protoregistry.NotFound) {
			return nil, err
		}
	}

	return r.fallback.FindFileByPath(path)
}

func (r chainedResolver) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	if r.primary != nil {
		descriptor, err := r.primary.FindDescriptorByName(name)
		if err == nil {
			return descriptor, nil
		}
		if !errors.Is(err, protoregistry.NotFound) {
			return nil, err
		}
	}

	return r.fallback.FindDescriptorByName(name)
}
