package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShareLink(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "jwt"})
	})
	mux.HandleFunc("POST /files/{id}/share", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Link             bool   `json:"link"`
			Access           string `json:"access"`
			ExpiresInSeconds *int64 `json:"expires_in_seconds"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if !b.Link || b.Access != "view" || b.ExpiresInSeconds == nil || *b.ExpiresInSeconds != 3600 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"share_id": "s1", "token": "tok", "access": "view"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	exp := int64(3600)
	res, err := New(srv.URL, "kfd").Share(context.Background(), "f1", ShareOpts{
		Link: true, Access: "view", ExpiresInSeconds: &exp,
	})
	if err != nil || res.Token != "tok" || res.ShareID != "s1" {
		t.Fatalf("share: %+v err=%v", res, err)
	}
}
