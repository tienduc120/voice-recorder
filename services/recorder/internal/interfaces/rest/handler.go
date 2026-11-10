// Package rest serves the recorder's control page and its JSON API to this machine only.
package rest

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
)

// AppName is what /api/ping answers, so a second launch can tell this app from anything else on the port.
const AppName = "voice-recorder"

// maxBodyBytes caps a request body so no client can hold memory; every real body is a few hundred bytes.
const maxBodyBytes = 64 << 10

//go:embed web
var web embed.FS

type Recorder interface {
	Status(ctx context.Context) application.Status
	ChangeSettings(ctx context.Context, cmd application.SettingsCommand) (application.Status, error)
	Browse(ctx context.Context) (application.BrowseResult, error)
	Start(ctx context.Context, cmd application.StartCommand) (application.Status, error)
	Pause(ctx context.Context) (application.Status, error)
	Resume(ctx context.Context) (application.Status, error)
	Stop(ctx context.Context) (application.Status, error)
	ChooseMic(ctx context.Context, cmd application.ChooseMicCommand) (application.Status, error)
	Quit(ctx context.Context) error
}

type handler struct {
	rec    Recorder
	logger *slog.Logger
}

func NewHandler(rec Recorder, logger *slog.Logger) http.Handler {
	h := &handler{rec: rec, logger: logger}
	mux := http.NewServeMux()
	// Explicit types: Windows' registry can map .js to text/plain, which nosniff then refuses to run.
	mux.Handle("GET /{$}", asset("web/index.html", "text/html; charset=utf-8"))
	mux.Handle("GET /app.js", asset("web/app.js", "text/javascript; charset=utf-8"))
	mux.Handle("GET /style.css", asset("web/style.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, pingResponse{App: AppName})
	})
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, statusOf(h.rec.Status(r.Context())))
	})
	mux.HandleFunc("POST /api/start", h.start)
	mux.HandleFunc("POST /api/pause", h.command(h.rec.Pause))
	mux.HandleFunc("POST /api/resume", h.command(h.rec.Resume))
	mux.HandleFunc("POST /api/stop", h.command(h.rec.Stop))
	mux.HandleFunc("PUT /api/settings", h.settings)
	mux.HandleFunc("PUT /api/mic", h.mic)
	mux.HandleFunc("POST /api/browse", h.browse)
	mux.HandleFunc("POST /api/quit", h.quit)
	return secure(guard(mux))
}

func asset(name, contentType string) http.Handler {
	body, err := fs.ReadFile(web, name)
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err != nil {
			http.Error(w, "page missing from the build", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	})
}

func (h *handler) start(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if !readJSON(w, r, &req) {
		return
	}
	h.reply(w, r)(h.rec.Start(r.Context(), application.StartCommand{Title: req.Title}))
}

func (h *handler) settings(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if !readJSON(w, r, &req) {
		return
	}
	cmd := application.SettingsCommand{Folder: req.Folder, NoiseCleanup: req.NoiseCleanup}
	h.reply(w, r)(h.rec.ChangeSettings(r.Context(), cmd))
}

func (h *handler) mic(w http.ResponseWriter, r *http.Request) {
	var req micRequest
	if !readJSON(w, r, &req) {
		return
	}
	h.reply(w, r)(h.rec.ChooseMic(r.Context(), application.ChooseMicCommand{ID: req.ID}))
}

func (h *handler) browse(w http.ResponseWriter, r *http.Request) {
	res, err := h.rec.Browse(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, browseResponse{Folder: res.Folder, Chosen: res.Chosen})
}

func (h *handler) quit(w http.ResponseWriter, r *http.Request) {
	if err := h.rec.Quit(r.Context()); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) command(useCase func(context.Context) (application.Status, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.reply(w, r)(useCase(r.Context()))
	}
}

// reply answers a command with the new status, or its failure.
func (h *handler) reply(w http.ResponseWriter, r *http.Request) func(application.Status, error) {
	return func(st application.Status, err error) {
		if err != nil {
			h.fail(w, r, err)
			return
		}
		writeJSON(w, statusOf(st))
	}
}

// fail is the one place an error becomes a status (ADR-0014); anything unexpected stays in the log.
func (h *handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, application.ErrInvalidCommand):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, application.ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		h.logger.ErrorContext(r.Context(), "request failed", "path", r.URL.Path, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(into); err != nil {
		http.Error(w, "malformed body", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(body)
}
