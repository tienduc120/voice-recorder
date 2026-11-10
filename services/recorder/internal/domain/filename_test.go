package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/domain"
)

func startedAt() time.Time {
	return time.Date(2026, time.November, 9, 10, 30, 59, 0, time.FixedZone("ICT", 7*60*60))
}

func TestFileNameUsesTheLocalStartTimeTitleAndAttempt(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		title   string
		attempt int
		want    string
	}{
		{"sprint planning", 1, "2026-11-09_1030_sprint-planning.ogg"},
		{"", 1, "2026-11-09_1030.ogg"},
		{"sprint planning", 3, "2026-11-09_1030_sprint-planning-3.ogg"},
		{"", 2, "2026-11-09_1030-2.ogg"},
	} {
		if got := domain.FileName(startedAt(), tc.title, tc.attempt); got != tc.want {
			t.Errorf("FileName(%q, %d) = %q, want %q", tc.title, tc.attempt, got, tc.want)
		}
	}
}

func TestFileNameCleansTheTitle(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		"  Q&A: budget / 2027?  ": "Q-A-budget-2027",
		"họp sprint":              "họp-sprint",
		`<>:"/\|?*`:               "",
		"...":                     "",
		"a\x00b\tc":               "a-b-c",
		"--draft--":               "draft",
		"v1.2 review":             "v1.2-review",
		strings.Repeat("a", 100):  strings.Repeat("a", 60),
	} {
		expected := "2026-11-09_1030.ogg"
		if want != "" {
			expected = "2026-11-09_1030_" + want + ".ogg"
		}
		if got := domain.FileName(startedAt(), raw, 1); got != expected {
			t.Errorf("FileName(%q) = %q, want %q: the title must be safe in a Windows file name", raw, got, expected)
		}
	}
}
