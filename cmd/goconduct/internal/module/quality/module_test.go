package quality

import "testing"

func TestPluginUsesStableNameAndServices(t *testing.T) {
	candidate := Plugin()
	if candidate.Name() != "quality" {
		t.Fatalf("plugin name is %q", candidate.Name())
	}
	if candidate.Services() == nil {
		t.Fatal("plugin services are nil")
	}
}
