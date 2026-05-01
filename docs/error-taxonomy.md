# Error Taxonomy

## Categories

- `transport`
- `grpc_status`
- `reflection`
- `proto`
- `validation`
- `workspace`
- `application`
- `cancelled`

## UI handling rules

- `validation` сначала живет inline рядом с формой
- `transport` и `reflection` поднимаются в diagnostics panel
- `grpc_status` живет и в response summary, и в diagnostics
- `cancelled` отображается как нейтральный terminal outcome
- streaming failures keep raw diagnostic/error codes such as `grpc_status.invalid_argument` as machine values; localized UI copy is mapped separately from the stable code

## Source of truth

- backend: [internal/contracts/runtime_contract.go](/Users/rammbloor/GolandProjects/tether/internal/contracts/runtime_contract.go)
- frontend diagnostics rendering: [frontend/src/App.svelte](/Users/rammbloor/GolandProjects/tether/frontend/src/App.svelte)
