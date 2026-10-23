package config

import (
	"reflect"
	"testing"
)

// TestEnabledModules: since the archive (docs/stories/module-archivieren.md) the
// ISMS is the only module. Whatever VAKT_MODULES_ENABLED says, GET /modules
// reports vaktcomply alone — an archived or unknown name never reaches the
// frontend, and the ISMS cannot be switched off.
func TestEnabledModules(t *testing.T) {
	for _, csv := range []string{"vaktcomply", " VaktHR , vaktcomply,vaktscan", "vaktcomply,vaktfoo", "", "vaktscan"} {
		got := (&Config{ModulesEnabled: csv}).EnabledModules()
		if !reflect.DeepEqual(got, []string{"vaktcomply"}) {
			t.Errorf("EnabledModules(%q) = %v, want [vaktcomply]", csv, got)
		}
	}
}

func TestIsModuleEnabled_OnlyCore(t *testing.T) {
	c := &Config{ModulesEnabled: "vaktcomply,vaktscan,vakthr"}
	if !c.IsModuleEnabled("VaktComply") {
		t.Error("vaktcomply must be enabled")
	}
	for _, m := range ArchivedModules {
		if c.IsModuleEnabled(m) {
			t.Errorf("archived module %q must not be enabled", m)
		}
	}
}

func TestArchivedModulesRequested(t *testing.T) {
	cases := map[string][]string{
		"vaktcomply":                    nil,
		"":                              nil,
		"vaktcomply,vaktfoo":            nil,
		" VaktHR , vaktcomply,vaktscan": {"vaktscan", "vakthr"},
		"vaktscan,vaktvault,vaktaware,vaktprivacy,vakthr": {"vaktscan", "vaktvault", "vaktaware", "vaktprivacy", "vakthr"},
	}
	for csv, want := range cases {
		got := (&Config{ModulesEnabled: csv}).ArchivedModulesRequested()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ArchivedModulesRequested(%q) = %v, want %v", csv, got, want)
		}
	}
}
