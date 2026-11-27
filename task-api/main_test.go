package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestTaskLifecycle(t *testing.T) {
	handler := newTaskHandler()

	create := func(body, title string) task {
		t.Helper()
		response := serve(handler, http.MethodPost, "/tasks", body)
		var created task
		checkJSON(t, response, http.StatusCreated, &created)
		if created.ID <= 0 || created.Title != title {
			t.Fatalf("created task = %+v, want positive ID and title %q", created, title)
		}
		return created
	}
	first := create(`{"title":"first"}`, "first")
	second := create(`{"title":"second"}`, "second")
	if second.ID <= first.ID {
		t.Fatalf("task IDs did not increase: %d, %d", first.ID, second.ID)
	}
	path := "/tasks/" + strconv.Itoa(first.ID)

	var fetched task
	checkJSON(t, serve(handler, http.MethodGet, path, ""), http.StatusOK, &fetched)
	if fetched != first {
		t.Fatalf("fetched task = %+v, want %+v", fetched, first)
	}

	var listed []task
	checkJSON(t, serve(handler, http.MethodGet, "/tasks?limit=2", ""), http.StatusOK, &listed)
	if len(listed) != 2 || listed[0] != first || listed[1] != second {
		t.Fatalf("listed tasks = %+v, want [%+v %+v]", listed, first, second)
	}

	deleted := serve(handler, http.MethodDelete, path, "")
	if deleted.Code != http.StatusNoContent || deleted.Body.Len() != 0 {
		t.Fatalf("delete returned status %d and body %q, want 204 and an empty body", deleted.Code, deleted.Body)
	}
	missing := serve(handler, http.MethodGet, path, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("fetch after delete returned %d, want 404", missing.Code)
	}
}

func TestBadRequests(t *testing.T) {
	handler := newTaskHandler()
	for _, body := range []string{`{"title":`, `{"title":" "}`, `{"title":"valid"} {}`} {
		if response := serve(handler, http.MethodPost, "/tasks", body); response.Code != http.StatusBadRequest {
			t.Errorf("POST with body %q returned %d, want 400", body, response.Code)
		}
	}
	for _, limit := range []string{"", "0", "101", "foo"} {
		if response := serve(handler, http.MethodGet, "/tasks?limit="+limit, ""); response.Code != http.StatusBadRequest {
			t.Errorf("GET with limit %q returned %d, want 400", limit, response.Code)
		}
	}
}

func serve(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func checkJSON(t *testing.T, response *httptest.ResponseRecorder, status int, value any) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d. Body: %s", response.Code, status, response.Body)
	}
	if err := json.Unmarshal(response.Body.Bytes(), value); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
