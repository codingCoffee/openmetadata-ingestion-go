package config

// SourceConfig wraps the sourceConfig.config block.
type SourceConfig struct {
	Config DatabaseMetadata `yaml:"config"`
}

// DatabaseMetadata is the "DatabaseMetadata" sourceConfig.config. IncludeTables and
// IncludeViews are pointers so an unset value can default to true (matching the
// Python default), while an explicit false is respected.
type DatabaseMetadata struct {
	Type                string        `yaml:"type"`
	MarkDeletedTables   bool          `yaml:"markDeletedTables"`
	IncludeTables       *bool         `yaml:"includeTables"`
	IncludeViews        *bool         `yaml:"includeViews"`
	SchemaFilterPattern FilterPattern `yaml:"schemaFilterPattern"`
	TableFilterPattern  FilterPattern `yaml:"tableFilterPattern"`
}

// FilterPattern is an include/exclude regex pattern as expressed in YAML.
type FilterPattern struct {
	Includes []string `yaml:"includes"`
	Excludes []string `yaml:"excludes"`
}

// IncludeTablesOrDefault returns IncludeTables, defaulting to true when unset.
func (d DatabaseMetadata) IncludeTablesOrDefault() bool {
	if d.IncludeTables == nil {
		return true
	}
	return *d.IncludeTables
}

// IncludeViewsOrDefault returns IncludeViews, defaulting to true when unset.
func (d DatabaseMetadata) IncludeViewsOrDefault() bool {
	if d.IncludeViews == nil {
		return true
	}
	return *d.IncludeViews
}
