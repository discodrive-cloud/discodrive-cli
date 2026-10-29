package ddclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func musicMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/music/radio", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"r1","name":"Jazz","streamUrl":"http://s/1","homepageUrl":""}]`))
	})
	mux.HandleFunc("POST /me/music/radio", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"r2","name":"Rock","streamUrl":"http://s/2","homepageUrl":"http://h"}`))
	})
	mux.HandleFunc("DELETE /me/music/radio/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /me/music/podcasts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"p1","title":"Go Time","feedUrl":"http://f","coverUrl":"","hasCover":false}]`))
	})
	mux.HandleFunc("POST /me/music/podcasts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"p2","title":"New","feedUrl":"http://f2","coverUrl":"","hasCover":false}`))
	})
	mux.HandleFunc("POST /me/music/scan", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"scanning":true}`))
	})
	return mux
}

func TestListRadio(t *testing.T) {
	srv := httptest.NewServer(musicMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	got, err := c.ListRadio(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Jazz" {
		t.Fatalf("got %+v", got)
	}
}

func TestCreateRadioReturnsFullObject(t *testing.T) {
	srv := httptest.NewServer(musicMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	got, err := c.CreateRadio(context.Background(), Radio{Name: "Rock", StreamURL: "http://s/2", HomepageURL: "http://h"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "r2" || got.HomepageURL != "http://h" {
		t.Fatalf("expected a full object, got %+v", got)
	}
}

func TestDeleteRadio(t *testing.T) {
	srv := httptest.NewServer(musicMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	if err := c.DeleteRadio(context.Background(), "r1"); err != nil {
		t.Fatal(err)
	}
}

func TestAddPodcastAccepts200And201(t *testing.T) {
	srv := httptest.NewServer(musicMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	got, err := c.AddPodcast(context.Background(), "http://f2")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "p2" {
		t.Fatalf("got %+v", got)
	}
}

// AddPodcast must also accept 200 (the feed is already subscribed — the server returns the
// existing subscription), not only 201. Regressing to "201-only" would silently break this normal case.
func TestAddPodcastAccepts200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("POST /me/music/podcasts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"p1","title":"Existing","feedUrl":"http://f","coverUrl":"","hasCover":false}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")
	got, err := c.AddPodcast(context.Background(), "http://f")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "p1" || got.Title != "Existing" {
		t.Fatalf("expected the existing subscription from the 200 response, got %+v", got)
	}
}

func TestListMusicFolder(t *testing.T) {
	mux := musicMux()
	mux.HandleFunc("GET /me/music", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"enabled":true,"folder":{"id":"mf1","name":"music"},"hasPassword":true,"tagEditVersioning":true}`))
	})
	mux.HandleFunc("GET /files", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("parent_id") {
		case "mf1":
			_, _ = w.Write([]byte(`[{"id":"art1","name":"Cosmic Gate","is_dir":true,"size":null,"version":1}]`))
		case "art1":
			_, _ = w.Write([]byte(`[{"id":"tr1","name":"track.mp3","is_dir":false,"size":123,"version":1}]`))
		default:
			http.Error(w, `{"error":"unexpected parent_id"}`, http.StatusBadRequest)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")

	// root of the music folder
	top, err := c.ListMusicFolder(context.Background(), "")
	if err != nil || len(top) != 1 || top[0].Name != "Cosmic Gate" {
		t.Fatalf("root: %+v err=%v", top, err)
	}
	// descend into a subfolder
	inner, err := c.ListMusicFolder(context.Background(), "Cosmic Gate")
	if err != nil || len(inner) != 1 || inner[0].Name != "track.mp3" {
		t.Fatalf("subfolder: %+v err=%v", inner, err)
	}
	// a nonexistent subfolder is an error
	if _, err := c.ListMusicFolder(context.Background(), "No such folder"); err == nil {
		t.Fatal("expected an error on a nonexistent subfolder")
	}
}

func TestListMusicFolderNotConfigured(t *testing.T) {
	mux := musicMux()
	mux.HandleFunc("GET /me/music", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"enabled":false,"folder":null}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")
	if _, err := c.ListMusicFolder(context.Background(), ""); err == nil {
		t.Fatal("expected an error: music is not configured")
	}
}

func TestMusicScan(t *testing.T) {
	srv := httptest.NewServer(musicMux())
	defer srv.Close()
	c := New(srv.URL, "kfd")
	scanning, err := c.MusicScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !scanning {
		t.Fatal("expected scanning=true")
	}
}
