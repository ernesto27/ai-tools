package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func createTask(t *testing.T, handler http.Handler, title string) task {
	t.Helper()
	body, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewReader(body)))
	if response.Code != http.StatusCreated {
		t.Fatalf("POST /tasks = %d, want 201. Body: %s", response.Code, response.Body)
	}
	var created task
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	return created
}

func TestCreateTask(t *testing.T) {
	created := createTask(t, newHandler(), "example")
	if created.ID == 0 || created.Title != "example" {
		t.Fatalf("created task = %+v, want an assigned ID and title example", created)
	}
}

func TestFetchTask(t *testing.T) {
	handler := newHandler()
	created := createTask(t, handler, "example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/tasks/%d", created.ID), nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET task = %d, want 200. Body: %s", response.Code, response.Body)
	}
	var fetched task
	if err := json.NewDecoder(response.Body).Decode(&fetched); err != nil {
		t.Fatal(err)
	}
	if fetched != created {
		t.Fatalf("fetched task = %+v, want %+v", fetched, created)
	}
}

func TestListTasks(t *testing.T) {
	handler := newHandler()
	want := []task{createTask(t, handler, "one"), createTask(t, handler, "two")}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tasks?limit=2", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /tasks = %d, want 200. Body: %s", response.Code, response.Body)
	}
	var listed []task
	if err := json.NewDecoder(response.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(listed, want) {
		t.Fatalf("listed tasks = %+v, want %+v", listed, want)
	}
}

func TestDeleteTask(t *testing.T) {
	handler := newHandler()
	created := createTask(t, handler, "example")
	path := fmt.Sprintf("/tasks/%d", created.ID)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, path, nil))
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("DELETE task = %d with body %q, want 204 with an empty body", response.Code, response.Body)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("GET deleted task = %d, want 404", response.Code)
	}
}
