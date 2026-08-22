package injection

import (
	"errors"
	"testing"

	"github.com/samber/do/v2"
)

func TestResolveReturnsOneRegisteredService(t *testing.T) {
	injector := do.New()
	do.ProvideValue(injector, "ready")

	var err error
	service := Resolve[string](injector, &err)
	if err != nil || service != "ready" {
		t.Fatalf("resolve service: service=%q, error=%v", service, err)
	}
}

func TestResolveStoresFirstErrorAndSkipsLaterResolution(t *testing.T) {
	sentinel := errors.New("service failed")
	injector := do.New()
	do.Provide(injector, func(do.Injector) (string, error) {
		return "", sentinel
	})
	do.ProvideValue(injector, 42)

	var err error
	if service := Resolve[string](injector, &err); service != "" || !errors.Is(err, sentinel) {
		t.Fatalf("resolve failing service: service=%q, error=%v", service, err)
	}
	if service := Resolve[int](injector, &err); service != 0 || !errors.Is(err, sentinel) {
		t.Fatalf("resolve skipped service: service=%d, error=%v", service, err)
	}
}
