package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func booksCLIMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/ebooks/library", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"books":[{"id":"b1","nodeId":"n1","title":"Go","authors":["A"],"format":"epub","tags":[],"hasCover":true}],"total":1}`))
	})
	return mux
}

func TestBooksListCmd(t *testing.T) {
	srv := httptest.NewServer(booksCLIMux(t))
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"books", "list"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Go")) {
		t.Fatalf("out:\n%s", out.String())
	}
}

func TestBooksListPassesQuery(t *testing.T) {
	gotQ := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/ebooks/library", func(w http.ResponseWriter, r *http.Request) {
		gotQ <- r.URL.Query().Get("q")
		_, _ = w.Write([]byte(`{"books":[],"total":0}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"books", "list", "--q", "golang"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	select {
	case q := <-gotQ:
		if q != "golang" {
			t.Fatalf("q=%q", q)
		}
	default:
		t.Fatal("the request did not arrive")
	}
}
