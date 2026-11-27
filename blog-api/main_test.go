package main

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite", t.TempDir()+"/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := initDB(db); err != nil {
		t.Fatal(err)
	}
	h := routes(db)
	request := func(method, path, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != want {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	request("GET", "/posts", "", 200)
	request("POST", "/posts", `{"title":" ","content":"body"}`, 400)
	request("POST", "/posts", `{"title":"one","content":"body"} garbage`, 400)
	w := request("POST", "/posts", `{"title":"one","content":"body"}`, 201)
	if w.Header().Get("Location") != "/posts/1" {
		t.Fatal("missing location")
	}
	w = request("PUT", "/posts/1", `{"title":"updated","content":"new"}`, 200)
	var p post
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Title != "updated" || p.CreatedAt == "" {
		t.Fatalf("unexpected post: %+v, %v", p, err)
	}
	w = request("GET", "/posts", "", 200)
	var posts []post
	if err := json.Unmarshal(w.Body.Bytes(), &posts); err != nil || len(posts) != 1 {
		t.Fatalf("unexpected list: %s", w.Body.String())
	}
	request("GET", "/posts/1", "", 200)
	request("GET", "/posts/nope", "", 400)
	request("DELETE", "/posts/1", "", 204)
	request("GET", "/posts/1", "", 404)
	request("DELETE", "/posts/1", "", 404)
}
