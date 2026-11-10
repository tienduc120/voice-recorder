package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func quiet() *slog.Logger { return slog.New(slog.DiscardHandler) }

type fakeHW struct {
	mu          sync.Mutex
	browserURLs []string
	alerts      []string
	opened      int
}

func (f *fakeHW) urls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.browserURLs...)
}

func (f *fakeHW) alertTexts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.alerts...)
}

type noMics struct{}

func (noMics) List(context.Context) ([]application.Device, error) { return nil, nil }

type countingCapture struct{ hw *fakeHW }

func (c countingCapture) Open(string, chan<- []float32) (application.Stream, error) {
	c.hw.mu.Lock()
	defer c.hw.mu.Unlock()
	c.hw.opened++
	return nil, errors.New("no mic in tests")
}

type noPicker struct{}

func (noPicker) Pick(context.Context, string) (string, bool, error) { return "", false, nil }

type idleAwake struct{}

func (idleAwake) Hold()    {}
func (idleAwake) Release() {}

func (idleAwake) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func fakeHardware(t *testing.T) (hardware, *fakeHW) {
	t.Helper()
	f := &fakeHW{}
	return hardware{
		mics:    noMics{},
		capture: countingCapture{hw: f},
		picker:  noPicker{},
		awake:   idleAwake{},
		captureRun: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
		openBrowser: func(url string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.browserURLs = append(f.browserURLs, url)
			return nil
		},
		alert: func(_, text string) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.alerts = append(f.alerts, text)
		},
		documentsDir: t.TempDir(),
	}, f
}

func testConfig(t *testing.T) config {
	t.Helper()
	return config{addr: "127.0.0.1:0", appDir: t.TempDir()}
}

func request(t *testing.T, method, url, origin string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

// start runs an app until the test ends and returns it with its run result.
func start(t *testing.T, cfg config, hw hardware) (*app, context.CancelFunc, chan error) {
	t.Helper()
	a, err := newApp(t.Context(), quiet(), cfg, hw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- a.run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("the app did not stop")
		}
	})
	return a, cancel, done
}

func TestConfigListensOnLoopbackAndKeepsItsFilesInAppData(t *testing.T) {
	t.Parallel()

	env := map[string]string{"DEBUG_ADDR": "127.0.0.1:6060"}
	got := configFrom(func(k string) string { return env[k] }, `C:\Users\me\AppData\Roaming`)

	want := config{
		addr:      "127.0.0.1:47811",
		debugAddr: "127.0.0.1:6060",
		appDir:    filepath.Join(`C:\Users\me\AppData\Roaming`, "VoiceRecorder"),
	}
	if got != want {
		t.Errorf("config %+v, want %+v", got, want)
	}
	if empty := configFrom(func(string) string { return "" }, "x"); empty.debugAddr != "" {
		t.Errorf("debug address %q without DEBUG_ADDR, want none: profiling is opt-in", empty.debugAddr)
	}
}

func TestTheAppServesThePageAndStopsWhenItsContextEnds(t *testing.T) {
	t.Parallel()

	hw, _ := fakeHardware(t)
	a, cancel, done := start(t, testConfig(t), hw)
	base := "http://" + a.api.Addr()

	if code, body := request(
		t,
		http.MethodGet,
		base+"/",
		"",
	); code != http.StatusOK ||
		!strings.Contains(body, `id="start"`) {
		t.Errorf("page: %d, want 200 with the controls", code)
	}
	if code, body := request(t, http.MethodGet, base+"/api/status", ""); code != http.StatusOK ||
		!strings.Contains(body, `"state":"idle"`) {
		t.Errorf("status: %d %s, want 200 and idle", code, body)
	}

	cancel()

	if err := <-done; err != nil {
		t.Errorf("run = %v, want nil: a context that ends is a clean shutdown", err)
	}
	done <- nil
}

func TestQuitFromThePageStopsTheApp(t *testing.T) {
	t.Parallel()

	hw, _ := fakeHardware(t)
	a, _, done := start(t, testConfig(t), hw)
	base := "http://" + a.api.Addr()

	if code, body := request(t, http.MethodPost, base+"/api/quit", base); code != http.StatusNoContent {
		t.Fatalf("quit: %d %s, want 204", code, body)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run = %v, want nil after Quit", err)
		}
		done <- nil
	case <-time.After(5 * time.Second):
		t.Error("the app kept running after Quit")
	}
}

func TestASecondLaunchOpensTheRunningCopyInsteadOfRecording(t *testing.T) {
	t.Parallel()

	first, _ := fakeHardware(t)
	running, _, _ := start(t, testConfig(t), first)
	second, seen := fakeHardware(t)
	cfg := config{addr: running.api.Addr(), appDir: t.TempDir()}

	code := launch(t.Context(), quiet(), cfg, second)

	want := []string{"http://" + running.api.Addr() + "/"}
	if code != exitOK || !equal(seen.urls(), want) || seen.opened != 0 {
		t.Errorf(
			"exit %d, browser %q, mics opened %d: want 0, %q and nothing recorded",
			code,
			seen.urls(),
			seen.opened,
			want,
		)
	}
}

func TestALaunchBlockedByAnotherProgramSaysSoAndFails(t *testing.T) {
	t.Parallel()

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "hello")
	}))
	t.Cleanup(other.Close)
	hw, seen := fakeHardware(t)
	addr := strings.TrimPrefix(other.URL, "http://")

	code := launch(t.Context(), quiet(), config{addr: addr, appDir: t.TempDir()}, hw)

	alerts := seen.alertTexts()
	if code != exitFail || len(alerts) != 1 || !strings.Contains(alerts[0], addr) {
		t.Errorf("exit %d, alerts %q: want 1 and one message naming %s", code, alerts, addr)
	}
}

func TestNewAppRefusesAPublicDebugAddress(t *testing.T) {
	t.Parallel()

	hw, _ := fakeHardware(t)
	cfg := testConfig(t)
	cfg.debugAddr = "0.0.0.0:0"

	a, err := newApp(t.Context(), quiet(), cfg, hw)
	if err == nil {
		_ = a.api.Close()
		t.Error("profiling endpoints must never listen beyond loopback")
	}
}

func TestTheDefaultFolderIsCreatedWhenItIsTheOneInUse(t *testing.T) {
	t.Parallel()

	hw, _ := fakeHardware(t)
	a, err := newApp(t.Context(), quiet(), testConfig(t), hw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.api.Close() })

	if info, statErr := os.Stat(filepath.Join(hw.documentsDir, "Recordings")); statErr != nil || !info.IsDir() {
		t.Errorf("Documents\\Recordings: %v, want the default folder created on first run", statErr)
	}
}

func TestTheLogKeepsThePreviousRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first, err := openLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(first, "first")
	_ = first.Close()

	second, err := openLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = second.Close()

	previous, _ := os.ReadFile(filepath.Join(dir, "voice-recorder.log.1"))
	fresh, _ := os.ReadFile(filepath.Join(dir, "voice-recorder.log"))
	if string(previous) != "first" || len(fresh) != 0 {
		t.Errorf("previous %q, fresh %q: want the last run kept and a new log started", previous, fresh)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
