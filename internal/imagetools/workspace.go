package imagetools

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Workspace     string
	MaxImageBytes int64
}

type ImageTools struct {
	root          string
	maxImageBytes int64
}

func New(config Config) (*ImageTools, error) {
	workspace := config.Workspace
	if workspace == "" {
		workspace = "."
	}
	root, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace is not a directory: %s", root)
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	maxImageBytes := config.MaxImageBytes
	if maxImageBytes <= 0 {
		maxImageBytes = 30 * 1024 * 1024
	}
	return &ImageTools{root: root, maxImageBytes: maxImageBytes}, nil
}

// resolve turns a tool's path argument into a path inside the workspace.
func (t *ImageTools) resolve(path string) (string, error) {
	return confine(t.root, path)
}

func (t *ImageTools) resolveOutput(path string) (string, error) {
	full, err := t.resolve(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return "", err
	}
	return full, nil
}

func (t *ImageTools) rel(path string) string {
	rel, err := filepath.Rel(t.root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
