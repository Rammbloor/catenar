package endpoint

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"

	"tether/internal/contracts"
)

type diagnosticEmitterSpy struct {
	eventName contracts.EventName
	payload   any
}

func (s *diagnosticEmitterSpy) Emit(eventName contracts.EventName, payload any) error {
	s.eventName = eventName
	s.payload = payload
	return nil
}

type testPKI struct {
	rootPEM       []byte
	serverCertPEM []byte
	serverKeyPEM  []byte
	clientCertPEM []byte
	clientKeyPEM  []byte
	rootPool      *x509.CertPool
}

func TestValidateEndpointPresetRejectsCaseInsensitiveMetadataCollision(t *testing.T) {
	t.Parallel()

	issues := ValidateEndpointPreset(contracts.EndpointPreset{
		Target:              "localhost:50051",
		TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
		ConnectTimeoutMs:    5000,
		RequestTimeoutMs:    1000,
		StreamIdleTimeoutMs: 0,
		MetadataDefaults: map[string]string{
			"X-Tenant": "a",
			"x-tenant": "b",
		},
	})

	if len(issues) == 0 {
		t.Fatalf("expected validation issues for case-insensitive metadata collision")
	}
}

func TestValidateEndpointPresetAllowsExplicitEmptyMetadataValue(t *testing.T) {
	t.Parallel()

	issues := ValidateEndpointPreset(contracts.EndpointPreset{
		Target:              "localhost:50051",
		TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModeSystemCA},
		ConnectTimeoutMs:    5000,
		RequestTimeoutMs:    1000,
		StreamIdleTimeoutMs: 0,
		MetadataDefaults: map[string]string{
			"x-empty": "",
		},
	})

	if len(issues) != 0 {
		t.Fatalf("expected explicit empty metadata value to be accepted, got %+v", issues)
	}
}

