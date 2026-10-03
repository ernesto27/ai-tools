package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func request(t *testing.T, handler http.Handler, method, path, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d; body = %s", method, path, w.Code, wantStatus, w.Body.String())
	}
	return w
}

func TestPostLifecycle(t *testing.T) {
	api := newBlogAPI()
	empty := request(t, api, "GET", "/posts", "", http.StatusOK)
	if strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Fatalf("empty collection = %s", empty.Body.String())
	}
	created := request(t, api, "POST", "/posts", `{"title":" First post ","content":" Hello "}`, http.StatusCreated)
	if created.Header().Get("Location") != "/posts/1" || created.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected response headers: %v", created.Header())
	}
	var original post
	if err := json.Unmarshal(created.Body.Bytes(), &original); err != nil {
		t.Fatal(err)
	}
	if original.ID != 1 || original.Title != "First post" || original.Content != "Hello" || original.CreatedAt.IsZero() || original.UpdatedAt != original.CreatedAt {
		t.Fatalf("unexpected created post: %+v", original)
	}
	fetched := request(t, api, "GET", "/posts/1", "", http.StatusOK)
	if fetched.Body.String() != created.Body.String() {
		t.Fatal("fetched post differs from created post")
	}
	updated := request(t, api, "PUT", "/posts/1", `{"title":"Updated","content":"New content"}`, http.StatusOK)
	var p post
	if err := json.Unmarshal(updated.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.ID != original.ID || p.CreatedAt != original.CreatedAt || p.UpdatedAt.Before(original.UpdatedAt) || p.Title != "Updated" || p.Content != "New content" {
		t.Fatalf("unexpected updated post: %+v", p)
	}
	request(t, api, "POST", "/posts", `{"title":"Second","content":"Post"}`, http.StatusCreated)
	listed := request(t, api, "GET", "/posts", "", http.StatusOK)
	var posts []post
	if err := json.Unmarshal(listed.Body.Bytes(), &posts); err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 || posts[0] != p || posts[1].ID != 2 {
		t.Fatalf("unexpected list: %+v", posts)
	}
	deleted := request(t, api, "DELETE", "/posts/1", "", http.StatusNoContent)
	if deleted.Body.Len() != 0 {
		t.Fatal("delete response must have no body")
	}
	request(t, api, "GET", "/posts/1", "", http.StatusNotFound)
	request(t, api, "DELETE", "/posts/1", "", http.StatusNotFound)
	request(t, api, "POST", "/posts", `{"title":"Third","content":"Post"}`, http.StatusCreated)
	request(t, api, "GET", "/posts/3", "", http.StatusOK)
}

func TestInvalidRequests(t *testing.T) {
	cases := []struct {
		name, method, path, body string
		status                   int
	}{
		{"malformed JSON", "POST", "/posts", "{", 400},
		{"empty body", "POST", "/posts", "", 400},
		{"missing content", "POST", "/posts", `{"title":"Title"}`, 400},
		{"blank title", "POST", "/posts", `{"title":"  ","content":"Content"}`, 400},
		{"wrong type", "POST", "/posts", `{"title":42,"content":"Content"}`, 400},
		{"unknown field", "POST", "/posts", `{"title":"Title","content":"Content","admin":true}`, 400},
		{"multiple values", "POST", "/posts", `{"title":"Title","content":"Content"} {}`, 400},
		{"null", "POST", "/posts", `null`, 400},
		{"oversized body", "POST", "/posts", `{"title":"Title","content":"` + strings.Repeat("x", 1<<20) + `"}`, 413},
		{"invalid id", "GET", "/posts/abc", "", 400},
		{"zero id", "GET", "/posts/0", "", 400},
		{"missing post", "GET", "/posts/1", "", 404},
		{"update missing post", "PUT", "/posts/1", `{"title":"Title","content":"Content"}`, 404},
		{"invalid update", "PUT", "/posts/1", `{"title":"Title"}`, 400},
		{"collection method", "DELETE", "/posts", "", 405},
		{"item method", "POST", "/posts/1", "", 405},
		{"unknown route", "GET", "/missing", "", 404},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newBlogAPI()
			w := request(t, api, tc.method, tc.path, tc.body, tc.status)
			var response map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response["error"] == "" {
				t.Fatalf("invalid error response: %s", w.Body.String())
			}
			if tc.status == 405 && w.Header().Get("Allow") == "" {
				t.Fatal("method rejection must advertise allowed methods")
			}
			list := request(t, api, "GET", "/posts", "", http.StatusOK)
			if strings.TrimSpace(list.Body.String()) != "[]" {
				t.Fatal("rejected request changed the collection")
			}
		})
	}
}

func TestConcurrentCreates(t *testing.T) {
	api := newBlogAPI()
	const count = 20
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/posts", strings.NewReader(fmt.Sprintf(`{"title":"Post %d","content":"Content"}`, i)))
			w := httptest.NewRecorder()
			api.ServeHTTP(w, r)
			if w.Code != http.StatusCreated {
				t.Errorf("create status = %d", w.Code)
			}
			api.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/posts", nil))
		}(i)
	}
	wg.Wait()
	w := request(t, api, "GET", "/posts", "", http.StatusOK)
	var posts []post
	if err := json.Unmarshal(w.Body.Bytes(), &posts); err != nil {
		t.Fatal(err)
	}
	if len(posts) != count {
		t.Fatalf("post count = %d, want %d", len(posts), count)
	}
	for i, p := range posts {
		if p.ID != uint64(i+1) {
			t.Fatalf("post %d has ID %d", i, p.ID)
		}
	}
}
