package doctor

import (
	"regexp"
	"time"
)

const (
	flagVersion = "--version"
	cmdVersion  = "version"
)

// goSlowStart covers the one-time toolchain download GOTOOLCHAIN=auto does when go.mod asks for a newer Go.
const goSlowStart = 3 * time.Minute

// DefaultTools lists what a contributor needs. Minimums are the versions the gates were written against.
func DefaultTools(goVersion string) []Tool {
	return []Tool{
		{
			Name:     "go",
			Command:  []string{"go", cmdVersion},
			Min:      goVersion,
			Required: true,
			Timeout:  goSlowStart,
			Install:  "install Go from https://go.dev/dl/ (go.mod's version is fetched automatically when GOTOOLCHAIN=auto)",
		},
		{
			Name:     "git",
			Command:  []string{"git", flagVersion},
			Required: true,
			Install:  "install Git for Windows or your package manager",
		},
		{
			Name: "golangci-lint", Command: []string{"golangci-lint", flagVersion}, Min: "2.15.0", Required: true,
			// Prebuilt binary: the project advises against go install / go tool for it.
			Install: "scoop install golangci-lint",
		},
		{
			Name: "gofumpt", Command: []string{"gofumpt", flagVersion}, Min: "0.12.0", Required: true,
			Install: "go install mvdan.cc/gofumpt@latest",
		},
		{
			Name: "govulncheck", Command: []string{"govulncheck", "-version"}, Min: "1.9.0", Required: true,
			// The output also prints the Go version first, so anchor on the scanner.
			Pattern: regexp.MustCompile(`govulncheck@v(\d+\.\d+\.\d+)`),
			Install: "go install golang.org/x/vuln/cmd/govulncheck@latest",
		},
		{
			Name:     "lefthook",
			Command:  []string{"lefthook", cmdVersion},
			Min:      "2.2.2",
			Required: true,
			Install:  "scoop install lefthook",
		},
		{
			// go test -race needs cgo on Windows; nothing else does.
			Name:     "gcc",
			Command:  []string{"gcc", flagVersion},
			Required: true,
			Install: "go test -race needs cgo on Windows: install MSYS2's mingw-w64-x86_64-gcc, or w64devkit from " +
				"https://github.com/skeeto/w64devkit/releases, and put its bin folder on PATH " +
				"(the shipped .exe is pure Go and needs nothing)",
		},
		{
			// arch-go reports "master" as its version, so only presence is checked.
			Name: "arch-go", Command: []string{"arch-go", "--help"}, Required: true,
			Install: "go install github.com/arch-go/arch-go/v2@latest",
		},
	}
}
