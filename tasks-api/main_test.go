package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestTaskLifecycle(t *testing.T) {
	handler := newHandler()
	first := createTask(t, handler, "first")
	second := createTask(t, handler, "second")
	path := fmt.Sprintf("/tasks/%d", first.ID)

	var fetched task
	readJSON(t, request(t, handler, "GET", path, "", http.StatusOK), &fetched)
	if fetched != first {
		t.Fatalf("fetched %+v, want %+v", fetched, first)
	}

	var listed []task
	readJSON(t, request(t, handler, "GET", "/tasks?limit=10", "", http.StatusOK), &listed)
	if !slices.Equal(listed, []task{first, second}) {
		t.Fatalf("listed %+v, want [%+v %+v]", listed, first, second)
	}

	deleted := request(t, handler, "DELETE", path, "", http.StatusNoContent)
	if deleted.Body.Len() != 0 {
		t.Fatalf("delete returned a body: %q", deleted.Body.String())
	}
	request(t, handler, "GET", path, "", http.StatusNotFound)
}

func TestBadRequests(t *testing.T) {
	handler := newHandler()
	for _, body := range []string{`{"title":`, `{"title":"  "}`, `{"title":"valid"} {}`} {
		request(t, handler, "POST", "/tasks", body, http.StatusBadRequest)
	}
	for _, limit := range []string{"0", "101", "abc", ""} {
		request(t, handler, "GET", "/tasks?limit="+limit, "", http.StatusBadRequest)
	}
}

func TestListRespectsLimit(t *testing.T) {
	handler := newHandler()
	first := createTask(t, handler, "first")
	second := createTask(t, handler, "second")
	createTask(t, handler, "third")

	var listed []task
	readJSON(t, request(t, handler, "GET", "/tasks?limit=2", "", http.StatusOK), &listed)
	if !slices.Equal(listed, []task{first, second}) {
		t.Fatalf("listed %+v, want [%+v %+v]", listed, first, second)
	}
}

func TestRepeatedDeleteReturnsNoContent(t *testing.T) {
	handler := newHandler()
	created := createTask(t, handler, "doomed")
	path := fmt.Sprintf("/tasks/%d", created.ID)
	for attempt := 1; attempt <= 2; attempt++ {
		deleted := request(t, handler, "DELETE", path, "", http.StatusNoContent)
		if deleted.Body.Len() != 0 {
			t.Fatalf("delete attempt %d returned a body: %q", attempt, deleted.Body.String())
		}
	}
}

func TestCreateRejectsOversizedInput(t *testing.T) {
	handler := newHandler()
	huge := `{"title":"` + strings.Repeat("a", 1<<20) + `"}`
	request(t, handler, "POST", "/tasks", huge, http.StatusRequestEntityTooLarge)
	request(t, handler, "POST", "/tasks", `{"title":"`+strings.Repeat("é", 201)+`"}`, http.StatusBadRequest)
	createTask(t, handler, strings.Repeat("é", 200))
}

func createTask(t *testing.T, handler http.Handler, title string) task {
	t.Helper()
	body, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		t.Fatal(err)
	}
	response := request(t, handler, "POST", "/tasks", string(body), http.StatusCreated)
	var created task
	readJSON(t, response, &created)
	if created.ID < 1 || created.Title != title {
		t.Fatalf("unexpected created task: %+v", created)
	}
	return created
}

func readJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}

func request(t *testing.T, handler http.Handler, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != status {
		t.Fatalf("%s %s returned %d, want %d: %s", method, path, response.Code, status, response.Body)
	}
	return response
}
