# Dependency Graph

This development tool analyzes Go components, imports, resolved function calls, and coupling. The
tool reports architecture findings.

Run all commands from the repository root. Use the Go version declared in the root `go.mod` file.

## Architecture

The tool has these explicit layers and adapters:

- `internal/report` contains the transport-neutral analysis report.
- `internal/calculation` contains pure deterministic graph calculations and formulas.
- `internal/query` selects deterministic report views for CLI consumers.
- `internal/architecture` contains strategic roles and independent architecture rules.
- `internal/application` selects a local source or a compatible graph cache.
- `application.go` adapts the local Go analyzer to the application ports.
- `runtime.go` is the composition root for the CLI and dashboard. It receives each factory and logger through
  its constructor.
- `internal/failure` owns the tool error categories. The tool does not import shared project libraries.
- The other root Go files provide the analyzer, Cobra adapter, cache adapter, and HTTP adapter.
- `_resources/web` contains the embedded presentation resources.

The pure packages do not import Cobra, HTTP, or presentation code. The HTTP adapter depends on graph
reader, refresh, subscription, and cache identity ports. The analyzer passes the request context to
`packages.Load`.

The tool is not a business module or an `appmodule.Plugin`. Therefore, it does not register a `do.Package`.
`main.go` constructs the standalone runtime and injects it into the Cobra adapter. Business modules continue
to use the repository `do.Package` and injector pairs.

## Start the dashboard

Run the tool without a subcommand:

```sh
go run ./internal/devtool/dependencygraph
```

Open <http://127.0.0.1:6062> in a web browser. Keep the command active while you use the dashboard.

Use `Ctrl+C` to stop the server.

## Query the active graph

Open a second terminal while the server is active. The following command reads the cached graph:

```sh
go run ./internal/devtool/dependencygraph summary --cache server
```

List the components with the highest afferent coupling:

```sh
go run ./internal/devtool/dependencygraph components \
  --cache server \
  --role library \
  --sort afferent \
  --limit 10
```

Select one configured presentation category:

```sh
go run ./internal/devtool/dependencygraph components \
  --cache server \
  --category plugin \
  --sort afferent
```

List the functions with the most incoming call sites:

```sh
go run ./internal/devtool/dependencygraph functions \
  --cache server \
  --sort incoming-call-sites \
  --limit 10
```

Inspect one function and its direct caller functions and callee functions:

```sh
go run ./internal/devtool/dependencygraph function \
  internal/library/logging.NewLogger \
  --cache server
```

List exact calls between two components:

```sh
go run ./internal/devtool/dependencygraph calls \
  --cache server \
  --source-component cmd/cloudcontrol \
  --target-component internal/library/logging \
  --limit 20
```

These commands return JSON. The query options filter the applicable results. No external JSON
filtering tool is necessary.

## Select the analysis scope

Repeat `--analysis-path` to replace the default paths. Repeat `--ignore-path` to replace the
default exclusions.

```sh
go run ./internal/devtool/dependencygraph \
  --analysis-path cmd \
  --analysis-path internal/module \
  --analysis-path internal/library \
  --ignore-path vendor \
  --ignore-path generated
```

Use the same scope parameters for the server and each cached query. A different scope makes the
cache incompatible.

Use local mode for an independent analysis:

```sh
go run ./internal/devtool/dependencygraph functions \
  --analysis-path internal/devtool/dependencygraph \
  --cache local \
  --sort outgoing-call-sites \
  --limit 10
```

## Use a configuration document

The default configuration path is `configuration.json` in the current directory.

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
    "ignoredPaths": ["vendor", "generated"],
    "components": {
      "taxonomy": [
        {
          "id": "plugin",
          "role": "library",
          "paths": ["plugins/{component}"]
        }
      ]
    }
  }
}
```

Each taxonomy entry has a presentation category and a strategic role. The category can be any
non-empty identifier. The role must use one of these values:

- `application`
- `application-module`
- `shared-module`
- `library`
- `infrastructure`
- `development`

Architecture rules use the role. The web presentation uses the category for filters, labels, groups,
and deterministic colors.

Select another document with `--configuration`:

```sh
go run ./internal/devtool/dependencygraph \
  --configuration dependencygraph.json
```

Use `configuration-schema` to print the complete configuration schema:

```sh
go run ./internal/devtool/dependencygraph configuration-schema
```

## Cache modes

- `auto` reads a compatible server cache and uses local analysis when the cache is unavailable.
- `server` requires a compatible server cache and returns an error when the cache is unavailable.
- `local` always calculates the graph in the current process.

## Run the module tests

```sh
go test ./internal/devtool/dependencygraph/...
```
