package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handlers struct {
	Health                    http.HandlerFunc
	AppStatus                 http.HandlerFunc
	Shutdown                  http.HandlerFunc
	AuthStatus                http.HandlerFunc
	SetToken                  http.HandlerFunc
	ClearToken                http.HandlerFunc
	ListRepositories          http.HandlerFunc
	UpdateRepositorySelection http.HandlerFunc
	StartSync                 http.HandlerFunc
	ListSyncJobs              http.HandlerFunc
	GetSyncJob                http.HandlerFunc
	ListActivities            http.HandlerFunc
	ListMembers               http.HandlerFunc
	ListPullRequests          http.HandlerFunc
	GetPullRequest            http.HandlerFunc
	ListRisks                 http.HandlerFunc
	UpdateRisk                http.HandlerFunc
	GenerateReport            http.HandlerFunc
	ListReports               http.HandlerFunc
	GetReport                 http.HandlerFunc
	DownloadReport            http.HandlerFunc
	GetSettings               http.HandlerFunc
	UpdateSettings            http.HandlerFunc
	SPA                       http.HandlerFunc
}

func Router(origin string, handlers Handlers) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Timeout(60*time.Second))
	router.Use(localOnly, sameOrigin(origin))
	router.Get("/api/health", handlers.Health)
	router.Get("/api/app/status", handlers.AppStatus)
	router.Post("/api/system/shutdown", handlers.Shutdown)
	router.Get("/api/github/auth/status", handlers.AuthStatus)
	router.Post("/api/github/auth/token", handlers.SetToken)
	router.Delete("/api/github/auth", handlers.ClearToken)
	router.Get("/api/repositories", handlers.ListRepositories)
	router.Patch("/api/repositories/selection", handlers.UpdateRepositorySelection)
	router.Post("/api/sync-jobs", handlers.StartSync)
	router.Get("/api/sync-jobs", handlers.ListSyncJobs)
	router.Get("/api/sync-jobs/{id}", handlers.GetSyncJob)
	router.Get("/api/activities", handlers.ListActivities)
	router.Get("/api/members", handlers.ListMembers)
	router.Get("/api/pull-requests", handlers.ListPullRequests)
	router.Get("/api/pull-requests/{id}", handlers.GetPullRequest)
	router.Get("/api/risks", handlers.ListRisks)
	router.Patch("/api/risks/{id}", handlers.UpdateRisk)
	router.Post("/api/reports", handlers.GenerateReport)
	router.Get("/api/reports", handlers.ListReports)
	router.Get("/api/reports/{id}", handlers.GetReport)
	router.Get("/api/reports/{id}/download", handlers.DownloadReport)
	router.Get("/api/settings", handlers.GetSettings)
	router.Patch("/api/settings", handlers.UpdateSettings)
	router.Get("/*", handlers.SPA)
	return router
}

func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.RemoteAddr, "127.0.0.1:") && !strings.HasPrefix(request.RemoteAddr, "[::1]:") {
			http.Error(response, "local access only", http.StatusForbidden)
			return
		}
		next.ServeHTTP(response, request)
	})
}

func sameOrigin(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet && request.Method != http.MethodHead && request.Method != http.MethodOptions {
				requestOrigin := request.Header.Get("Origin")
				if requestOrigin != "" && requestOrigin != origin && strings.Replace(requestOrigin, "localhost", "127.0.0.1", 1) != origin {
					http.Error(response, "invalid origin", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(response, request)
		})
	}
}
