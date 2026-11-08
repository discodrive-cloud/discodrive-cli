package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestTasksWithLists(t *testing.T) {
	var postBody string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/task-lists", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"L1","name":"Reminders","is_default":true,"is_owner":true},{"id":"L2","name":"Покупки","is_owner":false,"owner_email":"a@x"}]`))
	})
	mux.HandleFunc("GET /me/tasks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"uid":"t1","summary":"Хлеб","list_id":"L2"}]`))
	})
	mux.HandleFunc("POST /me/tasks", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		postBody = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"t2"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"tasks", "lists"}, &out, &errb); code != 0 {
		t.Fatalf("lists: code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Покупки") || !strings.Contains(out.String(), "a@x") {
		t.Fatalf("lists output:\n%s", out.String())
	}

	out.Reset()
	if code := Run([]string{"tasks"}, &out, &errb); code != 0 {
		t.Fatalf("list: code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Хлеб [Покупки]") {
		t.Fatalf("tasks show their list:\n%s", out.String())
	}

	out.Reset()
	if code := Run([]string{"tasks", "add", "--summary", "Сыр", "--list", "покупки"}, &out, &errb); code != 0 {
		t.Fatalf("add: code=%d err=%s", code, errb.String())
	}
	if !strings.Contains(postBody, `"list_id":"L2"`) {
		t.Fatalf("add --list sends the resolved id: %s", postBody)
	}
}
