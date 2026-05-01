# Runtime Contract v1

## Purpose

Локальная implementation-facing версия runtime/frontend контракта для `Wails v2`.

## Bound operations

Контракт резервирует следующие binding method names:

- `WorkspaceCreate`
- `WorkspaceOpen`
- `WorkspaceSave`
- `WorkspaceValidate`
- `EndpointTest`
- `CatalogLoadFromReflection`
- `CatalogLoadFromProtoSources`
- `RequestSave`
- `CallInvokeUnary`
- `CallStartStream`
- `CallSendMessage`
- `CallHalfClose`
- `CallCancel`
- `HistoryList`
- `HistoryGet`
- `DiagnosticsExport`

## Live event names

- `stream:state`
- `stream:event`
- `stream:error`
- `stream:completed`
- `diagnostics:update`

## Streaming behavior

- `CallStartStream` covers the current streaming slices without adding new Wails bindings.
- `server_stream` starts a live receive loop from one request message and emits headers, received messages, trailers and completion.
- `client_stream` currently supports `requestSpec.mode = static-sequence`: the backend sends every JSON message in order, half-closes the client send side, then waits for one response, status and trailers.
- `CallSendMessage` and `CallHalfClose` remain reserved contract names for the later interactive client/bidi streaming slices.
- History summaries and JSONL session logs are written for unary, server-streaming and client-streaming static sequence calls.

## Serialization rules

- Go runtime сериализует JSON-friendly DTO и не передает protobuf runtime types во frontend.
- Bounded операции используют envelope `ok + data|error`.
- Long-lived updates идут только через event bus.
- Frontend may localize human-facing labels/copy from stable ids, but raw event names, diagnostic codes and RPC enum values remain machine contract values.

## Source of truth

- backend: [internal/contracts/runtime_contract.go](/Users/rammbloor/GolandProjects/tether/internal/contracts/runtime_contract.go)
- frontend mirror: [frontend/src/lib/contracts.ts](/Users/rammbloor/GolandProjects/tether/frontend/src/lib/contracts.ts)
