// Package settingsfile remembers the recorder's settings in a JSON file between launches.
package settingsfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
)

const (
	dirMode  = 0o750
	fileMode = 0o600
	tempExt  = ".tmp"
	indent   = "  "
)

// stored is the file's shape; pointers tell a field the file leaves out from one it sets to its zero value.
type stored struct {
	Folder       *string `json:"folder,omitempty"`
	MicID        *string `json:"micId,omitempty"`
	NoiseCleanup *bool   `json:"noiseCleanup,omitempty"`
}

type Store struct {
	path     string
	defaults application.Settings
}

func New(path string, defaults application.Settings) *Store {
	return &Store{path: path, defaults: defaults}
}

// Load returns the defaults when there is no file yet, and the defaults with an error when the file cannot be read:
// a broken settings file must never stop a recording.
func (s *Store) Load() (application.Settings, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s.defaults, nil
	}
	if err != nil {
		return s.defaults, fmt.Errorf("read settings: %w", err)
	}
	var file stored
	if err = json.Unmarshal(data, &file); err != nil {
		return s.defaults, fmt.Errorf("settings file %s is not valid JSON: %w", s.path, err)
	}
	settings := s.defaults
	if file.Folder != nil {
		settings.Folder = *file.Folder
	}
	if file.MicID != nil {
		settings.MicID = *file.MicID
	}
	if file.NoiseCleanup != nil {
		settings.NoiseCleanup = *file.NoiseCleanup
	}
	return settings, nil
}

// Save writes a temporary file and renames it over the old one, so a crash mid-save leaves the old file whole.
func (s *Store) Save(settings application.Settings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), dirMode); err != nil {
		return fmt.Errorf("create the settings folder: %w", err)
	}
	data, err := json.MarshalIndent(stored{
		Folder:       &settings.Folder,
		MicID:        &settings.MicID,
		NoiseCleanup: &settings.NoiseCleanup,
	}, "", indent)
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	temp := s.path + tempExt
	if err = os.WriteFile(temp, data, fileMode); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err = os.Rename(temp, s.path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}
