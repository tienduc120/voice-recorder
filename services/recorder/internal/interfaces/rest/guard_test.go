package rest_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOnlyThisMachinesAddressesMayCall(t *testing.T) {
	t.Parallel()

	for hostHeader, want := range map[string]int{
		"127.0.0.1:47811":    http.StatusOK,
		"localhost:47811":    http.StatusOK,
		"evil.example:47811": http.StatusForbidden,
		"127.0.0.1:9999":     http.StatusForbidden,
		"":                   http.StatusForbidden,
	} {
		w := send(t, &fakeRecorder{}, http.MethodGet, "/api/status", "", withHost(hostHeader))

		if w.Code != want {
			t.Errorf("Host %q: %d, want %d: another site's name for this address must not reach the recorder",
				hostHeader, w.Code, want)
		}
	}
}

func TestARequestThatDidNotArriveOnTheLocalListenerIsRefused(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/status", strings.NewReader(""))
	req.Host = host
	w := httptest.NewRecorder()

	newHandler(&fakeRecorder{}).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status %d, want 403 when the listening port is unknown", w.Code)
	}
}

func TestCommandsNeedThisPagesOrigin(t *testing.T) {
	t.Parallel()

	for origin, want := range map[string]int{
		"":                       http.StatusForbidden,
		"http://evil.example":    http.StatusForbidden,
		"null":                   http.StatusForbidden,
		"http://localhost:47811": http.StatusOK,
		thisOrigin:               http.StatusOK,
	} {
		rec := &fakeRecorder{}
		w := send(t, rec, http.MethodPost, "/api/stop", "", withOrigin(origin))

		if w.Code != want || (want == http.StatusForbidden && len(rec.calls) != 0) {
			t.Errorf("Origin %q: %d, calls %q, want %d: another site's page must not drive the recorder",
				origin, w.Code, rec.calls, want)
		}
	}

	if w := send(t, &fakeRecorder{}, http.MethodGet, "/api/status", ""); w.Code != http.StatusOK {
		t.Errorf("GET without an Origin: %d, want 200: reading is not a command", w.Code)
	}
}
