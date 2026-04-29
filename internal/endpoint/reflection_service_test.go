package endpoint

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpccredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	v1reflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1"
	v1alphareflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"tether/internal/contracts"
)

func TestCatalogLoadFromReflectionPlaintextSuccess(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
	defer stop()

	service := NewService(ServiceDependencies{})
	spy := &diagnosticEmitterSpy{}
	service.SetEmitter(spy)

	response := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !response.Ok || response.Data == nil {
		t.Fatalf("expected reflection catalog, got %+v", response.Error)
	}

	if len(response.Data.Services) != 1 {
		t.Fatalf("expected one reflected service, got %+v", response.Data.Services)
	}

	serviceCatalog := response.Data.Services[0]
	if serviceCatalog.FullName != "tether.demo.v1.ReflectionDemo" {
		t.Fatalf("unexpected service catalog: %+v", serviceCatalog)
	}

	if len(serviceCatalog.Methods) != 2 {
		t.Fatalf("expected two reflected methods, got %+v", serviceCatalog.Methods)
	}

	if serviceCatalog.Methods[0].RPCType != contracts.RPCTypeUnary {
		t.Fatalf("expected unary method metadata, got %+v", serviceCatalog.Methods[0])
	}

	if serviceCatalog.Methods[1].RPCType != contracts.RPCTypeServerStream {
		t.Fatalf("expected server-stream metadata, got %+v", serviceCatalog.Methods[1])
	}

	assertWellKnownTypes(t, response.Data.WellKnownTypes, []string{
		"google.protobuf.Empty",
		"google.protobuf.Struct",
		"google.protobuf.Timestamp",
	})

	if response.Data.Diagnostic == nil || response.Data.Diagnostic.Code != "reflection.catalog_loaded" {
		t.Fatalf("expected reflection success diagnostic, got %+v", response.Data.Diagnostic)
	}

	if spy.eventName != contracts.EventDiagnosticsUpdate {
		t.Fatalf("expected diagnostics:update event, got %q", spy.eventName)
	}
}

func TestCatalogLoadFromReflectionUnavailableProducesReflectionDiagnostic(t *testing.T) {
	t.Parallel()

	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		disableReflection: true,
	})
	defer stop()

	service := NewService(ServiceDependencies{})
	spy := &diagnosticEmitterSpy{}
	service.SetEmitter(spy)

	response := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if response.Ok {
		t.Fatalf("expected reflection load to fail when reflection is disabled")
	}

	if response.Error == nil || response.Error.Code != "reflection.unavailable" || response.Error.Category != contracts.ErrorCategoryReflection {
		t.Fatalf("expected reflection.unavailable error, got %+v", response.Error)
	}

	if spy.payload == nil {
		t.Fatalf("expected emitted diagnostics payload")
	}
}

func TestCatalogLoadFromReflectionPreservesTransportDiagnostic(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	address := listener.Addr().String()
	_ = listener.Close()

	service := NewService(ServiceDependencies{})
	response := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    1000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if response.Ok {
		t.Fatalf("expected transport failure for closed endpoint")
	}

	if response.Error == nil || response.Error.Category != contracts.ErrorCategoryTransport {
		t.Fatalf("expected transport-classified error, got %+v", response.Error)
	}
}

