package ddclient

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPairInitAndPoll(t *testing.T) {
	var polls int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /pair/init", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Name, Kind string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name == "" || body.Kind != "desktop" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "ABCD-EFGH",
			"verification_uri": "/app/pair?code=ABCD-EFGH", "interval": 1, "expires_in": 600,
		})
	})
	mux.HandleFunc("POST /pair/token", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&polls, 1) < 3 {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "approved", "device_token": "kfd_xyz"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p, err := PairInit(context.Background(), srv.URL, "laptop", "desktop", false)
	if err != nil || p.DeviceCode != "dc" || p.UserCode != "ABCD-EFGH" || p.VerificationURI != "/app/pair?code=ABCD-EFGH" {
		t.Fatalf("init: %+v err=%v", p, err)
	}
	tok, err := PairPoll(context.Background(), srv.URL, p.DeviceCode, 5*time.Millisecond, false)
	if err != nil || tok != "kfd_xyz" {
		t.Fatalf("poll: tok=%q err=%v", tok, err)
	}
}

func TestPairInteractive(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /pair/init", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "WXYZ-1234",
			"verification_uri": "/app/pair?code=WXYZ-1234", "interval": 1, "expires_in": 600,
		})
	})
	mux.HandleFunc("POST /pair/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "approved", "device_token": "kfd_ok"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var out bytes.Buffer
	tok, err := PairInteractive(context.Background(), srv.URL, "laptop", "mcp", false, &out)
	if err != nil || tok != "kfd_ok" {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
	// the user should be shown the code and the confirmation link
	if !strings.Contains(out.String(), "WXYZ-1234") || !strings.Contains(out.String(), "/app/pair?code=WXYZ-1234") {
		t.Fatalf("no instructions in output:\n%s", out.String())
	}
}

func TestPairPollExpired(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /pair/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "expired"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if _, err := PairPoll(context.Background(), srv.URL, "dc", 5*time.Millisecond, false); err == nil {
		t.Fatal("expected error on expired")
	}
}
