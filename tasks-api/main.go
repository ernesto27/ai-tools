package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxBodyBytes  = 1 << 20
	maxTitleRunes = 200
)

type task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type taskStore struct {
	mu     sync.RWMutex
	nextID int
	tasks  map[int]task
}

func newHandler() http.Handler {
	store := &taskStore{tasks: make(map[int]task)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", store.create)
	mux.HandleFunc("GET /tasks", store.list)
	mux.HandleFunc("GET /tasks/{id}", store.fetch)
	mux.HandleFunc("DELETE /tasks/{id}", store.delete)
	return mux
}

func (s *taskStore) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title string `json:"title"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&input)
	if err == nil {
		if extra := decoder.Decode(new(json.RawMessage)); extra != io.EOF {
			err = extra
			if err == nil {
				err = errors.New("trailing data")
			}
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if err != nil {
		http.Error(w, "expected a single JSON object", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(input.Title) == "" {
		http.Error(w, "title must not be blank", http.StatusBadRequest)
		return
	}
	if utf8.RuneCountInString(input.Title) > maxTitleRunes {
		http.Error(w, "title must be at most 200 characters", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.nextID++
	created := task{ID: s.nextID, Title: input.Title}
	s.tasks[created.ID] = created
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, created)
}

func (s *taskStore) list(w http.ResponseWriter, r *http.Request) {
	limit := 20
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "invalid query", http.StatusBadRequest)
		return
	}
	if query.Has("limit") {
		limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			http.Error(w, "limit must be an integer from 1 to 100", http.StatusBadRequest)
			return
		}
	}

	s.mu.RLock()
	tasks := make([]task, 0, len(s.tasks))
	for _, item := range s.tasks {
		tasks = append(tasks, item)
	}
	s.mu.RUnlock()
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	tasks = tasks[:min(len(tasks), limit)]
	writeJSON(w, http.StatusOK, tasks)
}

func (s *taskStore) fetch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	s.mu.RLock()
	item, exists := s.tasks[id]
	s.mu.RUnlock()
	if !exists {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *taskStore) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.mu.Lock()
	delete(s.tasks, id)
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func main() {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
