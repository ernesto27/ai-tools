package main

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blog.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	handler := api{db}.routes()
	request := func(method, path, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec
	}
	request("POST", "/posts", `{"title":" ","content":"body"}`, 400)
	request("POST", "/posts", `{"title":"Hi","content":"body"} {}`, 400)
	created := request("POST", "/posts", `{"title":"Hi","content":"body"}`, 201)
	if created.Header().Get("Location") != "/posts/1" {
		t.Fatalf("unexpected location: %s", created.Header().Get("Location"))
	}
	var p post
	if err := json.Unmarshal(created.Body.Bytes(), &p); err != nil || p.ID != 1 || p.CreatedAt == "" {
		t.Fatalf("unexpected post: %+v, %v", p, err)
	}
	request("PUT", "/posts/1", `{"title":"Changed","content":"new"}`, 200)
	db.Close()
	db, err = openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	handler = api{db}.routes()
	got := request("GET", "/posts/1", "", 200)
	if err := json.Unmarshal(got.Body.Bytes(), &p); err != nil || p.Title != "Changed" || p.Content != "new" {
		t.Fatalf("persisted post: %+v, %v", p, err)
	}
	list := request("GET", "/posts", "", 200)
	var posts []post
	if err := json.Unmarshal(list.Body.Bytes(), &posts); err != nil || len(posts) != 1 {
		t.Fatalf("list: %s, %v", list.Body.String(), err)
	}
	request("DELETE", "/posts/1", "", 204)
	request("GET", "/posts/1", "", 404)
	request("DELETE", "/posts/1", "", 404)
	request("GET", "/posts/nope", "", 400)
	request("GET", "/posts", "", 200)
}
