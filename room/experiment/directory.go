package experiment

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	LatestSymlink = "latest"
)

type ExperimentDir struct {
	Path      string    // Absolute path to experiment directory
	ID        string    // Unique experiment identifier
	Timestamp time.Time // When the experiment was created
}

func UseExistingExperimentDirectory(path string) (*ExperimentDir, error) {
	return &ExperimentDir{
		Path: path,
	}, nil
}

// CreateExperimentDirectory creates a new experiment directory and returns its path
func CreateExperimentDirectory(path string) (*ExperimentDir, error) {
	if path == "" {
		path = "experiments"
	}
	// Ensure experiments directory exists
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("creating experiments directory: %w", err)
	}

	// Generate unique experiment ID
	id := GenerateExperimentID()

	// Create absolute path for new experiment
	absPath, err := filepath.Abs(filepath.Join(path, id))
	if err != nil {
		return nil, fmt.Errorf("getting absolute path: %w", err)
	}

	// Create the experiment directory
	if err := os.Mkdir(absPath, 0755); err != nil {
		return nil, fmt.Errorf("creating experiment directory: %w", err)
	}

	// Create symlink to latest experiment
	latestPath := filepath.Join(path, LatestSymlink)
	_ = os.Remove(latestPath) // Remove existing symlink if it exists
	if err := os.Symlink(id, latestPath); err != nil {
		// Don't fail if symlink creation fails
		fmt.Printf("Warning: failed to create latest symlink: %v\n", err)
	}

	return &ExperimentDir{
		Path:      absPath,
		ID:        id,
		Timestamp: time.Now().UTC(),
	}, nil
}

// GetFilePath returns the absolute path for a file in the experiment directory
func (e *ExperimentDir) GetFilePath(filename string) string {
	return filepath.Join(e.Path, filename)
}

// CopyConfigFile copies the provided config file to the experiment directory
func (e *ExperimentDir) CopyConfigFile(srcPath string) error {
	// Read source file
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	// Write to destination
	destPath := e.GetFilePath("config.yaml")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}
