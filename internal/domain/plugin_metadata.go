package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type PluginFamily string

const (
	PluginFamilyTool          PluginFamily = "tool"
	PluginFamilyProvider      PluginFamily = "provider"
	PluginFamilyVerifier      PluginFamily = "verifier"
	PluginFamilyAgentStrategy PluginFamily = "agent_strategy"
)

type PluginExecutionMode string

const (
	PluginExecutionModeExternal PluginExecutionMode = "external"
	PluginExecutionModeNative   PluginExecutionMode = "native"
)

var pluginVersionPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

type PluginMetadata struct {
	Family                PluginFamily        `json:"family"`
	LogicalID             string              `json:"logical_id"`
	DisplayName           string              `json:"display_name"`
	ContractVersion       string              `json:"contract_version"`
	ImplementationVersion string              `json:"implementation_version,omitempty"`
	ExecutionMode         PluginExecutionMode `json:"execution_mode"`
	SourcePath            string              `json:"source_path,omitempty"`
}

type Provenance struct {
	Plugins []PluginMetadata `json:"plugins,omitempty"`
}

func (f PluginFamily) Normalize() PluginFamily {
	return PluginFamily(strings.TrimSpace(string(f)))
}

func (f PluginFamily) Validate() error {
	switch f.Normalize() {
	case PluginFamilyTool, PluginFamilyProvider, PluginFamilyVerifier, PluginFamilyAgentStrategy:
		return nil
	case "":
		return fmt.Errorf("plugin family must not be empty")
	default:
		return fmt.Errorf("unknown plugin family %q", f)
	}
}

func (m PluginExecutionMode) Normalize() PluginExecutionMode {
	return PluginExecutionMode(strings.TrimSpace(string(m)))
}

func (m PluginExecutionMode) Validate() error {
	switch m.Normalize() {
	case PluginExecutionModeExternal, PluginExecutionModeNative:
		return nil
	case "":
		return fmt.Errorf("plugin execution mode must not be empty")
	default:
		return fmt.Errorf("unknown plugin execution mode %q", m)
	}
}

func (m PluginMetadata) Normalize() PluginMetadata {
	m.Family = m.Family.Normalize()
	m.LogicalID = strings.TrimSpace(m.LogicalID)
	m.DisplayName = strings.TrimSpace(m.DisplayName)
	m.ContractVersion = strings.TrimSpace(m.ContractVersion)
	m.ImplementationVersion = strings.TrimSpace(m.ImplementationVersion)
	m.ExecutionMode = m.ExecutionMode.Normalize()
	m.SourcePath = strings.TrimSpace(m.SourcePath)
	return m
}

func (m PluginMetadata) Validate() error {
	m = m.Normalize()
	if err := m.Family.Validate(); err != nil {
		return err
	}
	if m.LogicalID == "" {
		return fmt.Errorf("plugin logical ID must not be empty")
	}
	if m.DisplayName == "" {
		return fmt.Errorf("plugin display name must not be empty")
	}
	if err := validatePluginVersion("contract version", m.ContractVersion, true); err != nil {
		return err
	}
	if err := validatePluginVersion("implementation version", m.ImplementationVersion, false); err != nil {
		return err
	}
	if err := m.ExecutionMode.Validate(); err != nil {
		return err
	}
	return nil
}

func (m PluginMetadata) MarshalJSON() ([]byte, error) {
	type pluginMetadataJSON PluginMetadata
	normalized := m.Normalize()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(pluginMetadataJSON(normalized))
}

func (m *PluginMetadata) UnmarshalJSON(data []byte) error {
	type pluginMetadataJSON PluginMetadata
	var decoded pluginMetadataJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	normalized := PluginMetadata(decoded).Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}
	*m = normalized
	return nil
}

func (p Provenance) Normalize() Provenance {
	if len(p.Plugins) == 0 {
		return Provenance{}
	}
	plugins := make([]PluginMetadata, 0, len(p.Plugins))
	for _, plugin := range p.Plugins {
		plugins = append(plugins, plugin.Normalize())
	}
	return Provenance{Plugins: plugins}
}

func (p Provenance) Validate() error {
	p = p.Normalize()
	seen := make(map[string]struct{}, len(p.Plugins))
	for _, plugin := range p.Plugins {
		if err := plugin.Validate(); err != nil {
			return err
		}
		key := string(plugin.Family) + "\x00" + plugin.LogicalID
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate plugin provenance for family %q logical ID %q", plugin.Family, plugin.LogicalID)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (p Provenance) MarshalJSON() ([]byte, error) {
	type provenanceJSON Provenance
	normalized := p.Normalize()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(provenanceJSON(normalized))
}

func (p *Provenance) UnmarshalJSON(data []byte) error {
	type provenanceJSON Provenance
	var decoded provenanceJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	normalized := Provenance(decoded).Normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}
	*p = normalized
	return nil
}

func validatePluginVersion(field, version string, required bool) error {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		if required {
			return fmt.Errorf("plugin %s must not be empty", field)
		}
		return nil
	}
	if !pluginVersionPattern.MatchString(trimmed) {
		return fmt.Errorf("plugin %s %q must use semantic version format", field, trimmed)
	}
	return nil
}
