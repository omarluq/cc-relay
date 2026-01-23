// Package di provides dependency injection using samber/do v2.
// It creates and configures the DI container with all service providers.
package di

import (
	"context"
	"fmt"

	"github.com/samber/do/v2"
)

// ConfigPathKey is the named key for the config path string.
// This allows multiple string values in the container.
const ConfigPathKey = "config.path"

// Container wraps the do.Injector with cc-relay specific configuration.
type Container struct {
	injector *do.RootScope
}

// NewContainer creates and configures the DI container.
// The configPath parameter specifies the path to the configuration file.
// All service providers are registered during container creation.
func NewContainer(configPath string) (*Container, error) {
	injector := do.New()

	// Provide config path as a named value
	do.ProvideNamedValue(injector, ConfigPathKey, configPath)

	// Register all service providers
	RegisterSingletons(injector)

	return &Container{
		injector: injector,
	}, nil
}

// Injector returns the underlying do.Injector for service resolution.
func (c *Container) Injector() *do.RootScope {
	return c.injector
}

// Invoke resolves a service from the container.
// Returns an error if the service is not registered or fails to initialize.
func Invoke[T any](c *Container) (T, error) {
	return do.Invoke[T](c.injector)
}

// MustInvoke resolves a service from the container or panics.
// Use this only during application startup where errors are fatal.
func MustInvoke[T any](c *Container) T {
	return do.MustInvoke[T](c.injector)
}

// InvokeNamed resolves a named service from the container.
func InvokeNamed[T any](c *Container, name string) (T, error) {
	return do.InvokeNamed[T](c.injector, name)
}

// MustInvokeNamed resolves a named service from the container or panics.
func MustInvokeNamed[T any](c *Container, name string) T {
	return do.MustInvokeNamed[T](c.injector, name)
}

// Shutdown gracefully shuts down all services in reverse order of initialization.
// Services implementing the do.Shutdowner interface will have their Shutdown method called.
// Returns nil if shutdown succeeded, or an error if any service failed to shut down.
func (c *Container) Shutdown() error {
	report := c.injector.Shutdown()
	if report != nil && !report.Succeed {
		return fmt.Errorf("shutdown failed: %s", report.Error())
	}
	return nil
}

// ShutdownWithContext gracefully shuts down with context for timeout control.
func (c *Container) ShutdownWithContext(ctx context.Context) error {
	done := make(chan *do.ShutdownReport, 1)
	go func() {
		done <- c.injector.ShutdownWithContext(ctx)
	}()

	select {
	case report := <-done:
		if report != nil && !report.Succeed {
			return fmt.Errorf("shutdown failed: %s", report.Error())
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out: %w", ctx.Err())
	}
}

// HealthCheck verifies all registered services can be resolved.
// Returns nil if all services are healthy.
func (c *Container) HealthCheck() error {
	// Try to invoke each registered service type
	// This triggers lazy initialization and catches errors early
	if _, err := do.Invoke[*ConfigService](c.injector); err != nil {
		return fmt.Errorf("config service unhealthy: %w", err)
	}

	// Add more health checks as services are added
	return nil
}
