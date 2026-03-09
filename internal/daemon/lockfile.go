package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// LockInfo holds the daemon's port, auth token, and PID so that
// tower run and daemon restarts can reclaim the same endpoint.
type LockInfo struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

// WriteLockfile writes daemon lock info to path, creating parent dirs if needed.
func WriteLockfile(path string, info LockInfo) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ReadLockfile reads and parses a daemon lockfile.
func ReadLockfile(path string) (LockInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LockInfo{}, err
	}
	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return LockInfo{}, err
	}
	return info, nil
}

// RemoveLockfile deletes the lockfile. Missing file is not an error.
func RemoveLockfile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
