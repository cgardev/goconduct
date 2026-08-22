package plugin

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
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
		return nil, fmt.Errorf("base services are nil")
	}
	registrations := make([]func(do.Injector), 0, len(plugins)+1)
	registrations = append(registrations, baseServices)
	names := make(map[string]struct{}, len(plugins))
	for _, candidate := range plugins {
		if candidate == nil {
			return nil, fmt.Errorf("plugin is nil")
		}
		name := candidate.Name()
		if name == "" {
			return nil, fmt.Errorf("plugin name is empty")
		}
		if strings.TrimSpace(name) != name {
			return nil, fmt.Errorf("plugin name %q contains surrounding whitespace", name)
		}
		if _, duplicate := names[name]; duplicate {
			return nil, fmt.Errorf("plugin name %q is duplicated", name)
		}
		names[name] = struct{}{}
		services := candidate.Services()
		if services == nil {
			return nil, fmt.Errorf("plugin %q returned nil services", name)
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
		return fmt.Errorf("root command is nil")
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
		return fmt.Errorf("endpoint registrar is nil")
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
	return fmt.Errorf("shut down services: %s", report.Error())
}
