# goconduct

Deterministic quality and architecture guardrails for AI-assisted Go engineering.

> [!WARNING]
> `goconduct` is experimental alpha software.
> Its configuration, reports, and public API can change before the first stable release.

`goconduct` is a Go-only verification engine.
It turns versioned engineering rules into repeatable evidence for developers and coding agents.

The name combines Go with *conduct*: directing work and defining the behavior expected from that work.

## Why this project exists

AI code generation is probabilistic.
Software acceptance can still be deterministic.

Google explains why Go supports this model in
[Why Go is an ideal language for AI-assisted software engineering](https://developers.googleblog.com/why-go-is-an-ideal-language-for-ai-assisted-software-engineering/).
The language has a compact specification, stable tooling, explicit dependencies, and fast feedback.

Robert C. Martin describes a related verification workflow in
[Uncle Bob on Software Fundamentals in the age of AI](https://www.youtube.com/live/zcLPGC-tvgk).
That workflow uses deterministic checks to verify probabilistically generated code:

- CRAP scores combine test coverage and cyclomatic complexity.
- Mutation testing verifies that tests detect behavioral changes.
- Acceptance tests verify observable behavior.
- Architecture rules verify allowed dependency directions.
- Interactive views support human design review.

`goconduct` provides one extensible engine for those checks.
It keeps every result explicit, ordered, and suitable for an automated repair loop.

## Implemented alpha capabilities

- A public Go plugin contract with dependency injection, commands, activation, and HTTP endpoints.
- Independently usable evaluator packages under `plugin/<name>`.
- Joint evaluator execution through a deterministic `plugin.Catalog`.
- Go import and statically resolved function-call graphs.
- Configurable component classification for applications, modules, libraries, infrastructure, and tools.
- Configurable dependency grants and prohibitions with default allow or default deny behavior.
- Production and test dependency separation.
- Component and function cycle detection.
- Afferent coupling, efferent coupling, instability, abstractness, and main-sequence distance.
- Direct and transitive dependency metrics.
- Go statement coverage with path-specific limits.
- CRAP analysis through `crap4go` with global and path-specific limits.
- Duplication analysis through `dry4go`.
- Mutation-site scanning and mutation execution through `mutate4go`.
- Normalized, versioned, and deterministically ordered reports.
- A Protocol Buffer API implemented with Connect RPC.
- An Angular dashboard compiled into and served from the Go binary.

Gherkin acceptance checks and executable system-test plugins remain future work.

## Install

Install the current alpha from `main`:

```sh
go install github.com/cgardev/goconduct/cmd/goconduct@main
```

The project does not publish stable releases yet.

## Quick start

List the evaluators linked into the binary:

```sh
goconduct plugins
```

Run the configured evaluator set:

```sh
goconduct check --repository . --indent
```

Run selected evaluators:

```sh
goconduct check \
  --plugin architecture \
  --plugin coverage \
  --repository . \
  --fail-on error
```

Start the local dashboard and Connect RPC server:

```sh
goconduct --root .
```

Open <http://127.0.0.1:6062> while the process runs.
The default address accepts local connections only.

## Built-in plugins

| Plugin | Default behavior | External tool |
| --- | --- | --- |
| `architecture` | Analyzes imports, calls, cycles, coupling, and dependency policy. | None |
| `coverage` | Runs Go tests and reads statement coverage. | `go` |
| `crap` | Measures function risk and applies CRAP limits. | `crap4go` |
| `duplication` | Reports structural duplicate candidates. | `dry4go` |
| `mutation` | Scans mutation sites. Execution requires explicit configuration. | `mutate4go` |

Each plugin also provides its own command:

```sh
goconduct coverage --repository . --minimum 80
goconduct crap --repository . --maximum 8
goconduct duplication --repository . --maximum 0
goconduct mutation --repository . plugin
```

Use each command's `--help` output for its complete option set.

## Architecture queries

Write the architecture summary:

```sh
goconduct analyze --cache local --indent
```

Inspect components and functions:

```sh
goconduct components --cache local --sort afferent --limit 10
goconduct functions --cache local --sort incoming-call-sites --limit 10
goconduct component plugin/architecture --cache local
```

When the dashboard runs, another process can query its compatible cache:

```sh
goconduct summary --cache server
goconduct findings --cache server
```

## Configuration

`goconduct` reads an optional `.goconduct.json` document.
Pass `--configuration` to select another document.

The repository includes a complete
[example configuration](./.goconduct.example.json).
Generate the authoritative JSON Schema with:

```sh
goconduct configuration-schema > goconduct.schema.json
```

### Dependency policy

A dependency policy selects components by identifier, role, category, or application.
Prohibitions override grants.
The analyzer rejects stale selectors and duplicate rule identifiers.

```json
{
  "architecture": {
    "dependencies": {
      "productionDefault": "deny",
      "testDefault": "allow",
      "allow": [
        {
          "id": "application-modules-use-libraries",
          "from": {"roles": ["application-module"]},
          "to": {"roles": ["library"]},
          "reason": "Application modules consume reusable libraries."
        }
      ],
      "deny": []
    }
  }
}
```

Unmatched relationships follow the separate production and test defaults.
Use `deny` only after the policy contains every intended relationship.

### Path-specific quality limits

Coverage and CRAP policies use repository-relative patterns.
`**` matches across directory boundaries.
One path and metric must match at most one policy.

```json
{
  "quality": {
    "coverage": {
      "command": "go",
      "packages": ["./..."],
      "pathPolicies": [
        {
          "id": "domain-coverage",
          "include": ["internal/module/**"],
          "thresholds": [
            {
              "metric": "coverage.percent",
              "comparison": "minimum",
              "value": 100,
              "severity": "error"
            }
          ]
        }
      ]
    }
  }
}
```

## Use plugins as Go packages

Every built-in plugin is a regular Go package.
No plugin requires dynamic loading or a Go shared object.

Use the architecture evaluator independently:

```go
evaluator := architecture.NewEvaluator(slog.Default())
report, err := evaluator.Evaluate(ctx, plugin.Request{
    RepositoryRoot: ".",
    Paths:          []string{"cmd", "internal", "plugin"},
})
```

Quality plugins accept the public command runner:

```go
evaluator, err := coverage.NewEvaluator(
    plugin.NewCommandRunner(),
    coverage.DefaultConfiguration(),
)
```

Compose several evaluators through one catalog:

```go
catalog := plugin.NewCatalog()

if err := catalog.Register(architecture.NewEvaluator(slog.Default())); err != nil {
    return err
}
if err := catalog.Register(coverageEvaluator); err != nil {
    return err
}

reports, err := catalog.Evaluate(ctx, nil, plugin.Request{RepositoryRoot: "."})
```

Passing no evaluator names runs every registered evaluator in stable name order.
The constructors validate configuration and defensively copy mutable inputs.

Use each package's `Plugin()` adapter when building a complete application host.
The adapter contributes lazy services, lifecycle activation, Cobra commands, and Connect endpoints.

See [Plugin authoring](./docs/plugins.md) for the complete contract.

## API and web application

Protocol Buffer definitions are the sole transport source under `api/proto/v1`.
Generated Go and TypeScript code is never edited by hand.

The binary exposes these Connect services:

- `GraphService` provides architecture summaries, components, relationships, functions, calls, and live updates.
- `QualityService` lists evaluators and runs normalized combined checks.

The Angular application consumes generated TypeScript clients.
Its production build is copied into `plugin/architecture/_resources/web` and embedded with `go:embed`.
The user interface uses neutral colors, system typography, LESS, BEM, and Taiga UI components.

## Repository structure

```text
api/proto/v1/                         Protocol Buffer contracts
cmd/goconduct/                        Cobra composition root
cmd/goconduct/internal/configuration  Unified application configuration
cmd/goconduct/internal/module/quality Application-owned Connect module
internal/appmodule/                   Plugin host and request scope
internal/kernel/                      Shared dependency registrations
internal/library/                     Shared application infrastructure
internal/protogen/                    Generated Go transport code
plugin/                               Public plugin SDK and evidence model
plugin/architecture/                  Architecture evaluator and embedded dashboard
plugin/coverage/                      Coverage evaluator
plugin/crap/                          CRAP evaluator
plugin/duplication/                   Duplication evaluator
plugin/mutation/                      Mutation evaluator
policy/                               Public path-policy resolver
web/projects/app-goconduct/           Angular application
web/projects/lib-api-gen/             Generated TypeScript transport code
```

The composition root lists plugins only.
Each plugin owns its registrations, activation, commands, and endpoints.
See [Architecture](./docs/architecture.md) for dependency directions and startup behavior.

## Deterministic behavior

`goconduct` applies these constraints to machine-facing output:

- Every report declares a schema version.
- Metrics and findings use stable identifiers.
- Collections use explicit deterministic ordering.
- Reports preserve paths, actual values, limits, and severities.
- Configuration rejects unknown fields and ambiguous path policies.
- Child tools receive explicit arguments without an intermediate command shell.
- Architecture policy changes affect the graph revision and cache identity.

External tool versions can affect their measurements.
Pin those tools when comparing reports across environments.

## Development

Verify Go code:

```sh
go fmt ./...
go vet ./...
go test ./...
```

Verify and regenerate Protocol Buffer code:

```sh
cd api/proto
buf lint
buf generate
```

Verify and embed the Angular application:

```sh
cd web
pnpm install --frozen-lockfile
pnpm test:app-goconduct
pnpm build:app-goconduct
```

Do not edit generated transport code or embedded web assets manually.

## Roadmap

- Add Gherkin acceptance and deterministic system-test plugins.
- Report managed packages that remain unclassified.
- Add expiring architecture waivers with owners and reasons.
- Add changed-code and committed-baseline comparisons.
- Classify external dependencies through named policy groups.
- Add release compatibility guarantees after the alpha period.

## License

`goconduct` is available under the MIT License.
The embedded dashboard publishes dependency notices at `/3rdpartylicenses.txt`.
