package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"catenar/internal/contracts"
)

type UnaryInvokeRequest struct {
	Method         contracts.CatalogMethod
	Descriptor     protoreflect.MethodDescriptor
	Metadata       map[string]string
	Body           any
	RequestTimeout time.Duration
}

type UnaryInvokeResult struct {
	ResponseBody any
	Headers      map[string][]string
	Trailers     map[string][]string
	Status       contracts.StreamStatus
	Duration     time.Duration
}

func (r *grpcRuntime) InvokeUnary(ctx context.Context, conn GRPCClientConn, request UnaryInvokeRequest) (UnaryInvokeResult, *endpointDiagnostic) {
	startedAt := time.Now()

	requestMessage, requestDiagnostic := buildUnaryRequestMessage(request.Descriptor.Input(), request.Body)
	if requestDiagnostic != nil {
		return UnaryInvokeResult{}, requestDiagnostic
	}

	responseMessage := dynamicpb.NewMessage(request.Descriptor.Output())
	requestMetadata := metadata.New(request.Metadata)

	callCtx := ctx
	if len(request.Metadata) > 0 {
		callCtx = metadata.NewOutgoingContext(callCtx, requestMetadata)
	}

	var cancel context.CancelFunc
	if request.RequestTimeout > 0 {
		callCtx, cancel = context.WithTimeout(callCtx, request.RequestTimeout)
		defer cancel()
	}

	var headers metadata.MD
	var trailers metadata.MD
	err := conn.Invoke(
		callCtx,
		grpcMethodPath(request.Descriptor),
		requestMessage,
		responseMessage,
		grpc.Header(&headers),
		grpc.Trailer(&trailers),
	)
	if err != nil {
		result := UnaryInvokeResult{
			Headers:  cloneMetadataValues(headers),
			Trailers: cloneMetadataValues(trailers),
			Duration: time.Since(startedAt),
		}
		if grpcStatus, ok := status.FromError(err); ok {
			result.Status = contracts.StreamStatus{
				Code:    grpcStatus.Code().String(),
				Message: grpcStatus.Message(),
			}
		}
		return result, classifyUnaryInvokeError(request.Method, err)
	}

	responseBody, responseDiagnostic := marshalMessageJSONValue(responseMessage)
	if responseDiagnostic != nil {
		return UnaryInvokeResult{}, responseDiagnostic
	}

	return UnaryInvokeResult{
		ResponseBody: responseBody,
		Headers:      cloneMetadataValues(headers),
		Trailers:     cloneMetadataValues(trailers),
		Status: contracts.StreamStatus{
			Code:    codes.OK.String(),
			Message: "",
		},
		Duration: time.Since(startedAt),
	}, nil
}

func buildUnaryRequestMessage(messageDescriptor protoreflect.MessageDescriptor, body any) (proto.Message, *endpointDiagnostic) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     "validation.request_body_invalid",
			Category: contracts.ErrorCategoryValidation,
			Message:  "The request body could not be serialized into JSON for protobuf validation.",
			NextStep: "Fix the JSON payload shape and retry the unary call.",
			Details: map[string]string{
				"cause": err.Error(),
			},
		}
	}

	message := dynamicpb.NewMessage(messageDescriptor)
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(payload, message); err != nil {
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     "validation.request_body_invalid",
			Category: contracts.ErrorCategoryValidation,
			Message:  "The request body does not match the protobuf schema for the selected method.",
			NextStep: "Regenerate the starter payload or correct the request body fields before invoking the method again.",
			Details: map[string]string{
				"cause": err.Error(),
			},
		}
	}

	return message, nil
}

func marshalMessageJSONValue(message proto.Message) (any, *endpointDiagnostic) {
	payload, err := protojson.MarshalOptions{
		UseProtoNames: false,
	}.Marshal(message)
	if err != nil {
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     "application.response_decode_failed",
			Category: contracts.ErrorCategoryApplication,
			Message:  "The unary response body could not be encoded into the UI JSON contract.",
			NextStep: "Retry the call and inspect whether the response contains an unsupported protobuf shape.",
			Details: map[string]string{
				"cause": err.Error(),
			},
		}
	}

	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, &endpointDiagnostic{
			Level:    "error",
			Code:     "application.response_decode_failed",
			Category: contracts.ErrorCategoryApplication,
			Message:  "The unary response body could not be materialized as JSON.",
			NextStep: "Retry the call and export diagnostics if the response cannot be rendered consistently.",
			Details: map[string]string{
				"cause": err.Error(),
			},
		}
	}

	return value, nil
}

func classifyUnaryInvokeError(method contracts.CatalogMethod, err error) *endpointDiagnostic {
	if grpcStatus, ok := status.FromError(err); ok {
		statusCode := grpcStatus.Code().String()
		if isTransportLikeUnaryError(grpcStatus) {
			return &endpointDiagnostic{
				Level:    "error",
				Code:     "transport.invoke_failed",
				Category: contracts.ErrorCategoryTransport,
				Message:  "The unary call failed because the transport connection was interrupted before a stable application response completed.",
				NextStep: "Retry the call after checking endpoint reachability, proxies and whether the server kept the gRPC connection open for the full request.",
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
			Message:  fmt.Sprintf("The unary call finished with gRPC status %s: %s", statusCode, grpcStatus.Message()),
			NextStep: "Inspect the returned gRPC status, request payload and server-side validation or authorization rules before retrying.",
			Details: map[string]string{
				"method":         method.FullName,
				"grpcStatusCode": statusCode,
				"cause":          grpcStatus.Message(),
			},
		}
	}

	return &endpointDiagnostic{
		Level:    "error",
		Code:     "transport.invoke_failed",
		Category: contracts.ErrorCategoryTransport,
		Message:  "The unary call failed before a gRPC status could be returned.",
		NextStep: "Retry the call after checking transport connectivity, endpoint readiness and local runtime errors.",
		Details: map[string]string{
			"method": method.FullName,
			"cause":  err.Error(),
		},
	}
}

func isTransportLikeUnaryError(grpcStatus *status.Status) bool {
	if grpcStatus == nil {
		return false
	}

	if grpcStatus.Code() != codes.Unavailable {
		return false
	}

	lowerMessage := strings.ToLower(grpcStatus.Message())
	transportHints := []string{
		"connection error",
		"transport is closing",
		"client connection is closing",
		"error reading from server",
		"eof",
		"broken pipe",
		"connection reset",
	}
	for _, hint := range transportHints {
		if strings.Contains(lowerMessage, hint) {
			return true
		}
	}

	return false
}

func cloneMetadataValues(values metadata.MD) map[string][]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string][]string, len(values))
	for key, value := range values {
		cloned[key] = append([]string(nil), value...)
	}

	return cloned
}

func grpcMethodPath(methodDescriptor protoreflect.MethodDescriptor) string {
	return "/" + string(methodDescriptor.Parent().FullName()) + "/" + string(methodDescriptor.Name())
}

func grpcStatusCodeIdentifier(statusCode string) string {
	var builder strings.Builder
	for index, symbol := range statusCode {
		if unicode.IsUpper(symbol) {
			if index > 0 {
				builder.WriteByte('_')
			}
			builder.WriteRune(unicode.ToLower(symbol))
			continue
		}

		builder.WriteRune(symbol)
	}

	return builder.String()
}
