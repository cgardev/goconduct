# Architecture

`goconduct` is one Go module with one executable application and several public plugin packages.
The application composes plugins statically and exposes their commands and transport endpoints.

## Dependency direction

```text
cmd/goconduct
  -> cmd/goconduct/internal/module/quality
  -> internal/appmodule
  -> internal/kernel
  -> plugin/*

plugin/*
  -> plugin
  -> policy

web
  -> generated TypeScript Protocol Buffer clients
  -> Connect RPC
  -> cmd/goconduct/internal/module/quality
  -> plugin.Catalog
```

The executable is the composition root.
It selects plugin packages and maps external configuration to their public configuration types.

The public `plugin` package owns the extension contract, normalized evidence, evaluator catalog, and process boundary.
It does not import any built-in plugin.

Each `plugin/<name>` package owns one quality capability.
The package exposes a direct evaluator constructor and a lifecycle adapter.

The `policy` package owns deterministic path selection and numeric thresholds.
Plugins reuse that package instead of implementing different pattern semantics.

## Application startup

1. The composition root creates the shared injector from `internal/kernel`.
2. The host validates unique plugin names and registers every plugin service package.
3. Cobra parses global configuration flags.
4. The configuration loader reads one optional strict JSON document over safe defaults.
5. The composition root provides typed configuration values to the injector.
6. The host activates plugins in declaration order.
7. Each evaluator registers itself in `plugin.Catalog`.
8. The selected command executes or the shared HTTP server starts.
9. Shutdown stops instantiated services through the dependency container.

The configuration-schema command skips activation because it needs no runtime services.

## Plugin boundary

The lifecycle contract has five responsibilities:

```go
type Plugin interface {
    Name() string
    Services() func(do.Injector)
    Activate(context.Context, do.Injector) error
    RegisterCommands(do.Injector, *cobra.Command) error
    RegisterEndpoints(do.Injector, EndpointRegistrar) error
}
```

`Services` returns lazy dependency registrations.
`Activate` resolves services that must start before requests arrive.
`RegisterCommands` extends the shared Cobra root.
`RegisterEndpoints` mounts handlers on the shared HTTP server.

The composition root never names a plugin's internal services.

## Quality module

The `quality` module belongs to the `goconduct` application.
It is flat and follows the module role taxonomy:

- `module.go` contains DI registration and the lifecycle adapter.
- `domain.go` contains its ports and normalized result types.
- `usecase.go` contains list and check use cases.
- `api.go` maps use cases to generated Connect handlers.
- `configuration.go` owns module configuration.

The module resolves `plugin.Catalog` through request scope.
It does not import the composition root or application configuration package.

## Transport boundary

Files under `api/proto/v1` define the complete web transport.
Buf generates Go handlers and TypeScript clients from those files.

The Connect HTTP server mounts every service on one `http.ServeMux`.
The architecture plugin also mounts the embedded Angular application at `/`.
Specific Connect paths take precedence over that fallback path.

## Embedded web application

Angular builds the `app-goconduct` project into `web/dist/app-goconduct`.
The embed script copies that output into `plugin/architecture/_resources/web`.
The Go compiler then includes those files in the executable.

The production binary does not require Node.js or a separate static-file server.

## Deterministic evidence

Each evaluator returns `plugin.Report`.
The report constructor validates identifiers, values, severities, and duplicates.
It sorts metrics and findings before returning the report.

`plugin.Catalog` sorts selected evaluator names before execution.
The combined command preserves that order.

Architecture analysis hashes the normalized graph into a revision.
Configuration that changes allowed dependencies also changes the cache identity.

## Extension boundaries

Built-in plugins are linked Go packages.
The project does not load Go shared objects at runtime.
This design avoids compiler-version coupling and platform-specific plugin support.

An external process can still provide measurements through an evaluator package.
That evaluator must normalize tool output into the public evidence contract.