func TestEndpointTestRejectsInvalidPlaintextServerNameOverride(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceDependencies{})
	response := service.EndpointTest(context.Background(), contracts.EndpointTestInput{
		Endpoint: contracts.EndpointPreset{
			Target:              "localhost:50051",
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext, ServerNameOverride: "localhost"},
			ConnectTimeoutMs:    5000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if response.Ok {
		t.Fatalf("expected invalid plaintext endpoint to fail validation")
	}

	if response.Error == nil || response.Error.Category != contracts.ErrorCategoryValidation {
		t.Fatalf("expected validation error, got %+v", response.Error)
	}
}

func TestEndpointTestPlaintextSuccess(t *testing.T) {
	t.Parallel()

	address, stop := startGRPCServer(t, nil)
	defer stop()

	service := NewService(ServiceDependencies{})
	spy := &diagnosticEmitterSpy{}
	service.SetEmitter(spy)

	response := service.EndpointTest(context.Background(), contracts.EndpointTestInput{
		Endpoint: contracts.EndpointPreset{
			Target:              address,
			TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	assertEndpointReadyResult(t, response, contracts.TLSModePlaintext)
	if spy.eventName != contracts.EventDiagnosticsUpdate {
		t.Fatalf("expected diagnostics:update event, got %q", spy.eventName)
	}
}

func TestEndpointTestSystemCASuccessWithSplitAuthorityAndServerName(t *testing.T) {
	t.Parallel()

	pki := generateTestPKI(t)
	address, stop := startGRPCServer(t, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{mustKeyPair(t, pki.serverCertPEM, pki.serverKeyPEM)},
		NextProtos:   []string{"h2"},
	})
	defer stop()

	service := NewService(ServiceDependencies{
		TransportAdapter: newGRPCTransportAdapter(grpcTransportAdapterOptions{
			systemCertPool: func() (*x509.CertPool, error) {
				return pki.rootPool, nil
			},
		}),
	})

	response := service.EndpointTest(context.Background(), contracts.EndpointTestInput{
		Endpoint: contracts.EndpointPreset{
			Target:    address,
			Authority: "api.internal.example",
			TLS: contracts.EndpointTLSSettings{
				Mode:               contracts.TLSModeSystemCA,
				ServerNameOverride: "localhost",
			},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	assertEndpointReadyResult(t, response, contracts.TLSModeSystemCA)
}

func TestEndpointTestCustomCASuccess(t *testing.T) {
	t.Parallel()

	pki := generateTestPKI(t)
	address, stop := startGRPCServer(t, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{mustKeyPair(t, pki.serverCertPEM, pki.serverKeyPEM)},
		NextProtos:   []string{"h2"},
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

	response := service.EndpointTest(context.Background(), contracts.EndpointTestInput{
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

	assertEndpointReadyResult(t, response, contracts.TLSModeCustomCA)
}

func TestEndpointTestMTLSSuccess(t *testing.T) {
	t.Parallel()

	pki := generateTestPKI(t)
	address, stop := startGRPCServer(t, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{mustKeyPair(t, pki.serverCertPEM, pki.serverKeyPEM)},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pki.rootPool,
		NextProtos:   []string{"h2"},
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
		{
			Backend:   "file",
			Namespace: "tls",
			Key:       "client.crt",
			Path:      writeFile(t, materialsDir, "client.crt", pki.clientCertPEM),
			Kind:      string(SecretUsageTLSClientCert),
		},
		{
			Backend:   "file",
			Namespace: "tls",
			Key:       "client.key",
			Path:      writeFile(t, materialsDir, "client.key", pki.clientKeyPEM),
			Kind:      string(SecretUsageTLSClientKey),
		},
	})

	service := NewService(ServiceDependencies{
		SecretStore: NewFileSecretStore(newJSONMaterialIndex(indexPath)),
	})

	response := service.EndpointTest(context.Background(), contracts.EndpointTestInput{
		Endpoint: contracts.EndpointPreset{
			Target: address,
			TLS: contracts.EndpointTLSSettings{
				Mode:               contracts.TLSModeMTLS,
				ServerNameOverride: "localhost",
				CACert:             "secret-ref:file/tls/ca.pem",
				ClientCert:         "secret-ref:file/tls/client.crt",
				ClientKey:          "secret-ref:file/tls/client.key",
			},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	assertEndpointReadyResult(t, response, contracts.TLSModeMTLS)
}

func TestEndpointTestTLSHostnameMismatchProducesTransportDiagnostic(t *testing.T) {
	t.Parallel()

	pki := generateTestPKI(t)
	address, stop := startGRPCServer(t, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{mustKeyPair(t, pki.serverCertPEM, pki.serverKeyPEM)},
		NextProtos:   []string{"h2"},
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

	response := service.EndpointTest(context.Background(), contracts.EndpointTestInput{
		Endpoint: contracts.EndpointPreset{
			Target: address,
			TLS: contracts.EndpointTLSSettings{
				Mode:               contracts.TLSModeCustomCA,
				ServerNameOverride: "wrong.local",
				CACert:             "secret-ref:file/tls/ca.pem",
			},
			ConnectTimeoutMs:    3000,
			RequestTimeoutMs:    1000,
			StreamIdleTimeoutMs: 0,
		},
	})

	if !response.Ok || response.Data == nil {
		t.Fatalf("expected endpoint test result, got %+v", response.Error)
	}

	if response.Data.TLSOK {
		t.Fatalf("expected TLS handshake to fail")
	}

	if response.Data.Diagnostic == nil || response.Data.Diagnostic.Code != "transport.tls_hostname_mismatch" {
		t.Fatalf("expected hostname mismatch diagnostic, got %+v", response.Data.Diagnostic)
	}
}

func TestCallInvokeUnaryClassifiesCatalogNotLoadedAsApplicationStateError(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceDependencies{
		AppDataDir: t.TempDir(),
	})

	testCases := []struct {
		name          string
		catalogSource contracts.CatalogSourceKind
	}{
		{name: "reflection", catalogSource: contracts.CatalogSourceReflection},
		{name: "proto", catalogSource: contracts.CatalogSourceProto},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := service.CallInvokeUnary(context.Background(), contracts.CallInvokeUnaryInput{
				CatalogSource: tc.catalogSource,
				EndpointID:    "ep-missing",
				Method:        "tether.demo.v1.ReflectionDemo.Ping",
				Body:          map[string]any{},
			})

			if response.Ok {
				t.Fatalf("expected unary invoke to fail without a cached catalog")
			}
			if response.Error == nil {
				t.Fatalf("expected error envelope")
			}
			if response.Error.Code != "application.catalog_not_loaded" {
				t.Fatalf("expected application.catalog_not_loaded, got %+v", response.Error)
			}
			if response.Error.Category != contracts.ErrorCategoryApplication {
				t.Fatalf("expected application category, got %+v", response.Error)
			}
			if response.Error.Details["endpointId"] != "ep-missing" {
				t.Fatalf("expected endpoint details, got %+v", response.Error.Details)
			}
			if response.Error.Details["catalogSource"] != string(tc.catalogSource) {
				t.Fatalf("expected catalog source details, got %+v", response.Error.Details)
			}
		})
	}
}

func TestCallInvokeUnaryClassifiesCachedMethodMissesAsApplicationStateError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		catalogSource contracts.CatalogSourceKind
		loadEndpoint  func(*testing.T, *Service) string
	}{
		{
			name:          "reflection",
			catalogSource: contracts.CatalogSourceReflection,
			loadEndpoint: func(t *testing.T, service *Service) string {
				t.Helper()

				address, stop := startReflectionCatalogServer(t, reflectionCatalogServerOptions{})
				t.Cleanup(stop)

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

				return response.Data.Endpoint.ID
			},
		},
		{
			name:          "proto",
			catalogSource: contracts.CatalogSourceProto,
			loadEndpoint: func(t *testing.T, service *Service) string {
				t.Helper()

				serviceRoot, importRoot := writeProtoCatalogFixture(t)
				response := service.CatalogLoadFromProtoSources(context.Background(), contracts.CatalogLoadFromProtoSourcesInput{
					Endpoint: contracts.EndpointPreset{
						Target:              "127.0.0.1:50051",
						TLS:                 contracts.EndpointTLSSettings{Mode: contracts.TLSModePlaintext},
						ConnectTimeoutMs:    3000,
						RequestTimeoutMs:    1000,
						StreamIdleTimeoutMs: 0,
					},
					ProtoSources: []contracts.ProtoSource{
						{Type: contracts.ProtoSourceTypeDirectory, Path: serviceRoot},
					},
					ImportPaths: []string{importRoot},
				})
				if !response.Ok || response.Data == nil {
					t.Fatalf("expected proto catalog, got %+v", response.Error)
				}

				return response.Data.Endpoint.ID
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService(ServiceDependencies{
				AppDataDir: t.TempDir(),
			})
			endpointID := tc.loadEndpoint(t, service)

			response := service.CallInvokeUnary(context.Background(), contracts.CallInvokeUnaryInput{
				CatalogSource: tc.catalogSource,
				EndpointID:    endpointID,
				Method:        "tether.demo.v1.ReflectionDemo.Missing",
				Body:          map[string]any{},
			})

			if response.Ok {
				t.Fatalf("expected unary invoke to fail for a stale cached method")
			}
			if response.Error == nil {
				t.Fatalf("expected error envelope")
			}
			if response.Error.Code != "application.method_not_found" {
				t.Fatalf("expected application.method_not_found, got %+v", response.Error)
			}
			if response.Error.Category != contracts.ErrorCategoryApplication {
				t.Fatalf("expected application category, got %+v", response.Error)
			}
			if response.Error.Details["endpointId"] != endpointID {
				t.Fatalf("expected endpoint details, got %+v", response.Error.Details)
			}
			if response.Error.Details["method"] != "tether.demo.v1.ReflectionDemo.Missing" {
				t.Fatalf("expected missing method details, got %+v", response.Error.Details)
			}
			if response.Error.Details["catalogSource"] != string(tc.catalogSource) {
				t.Fatalf("expected catalog source details, got %+v", response.Error.Details)
			}
		})
	}
}

func TestHistoryGetRedactsAndRewritesLegacySummaryMetadata(t *testing.T) {
	t.Parallel()

	appDataDir := t.TempDir()
	service := NewService(ServiceDependencies{
		AppDataDir: appDataDir,
	})

	summaryPath := filepath.Join(appDataDir, "history", "summaries", "legacy-call.json")
	if err := os.MkdirAll(filepath.Dir(summaryPath), 0o755); err != nil {
		t.Fatalf("mkdir summary dir: %v", err)
	}

	if err := writeJSONFile(summaryPath, storedUnaryHistoryDetail{
		RequestBody: map[string]any{"ping": "pong"},
		Headers: map[string][]string{
			"set-cookie": {"legacy-session-secret"},
		},
		Trailers: map[string][]string{
			"set-cookie": {"legacy-refresh-secret"},
		},
		Status: contracts.StreamStatus{
			Code: "OK",
		},
		Events: []contracts.HistoryLogEvent{},
	}); err != nil {
		t.Fatalf("write legacy summary: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := service.historyStore.SaveCallSummary(context.Background(), contracts.HistoryCallSummary{
		CallID:         "legacy-call",
		SessionID:      "legacy-session",
		WorkspaceID:    "legacy-workspace",
		Method:         "tether.demo.v1.ReflectionDemo.Ping",
		RPCType:        contracts.RPCTypeUnary,
		EndpointID:     "legacy-endpoint",
		State:          contracts.StreamStateClosed,
		GRPCStatusCode: "OK",
		StartedAt:      now,
		FinishedAt:     now,
		RequestCount:   1,
		ResponseCount:  0,
		SummaryPath:    summaryPath,
	}); err != nil {
		t.Fatalf("save legacy call summary: %v", err)
	}

	response := service.HistoryGet(context.Background(), "legacy-call")
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected history detail, got %+v", response.Error)
	}

	if response.Data.Headers["set-cookie"][0] != "[REDACTED]" {
		t.Fatalf("expected redacted legacy headers, got %+v", response.Data.Headers)
	}
	if response.Data.Trailers["set-cookie"][0] != "[REDACTED]" {
		t.Fatalf("expected redacted legacy trailers, got %+v", response.Data.Trailers)
	}

	rewrittenPayload, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read rewritten summary: %v", err)
	}
	if strings.Contains(string(rewrittenPayload), "legacy-session-secret") || strings.Contains(string(rewrittenPayload), "legacy-refresh-secret") {
		t.Fatalf("expected rewritten summary to drop raw secret metadata, got %s", rewrittenPayload)
	}
}

func assertEndpointReadyResult(t *testing.T, response contracts.EndpointTestResponse, tlsMode contracts.TLSMode) {
	t.Helper()

	if !response.Ok {
		t.Fatalf("expected endpoint test response to be ok, got %+v", response.Error)
	}

	if response.Data == nil {
		t.Fatalf("expected endpoint test data")
	}

	if !response.Data.TransportReachable {
		t.Fatalf("expected transport to be reachable")
	}

	if tlsMode == contracts.TLSModePlaintext {
		if response.Data.TLSConfigured {
			t.Fatalf("expected TLS to be disabled for plaintext mode")
		}
	} else if !response.Data.TLSOK {
		t.Fatalf("expected TLS check to succeed for mode %q", tlsMode)
	}

	if !response.Data.GRPCReady || !response.Data.GRPCReadyProven {
		t.Fatalf("expected gRPC readiness to be proven, got %+v", response.Data)
	}
}

func startGRPCServer(t *testing.T, serverTLS *tls.Config) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	serverOptions := make([]grpc.ServerOption, 0, 1)
	if serverTLS != nil {
		serverOptions = append(serverOptions, grpc.Creds(grpccredentials.NewTLS(serverTLS)))
	}

	server := grpc.NewServer(serverOptions...)
	go func() {
		_ = server.Serve(listener)
	}()

	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
	}
}

func generateTestPKI(t *testing.T) testPKI {
	t.Helper()

	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "tether-test-root",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER := createCertificate(t, caTemplate, caTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}

	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate server key: %v", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	serverDER := createCertificate(t, serverTemplate, caCert, &serverPrivateKey.PublicKey, caPrivateKey)

	clientPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			CommonName: "tether-client",
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientDER := createCertificate(t, clientTemplate, caCert, &clientPrivateKey.PublicKey, caPrivateKey)

	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverPrivateKey)})
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientPrivateKey)})

	rootPool := x509.NewCertPool()
	rootPool.AppendCertsFromPEM(rootPEM)

	return testPKI{
		rootPEM:       rootPEM,
		serverCertPEM: serverCertPEM,
		serverKeyPEM:  serverKeyPEM,
		clientCertPEM: clientCertPEM,
		clientKeyPEM:  clientKeyPEM,
		rootPool:      rootPool,
	}
}

func createCertificate(t *testing.T, template, parent *x509.Certificate, publicKey any, signer *rsa.PrivateKey) []byte {
	t.Helper()

	certificateDER, err := x509.CreateCertificate(rand.Reader, template, parent, publicKey, signer)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	return certificateDER
}

func mustKeyPair(t *testing.T, certificatePEM, keyPEM []byte) tls.Certificate {
	t.Helper()

	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatalf("x509 key pair: %v", err)
	}

	return certificate
}

func writeMaterialIndex(t *testing.T, dir string, records []MaterialRecord) string {
	t.Helper()

	indexPath := filepath.Join(dir, "materials", "index.json")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatalf("mkdir materials dir: %v", err)
	}

	content, err := json.Marshal(struct {
		Records []MaterialRecord `json:"records"`
	}{
		Records: records,
	})
	if err != nil {
		t.Fatalf("marshal material index: %v", err)
	}

	if err := os.WriteFile(indexPath, content, 0o600); err != nil {
		t.Fatalf("write material index: %v", err)
	}

	return indexPath
}

func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}

	return path
}
