package rest

import (
	"net"
	"net/http"
	"strings"
)

const (
	// contentPolicy allows only the page's own files and forbids framing, so no other site can show the page in a
	// frame and steer clicks on Start or Stop.
	contentPolicy = "default-src 'self'; frame-ancestors 'none'"
	apiPrefix     = "/api/"
)

func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", contentPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if strings.HasPrefix(r.URL.Path, apiPrefix) {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// guard admits only requests addressed to this machine by name, which defeats DNS rebinding, and commands only
// from this app's own page, which stops another site open in the same browser from driving the recorder.
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		port, ok := localPort(r)
		if !ok || !isLocal(r.Host, port) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !isThisPage(r.Header.Get("Origin"), port) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// localPort is the port the request arrived on; without it nothing can be checked, so the request is refused.
func localPort(r *http.Request) (string, bool) {
	addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return "", false
	}
	_, port, err := net.SplitHostPort(addr.String())
	return port, err == nil
}

func isLocal(host, port string) bool {
	return host == "127.0.0.1:"+port || host == "localhost:"+port
}

func isThisPage(origin, port string) bool {
	return origin == "http://127.0.0.1:"+port || origin == "http://localhost:"+port
}
