package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func tasksMux(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/tasks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"uid":"t1","summary":"Buy bread","priority":1,"completed":false}]`))
	})
	mux.HandleFunc("POST /me/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"t2"}`))
	})
	mux.HandleFunc("PUT /me/tasks/{uid}/done", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Completed bool `json:"completed"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if !b.Completed {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"uid":"t1"}`))
	})
	mux.HandleFunc("PUT /me/tasks/{uid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"uid":"t1"}`))
	})
	mux.HandleFunc("DELETE /me/tasks/{uid}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func TestTasks(t *testing.T) {
	srv := httptest.NewServer(tasksMux(t))
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	list, err := c.ListTasks(ctx)
	if err != nil || len(list) != 1 || list[0].UID != "t1" || list[0].Priority != 1 || list[0].Completed {
		t.Fatalf("list: %+v err=%v", list, err)
	}
	uid, err := c.CreateTask(ctx, Task{Summary: "Test"})
	if err != nil || uid != "t2" {
		t.Fatalf("create: %q err=%v", uid, err)
	}
	if _, err := c.SetTaskDone(ctx, "t1", true); err != nil {
		t.Fatalf("done: %v", err)
	}
	if err := c.DeleteTask(ctx, "t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestUpdateTask(t *testing.T) {
	srv := httptest.NewServer(tasksMux(t))
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	uid, err := c.UpdateTask(ctx, "t1", Task{Summary: "Updated"})
	if err != nil || uid != "t1" {
		t.Fatalf("update: %q err=%v", uid, err)
	}
}
