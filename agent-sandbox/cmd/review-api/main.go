package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type task struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

type api struct {
	mu     sync.Mutex
	tasks  map[int]task
	nextID int
}

type errorResponse struct {
	Error string `json:"error"`
}

func main() {
	a := &api{tasks: make(map[int]task), nextID: 1}
	log.Fatal(http.ListenAndServe(":8080", a))
}

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if r.URL.Path == "/tasks" {
		switch r.Method {
		case http.MethodPost:
			a.createTask(w, r)
		case http.MethodGet:
			a.listTasks(w)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if strings.HasPrefix(r.URL.Path, "/tasks/") {
		a.handleTask(w, r, strings.TrimPrefix(r.URL.Path, "/tasks/"))
		return
	}

	http.NotFound(w, r)
}

func (a *api) createTask(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title string `json:"title"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(input.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	a.mu.Lock()
	t := task{ID: a.nextID, Title: input.Title}
	a.nextID++
	a.tasks[t.ID] = t
	a.mu.Unlock()

	writeJSON(w, http.StatusCreated, t)
}

func (a *api) listTasks(w http.ResponseWriter) {
	a.mu.Lock()
	tasks := make([]task, 0, len(a.tasks))
	for _, t := range a.tasks {
		tasks = append(tasks, t)
	}
	a.mu.Unlock()

	writeJSON(w, http.StatusOK, tasks)
}

func (a *api) handleTask(w http.ResponseWriter, r *http.Request, rawID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, err := strconv.Atoi(rawID)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid task ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.mu.Lock()
		t, exists := a.tasks[id]
		a.mu.Unlock()
		if !exists {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeJSON(w, http.StatusOK, t)
	case http.MethodDelete:
		a.mu.Lock()
		_, exists := a.tasks[id]
		if exists {
			delete(a.tasks, id)
		}
		a.mu.Unlock()
		if !exists {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
