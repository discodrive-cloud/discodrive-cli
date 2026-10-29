package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "jwt"})
	})
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "u1", "email": "me@example.com", "role": "user",
			"tenant_id": "t1", "must_change_password": false,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	u, err := New(srv.URL, "kfd").Me(context.Background())
	if err != nil || u.Email != "me@example.com" || u.Role != "user" {
		t.Fatalf("me: %+v err=%v", u, err)
	}
}
