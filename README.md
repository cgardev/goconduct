# goconduct

Deterministic quality and architecture guardrails for AI-assisted Go engineering.

> [!WARNING]
> `goconduct` is experimental alpha software.
> Its configuration, reports, and command surface can change without compatibility guarantees.

`goconduct` is a Go-only verification engine.
It turns engineering rules into repeatable checks for humans and coding agents.

The name combines Go with *conduct*: to direct an activity and to define expected behavior.

## Why goconduct exists

AI code generation is probabilistic.
Software acceptance should be deterministic.

The Google Developers article
[Why Go is an ideal language for AI-assisted software engineering](https://developers.googleblog.com/why-go-is-an-ideal-language-for-ai-assisted-software-engineering/)
describes why Go fits this workflow.
Go provides simple syntax, strong compatibility, readable code, and integrated deterministic tools.

Robert C. Martin presents a related workflow in
[Uncle Bob on Software Fundamentals in the age of AI](https://www.youtube.com/live/zcLPGC-tvgk).
The workflow evaluates generated software with executable evidence:

1. **CRAP score** combines test coverage and cyclomatic complexity.
   Martin describes a human limit below 4 and agent limits of 6 or possibly 8.
2. **Mutation testing** changes covered expressions and expects relevant tests to fail.
   The target is zero surviving mutations.
3. **Acceptance and system tests** verify Gherkin scenarios and automated end-to-end procedures.
4. **Architecture rules** declare valid dependency directions and reject every invalid relationship.
5. **Architecture views** expose dependencies at several levels for human review.
6. **Short iterations** let people inspect and reshape the design before accidental structure grows.

These thresholds are policy examples.
Each repository must calibrate its own limits and record them in version control.

## Current alpha

The current alpha provides the architecture foundation:

- Go package and component discovery.
- Configurable component classification through path templates.
- Production and test import relationships with source locations.
- Statically resolved function calls and call sites.
- Component and function dependency cycles.
- Afferent and efferent coupling.
- Instability, abstractness, and distance from the main sequence.
- Direct and transitive dependency metrics.
- Deterministic architecture findings.
- Stable JSON output for command-line consumers.
- An embedded interactive dependency dashboard.
- A compatible local dashboard cache for fast queries.

The current alpha does not yet provide the plugin runtime.
It also does not yet execute `crap4go`, `mutate4go`, `dry4go`, or Gherkin tests.

## Target verification model

`goconduct` will use one normalized evidence model.
Built-in checks and external plugins will contribute evidence to that model.

The planned pipeline has four deterministic stages:

1. Collect facts from Go source, tests, coverage profiles, and external analyzers.
2. Normalize facts into versioned records with stable identifiers.
3. Evaluate repository policies against those records.
4. Emit ordered findings for people, coding agents, and continuous integration.

Planned integrations include:

| Integration | Evidence |
| --- | --- |
| `crap4go` | CRAP score, coverage, and cyclomatic complexity per function. |
| `mutate4go` | Killed, survived, skipped, and invalid mutations. |
| `dry4go` | Structurally similar Go functions and likely duplication. |
| Go coverage | Coverage per package, file, function, and configured path. |
| Acceptance checks | Gherkin scenarios and executable system procedures. |
| Architecture checks | Allowed imports, prohibited imports, cycles, and coupling limits. |

The external plugin protocol will use versioned JSON over child processes.
This approach avoids the platform and toolchain limits of Go runtime plugins.
Built-in plugins will implement the same logical contract through Go interfaces.

## Install the alpha

Install the current `main` branch:

```sh
go install github.com/cgardev/goconduct@main
```

The project does not publish stable releases yet.

## Analyze a repository

Write a deterministic report:

```sh
goconduct analyze --root . --fail-on error
```

Write the complete graph:

```sh
goconduct analyze --root . --view graph --indent
```

Start the local dashboard:

```sh
goconduct --root .
```

Open <http://127.0.0.1:6062> while the command remains active.

Query the active graph from another terminal:

```sh
goconduct summary --cache server
goconduct components --cache server --sort afferent --limit 10
goconduct functions --cache server --sort incoming-call-sites --limit 10
```

Inspect one function:

```sh
goconduct function internal/library/logging.NewLogger --cache server
```

List resolved calls between two components:

```sh
goconduct calls \
  --cache server \
  --source-component cmd/control \
  --target-component internal/library/logging
```

## Current configuration

`goconduct` reads the optional `.goconduct.json` file from the current directory.
Use `--configuration` to select another file.

```json
{
  "server": {
    "address": "127.0.0.1:6062",
    "refreshInterval": "750ms"
  },
  "cache": {
    "mode": "auto",
    "requestTimeout": "2s"
  },
  "analysis": {
    "repositoryRoot": ".",
    "paths": ["cmd", "internal"],
    "ignoredPaths": ["vendor", "generated", "target"],
    "components": {
      "applications": ["cmd/{application}"],
      "applicationModules": ["cmd/{application}/internal/module/{component}"],
      "sharedModules": ["internal/module/{component}"],
      "libraries": ["internal/library/{component}"],
      "infrastructure": ["internal/{component}"],
      "developmentTools": ["internal/devtool/{component}"]
    }
  }
}
```

Print the current JSON Schema:

```sh
goconduct configuration-schema
```

The current architecture registry evaluates these rules:

- Production dependency cycles.
- Source analysis failures.
- Dependencies on less stable components.
- Stable components with low abstraction.
- Production imports from development tools.
- Libraries importing application features.
- Shared components importing application code.
- Imports between modules from different applications.

## Planned path policies

The plugin configuration below describes the intended direction.
The current alpha does not accept this section yet.

```json
{
  "quality": {
    "pathPolicies": [
      {
        "include": ["internal/domain/**"],
        "coverage": {"minimumPercent": 100},
        "crap": {"maximumScore": 6},
        "mutation": {"minimumKilledPercent": 100}
      },
      {
        "include": ["cmd/**"],
        "coverage": {"minimumPercent": 85},
        "crap": {"maximumScore": 8}
      }
    ]
  },
  "plugins": [
    {"id": "crap4go", "command": "crap4go"},
    {"id": "mutate4go", "command": "mutate4go"},
    {"id": "dry4go", "command": "dry4go"}
  ]
}
```

Every policy will identify its selected paths, metric, limit, severity, and optional expiry.
The engine will reject ambiguous path policies and unknown plugin outputs.

## Architecture

The source separates deterministic logic from adapters:

- `internal/report` owns the transport-neutral evidence model.
- `internal/calculation` owns pure graph calculations and formulas.
- `internal/query` owns deterministic report projections.
- `internal/architecture` owns independent architecture rules.
- `internal/application` selects local analysis or a compatible graph cache.
- Root files provide Go analysis, Cobra commands, HTTP adapters, and composition.
- `_resources/web` contains the embedded dashboard.

The pure packages do not import Cobra, HTTP, or presentation code.

## Development

Run the verification suite:

```sh
go test ./...
go vet ./...
```

Run `gofmt` before every contribution that changes Go code.

## Roadmap

- Declare allowed dependency graphs with default-deny rules.
- Report unclassified and ambiguously classified packages.
- Add expiring architecture waivers with reasons and owners.
- Add versioned executable plugins and capability manifests.
- Add path-specific coverage, CRAP, mutation, and duplication limits.
- Add changed-code policies for incremental adoption.
- Add Gherkin and end-to-end acceptance evidence.
- Export machine-readable results for coding-agent repair loops.
- Compare current evidence with a committed baseline.
- Preserve deterministic output across machines and repeated runs.

## License

`goconduct` is available under the MIT License.
