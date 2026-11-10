package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/interfaces/rest"
)

const (
	port       = 47811
	host       = "127.0.0.1:47811"
	thisOrigin = "http://127.0.0.1:47811"
)

type fakeRecorder struct {
	status application.Status
	browse application.BrowseResult
	err    error
	calls  []string
}

func (f *fakeRecorder) Status(context.Context) application.Status { return f.status }

func (f *fakeRecorder) ChangeSettings(_ context.Context, cmd application.SettingsCommand) (application.Status, error) {
	f.calls = append(f.calls, fmt.Sprintf("settings %q %v", cmd.Folder, cmd.NoiseCleanup))
	return f.status, f.err
}

func (f *fakeRecorder) Browse(context.Context) (application.BrowseResult, error) {
	f.calls = append(f.calls, "browse")
	return f.browse, f.err
}

func (f *fakeRecorder) Start(_ context.Context, cmd application.StartCommand) (application.Status, error) {
	f.calls = append(f.calls, fmt.Sprintf("start %q", cmd.Title))
	return f.status, f.err
}

func (f *fakeRecorder) Pause(context.Context) (application.Status, error) {
	f.calls = append(f.calls, "pause")
	return f.status, f.err
}

func (f *fakeRecorder) Resume(context.Context) (application.Status, error) {
	f.calls = append(f.calls, "resume")
	return f.status, f.err
}

func (f *fakeRecorder) Stop(context.Context) (application.Status, error) {
	f.calls = append(f.calls, "stop")
	return f.status, f.err
}

func (f *fakeRecorder) ChooseMic(_ context.Context, cmd application.ChooseMicCommand) (application.Status, error) {
	f.calls = append(f.calls, fmt.Sprintf("mic %q", cmd.ID))
	return f.status, f.err
}

func (f *fakeRecorder) Quit(context.Context) error {
	f.calls = append(f.calls, "quit")
	return f.err
}

func quiet() *slog.Logger { return slog.New(slog.DiscardHandler) }

func newHandler(rec rest.Recorder) http.Handler { return rest.NewHandler(rec, quiet()) }

type option func(*http.Request)

func withHost(h string) option { return func(r *http.Request) { r.Host = h } }

func withOrigin(o string) option {
	return func(r *http.Request) {
		if o == "" {
			r.Header.Del("Origin")
		} else {
			r.Header.Set("Origin", o)
		}
	}
}

// send calls the handler as the local server would: the request arrived on 127.0.0.1:47811 and, for a change, came
// from this app's own page.
func send(t *testing.T, rec rest.Recorder, method, path, body string, opts ...option) *httptest.ResponseRecorder {
	t.Helper()
	local := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}
	ctx := context.WithValue(t.Context(), http.LocalAddrContextKey, local)
	req := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
	req.Host = host
	if method != http.MethodGet {
		req.Header.Set("Origin", thisOrigin)
	}
	for _, o := range opts {
		o(req)
	}
	w := httptest.NewRecorder()
	rest.NewHandler(rec, quiet()).ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", w.Body.String(), err)
	}
	return got
}

func TestStatusIsReturnedAsJSON(t *testing.T) {
	t.Parallel()

	rec := &fakeRecorder{status: application.Status{
		State:    application.StateRecording,
		Recorded: 2537 * time.Second,
		Level:    0.5,
		Folder:   `C:\Rec`,
		Mics:     []application.Device{{ID: "usb", Name: "USB Microphone", IsDefault: true}},
		MicID:    "usb",
		File:     application.FileInfo{Name: "2026-11-09_1030_sprint-planning.ogg", Size: 10276044},
		Notice:   application.Notice{Kind: application.NoticeMicConnected, Mic: "USB Microphone"},
	}}

	w := send(t, rec, http.MethodGet, "/api/status", "")

	want := map[string]any{
		"state": "recording", "recordedMs": 2537000.0, "level": 0.5, "folder": `C:\Rec`, "folderProblem": "",
		"noiseCleanup": false,
		"mics":         []any{map[string]any{"id": "usb", "name": "USB Microphone", "isDefault": true}},
		"micId":        "usb",
		"file":         map[string]any{"name": "2026-11-09_1030_sprint-planning.ogg", "size": 10276044.0},
		"notice":       map[string]any{"kind": "mic-connected", "mic": "USB Microphone", "previousMic": ""},
	}
	if got := decode(t, w); w.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Errorf("status %d body %+v, want 200 %+v", w.Code, got, want)
	}
}

