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

internal/appmodule
  -> plugin.Host

internal/kernel
  -> plugin.Catalog
  -> plugin.CommandRunner

plugin/*
  -> plugin
  -> policy

plugin, policy, plugin/*
  -> failure

failure
  -> the Go standard library only

web
  -> generated TypeScript Protocol Buffer clients
  -> Connect RPC
  -> cmd/goconduct/internal/module/quality
  -> plugin.Catalog
```

The executable is the only composition root.
`modules.go` selects concrete plugins and provides their typed configuration.
`infrastructure.go` composes the kernel, request scope, logger, HTTP server, and health endpoint.
`application.go` enforces activation, endpoint registration, serving, and shutdown order.

The public `plugin` package owns the extension contract, host, normalized evidence, evaluator catalog, and process boundary.
It does not import any built-in plugin.

Each `plugin/<name>` package owns one quality capability.
The package exposes a direct evaluator constructor and a lifecycle adapter.

The `policy` package owns deterministic path selection and numeric thresholds.
Plugins reuse that package instead of implementing different pattern semantics.

## Application startup

1. The root builds `plugin.Host` from the kernel and declared plugins.
2. The host validates names and registers all plugin services.
3. Cobra parses the selected command and global overrides.
4. The loader reads one optional JSON document over safe defaults.
5. The root validates the effective configuration after applying overrides.
6. The root provides typed configuration values to the injector.
7. The host activates every plugin in declaration order.
8. Activation eagerly resolves every request service.
9. The root builds one shared Connect interceptor chain.
10. Plugins register endpoints with that chain.
11. The root mounts `/healthz` and starts the HTTP server.
12. Injector shutdown stops services in reverse dependency order.

The configuration-schema command skips activation because it needs no runtime services.

## Plugin boundary

The lifecycle contract has five responsibilities:

```go
type Plugin interface {
    Name() string
    Services() func(do.Injector)
    Activate(context.Context, do.Injector) error
    RegisterCommands(do.Injector, *cobra.Command) error
    RegisterEndpoints(
        do.Injector,
        EndpointRegistrar,
        ...connect.HandlerOption,
    ) error
}
```

`Services` returns lazy dependency registrations.
`Activate` resolves services that must start before requests arrive.
`RegisterCommands` extends the shared Cobra root.
`RegisterEndpoints` mounts handlers with the root's shared Connect options.

The composition root never names a plugin's internal services.

`plugin.Host` is public because external applications must compose third-party plugins.
`internal/appmodule` aliases that host and adds request-scope resolution for this application.

## Kernel boundary

`internal/kernel` owns only services required by every linked plugin:

- the process logger;
- the deterministic evaluator catalog;
- the external command runner.

The composition root owns `appmodule.SelfScope` because request scope is an application decision.

The kernel does not include a database, transactor, event bus, or transactional outbox.
The current product has no durable state or domain events.
When a persistent module appears, the kernel must own one production database and its shared transaction infrastructure.

## Analysis libraries

`internal/library` holds the measurement code the plugins share. Each package
answers one question and depends on the Go standard library only.

- `gosource` lists the production Go files of one repository scope.
- `gocoverage` answers coverage questions over a Go coverage profile.
- `gocomplexity` reads functions, counts decision points, and scores change risk.
- `gomutation` discovers the expressions one mutation can change.
- `gosimilarity` compares normalized syntax trees.

`plugin/crap`, `plugin/duplication`, and `plugin/mutation` compose these
packages. No plugin starts another analysis tool, so one binary carries every
measurement and no report depends on the output format of a separate program.

## Failure boundary

The public `failure` package owns the closed set of error categories.
It depends on the Go standard library only, so every layer can import it.

Each package classifies its own failures where it creates them.
A caller adds context with `fmt.Errorf` and `%w`, which keeps the category.
`internal/library/connecterror` maps one category to one Connect code at the API boundary.
It logs an unclassified error and returns `Internal`, so no internal detail reaches a client.

The Connect translator is the only package outside `failure` that calls `errors.New`.
It builds the sanitized message that the client receives.
An architecture test rejects any other unclassified error in production code.

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
The root applies validation and structured request logging to every generated handler.
API handlers translate classified failures with one shared translator.
The architecture plugin also mounts the embedded Angular application at `/`.
Specific Connect paths take precedence over that fallback path.
The root-owned `/healthz` endpoint also takes precedence over the fallback.

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