func TestCatalogLoadFromReflectionCustomCASuccess(t *testing.T) {
	t.Parallel()

	pki := generateTestPKI(t)
	address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{
		serverTLS: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{mustKeyPair(t, pki.serverCertPEM, pki.serverKeyPEM)},
			NextProtos:   []string{"h2"},
		},
	})
	defer stop()

	materialsDir := t.TempDir()
	indexPath := writeMaterialIndex(t, materialsDir, []MaterialRecord{
		{
			Backend:   "file",
			Namespace: "tls",
			Key:       "ca.pem",
			Path:      writeFile(t, materialsDir, "ca.pem", pki.rootPEM),
			Kind:      string(SecretUsageTLSCA),
		},
	})

	service := NewService(ServiceDependencies{
		SecretStore: NewFileSecretStore(newJSONMaterialIndex(indexPath)),
	})

	response := service.CatalogLoadFromReflection(context.Background(), contracts.CatalogLoadFromReflectionInput{
		Endpoint: contracts.EndpointPreset{
			Target: address,
			TLS: contracts.EndpointTLSSettings{
				Mode:               contracts.TLSModeCustomCA,
				ServerNameOverride: "localhost",
				CACert:             "secret-ref:file/tls/ca.pem",
			},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !response.Ok || response.Data == nil {
		t.Fatalf("expected TLS reflection catalog, got %+v", response.Error)
	}

	if response.Data.Services[0].FullName != "tether.demo.v1.ReflectionDemo" {
		t.Fatalf("unexpected reflected TLS service catalog: %+v", response.Data.Services)
	}
}

type reflectionCatalogServerOptions struct {
	disableReflection     bool
	serverTLS             *tls.Config
	watchMessages         []int64
	watchBlockUntilCancel bool
	watchStatusCode       codes.Code
	watchStatusMessage    string
}

type reflectionDemoMarker interface {
	isReflectionDemo()
}

type reflectionDemoService struct {
	watchMessages         []int64
	watchBlockUntilCancel bool
	watchStatusCode       codes.Code
	watchStatusMessage    string
}

func (reflectionDemoService) isReflectionDemo() {}

var reflectionDemoServiceDesc = grpc.ServiceDesc{
	ServiceName: "tether.demo.v1.ReflectionDemo",
	HandlerType: (*reflectionDemoMarker)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				request := &timestamppb.Timestamp{}
				if err := dec(request); err != nil {
					return nil, err
				}

				if request.Seconds < 0 {
					return nil, status.Error(codes.InvalidArgument, "timestamp seconds must be non-negative")
				}

				if err := grpc.SetHeader(ctx, metadata.Pairs(
					"x-reflection-demo", "ping",
					"set-cookie", "session=reflection-secret",
				)); err != nil {
					return nil, err
				}
				if err := grpc.SetTrailer(ctx, metadata.Pairs(
					"x-reflection-demo-trailer", "ok",
					"set-cookie", "refresh=reflection-secret",
				)); err != nil {
					return nil, err
				}

				return structpb.NewStruct(map[string]any{
					"echoSeconds": request.Seconds,
				})
			},
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Watch",
			ServerStreams: true,
			Handler: func(service any, stream grpc.ServerStream) error {
				request := &emptypb.Empty{}
				if err := stream.RecvMsg(request); err != nil {
					return err
				}

				demoService, _ := service.(reflectionDemoService)
				if err := grpc.SetHeader(stream.Context(), metadata.Pairs(
					"x-reflection-demo", "watch",
					"set-cookie", "stream=stream-cookie-secret",
				)); err != nil {
					return err
				}
				if err := grpc.SetTrailer(stream.Context(), metadata.Pairs(
					"x-reflection-demo-trailer", "stream-ok",
					"set-cookie", "stream-refresh=stream-cookie-secret",
				)); err != nil {
					return err
				}

				if demoService.watchStatusCode != codes.OK {
					return status.Error(demoService.watchStatusCode, demoService.watchStatusMessage)
				}

				if demoService.watchBlockUntilCancel {
					<-stream.Context().Done()
					return stream.Context().Err()
				}

				messages := demoService.watchMessages
				if len(messages) == 0 {
					messages = []int64{1}
				}
				for _, seconds := range messages {
					if err := stream.SendMsg(&timestamppb.Timestamp{Seconds: seconds}); err != nil {
						return err
					}
				}

				return nil
			},
		},
	},
}

func startReflectionCatalogServer(t *testing.T, options reflectionCatalogServerOptions) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	serverOptions := make([]grpc.ServerOption, 0, 1)
	if options.serverTLS != nil {
		serverOptions = append(serverOptions, grpc.Creds(grpccredentials.NewTLS(options.serverTLS)))
	}

	server := grpc.NewServer(serverOptions...)
	server.RegisterService(&reflectionDemoServiceDesc, reflectionDemoService{
		watchMessages:         append([]int64(nil), options.watchMessages...),
		watchBlockUntilCancel: options.watchBlockUntilCancel,
		watchStatusCode:       options.watchStatusCode,
		watchStatusMessage:    options.watchStatusMessage,
	})

	if !options.disableReflection {
		resolver := buildReflectionCatalogResolver(t)
		reflectionOptions := reflection.ServerOptions{
			Services:           server,
			DescriptorResolver: resolver,
		}
		v1reflectiongrpc.RegisterServerReflectionServer(server, reflection.NewServerV1(reflectionOptions))
		v1alphareflectiongrpc.RegisterServerReflectionServer(server, reflection.NewServer(reflectionOptions))
	}

	go func() {
		_ = server.Serve(listener)
	}()

	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
	}
}

func buildReflectionCatalogResolver(t *testing.T) protodesc.Resolver {
	t.Helper()

	fileDescriptorProto := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("tether/demo/v1/reflection_demo.proto"),
		Package: proto.String("tether.demo.v1"),
		Dependency: []string{
			"google/protobuf/empty.proto",
			"google/protobuf/struct.proto",
			"google/protobuf/timestamp.proto",
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("ReflectionDemo"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("Ping"),
						InputType:  proto.String(".google.protobuf.Timestamp"),
						OutputType: proto.String(".google.protobuf.Struct"),
					},
					{
						Name:            proto.String("Watch"),
						InputType:       proto.String(".google.protobuf.Empty"),
						OutputType:      proto.String(".google.protobuf.Timestamp"),
						ServerStreaming: proto.Bool(true),
					},
				},
			},
		},
	}

	fileDescriptor, err := protodesc.NewFile(fileDescriptorProto, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("new file descriptor: %v", err)
	}

	files := new(protoregistry.Files)
	if err := files.RegisterFile(fileDescriptor); err != nil {
		t.Fatalf("register file descriptor: %v", err)
	}

	return chainedResolver{
		primary:  files,
		fallback: protoregistry.GlobalFiles,
	}
}

func assertWellKnownTypes(t *testing.T, refs []contracts.CatalogMessageRef, expected []string) {
	t.Helper()

	actual := make([]string, 0, len(refs))
	for _, ref := range refs {
		actual = append(actual, ref.FullName)
	}

	for _, expectedRef := range expected {
		found := false
		for _, actualRef := range actual {
			if actualRef == expectedRef {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf("expected well-known type %s, got %+v", expectedRef, actual)
		}
	}
}
