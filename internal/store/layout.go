package store

import (
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
)

type Layout struct {
	RootDir      string
	DatabasePath string
	ParkedDir    string
	ArtifactsDir string
}

// DefaultLayout follows the locked local-first paths from the foundation spec.
func DefaultLayout() (Layout, error) {
	var root string

	switch goruntime.GOOS {
	case "windows":
		root = os.Getenv("LOCALAPPDATA")
		if root == "" {
			root = os.Getenv("LocalAppData")
		}
		if root == "" {
			return Layout{}, errors.New("LOCALAPPDATA is not set")
		}
		root = filepath.Join(root, "Tower")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return Layout{}, err
		}
		root = filepath.Join(home, "Library", "Application Support", "Tower")
	default:
		configRoot, err := os.UserConfigDir()
		if err != nil {
			return Layout{}, err
		}
		root = filepath.Join(configRoot, "Tower")
	}

	return Layout{
		RootDir:      root,
		DatabasePath: filepath.Join(root, "tower.db"),
		ParkedDir:    filepath.Join(root, "parked"),
		ArtifactsDir: filepath.Join(root, "artifacts"),
	}, nil
}
