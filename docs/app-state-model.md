# App State Model

## Top-level views

- `home`
- `workspace`
- `session`

## Overlays

- `history-overlay`
- `settings-overlay`
- `diagnostics-overlay`

## Primary flow

`home -> workspace -> session`

## Stream session rules

- canonical states: `idle`, `connecting`, `open`, `half_closed_local`, `half_closed_remote`, `closed`, `cancelled`, `error`
- terminal states: `closed`, `cancelled`, `error`
- conditions: `truncated`
- в MVP поддерживается только одна active live interactive session

## Source of truth

- backend: [internal/contracts/runtime_contract.go](/Users/rammbloor/GolandProjects/tether/internal/contracts/runtime_contract.go)
- frontend store: [frontend/src/lib/state/app-shell.ts](/Users/rammbloor/GolandProjects/tether/frontend/src/lib/state/app-shell.ts)
