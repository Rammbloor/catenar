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

## Serialization rules

- Go runtime сериализует JSON-friendly DTO и не передает protobuf runtime types во frontend.
- Bounded операции используют envelope `ok + data|error`.
- Long-lived updates идут только через event bus.

## Source of truth

- backend: [internal/contracts/runtime_contract.go](/Users/rammbloor/GolandProjects/tether/internal/contracts/runtime_contract.go)
- frontend mirror: [frontend/src/lib/contracts.ts](/Users/rammbloor/GolandProjects/tether/frontend/src/lib/contracts.ts)
