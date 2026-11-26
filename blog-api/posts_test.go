package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPostLifecyclePersistsToSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blog #1.db")
	db, err := openDatabase(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	handler := newHandler(db)
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
		if response.Code != status {
			t.Fatalf("%s %s returned %d, want %d: %s", method, path, response.Code, status, response.Body)
		}
		return response
	}
	decodePost := func(response *httptest.ResponseRecorder) post {
		t.Helper()
		var value post
		if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}

	createdResponse := request(http.MethodPost, "/posts", `{"title":"First post","content":"  Hello, SQLite!\n"}`, http.StatusCreated)
	location := createdResponse.Header().Get("Location")
	created := decodePost(createdResponse)
	if location != "/posts/1" || created.ID != 1 || created.Title != "First post" || created.Content != "  Hello, SQLite!\n" {
		t.Fatalf("unexpected created post at %q: %+v", location, created)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, created.CreatedAt)
	if err != nil || created.UpdatedAt != created.CreatedAt {
		t.Fatalf("unexpected creation timestamps: %+v", created)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openDatabase(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	handler = newHandler(reopened)
	if got := decodePost(request(http.MethodGet, location, "", http.StatusOK)); got != created {
		t.Fatalf("persisted post = %+v, want %+v", got, created)
	}

	request(http.MethodPut, location, `{"title":"Invalid","content":" "}`, http.StatusBadRequest)
	if got := decodePost(request(http.MethodGet, location, "", http.StatusOK)); got != created {
		t.Fatalf("invalid update changed the post: %+v", got)
	}
	updated := decodePost(request(http.MethodPut, location, `{"title":"Updated","content":"New content"}`, http.StatusOK))
	updatedAt, err := time.Parse(time.RFC3339Nano, updated.UpdatedAt)
	if err != nil || updatedAt.Before(createdAt) || updated.CreatedAt != created.CreatedAt ||
		updated.ID != created.ID || updated.Title != "Updated" || updated.Content != "New content" {
		t.Fatalf("unexpected updated post: %+v", updated)
	}
	request(http.MethodPost, "/posts", `{"title":"Second","content":"Another post"}`, http.StatusCreated)
	list := request(http.MethodGet, "/posts?limit=1&offset=1", "", http.StatusOK)
	var posts []post
	if err := json.NewDecoder(list.Body).Decode(&posts); err != nil || len(posts) != 1 || posts[0] != updated {
		t.Fatalf("unexpected posts page: %+v (%v)", posts, err)
	}

	deleted := request(http.MethodDelete, location, "", http.StatusNoContent)
	if deleted.Body.Len() != 0 {
		t.Fatalf("delete response has a body: %s", deleted.Body)
	}
	request(http.MethodGet, location, "", http.StatusNotFound)
	request(http.MethodPut, location, `{"title":"Missing","content":"No post"}`, http.StatusNotFound)
	request(http.MethodDelete, location, "", http.StatusNotFound)
}
