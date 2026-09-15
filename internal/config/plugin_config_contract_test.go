package config

import (
	"reflect"
	"testing"
)

// Regression: PluginsConfig advertised Stats and Auth sections that no
// production code consumed. plugins.stats shipped enabled by default while
// nothing registered a stats plugin, and plugins.auth let users configure
// authentication (enabled + password) that nothing enforced — commands ran
// unauthenticated. The config surface may only advertise sections that
// production code consumes (server.go wires Metrics and SlowLog).
func TestPluginConfigAdvertisesOnlyConsumedSections(t *testing.T) {
	consumed := map[string]bool{
		"Metrics": true, // server.go: plugin registration + /metrics endpoint
		"SlowLog": true, // server.go: GlobalSlowLog enabled/threshold/max_entries
	}

	typ := reflect.TypeOf(PluginsConfig{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if !consumed[name] {
			t.Errorf("dead config section Plugins.%s: no production consumer", name)
		}
	}
}
