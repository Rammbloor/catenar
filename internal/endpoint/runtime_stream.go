package endpoint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"tether/internal/contracts"
)

type ServerStreamStartRequest struct {
	Method     contracts.CatalogMethod
	Descriptor protoreflect.MethodDescriptor
	Metadata   map[string]string
	Body       any
}

type ServerStreamStartResult struct {
	Stream grpc.ClientStream
	Cancel context.CancelFunc
}

type ServerStreamMessage struct {
	Body       any
	Index      int
	SizeBytes  int64
	ReceivedAt time.Time
}

type ServerStreamConsumeRequest struct {
	Method      contracts.CatalogMethod
	Descriptor  protoreflect.MethodDescriptor
	Stream      grpc.ClientStream
	Cancel      context.CancelFunc
	IdleTimeout time.Duration
	OnHeaders   func(map[string][]string, time.Time)
	OnMessage   func(ServerStreamMessage)
}

type ServerStreamConsumeResult struct {
	Headers       map[string][]string
	Trailers      map[string][]string
	Status        contracts.StreamStatus
	Messages      []ServerStreamMessage
	ResponseCount int
	FinishedAt    time.Time
}

func (r *grpcRuntime) StartServerStream(ctx context.Context, conn GRPCClientConn, request ServerStreamStartRequest) (ServerStreamStartResult, *endpointDiagnostic) {
	requestMessage, requestDiagnostic := buildUnaryRequestMessage(request.Descriptor.Input(), request.Body)
	if requestDiagnostic != nil {
		return ServerStreamStartResult{}, requestDiagnostic
	}

	callCtx, cancel := context.WithCancel(ctx)

	if len(request.Metadata) > 0 {
		callCtx = metadata.NewOutgoingContext(callCtx, metadata.New(request.Metadata))
	}

	stream, err := conn.NewStream(
		callCtx,
		&grpc.StreamDesc{ServerStreams: true},
		grpcMethodPath(request.Descriptor),
	)
	if err != nil {
		cancel()
		return ServerStreamStartResult{}, classifyServerStreamError(request.Method, err)
	}

	if err := stream.SendMsg(requestMessage); err != nil {
		cancel()
		return ServerStreamStartResult{}, classifyServerStreamError(request.Method, err)
	}

	if err := stream.CloseSend(); err != nil {
		cancel()
		return ServerStreamStartResult{}, classifyServerStreamError(request.Method, err)
	}

	return ServerStreamStartResult{
		Stream: stream,
		Cancel: cancel,
	}, nil
}

func (r *grpcRuntime) ConsumeServerStream(request ServerStreamConsumeRequest) (ServerStreamConsumeResult, *endpointDiagnostic) {
	idleMonitor := newServerStreamIdleMonitor(request.IdleTimeout, request.Cancel)
	defer idleMonitor.Stop()

	headers, headerErr := request.Stream.Header()
	if headerErr != nil {
		result := serverStreamErrorResult(request.Stream, nil, headerErr)
		if idleMonitor.TimedOut() {
			result.Status = streamIdleTimeoutStatus(request.IdleTimeout)
			return result, classifyServerStreamIdleTimeout(request.Method, request.IdleTimeout)
		}
		return result, classifyServerStreamError(request.Method, headerErr)
	}

	clonedHeaders := cloneMetadataValues(headers)
	if request.OnHeaders != nil && len(clonedHeaders) > 0 {
		request.OnHeaders(clonedHeaders, time.Now().UTC())
	}
	idleMonitor.Reset()

	messages := make([]ServerStreamMessage, 0)
	for {
		responseMessage := dynamicpb.NewMessage(request.Descriptor.Output())
		err := request.Stream.RecvMsg(responseMessage)
		if errors.Is(err, io.EOF) {
			trailers := cloneMetadataValues(request.Stream.Trailer())
			return ServerStreamConsumeResult{
				Headers:       clonedHeaders,
				Trailers:      trailers,
				Status:        contracts.StreamStatus{Code: codes.OK.String()},
				Messages:      messages,
				ResponseCount: len(messages),
				FinishedAt:    time.Now().UTC(),
			}, nil
		}
		if err != nil {
			result := serverStreamErrorResult(request.Stream, clonedHeaders, err)
			result.Messages = messages
			result.ResponseCount = len(messages)
			if idleMonitor.TimedOut() {
				result.Status = streamIdleTimeoutStatus(request.IdleTimeout)
				return result, classifyServerStreamIdleTimeout(request.Method, request.IdleTimeout)
			}
			return result, classifyServerStreamError(request.Method, err)
		}

		body, responseDiagnostic := marshalMessageJSONValue(responseMessage)
		if responseDiagnostic != nil {
			return ServerStreamConsumeResult{
				Headers:       clonedHeaders,
				Trailers:      cloneMetadataValues(request.Stream.Trailer()),
				Messages:      messages,
				ResponseCount: len(messages),
				FinishedAt:    time.Now().UTC(),
			}, responseDiagnostic
		}

		message := ServerStreamMessage{
			Body:       body,
			Index:      len(messages),
			SizeBytes:  measureJSONSize(body),
			ReceivedAt: time.Now().UTC(),
		}
		messages = append(messages, message)
		if request.OnMessage != nil {
			request.OnMessage(message)
		}
		idleMonitor.Reset()
	}
}

type serverStreamIdleMonitor struct {
	timer    *time.Timer
	timeout  time.Duration
	timedOut atomic.Bool
	stopped  atomic.Bool
}

