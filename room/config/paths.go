package config

import (
	"os"
	"path/filepath"
)

// PathResolver handles resolution of relative paths in the config
type PathResolver struct {
	baseDir string
}

// NewPathResolver creates a new PathResolver relative to the given base directory
func NewPathResolver(baseDir string) *PathResolver {
	return &PathResolver{baseDir: baseDir}
}

// ResolvePath resolves a potentially relative path to an absolute path
func (pr *PathResolver) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(pr.baseDir, path)
}

// FileExists checks if a file exists and is readable
func (pr *PathResolver) FileExists(path string) bool {
	absPath := pr.ResolvePath(path)
	_, err := os.Stat(absPath)
	return err == nil
}
