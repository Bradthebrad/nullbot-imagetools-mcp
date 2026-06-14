package imagetools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (t *ImageTools) resolve(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", path)
	}
	full := filepath.Join(t.root, filepath.Clean(path))
	if err := t.ensureInside(full); err != nil {
		return "", err
	}
	return full, nil
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

func (t *ImageTools) ensureInside(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if abs == t.root {
		return nil
	}
	prefix := t.root + string(os.PathSeparator)
	if !strings.HasPrefix(abs, prefix) {
		return fmt.Errorf("path escapes workspace: %s", path)
	}
	return nil
}

func (t *ImageTools) rel(path string) string {
	rel, err := filepath.Rel(t.root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
