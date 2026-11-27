package main

import (
	"cmp"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type task struct {
	ID    uint64 `json:"id"`
	Title string `json:"title"`
}

type taskStore struct {
	mu     sync.RWMutex
	nextID uint64
	tasks  map[uint64]task
}

func (s *taskStore) create(title string) task {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	created := task{ID: s.nextID, Title: title}
	s.tasks[created.ID] = created
	return created
}

func (s *taskStore) get(id uint64) (task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	found, ok := s.tasks[id]
	return found, ok
}

func (s *taskStore) list() []task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make([]task, 0, len(s.tasks))
	for _, stored := range s.tasks {
		tasks = append(tasks, stored)
	}
	return tasks
}

func (s *taskStore) remove(id uint64) (alreadyDeleted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[id]; !ok {
		return id > 0 && id <= s.nextID
	}
	delete(s.tasks, id)
	return false
}

func newHandler() http.Handler {
	store := &taskStore{tasks: make(map[uint64]task)}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Title string `json:"title"`
		}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(input.Title) == "" {
			http.Error(w, "title must not be blank", http.StatusBadRequest)
			return
		}

		writeJSON(w, http.StatusCreated, store.create(input.Title))
	})

	mux.HandleFunc("GET /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		found, ok := store.get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, found)
	})

	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
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

		tasks := store.list()
		slices.SortFunc(tasks, func(a, b task) int {
			return cmp.Compare(a.ID, b.ID)
		})
		writeJSON(w, http.StatusOK, tasks[:min(limit+1, len(tasks))])
	})

	mux.HandleFunc("DELETE /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
		if err == nil && store.remove(id) {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func main() {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
