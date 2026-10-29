package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func contactsMux(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/contacts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"uid":"c1","full_name":"Alice","emails":["a@x.io"],"phones":[]}]`))
	})
	mux.HandleFunc("GET /me/contacts/{uid}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("uid") != "c1" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"contact not found"}`))
			return
		}
		_, _ = w.Write([]byte(`{"uid":"c1","full_name":"Alice","emails":[{"type":"home","value":"a@x.io"}],"phones":[],"org":"Acme"}`))
	})
	mux.HandleFunc("POST /me/contacts", func(w http.ResponseWriter, r *http.Request) {
		var f Contact
		_ = json.NewDecoder(r.Body).Decode(&f)
		if f.FullName == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"new"}`))
	})
	mux.HandleFunc("DELETE /me/contacts/{uid}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func TestContactsListGetCreateDelete(t *testing.T) {
	srv := httptest.NewServer(contactsMux(t))
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	list, err := c.ListContacts(ctx)
	if err != nil || len(list) != 1 || list[0].UID != "c1" || len(list[0].Emails) != 1 || list[0].Emails[0] != "a@x.io" {
		t.Fatalf("list: %+v err=%v", list, err)
	}
	got, err := c.GetContact(ctx, "c1")
	if err != nil || got.Org != "Acme" || len(got.Emails) != 1 || got.Emails[0].Type != "home" || got.Emails[0].Value != "a@x.io" {
		t.Fatalf("get: %+v err=%v", got, err)
	}
	if _, err := c.GetContact(ctx, "missing"); !IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
	uid, err := c.CreateContact(ctx, Contact{FullName: "Bob"})
	if err != nil || uid != "new" {
		t.Fatalf("create: uid=%q err=%v", uid, err)
	}
	if err := c.DeleteContact(ctx, "c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// TestUpdateContactAndDelete200: UpdateContact (PUT → {uid}) and DeleteContact's tolerance of 200.
func TestUpdateContactAndDelete200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("PUT /me/contacts/{uid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"uid":"c1"}`))
	})
	mux.HandleFunc("DELETE /me/contacts/{uid}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // 200 instead of 204 — exercises the tolerant branch of expect204
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	uid, err := c.UpdateContact(ctx, "c1", Contact{FullName: "Alice"})
	if err != nil || uid != "c1" {
		t.Fatalf("update: uid=%q err=%v", uid, err)
	}
	if err := c.DeleteContact(ctx, "c1"); err != nil {
		t.Fatalf("delete(200): %v", err)
	}
}