func TestEachCommandReachesItsUseCase(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		method, path, body, call string
		code                     int
	}{
		{http.MethodPost, "/api/start", `{"title":"sprint planning"}`, `start "sprint planning"`, http.StatusOK},
		{http.MethodPost, "/api/pause", "", "pause", http.StatusOK},
		{http.MethodPost, "/api/resume", "", "resume", http.StatusOK},
		{http.MethodPost, "/api/stop", "", "stop", http.StatusOK},
		{http.MethodPut, "/api/settings", `{"folder":"C:\\Rec","noiseCleanup":false}`, `settings "C:\\Rec" false`, http.StatusOK},
		{http.MethodPut, "/api/mic", `{"id":"usb"}`, `mic "usb"`, http.StatusOK},
		{http.MethodPost, "/api/browse", "", "browse", http.StatusOK},
		{http.MethodPost, "/api/quit", "", "quit", http.StatusNoContent},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			rec := &fakeRecorder{
				status: application.Status{State: application.StateIdle},
				browse: application.BrowseResult{Folder: `D:\Meetings`, Chosen: true},
			}
			w := send(t, rec, tc.method, tc.path, tc.body)

			if w.Code != tc.code || !reflect.DeepEqual(rec.calls, []string{tc.call}) {
				t.Errorf(
					"%s %s: status %d calls %q, want %d and %q",
					tc.method,
					tc.path,
					w.Code,
					rec.calls,
					tc.code,
					tc.call,
				)
			}
		})
	}
}

func TestBrowseReturnsTheChosenFolder(t *testing.T) {
	t.Parallel()

	rec := &fakeRecorder{browse: application.BrowseResult{Folder: `D:\Meetings`, Chosen: true}}

	got := decode(t, send(t, rec, http.MethodPost, "/api/browse", ""))

	if want := map[string]any{"folder": `D:\Meetings`, "chosen": true}; !reflect.DeepEqual(got, want) {
		t.Errorf("browse = %+v, want %+v", got, want)
	}
}

func TestFailuresMapToStatusCodes(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		body string
		err  error
		want int
	}{
		"invalid command": {`{}`, fmt.Errorf("%w: no such mic", application.ErrInvalidCommand), http.StatusBadRequest},
		"conflict":        {`{}`, fmt.Errorf("%w: already recording", application.ErrConflict), http.StatusConflict},
		"malformed body":  {`{`, nil, http.StatusBadRequest},
		"internal":        {`{}`, errors.New(`open C:\secret: access denied`), http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := send(t, &fakeRecorder{err: tc.err}, http.MethodPost, "/api/start", tc.body)

			if w.Code != tc.want || strings.Contains(w.Body.String(), "secret") {
				t.Errorf("status %d body %q, want %d and no internal detail", w.Code, w.Body.String(), tc.want)
			}
		})
	}
}

func TestAnOversizedBodyIsRefused(t *testing.T) {
	t.Parallel()

	rec := &fakeRecorder{}
	body := `{"title":"` + strings.Repeat("x", 100<<10) + `"}`

	w := send(t, rec, http.MethodPost, "/api/start", body)

	if w.Code != http.StatusBadRequest || len(rec.calls) != 0 {
		t.Errorf("status %d, calls %q: want 400 and the body never reaching Start", w.Code, rec.calls)
	}
}

func TestThePageIsServedFromTheBinaryWithSafeHeaders(t *testing.T) {
	t.Parallel()

	for path, contentType := range map[string]string{
		"/":          "text/html; charset=utf-8",
		"/app.js":    "text/javascript; charset=utf-8",
		"/style.css": "text/css; charset=utf-8",
	} {
		w := send(t, &fakeRecorder{}, http.MethodGet, path, "")

		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != contentType {
			t.Errorf("GET %s: %d %q, want 200 %q", path, w.Code, w.Header().Get("Content-Type"), contentType)
		}
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") ||
			w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("GET %s: headers %v, want a CSP that forbids framing and nosniff", path, w.Header())
		}
	}
	if w := send(t, &fakeRecorder{}, http.MethodGet, "/", ""); !strings.Contains(w.Body.String(), `id="start"`) {
		t.Error("the page lacks its Start button")
	}
}

func TestPingNamesTheApp(t *testing.T) {
	t.Parallel()

	got := decode(t, send(t, &fakeRecorder{}, http.MethodGet, "/api/ping", ""))

	if want := map[string]any{"app": "voice-recorder"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ping = %+v, want %+v", got, want)
	}
}
