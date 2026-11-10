package settingsfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/settingsfile"
)

func defaults() application.Settings {
	return application.Settings{Folder: `C:\Docs\Recordings`, NoiseCleanup: true}
}

func TestSavedSettingsComeBack(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	saved := application.Settings{Folder: `D:\Meetings`, MicID: "{0.0.1.00000000}.{abc}", NoiseCleanup: false}
	if err := settingsfile.New(path, defaults()).Save(saved); err != nil {
		t.Fatal(err)
	}

	got, err := settingsfile.New(path, defaults()).Load()

	if err != nil || got != saved {
		t.Errorf("loaded %+v, %v; want %+v", got, err, saved)
	}
}

func TestAMissingFileGivesTheDefaults(t *testing.T) {
	t.Parallel()

	got, err := settingsfile.New(filepath.Join(t.TempDir(), "config.json"), defaults()).Load()

	if err != nil || got != defaults() {
		t.Errorf("loaded %+v, %v; want the defaults and no error on a first run", got, err)
	}
}

func TestACorruptFileGivesTheDefaultsAndSaysWhy(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := settingsfile.New(path, defaults())

	got, err := store.Load()
	if err == nil || got != defaults() {
		t.Errorf("loaded %+v, %v; want the defaults and an error saying why", got, err)
	}

	repaired := application.Settings{Folder: `D:\New`, NoiseCleanup: true}
	if err = store.Save(repaired); err != nil {
		t.Fatal(err)
	}
	if got, err = store.Load(); err != nil || got != repaired {
		t.Errorf("after a save, loaded %+v, %v; want %+v", got, err, repaired)
	}
}

func TestFieldsMissingFromTheFileKeepTheirDefaults(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"folder":"D:\\Rec"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := settingsfile.New(path, defaults()).Load()

	want := application.Settings{Folder: `D:\Rec`, NoiseCleanup: true}
	if err != nil || got != want {
		t.Errorf("loaded %+v, %v; want %+v: an absent field keeps its default", got, err, want)
	}
}

func TestSaveCreatesTheFolderAndLeavesNoTempFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "VoiceRecorder")

	if err := settingsfile.New(filepath.Join(dir, "config.json"), defaults()).Save(defaults()); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Errorf("folder holds %v (%v), want only config.json", entries, err)
	}
}
