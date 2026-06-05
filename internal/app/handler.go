package app

import (
	"encoding/json"
	"net/http"
)

const (
	AppName = "erzhuang-project"
	Version = "v2"
)

type HealthResponse struct {
	App     string `json:"app"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /api/tasks", tasksHandler)
	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		App:     AppName,
		Status:  "ok",
		Version: Version,
	})
}

func tasksHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []Task{
		{ID: 1, Title: "学习 Codex 本地开发", Done: true},
		{ID: 2, Title: "用 Git 管理版本", Done: false},
		{ID: 3, Title: "部署到腾讯云 Lighthouse", Done: false},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
