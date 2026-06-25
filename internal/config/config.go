// Package config unmarshals the OpenMetadata ingestion workflow YAML. The struct
// layout intentionally mirrors the Python openmetadata-ingestion workflow config
// so existing YAML files can be reused unchanged. Unknown fields are ignored
// (KnownFields is left off) so the many Python options we do not model are
// tolerated rather than rejected.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Workflow is the root of a workflow YAML file.
type Workflow struct {
	Source         Source         `yaml:"source"`
	Sink           Sink           `yaml:"sink"`
	WorkflowConfig WorkflowConfig `yaml:"workflowConfig"`
}

// Source describes what to ingest and how to connect to it.
type Source struct {
	Type              string            `yaml:"type"` // postgres | mysql | clickhouse
	ServiceName       string            `yaml:"serviceName"`
	ServiceConnection ServiceConnection `yaml:"serviceConnection"`
	SourceConfig      SourceConfig      `yaml:"sourceConfig"`
}

// Sink describes where extracted metadata is written. Only the "metadata-rest"
// sink (push to the OpenMetadata API) is supported.
type Sink struct {
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"`
}

// WorkflowConfig holds run-level settings and the OpenMetadata server connection.
type WorkflowConfig struct {
	LoggerLevel              string                   `yaml:"loggerLevel"`
	OpenMetadataServerConfig OpenMetadataServerConfig `yaml:"openMetadataServerConfig"`
}

// OpenMetadataServerConfig is the connection to the OpenMetadata API.
type OpenMetadataServerConfig struct {
	HostPort       string         `yaml:"hostPort"`
	AuthProvider   string         `yaml:"authProvider"`
	SecurityConfig SecurityConfig `yaml:"securityConfig"`
}

// SecurityConfig carries the JWT bot token used to authenticate to OpenMetadata.
type SecurityConfig struct {
	JWTToken string `yaml:"jwtToken"`
}

// Load reads and parses a workflow YAML file from path.
func Load(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}
	if err := wf.validate(); err != nil {
		return nil, err
	}
	return &wf, nil
}

// validate checks the minimum fields required to run a workflow.
func (w *Workflow) validate() error {
	if w.Source.Type == "" {
		return fmt.Errorf("source.type is required")
	}
	if w.Source.ServiceName == "" {
		return fmt.Errorf("source.serviceName is required")
	}
	if w.Source.ServiceConnection.Config == nil {
		return fmt.Errorf("source.serviceConnection.config is required")
	}
	om := w.WorkflowConfig.OpenMetadataServerConfig
	if om.HostPort == "" {
		return fmt.Errorf("workflowConfig.openMetadataServerConfig.hostPort is required")
	}
	if om.SecurityConfig.JWTToken == "" {
		return fmt.Errorf("workflowConfig.openMetadataServerConfig.securityConfig.jwtToken is required")
	}
	return nil
}
