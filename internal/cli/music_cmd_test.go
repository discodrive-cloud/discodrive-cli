package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func musicCLIMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/music/radio", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"r1","name":"Jazz","streamUrl":"http://s/1","homepageUrl":""}]`))
	})
	return mux
}

func TestMusicRadioListCmd(t *testing.T) {
	srv := httptest.NewServer(musicCLIMux(t))
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"music", "radio"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Jazz")) {
		t.Fatalf("out:\n%s", out.String())
	}
}

func TestMusicListCmd(t *testing.T) {
	mux := musicCLIMux(t)
	mux.HandleFunc("GET /me/music", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"enabled":true,"folder":{"id":"mf1","name":"music"},"hasPassword":true,"tagEditVersioning":true}`))
	})
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("parent_id") != "mf1" {
			http.Error(w, `{"error":"expected parent_id=mf1"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`[{"id":"a1","name":"Cosmic Gate","is_dir":true,"size":null,"version":1}]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"music", "list"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Cosmic Gate")) {
		t.Fatalf("out:\n%s", out.String())
	}
}

func TestMusicListNotConfigured(t *testing.T) {
	mux := musicCLIMux(t)
	mux.HandleFunc("GET /me/music", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"enabled":false,"folder":null,"hasPassword":false,"tagEditVersioning":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"music", "list"}, &out, &errb); code == 0 {
		t.Fatal("expected an error: music is not configured")
	}
}

func TestMusicScanCmd(t *testing.T) {
	called := make(chan bool, 1)
	mux := musicCLIMux(t)
	mux.HandleFunc("POST /me/music/scan", func(w http.ResponseWriter, r *http.Request) {
		called <- true
		_, _ = w.Write([]byte(`{"scanning":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"music", "scan"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	select {
	case <-called:
	default:
		t.Fatal("POST /me/music/scan was not called")
	}
}
