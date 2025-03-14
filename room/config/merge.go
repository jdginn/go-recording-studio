package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// MergeMaterials merges materials from a file with inline materials
func (m *Materials) MergeMaterials() error {
	if m.FromFile == "" {
		return nil
	}

	// Read and parse the materials file
	data, err := os.ReadFile(m.FromFile)
	if err != nil {
		return fmt.Errorf("reading materials file: %w", err)
	}

	var fileMaterials map[string]Material
	if err := json.Unmarshal(data, &fileMaterials); err != nil {
		return fmt.Errorf("parsing materials file: %w", err)
	}

	// Initialize inline map if it doesn't exist
	if m.Inline == nil {
		m.Inline = make(map[string]Material)
	}

	// Merge materials, with inline taking precedence
	for name, material := range fileMaterials {
		if _, exists := m.Inline[name]; !exists {
			m.Inline[name] = material
		}
	}

	return nil
}

// MergeSurfaceAssignments merges surface assignments from a file with inline assignments
func (sa *SurfaceAssignments) MergeSurfaceAssignments() error {
	if sa.FromFile == "" {
		return nil
	}

	// Read and parse the surface assignments file
	data, err := os.ReadFile(sa.FromFile)
	if err != nil {
		return fmt.Errorf("reading surface assignments file: %w", err)
	}

	var fileAssignments map[string]string
	if err := json.Unmarshal(data, &fileAssignments); err != nil {
		return fmt.Errorf("parsing surface assignments file: %w", err)
	}

	// Initialize inline map if it doesn't exist
	if sa.Inline == nil {
		sa.Inline = make(map[string]string)
	}

	// Merge assignments, with inline taking precedence
	for surface, material := range fileAssignments {
		if _, exists := sa.Inline[surface]; !exists {
			sa.Inline[surface] = material
		}
	}

	return nil
}

// Helper method to check if a material exists
func (m *Materials) HasMaterial(name string) bool {
	_, exists := m.Inline[name]
	return exists
}

// LoadAndMerge loads all external files and merges their contents
func (c *ExperimentConfig) LoadAndMerge() error {
	// Merge materials first since surface assignments depend on them
	if err := c.Materials.MergeMaterials(); err != nil {
		return fmt.Errorf("merging materials: %w", err)
	}

	// Then merge surface assignments
	if err := c.SurfaceAssignments.MergeSurfaceAssignments(); err != nil {
		return fmt.Errorf("merging surface assignments: %w", err)
	}

	return nil
}
