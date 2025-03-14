package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadOptions configures the behavior of config loading
type LoadOptions struct {
	ValidateImmediately bool
	ResolvePaths        bool
	MergeFiles          bool
}

// LoadFromFile loads an ExperimentConfig from a YAML file
func LoadFromFile(path string, opts LoadOptions) (*ExperimentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	config := &ExperimentConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if opts.ResolvePaths {
		baseDir := filepath.Dir(path)
		resolver := NewPathResolver(baseDir)
		if err := config.ResolvePaths(resolver); err != nil {
			return nil, fmt.Errorf("resolving paths: %w", err)
		}
	}

	if opts.MergeFiles {
		if err := config.LoadAndMerge(); err != nil {
			return nil, fmt.Errorf("merging external files: %w", err)
		}
	}

	if opts.ValidateImmediately {
		if errs := config.Validate(); len(errs) > 0 {
			return nil, fmt.Errorf("validation errors: %v", errs)
		}
	}

	return config, nil
}

// SaveToFile saves an ExperimentConfig to a YAML file
func SaveToFile(config *ExperimentConfig, path string) error {
	// Update metadata before saving
	collector, err := NewMetadataCollector()
	if err != nil {
		return fmt.Errorf("creating metadata collector: %w", err)
	}
	collector.PopulateMetadata(config)

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// ResolvePaths resolves all relative paths in the config to absolute paths
func (c *ExperimentConfig) ResolvePaths(resolver *PathResolver) error {
	// Resolve input mesh path
	c.Input.Mesh.Path = resolver.ResolvePath(c.Input.Mesh.Path)

	// Resolve materials from_file if present
	if c.Materials.FromFile != "" {
		c.Materials.FromFile = resolver.ResolvePath(c.Materials.FromFile)
	}

	// Resolve surface assignments from_file if present
	if c.SurfaceAssignments.FromFile != "" {
		c.SurfaceAssignments.FromFile = resolver.ResolvePath(c.SurfaceAssignments.FromFile)
	}

	return nil
}
