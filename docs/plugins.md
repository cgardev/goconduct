# Plugin authoring

A `goconduct` plugin is a normal Go package.
It can expose an evaluator only or implement the complete application lifecycle.

## Evaluator contract

An evaluator has one stable name and one deterministic operation:

```go
type Evaluator interface {
    Name() string
    Evaluate(context.Context, Request) (Report, error)
}
```

The request contains a repository root and optional paths.
The evaluator returns metrics and policy findings through `plugin.NewReport`.

Use stable identifiers for metrics and findings.
Do not include timestamps, temporary paths, random values, or map iteration order.

## Recommended package shape

```text
plugin/example/
  configuration.go
  evaluator.go
  evaluator_test.go
  module.go
  module_test.go
```

`configuration.go` owns typed settings and safe defaults.
`evaluator.go` owns direct evaluation and normalized evidence.
`module.go` owns DI registration, lifecycle adaptation, and the Cobra command.

Add parser files only when an external tool has a separate structured output format.

## Direct use

The exported constructor takes all collaborators explicitly.
It must not accept `do.Injector` or read global state.

```go
evaluator, err := example.NewEvaluator(
    plugin.NewCommandRunner(),
    example.DefaultConfiguration(),
)
if err != nil {
    return err
}

report, err := evaluator.Evaluate(ctx, plugin.Request{
    RepositoryRoot: ".",
    Paths:          []string{"internal/module"},
})
```

Inject `plugin.CommandRunner` when the evaluator invokes another executable.
Tests can provide a deterministic runner without creating child processes.

## Joint use

Register evaluators in one catalog:

```go
catalog := plugin.NewCatalog()
for _, evaluator := range evaluators {
    if err := catalog.Register(evaluator); err != nil {
        return err
    }
}

reports, err := catalog.Evaluate(ctx, selectedNames, request)
```

An empty selection runs all registered evaluators.
The catalog sorts names and rejects duplicates.

## Lifecycle adapter

Use `do.Package` for plugin registrations:

```go
var Module = do.Package(newEvaluatorInjector())
```

The injector resolves collaborators and delegates to the public constructor:

```go
func newEvaluatorInjector() func(do.Injector) {
    return do.Lazy[*Evaluator](func(injector do.Injector) (*Evaluator, error) {
        runner, err := do.Invoke[plugin.CommandRunner](injector)
        if err != nil {
            return nil, err
        }
        configuration, err := do.Invoke[Configuration](injector)
        if err != nil {
            configuration = DefaultConfiguration()
        }
        return NewEvaluator(runner, configuration)
    })
}
```

Activation resolves the evaluator and registers it in the shared catalog.
Command registration resolves only services required to construct commands.
Endpoint registration mounts generated Connect handlers through `EndpointRegistrar`.
Pass every received `connect.HandlerOption` to each generated handler.

The public host composes several lifecycle adapters:

```go
baseServices := func(injector do.Injector) {
    do.ProvideValue(injector, slog.Default())
    do.ProvideValue(injector, plugin.NewCatalog())
    do.ProvideValue[plugin.CommandRunner](injector, plugin.NewCommandRunner())
}

host, err := plugin.NewHost(baseServices, plugins...)
if err != nil {
    return err
}

if err := host.Activate(ctx); err != nil {
    return err
}

// Register commands and endpoints, then run the application.

if err := host.Shutdown(); err != nil {
    return err
}
```

The host registers all service packages before it activates any plugin.
This order lets one plugin depend on another plugin's public contract.
The host also forwards shared Connect options and owns injector shutdown.

## Configuration rules

- Use named types for closed discriminators.
- Validate every selected variant during construction.
- Copy slices and maps before storing them.
- Reject unknown values and ambiguous path policies.
- Keep external executable names configurable.
- Keep mutation execution disabled by default.

## Evidence rules

- Use one schema version across built-in evaluators.
- Use repository-relative paths with forward slashes.
- Report numeric measurements as metrics.
- Report policy failures as findings.
- Include actual and limit values when a numeric threshold fails.
- Use `notice`, `warning`, or `error` severity.
- Sort tool output before creating a report.

## Verification

Every plugin needs unit tests for configuration, parsing, evidence, and failure behavior.
Add an integration test when the plugin executes a real local tool.

Run these checks before contributing:

```sh
go fmt ./...
go vet ./...
go test ./...
```
