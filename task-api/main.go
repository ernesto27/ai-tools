package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type taskAPI struct {
	mu     sync.RWMutex
	lastID int
	tasks  map[int]task
}

func main() {
	log.Fatal(http.ListenAndServe("localhost:8080", newTaskHandler()))
}

func newTaskHandler() http.Handler {
	api := &taskAPI{tasks: make(map[int]task)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", api.createTask)
	mux.HandleFunc("GET /tasks/{id}", api.getTask)
	mux.HandleFunc("GET /tasks", api.listTasks)
	mux.HandleFunc("DELETE /tasks/{id}", api.deleteTask)
	return mux
}

func (api *taskAPI) createTask(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title string `json:"title"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid task JSON", http.StatusBadRequest)
		return
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "invalid task JSON", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(input.Title) == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	api.mu.Lock()
	api.lastID++
	created := task{ID: api.lastID, Title: input.Title}
	api.tasks[created.ID] = created
	api.mu.Unlock()

	writeJSON(w, http.StatusCreated, created)
}

func (api *taskAPI) getTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	api.mu.RLock()
	found, exists := api.tasks[id]
	api.mu.RUnlock()
	if !exists {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, found)
}

func (api *taskAPI) listTasks(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "invalid query", http.StatusBadRequest)
		return
	}
	limit := 20
	if query.Has("limit") {
		limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			http.Error(w, "limit must be an integer from 1 to 100", http.StatusBadRequest)
			return
		}
	}

	api.mu.RLock()
	tasks := make([]task, 0, len(api.tasks))
	for _, entry := range api.tasks {
		tasks = append(tasks, entry)
	}
	api.mu.RUnlock()
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})
	if len(tasks) > limit {
		tasks = tasks[:limit+1]
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (api *taskAPI) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err == nil {
		api.mu.Lock()
		_, exists := api.tasks[id]
		wasDeleted := !exists && id > 0 && id <= api.lastID
		delete(api.tasks, id)
		api.mu.Unlock()
		if wasDeleted {
			http.NotFound(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}
