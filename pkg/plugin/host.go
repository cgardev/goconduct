package plugin

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/pkg/failure"
)

// Host owns one validated plugin registry and its shared injector.
// It registers all services before it activates any plugin.
type Host struct {
	injector do.Injector
	plugins  []Plugin
}

// NewHost validates plugins and builds their shared dependency graph.
func NewHost(baseServices func(do.Injector), plugins ...Plugin) (*Host, error) {
	if baseServices == nil {
		return nil, failure.Validation("base services are nil", nil)
	}
	registrations := make([]func(do.Injector), 0, len(plugins)+1)
	registrations = append(registrations, baseServices)
	names := make(map[string]struct{}, len(plugins))
	for _, candidate := range plugins {
		if candidate == nil {
			return nil, failure.Validation("plugin is nil", nil)
		}
		name := candidate.Name()
		if name == "" {
			return nil, failure.Validation("plugin name is empty", nil)
		}
		if strings.TrimSpace(name) != name {
			return nil, failure.Validation(
				fmt.Sprintf("plugin name %q contains surrounding whitespace", name),
				nil,
			)
		}
		if _, duplicate := names[name]; duplicate {
			return nil, failure.Duplicate("plugin", name, nil)
		}
		names[name] = struct{}{}
		services := candidate.Services()
		if services == nil {
			return nil, failure.Validation(fmt.Sprintf("plugin %q returned nil services", name), nil)
		}
		registrations = append(registrations, services)
	}
	return &Host{
		injector: do.New(registrations...),
		plugins:  slices.Clone(plugins),
	}, nil
}

// Injector returns the shared dependency graph.
func (host *Host) Injector() do.Injector {
	return host.injector
}

// Activate starts plugins in declaration order.
func (host *Host) Activate(ctx context.Context) error {
	for _, candidate := range host.plugins {
		if err := candidate.Activate(ctx, host.injector); err != nil {
			return fmt.Errorf("activate plugin %q: %w", candidate.Name(), err)
		}
	}
	return nil
}

// RegisterCommands lets plugins extend one Cobra command.
func (host *Host) RegisterCommands(root *cobra.Command) error {
	if root == nil {
		return failure.Validation("root command is nil", nil)
	}
	for _, candidate := range host.plugins {
		if err := candidate.RegisterCommands(host.injector, root); err != nil {
			return fmt.Errorf("register commands for plugin %q: %w", candidate.Name(), err)
		}
	}
	return nil
}

// RegisterEndpoints lets plugins extend one HTTP registrar.
func (host *Host) RegisterEndpoints(
	registrar EndpointRegistrar,
	options ...connect.HandlerOption,
) error {
	if registrar == nil {
		return failure.Validation("endpoint registrar is nil", nil)
	}
	for _, candidate := range host.plugins {
		if err := candidate.RegisterEndpoints(host.injector, registrar, options...); err != nil {
			return fmt.Errorf("register endpoints for plugin %q: %w", candidate.Name(), err)
		}
	}
	return nil
}

// Shutdown stops instantiated services in reverse dependency order.
func (host *Host) Shutdown() error {
	return shutdownReport(host.injector.Shutdown())
}

// ShutdownWithContext bounds service shutdown with a caller deadline.
func (host *Host) ShutdownWithContext(ctx context.Context) error {
	return shutdownReport(host.injector.ShutdownWithContext(ctx))
}

func shutdownReport(report *do.ShutdownReport) error {
	if report == nil || report.Succeed {
		return nil
	}
	return failure.Internal("shut down services", errors.Join(shutdownCauses(report)...))
}

// shutdownCauses orders the per-service errors so one report stays deterministic.
func shutdownCauses(report *do.ShutdownReport) []error {
	descriptions := make([]do.ServiceDescription, 0, len(report.Errors))
	for description := range report.Errors {
		descriptions = append(descriptions, description)
	}
	slices.SortFunc(descriptions, compareServiceDescription)
	causes := make([]error, 0, len(descriptions))
	for _, description := range descriptions {
		causes = append(causes, fmt.Errorf(
			"%s > %s: %w",
			description.ScopeName,
			description.Service,
			report.Errors[description],
		))
	}
	return causes
}

func compareServiceDescription(left, right do.ServiceDescription) int {
	if comparison := cmp.Compare(left.ScopeName, right.ScopeName); comparison != 0 {
		return comparison
	}
	return cmp.Compare(left.Service, right.Service)
}
