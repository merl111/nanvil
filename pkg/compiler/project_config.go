package compiler

import (
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"gopkg.in/yaml.v3"
)

// ProjectConfig contains contract project metadata from a YAML config file.
type ProjectConfig struct {
	Name               string
	SourceURL          string
	SafeMethods        []string
	SupportedStandards []string
	Events             []HybridEvent
	Permissions        []yamlPermission
	Overloads          map[string]string               `yaml:"overloads,omitempty"`
	NamedTypes         map[string]binding.ExtendedType `yaml:"namedtypes,omitempty"`
}

// ParseProjectConfig reads a contract configuration file (.yaml).
func ParseProjectConfig(confFile string) (ProjectConfig, error) {
	var conf ProjectConfig
	confBytes, err := os.ReadFile(confFile)
	if err != nil {
		return conf, err
	}
	if err := yaml.Unmarshal(confBytes, &conf); err != nil {
		return conf, fmt.Errorf("bad config: %w", err)
	}
	return conf, nil
}

// PermissionsManifest converts YAML permissions to manifest permissions.
func (c ProjectConfig) PermissionsManifest() []manifest.Permission {
	out := make([]manifest.Permission, len(c.Permissions))
	for i := range c.Permissions {
		out[i] = manifest.Permission(c.Permissions[i])
	}
	return out
}
