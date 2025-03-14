package config

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// MetadataCollector handles collecting metadata for experiments
type MetadataCollector struct {
	timestamp time.Time
	gitCommit string
}

// NewMetadataCollector creates a new MetadataCollector with current timestamp
func NewMetadataCollector() (*MetadataCollector, error) {
	gitCommit, err := getCurrentGitCommit()
	if err != nil {
		return nil, fmt.Errorf("failed to get git commit: %w", err)
	}

	return &MetadataCollector{
		timestamp: time.Now().UTC(),
		gitCommit: gitCommit,
	}, nil
}

// getCurrentGitCommit gets the current git commit hash
func getCurrentGitCommit() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// PopulateMetadata fills in the metadata fields of the config
func (mc *MetadataCollector) PopulateMetadata(config *ExperimentConfig) {
	config.Metadata.Timestamp = mc.timestamp.Format("2006-01-02 15:04:05")
	config.Metadata.GitCommit = mc.gitCommit
}
