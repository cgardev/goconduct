package appmodule

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"

	"github.com/cgardev/goconduct/plugin"
)

// Host owns one validated plugin registry and shared injector.
type Host struct {
	injector do.Injector
	plugins  []plugin.Plugin
}

// NewHost builds one injector from base and plugin services.
func NewHost(baseServices func(do.Injector), plugins ...plugin.Plugin) (*Host, error) {
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
		if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name {
			return nil, fmt.Errorf("plugin name %q is invalid", name)
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

// Injector returns the shared service container.
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

// RegisterCommands lets plugins extend the application command.
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

// RegisterEndpoints lets plugins extend the shared HTTP server.
func (host *Host) RegisterEndpoints(registrar plugin.EndpointRegistrar) error {
	if registrar == nil {
		return fmt.Errorf("endpoint registrar is nil")
	}
	for _, candidate := range host.plugins {
		if err := candidate.RegisterEndpoints(host.injector, registrar); err != nil {
			return fmt.Errorf("register endpoints for plugin %q: %w", candidate.Name(), err)
		}
	}
	return nil
}

// Shutdown stops instantiated services in reverse dependency order.
func (host *Host) Shutdown() error {
	report := host.injector.Shutdown()
	if report == nil || report.Succeed {
		return nil
	}
	return fmt.Errorf("shut down services: %s", report.Error())
}

// ShutdownWithContext bounds service shutdown with a caller deadline.
func (host *Host) ShutdownWithContext(ctx context.Context) error {
	report := host.injector.ShutdownWithContext(ctx)
	if report == nil || report.Succeed {
		return nil
	}
	return fmt.Errorf("shut down services: %s", report.Error())
}
