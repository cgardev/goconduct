// Package injection provides one error-accumulating dependency resolver.
package injection

import "github.com/samber/do/v2"

// Resolve returns one dependency and stores the first resolution error.
func Resolve[Service any](injector do.Injector, resolutionError *error) Service {
	var zero Service
	if *resolutionError != nil {
		return zero
	}
	service, err := do.Invoke[Service](injector)
	if err != nil {
		*resolutionError = err
		return zero
	}
	return service
}
