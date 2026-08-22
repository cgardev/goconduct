package quality

import (
	"context"
	"slices"

	"github.com/samber/do/v2"

	"github.com/cgardev/goconduct/plugin"
)

// ListPluginsUseCase returns the active evaluator catalog.
type ListPluginsUseCase struct {
	catalog *plugin.Catalog
}

// ListPluginsUseCaseParams reserves the stable use-case input shape.
type ListPluginsUseCaseParams struct{}

func newListPluginsUseCaseInjector() func(do.Injector) {
	return do.Lazy[*ListPluginsUseCase](func(injector do.Injector) (*ListPluginsUseCase, error) {
		catalog, err := do.Invoke[*plugin.Catalog](injector)
		if err != nil {
			return nil, err
		}
		return NewListPluginsUseCase(catalog), nil
	})
}

// NewListPluginsUseCase creates a plugin catalog query.
func NewListPluginsUseCase(catalog *plugin.Catalog) *ListPluginsUseCase {
	return &ListPluginsUseCase{catalog: catalog}
}

// Execute returns stable evaluator names.
func (useCase *ListPluginsUseCase) Execute(
	ctx context.Context,
	_ ListPluginsUseCaseParams,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return useCase.catalog.Names(), nil
}

// RunCheckUseCase executes selected evaluators over one repository scope.
type RunCheckUseCase struct {
	catalog       *plugin.Catalog
	configuration Configuration
}

// RunCheckUseCaseParams selects evaluator and repository scopes.
type RunCheckUseCaseParams struct {
	RepositoryRoot string
	Plugins        []string
	Paths          []string
}

func newRunCheckUseCaseInjector() func(do.Injector) {
	return do.Lazy[*RunCheckUseCase](func(injector do.Injector) (*RunCheckUseCase, error) {
		catalog, err := do.Invoke[*plugin.Catalog](injector)
		if err != nil {
			return nil, err
		}
		configuration, err := do.Invoke[Configuration](injector)
		if err != nil {
			return nil, err
		}
		return NewRunCheckUseCase(catalog, configuration), nil
	})
}

// NewRunCheckUseCase creates a combined plugin evaluator.
func NewRunCheckUseCase(
	catalog *plugin.Catalog,
	configuration Configuration,
) *RunCheckUseCase {
	return &RunCheckUseCase{catalog: catalog, configuration: cloneConfiguration(configuration)}
}

// Execute runs plugins in deterministic catalog order.
func (useCase *RunCheckUseCase) Execute(
	ctx context.Context,
	params RunCheckUseCaseParams,
) (CheckResult, error) {
	repositoryRoot := params.RepositoryRoot
	if repositoryRoot == "" {
		repositoryRoot = useCase.configuration.RepositoryRoot
	}
	plugins := slices.Clone(params.Plugins)
	if len(plugins) == 0 {
		plugins = slices.Clone(useCase.configuration.Plugins)
	}
	paths := slices.Clone(params.Paths)
	if len(paths) == 0 {
		paths = slices.Clone(useCase.configuration.Paths)
	}
	reports, err := useCase.catalog.Evaluate(ctx, plugins, plugin.Request{
		RepositoryRoot: repositoryRoot,
		Paths:          paths,
	})
	if err != nil {
		return CheckResult{}, err
	}
	return newCheckResult(reports), nil
}