func newServerStreamIdleMonitor(timeout time.Duration, cancel context.CancelFunc) *serverStreamIdleMonitor {
	monitor := &serverStreamIdleMonitor{}
	if timeout <= 0 || cancel == nil {
		return monitor
	}

	monitor.timeout = timeout
	monitor.timer = time.AfterFunc(timeout, func() {
		if monitor.stopped.Load() {
			return
		}
		monitor.timedOut.Store(true)
		cancel()
	})
	return monitor
}

func (m *serverStreamIdleMonitor) Reset() {
	if m == nil || m.timer == nil || m.stopped.Load() {
		return
	}

	m.timer.Reset(m.timeout)
}

func (m *serverStreamIdleMonitor) Stop() {
	if m == nil || m.timer == nil {
		return
	}

	m.stopped.Store(true)
	m.timer.Stop()
}

func (m *serverStreamIdleMonitor) TimedOut() bool {
	return m != nil && m.timedOut.Load()
}

func serverStreamErrorResult(stream grpc.ClientStream, headers map[string][]string, err error) ServerStreamConsumeResult {
	result := ServerStreamConsumeResult{
		Headers:    headers,
		Trailers:   cloneMetadataValues(stream.Trailer()),
		FinishedAt: time.Now().UTC(),
	}
	if grpcStatus, ok := status.FromError(err); ok {
		result.Status = contracts.StreamStatus{
			Code:    grpcStatus.Code().String(),
			Message: grpcStatus.Message(),
		}
		return result
	}

	if errors.Is(err, context.Canceled) {
		result.Status = contracts.StreamStatus{
			Code:    codes.Canceled.String(),
			Message: err.Error(),
		}
	}

	return result
}

func classifyServerStreamError(method contracts.CatalogMethod, err error) *endpointDiagnostic {
	if errors.Is(err, context.Canceled) {
		return &endpointDiagnostic{
			Level:    "info",
			Code:     "cancelled.stream_cancelled",
			Category: contracts.ErrorCategoryCancelled,
			Message:  "The server-streaming call was cancelled by the user.",
			NextStep: "Start the stream again if more messages are needed.",
			Details: map[string]string{
				"method": method.FullName,
				"cause":  err.Error(),
			},
		}
	}

	if grpcStatus, ok := status.FromError(err); ok {
		statusCode := grpcStatus.Code().String()
		if grpcStatus.Code() == codes.Canceled {
			return &endpointDiagnostic{
				Level:    "info",
				Code:     "cancelled.stream_cancelled",
				Category: contracts.ErrorCategoryCancelled,
				Message:  "The server-streaming call was cancelled by the user.",
				NextStep: "Start the stream again if more messages are needed.",
				Details: map[string]string{
					"method":         method.FullName,
					"grpcStatusCode": statusCode,
					"cause":          grpcStatus.Message(),
				},
			}
		}

		if isTransportLikeUnaryError(grpcStatus) {
			return &endpointDiagnostic{
				Level:    "error",
				Code:     "transport.stream_failed",
				Category: contracts.ErrorCategoryTransport,
				Message:  "The server-streaming call failed because the transport connection was interrupted.",
				NextStep: "Retry the call after checking endpoint reachability, proxies and whether the server keeps the gRPC connection open for streams.",
				Details: map[string]string{
					"method":         method.FullName,
					"grpcStatusCode": statusCode,
					"cause":          grpcStatus.Message(),
				},
			}
		}

		return &endpointDiagnostic{
			Level:    "error",
			Code:     "grpc_status." + grpcStatusCodeIdentifier(statusCode),
			Category: contracts.ErrorCategoryGRPCStatus,
			Message:  fmt.Sprintf("The server-streaming call finished with gRPC status %s: %s", statusCode, grpcStatus.Message()),
			NextStep: "Inspect the returned gRPC status, request payload and server-side stream handling rules before retrying.",
			Details: map[string]string{
				"method":         method.FullName,
				"grpcStatusCode": statusCode,
				"cause":          grpcStatus.Message(),
			},
		}
	}

	if strings.Contains(strings.ToLower(err.Error()), "context canceled") {
		return &endpointDiagnostic{
			Level:    "info",
			Code:     "cancelled.stream_cancelled",
			Category: contracts.ErrorCategoryCancelled,
			Message:  "The server-streaming call was cancelled by the user.",
			NextStep: "Start the stream again if more messages are needed.",
			Details: map[string]string{
				"method": method.FullName,
				"cause":  err.Error(),
			},
		}
	}

	return &endpointDiagnostic{
		Level:    "error",
		Code:     "transport.stream_failed",
		Category: contracts.ErrorCategoryTransport,
		Message:  "The server-streaming call failed before a gRPC status could be returned.",
		NextStep: "Retry the call after checking transport connectivity, endpoint readiness and local runtime errors.",
		Details: map[string]string{
			"method": method.FullName,
			"cause":  err.Error(),
		},
	}
}

func classifyServerStreamIdleTimeout(method contracts.CatalogMethod, timeout time.Duration) *endpointDiagnostic {
	return &endpointDiagnostic{
		Level:    "error",
		Code:     "transport.stream_idle_timeout",
		Category: contracts.ErrorCategoryTransport,
		Message:  "The server-streaming call exceeded the configured idle timeout without receiving new stream data.",
		NextStep: "Increase the stream idle timeout or retry after confirming the server is expected to keep the stream quiet for this long.",
		Details: map[string]string{
			"method":         method.FullName,
			"grpcStatusCode": codes.DeadlineExceeded.String(),
			"idleTimeoutMs":  fmt.Sprintf("%d", timeout.Milliseconds()),
		},
	}
}

func streamIdleTimeoutStatus(timeout time.Duration) contracts.StreamStatus {
	return contracts.StreamStatus{
		Code:    codes.DeadlineExceeded.String(),
		Message: fmt.Sprintf("stream idle timeout after %s", timeout),
	}
}
