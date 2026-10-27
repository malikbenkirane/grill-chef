package daemoninfo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Info is the pid + port pair of the HTTP listener (issue 1): state lives
// in a marker file next to the db, rewritten on bind, read by recovery
// tooling and the human `stop` CLI without touching the db or its WAL.
type Info struct {
	PID  int `json:"pid"`
	Port int `json:"port"`
}

// Write is tmp+rename atomic so a reader never sees a torn marker.
func Write(path string, info Info) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("daemoninfo: mkdir: %w", err)
	}
	raw, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("daemoninfo: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("daemoninfo: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("daemoninfo: rename: %w", err)
	}
	return nil
}

func Read(path string) (Info, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Info{}, fmt.Errorf("daemoninfo: read %q: %w", path, err)
	}
	var info Info
	if err := json.Unmarshal(raw, &info); err != nil {
		return Info{}, fmt.Errorf("daemoninfo: unmarshal: %w", err)
	}
	return info, nil
}
