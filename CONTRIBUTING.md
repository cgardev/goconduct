# Contributing

`goconduct` is experimental alpha software.
Small changes with deterministic tests are easier to review than broad rewrites.

## Prerequisites

- Go at the version declared in `go.mod`.
- Buf and the Go Protocol Buffer generators for transport changes.
- Node.js and `pnpm` for web changes.
- `crap4go`, `dry4go`, and `mutate4go` for their integration paths.

## Change rules

- Keep the project Go-only outside the Angular frontend and generated TypeScript code.
- Prefer the Go standard library before adding a dependency.
- Add unit and integration tests for changed Go behavior.
- Define every transport change in `api/proto/v1` first.
- Regenerate transport code after changing a `.proto` file.
- Keep plugin constructors independent from the dependency container.
- Keep output identifiers and ordering deterministic.
- Keep the dashboard on system typography and neutral design tokens.
- Do not edit generated code or embedded web bundles manually.

## Verification

Run Go verification:

```sh
go fmt ./...
go vet ./...
go test ./...
```

Run transport verification after a schema change:

```sh
cd api/proto
buf lint
buf generate
```

Run web verification after a frontend change:

```sh
cd web
pnpm install --frozen-lockfile
pnpm test:app-goconduct
pnpm build:app-goconduct
```

Commit generated transport code and embedded web assets with their source changes.

## New plugins

Read [Plugin authoring](./docs/plugins.md) before adding a plugin.
A plugin package must work through its direct evaluator constructor.
Its lifecycle adapter must also work inside the combined application host.
