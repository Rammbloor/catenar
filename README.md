# tether

Desktop-first gRPC GUI client foundations built with `Wails v2 + Go + Svelte 5 + TypeScript + Vite`.

## Epic 0 status

- `Slice 0.1` complete: app shell, layout regions, Wails bindings and `runtime.EventsEmit` diagnostics round-trip.
- `Slice 0.2` complete: runtime/frontend contract manifest, invoke DTO types and module boundaries encoded in shared code.
- `Slice 0.3` complete: shared stream state machine, navigation model and error taxonomy wired into backend and UI shell.

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
