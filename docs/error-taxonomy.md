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

## Source of truth

- backend: [internal/contracts/runtime_contract.go](/Users/rammbloor/GolandProjects/tether/internal/contracts/runtime_contract.go)
- frontend diagnostics rendering: [frontend/src/App.svelte](/Users/rammbloor/GolandProjects/tether/frontend/src/App.svelte)
