package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func tasksCLIMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/tasks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"uid":"t1","summary":"Buy bread","priority":1,"completed":false}]`))
	})
	return mux
}

func TestTasksListCmd(t *testing.T) {
	srv := httptest.NewServer(tasksCLIMux(t))
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"tasks"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Buy bread")) {
		t.Fatalf("out:\n%s", out.String())
	}
}

func TestTasksDoneCmd(t *testing.T) {
	done := make(chan bool, 1)
	mux := tasksCLIMux(t)
	mux.HandleFunc("PUT /me/tasks/{uid}/done", func(w http.ResponseWriter, r *http.Request) {
		done <- true
		_, _ = w.Write([]byte(`{"uid":"t1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"tasks", "done", "t1"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	select {
	case <-done:
	default:
		t.Fatal("PUT .../done was not called")
	}
}
