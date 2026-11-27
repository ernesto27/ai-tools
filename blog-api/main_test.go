package main

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostLifecycleAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blog.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	h := api{db}.routes()
	request := func(method, path, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != want {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	w := request("POST", "/posts", `{"title":" First ","content":"Hello"}`, 201)
	var created post
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID != 1 || created.Title != "First" || w.Header().Get("Location") != "/posts/1" {
		t.Fatalf("bad create: %s (%v)", w.Body, err)
	}
	request("POST", "/posts", `{"title":"","content":"Hello"}`, 400)
	request("POST", "/posts", `{"title":"x","content":"y","extra":1}`, 400)
	request("GET", "/posts/nope", "", 400)
	request("PUT", "/posts/1", `{"title":"Updated","content":"Body"}`, 200)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h = api{db}.routes()
	w = request("GET", "/posts/1", "", 200)
	var saved post
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || saved.Title != "Updated" || saved.CreatedAt != created.CreatedAt {
		t.Fatalf("bad saved post: %s (%v)", w.Body, err)
	}
	w = request("GET", "/posts", "", 200)
	var posts []post
	if err := json.Unmarshal(w.Body.Bytes(), &posts); err != nil || len(posts) != 1 {
		t.Fatalf("bad list: %s (%v)", w.Body, err)
	}
	request("DELETE", "/posts/1", "", 204)
	request("GET", "/posts/1", "", 404)
	request("DELETE", "/posts/1", "", 404)
	request("PUT", "/posts/1", `{"title":"x","content":"y"}`, 404)
	w = request("GET", "/posts", "", 200)
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("expected empty array: %s", w.Body)
	}
}
