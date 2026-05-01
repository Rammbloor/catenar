# tether

Desktop-first gRPC GUI client foundations built with `Wails v2 + Go + Svelte 5 + TypeScript + Vite`.

## Epic 0 status

- `Slice 0.1` complete: app shell, layout regions, Wails bindings and `runtime.EventsEmit` diagnostics round-trip.
- `Slice 0.2` complete: runtime/frontend contract manifest, invoke DTO types and module boundaries encoded in shared code.
- `Slice 0.3` complete: shared stream state machine, navigation model and error taxonomy wired into backend and UI shell.

## Epic 1 status

- `Slice 1.1` complete: endpoint/TLS preflight and diagnostics classification for transport readiness.
- `Slice 1.2` complete: reflection-driven service catalog loading with stable `reflection.*` diagnostics and well-known type surfacing.
- `Slice 1.3` complete: reflection-selected unary request templates, end-to-end unary invoke, response panel data, SQLite-backed history summaries and JSONL session log artifacts.

## Epic 2 status

- `Slice 2.1` complete: local proto source loading, import path handling and proto-backed request templates share the same catalog surface as reflection.
- `UX/i18n stabilization` complete: RU/EN language selection covers the app shell, navigation, diagnostics, workspace/session panels and bootstrap-derived copy while preserving raw contract values such as event names and diagnostic codes.

## Epic 3 status

- `Slice 3.1` complete: server-streaming calls run through `CallStartStream`, emit live `stream:*` events and persist history/session log artifacts.
- `Slice 3.2` complete: client-streaming static sequences can send a fixed JSON message list, half-close the client send side, receive one response/status/trailers set and persist the resulting history/session artifacts.

## Local development

1. Install frontend dependencies:

   ```bash
   cd frontend
   npm install
   ```

2. Run the desktop app:

   ```bash
   /Users/rammbloor/go/bin/wails dev
   ```

3. Run verification gates:

   ```bash
   cd frontend && npm run check && npm run test && npm run build
   cd ..
   go test ./...
   /Users/rammbloor/go/bin/wails build -clean
   ```

## Documentation

- [Epic 0 plan](./docs/epic-0-plan.md)
- [Runtime contract v1](./docs/runtime-contract-v1.md)
- [App state model](./docs/app-state-model.md)
- [Error taxonomy](./docs/error-taxonomy.md)
