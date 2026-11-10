package audiofile_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/audiofile"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/denoise"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/opusenc"
)

func realFiles() *audiofile.Files {
	return audiofile.NewFiles(
		func() (audiofile.Encoder, error) {
			e, err := opusenc.New()
			if err != nil {
				return nil, err
			}
			return e, nil
		},
		func() (audiofile.Denoiser, error) {
			d, err := denoise.New()
			if err != nil {
				return nil, err
			}
			return d, nil
		},
	)
}

func xName(attempt int) string {
	if attempt == 1 {
		return "x.ogg"
	}
	return fmt.Sprintf("x-%d.ogg", attempt)
}

func TestCreateSkipsNamesThatAreTaken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	taken := filepath.Join(dir, "x.ogg")
	if err := os.WriteFile(taken, []byte("an earlier meeting"), 0o600); err != nil {
		t.Fatal(err)
	}

	f, err := realFiles().Create(dir, xName, application.AudioOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(taken); f.Name() != "x-2.ogg" || string(got) != "an earlier meeting" {
		t.Errorf("created %q and x.ogg holds %q: want x-2.ogg and the earlier file untouched", f.Name(), got)
	}
}

func TestCheckFolderAcceptsOnlyAWritableFolder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := realFiles().CheckFolder(dir); err != nil {
		t.Errorf("a writable folder: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("the check left %d files behind, want none", len(entries))
	}

	aFile := filepath.Join(dir, "a-file.txt")
	if err := os.WriteFile(aFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, folder := range map[string]string{
		"missing":  filepath.Join(dir, "missing"),
		"a file":   aFile,
		"relative": `relative\dir`,
		"empty":    "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := realFiles().CheckFolder(folder); !errors.Is(err, application.ErrFolderNotWritable) {
				t.Errorf("CheckFolder(%q) = %v, want ErrFolderNotWritable", folder, err)
			}
		})
	}
}
