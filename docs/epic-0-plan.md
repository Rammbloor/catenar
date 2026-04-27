# Epic 0 Execution Plan

## Scope

Этот репозиторий реализует backlog начиная с `Slice 0.1` и фиксирует продовую основу для следующих epics.

## Current status

- `Slice 0.1` — app shell, Wails binding bridge и packaging/CI smoke подготовлены.
- `Slice 0.2` — контракт runtime/frontend v1 зафиксирован в коде и документации.
- `Slice 0.3` — state/error model зафиксированы в shared contract и UI shell state.

## Delivery rules

- не переходим к следующему slice, пока текущий не закрыт проверками;
- binding/event/state identifiers считаются contract surface и меняются только осознанно;
- временные demo-only обходы в кодовой базе не допускаются.

## Verification gates

- `npm run check`
- `npm run test`
- `npm run build`
- `go test ./...`
- `wails build -clean`

## Next slices after Epic 0

1. `Slice 1.1` — endpoint and TLS model
2. `Slice 1.2` — reflection-based exploration
3. `Slice 1.3` — unary invoke flow
