package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouterServesMigratedEmptyDatabase(t *testing.T) {
	application, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	router := application.Router("http://127.0.0.1:19421")

	for _, path := range []string{
		"/api/health", "/api/app/status", "/api/github/auth/status",
		"/api/repositories", "/api/activities", "/api/members",
		"/api/pull-requests", "/api/risks", "/api/reports",
		"/api/sync-jobs", "/api/settings",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.RemoteAddr = "127.0.0.1:12345"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, body=%s", path, response.Code, response.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/api/sync-jobs", strings.NewReader(`{"repository_ids":[]}`))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Origin", "http://127.0.0.1:19421")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/sync-jobs status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "INVALID_ARGUMENT" || payload.Error.RequestID == "" {
		t.Fatalf("error payload = %#v", payload)
	}
}

func TestRouterRejectsNonLocalRequest(t *testing.T) {
	application, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.RemoteAddr = "192.0.2.10:12345"
	response := httptest.NewRecorder()
	application.Router("http://127.0.0.1:19421").ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}
