package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthentication(t *testing.T) {
	a := &app{token: strings.Repeat("a", 32)}
	called := false
	handler := a.auth(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) })
	for _, token := range []string{"", "Bearer wrong"} {
		r := httptest.NewRequest("POST", "/api/evidence", nil)
		r.Header.Set("Authorization", token)
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != 401 || called {
			t.Fatal("unauthorized request accepted")
		}
	}
	r := httptest.NewRequest("POST", "/api/evidence", nil)
	r.Header.Set("Authorization", "Bearer "+a.token)
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != 204 || !called {
		t.Fatal("valid token rejected")
	}
}
func TestInputValidationBeforeDependencies(t *testing.T) {
	a := &app{}
	for _, body := range []string{`{}`, `{"id":"../x","title":"title","content":"x"}`, `{"id":"valid","title":"title","content":"x","extra":true}`, `{"id":"valid","title":"title","content":"x"} {}`, strings.Repeat("x", 70000)} {
		w := httptest.NewRecorder()
		a.create(w, httptest.NewRequest("POST", "/api/evidence", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	}
}
