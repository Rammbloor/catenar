package endpoint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
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
	Method         contracts.CatalogMethod
	Descriptor     protoreflect.MethodDescriptor
	Metadata       map[string]string
	Body           any
	RequestTimeout time.Duration
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
	Method     contracts.CatalogMethod
	Descriptor protoreflect.MethodDescriptor
	Stream     grpc.ClientStream
	OnHeaders  func(map[string][]string, time.Time)
	OnMessage  func(ServerStreamMessage)
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

	callCtx := ctx
	var cancel context.CancelFunc = func() {}
	if request.RequestTimeout > 0 {
		callCtx, cancel = context.WithTimeout(callCtx, request.RequestTimeout)
	} else {
		callCtx, cancel = context.WithCancel(callCtx)
	}

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
	headers, headerErr := request.Stream.Header()
	if headerErr != nil {
		return serverStreamErrorResult(request.Stream, nil, headerErr), classifyServerStreamError(request.Method, headerErr)
	}

	clonedHeaders := cloneMetadataValues(headers)
	if request.OnHeaders != nil && len(clonedHeaders) > 0 {
		request.OnHeaders(clonedHeaders, time.Now().UTC())
	}

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
	}
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
