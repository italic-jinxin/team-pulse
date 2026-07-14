package app

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed webdist/*
var webAssets embed.FS

type runtimeAuth struct {
	sync.RWMutex
	token  string
	source string
}

var auth runtimeAuth

func (a *App) Router(origin string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Timeout(60*time.Second))
	r.Use(localOnly, sameOrigin(origin))
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"status": "ok", "version": "0.1.0"})
	})
	r.Get("/api/app/status", a.status)
	r.Post("/api/system/shutdown", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 202, map[string]string{"status": "shutting_down"})
		select {
		case <-a.shutdown:
		default:
			close(a.shutdown)
		}
	})
	r.Get("/api/github/auth/status", a.authStatus)
	r.Post("/api/github/auth/token", a.setToken)
	r.Delete("/api/github/auth", a.clearToken)
	r.Get("/api/github/repositories", a.githubRepositories)
	r.Get("/api/repositories", a.listRepositories)
	r.Post("/api/repositories/sync", a.startSync)
	r.Get("/api/activity", a.listActivity)
	r.Get("/api/members", a.listMembers)
	r.Get("/api/pull-requests", a.listPullRequests)
	r.Get("/api/risks", a.listRisks)
	r.Get("/api/risk-rules", a.getRiskRules)
	r.Put("/api/risk-rules", a.setRiskRules)
	r.Patch("/api/risks/{id}", a.updateRisk)
	r.Get("/api/jobs", a.listJobs)
	r.Get("/api/jobs/{id}", a.getJob)
	r.Post("/api/reports/generate", a.generateReport)
	r.Get("/api/reports", a.listReports)
	r.Get("/api/reports/{id}", a.getReport)

	dist, _ := fs.Sub(webAssets, "webdist")
	files := http.FileServer(http.FS(dist))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(dist, path); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		b, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.Error(w, "web assets not built", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	return r
}

func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.RemoteAddr, "127.0.0.1:") && !strings.HasPrefix(r.RemoteAddr, "[::1]:") {
			http.Error(w, "local access only", 403)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func sameOrigin(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
				if o := r.Header.Get("Origin"); o != "" && o != origin && strings.Replace(o, "localhost", "127.0.0.1", 1) != origin {
					http.Error(w, "invalid origin", 403)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}
