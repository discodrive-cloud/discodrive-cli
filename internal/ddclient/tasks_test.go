package ddclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

	list, err := c.ListTasks(ctx, "")
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

const taskListsJSON = `[
 {"id":"L1","name":"Reminders","is_default":true,"is_owner":true},
 {"id":"L2","name":"Покупки","is_owner":true},
 {"id":"L3","name":"Reminders","is_owner":false,"owner_email":"a@x"}]`

func TestTaskListsAndResolve(t *testing.T) {
	var gotQuery, gotListID string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/task-lists", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(taskListsJSON))
	})
	mux.HandleFunc("GET /me/tasks", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("list")
		_, _ = w.Write([]byte(`[{"uid":"t1","summary":"Хлеб","list_id":"L2"}]`))
	})
	mux.HandleFunc("POST /me/tasks", func(w http.ResponseWriter, r *http.Request) {
		var b Task
		_ = json.NewDecoder(r.Body).Decode(&b)
		gotListID = b.ListID
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"t2"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	lists, err := c.ListTaskLists(ctx)
	if err != nil || len(lists) != 3 || !lists[0].IsDefault || lists[2].IsOwner || lists[2].OwnerEmail != "a@x" {
		t.Fatalf("lists: %+v err=%v", lists, err)
	}
	if l, err := c.ResolveTaskList(ctx, "покупки"); err != nil || l.ID != "L2" {
		t.Fatalf("by name, any case: %+v err=%v", l, err)
	}
	if l, err := c.ResolveTaskList(ctx, "L3"); err != nil || l.ID != "L3" {
		t.Fatalf("by id: %+v err=%v", l, err)
	}
	if _, err := c.ResolveTaskList(ctx, "Reminders"); err == nil || !strings.Contains(err.Error(), "L1") || !strings.Contains(err.Error(), "L3") {
		t.Fatalf("a shared name is ambiguous and names the ids: %v", err)
	}
	if _, err := c.ResolveTaskList(ctx, "Дом"); !errors.Is(err, ErrTaskListNotFound) {
		t.Fatalf("unknown list: %v", err)
	}

	items, err := c.ListTasks(ctx, "L2")
	if err != nil || gotQuery != "L2" || len(items) != 1 || items[0].ListID != "L2" {
		t.Fatalf("list of one list: %+v query=%q err=%v", items, gotQuery, err)
	}
	if _, err := c.ListTasks(ctx, ""); err != nil || gotQuery != "" {
		t.Fatalf("all lists send no list parameter: query=%q err=%v", gotQuery, err)
	}
	if _, err := c.CreateTask(ctx, Task{Summary: "Сыр", ListID: "L2"}); err != nil || gotListID != "L2" {
		t.Fatalf("create in a list: %q err=%v", gotListID, err)
	}
	if _, err := c.CreateTask(ctx, Task{Summary: "x"}); err != nil || gotListID != "" {
		t.Fatalf("create without a list sends none: %q err=%v", gotListID, err)
	}
}
