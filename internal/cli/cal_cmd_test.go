package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func calCLIMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start") == "" || r.URL.Query().Get("end") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`[{"uid":"e1","summary":"Meeting","start":"2026-07-02T10:00:00Z","end":"2026-07-02T11:00:00Z","calendar_id":"cal1"}]`))
	})
	mux.HandleFunc("GET /me/calendars", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"cal1","name":"Personal","is_default":true,"is_owner":true}]`))
	})
	return mux
}

func TestCalEventsCmd(t *testing.T) {
	srv := httptest.NewServer(calCLIMux(t))
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"cal", "events", "--from", "2026-07-01T00:00:00Z", "--to", "2026-07-08T00:00:00Z"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Meeting")) {
		t.Fatalf("out:\n%s", out.String())
	}
}

func TestCalRmIrreversibleHelp(t *testing.T) {
	var out, errb bytes.Buffer
	Run([]string{"cal", "--help"}, &out, &errb)
	if !bytes.Contains(out.Bytes(), []byte("Irreversible")) {
		t.Fatalf("cal --help must mark rm as irreversible:\n%s", out.String())
	}
}

// The --alarm value must reach the server's POST body as the "alarm" field.
func TestCalAddAlarmFlag(t *testing.T) {
	mux := calCLIMux(t)
	gotAlarm := make(chan string, 1)
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		alarm, _ := body["alarm"].(string)
		gotAlarm <- alarm
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	code := Run([]string{"cal", "add", "--summary", "Sync call",
		"--start", "2026-07-21T10:00:00Z", "--alarm", "30"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	select {
	case got := <-gotAlarm:
		if got != "30" {
			t.Fatalf("alarm in request body = %q, want %q", got, "30")
		}
	default:
		t.Fatal("server did not receive POST /me/calendar/events")
	}
}

// A garbage --alarm value must fail before any request reaches the server.
func TestCalAddAlarmInvalid(t *testing.T) {
	mux := calCLIMux(t)
	posted := make(chan struct{}, 1)
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		posted <- struct{}{}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	code := Run([]string{"cal", "add", "--summary", "Sync call",
		"--start", "2026-07-21T10:00:00Z", "--alarm", "half an hour"}, &out, &errb)
	if code == 0 {
		t.Fatal("expected a non-zero exit code for --alarm \"half an hour\"")
	}
	select {
	case <-posted:
		t.Fatal("a request with a garbage alarm must not reach the server")
	default:
	}
}

// The extended fields must reach the POST body, with the times in UTC.
func TestCalAddExtendedFields(t *testing.T) {
	t.Setenv("TZ", "Europe/Berlin") // recurring events need an IANA zone; Windows exposes none
	mux := calCLIMux(t)
	gotBody := make(chan map[string]any, 1)
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotBody <- body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	code := Run([]string{"cal", "add", "--summary", "Standup", "--start", "2026-07-21T10:00:00+03:00",
		"--description", "daily sync", "--location", "Zoom", "--repeat", "daily",
		"--until", "2026-08-21T10:00:00+03:00", "--alarm", "5"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	body := <-gotBody
	for field, want := range map[string]any{
		"description": "daily sync", "location": "Zoom", "freq": "DAILY",
		"start": "2026-07-21T07:00:00Z", "until": "2026-08-21T00:00:00Z", "alarm": "5",
	} {
		if body[field] != want {
			t.Errorf("%s = %v, want %v", field, body[field], want)
		}
	}
}

func TestCalAddAllDay(t *testing.T) {
	mux := calCLIMux(t)
	gotBody := make(chan map[string]any, 1)
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotBody <- body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"cal", "add", "--summary", "Vacation", "--start", "2026-08-01",
		"--end", "2026-08-15", "--all-day"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	body := <-gotBody
	if body["all_day"] != true || body["start"] != "2026-08-01T00:00:00Z" || body["end"] != "2026-08-15T00:00:00Z" {
		t.Fatalf("all-day body: %v", body)
	}
}

// cal set changes only the flags given and leaves the rest of the event alone.
func TestCalSetPartial(t *testing.T) {
	mux := calCLIMux(t)
	gotBody := make(chan map[string]any, 1)
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"uid":"e1","summary":"Standup","location":"Office","description":"daily sync",
			"start":"2026-07-21T10:00:00Z","end":"2026-07-21T10:15:00Z","alarm":"5","calendar_id":"cal1"}`))
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotBody <- body
		_, _ = w.Write([]byte(`{"uid":"e1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"cal", "set", "e1", "--location", "Zoom"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	body := <-gotBody
	if body["location"] != "Zoom" || body["summary"] != "Standup" || body["description"] != "daily sync" || body["alarm"] != "5" {
		t.Fatalf("partial update body: %v", body)
	}
}

// --alarm keep is an update-only value: it leaves the reminders alone and is the
// way to rewrite an event without changing anything about it.
func TestCalSetAlarmKeep(t *testing.T) {
	mux := calCLIMux(t)
	gotBody := make(chan map[string]any, 1)
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"uid":"e1","summary":"Standup","start":"2026-07-21T10:00:00+06:00","end":"2026-07-21T10:15:00+06:00","alarm":"5","calendar_id":"cal1"}`))
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotBody <- body
		_, _ = w.Write([]byte(`{"uid":"e1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"cal", "set", "e1", "--alarm", "keep"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	body := <-gotBody
	if body["alarm"] != "keep" {
		t.Fatalf("alarm = %v, want \"keep\"", body["alarm"])
	}
	if body["start"] != "2026-07-21T04:00:00Z" {
		t.Fatalf("rewriting must normalize the time to UTC, got %v", body["start"])
	}
}

// Without a single field flag, cal set must not touch the event at all.
func TestCalSetWithoutFlags(t *testing.T) {
	mux := calCLIMux(t)
	written := make(chan struct{}, 1)
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		written <- struct{}{}
		_, _ = w.Write([]byte(`{"uid":"e1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"cal", "set", "e1"}, &out, &errb); code == 0 {
		t.Fatal("expected a non-zero exit code when no field flag is given")
	}
	select {
	case <-written:
		t.Fatal("nothing must be written without a field flag")
	default:
	}
}

func TestCalShow(t *testing.T) {
	mux := calCLIMux(t)
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"uid":"e1","summary":"Standup","description":"daily sync",
			"start":"2026-07-21T10:00:00Z","all_day":false,"freq":"DAILY","alarm":"5","calendar_id":"cal1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"cal", "show", "e1"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	for _, want := range []string{"Standup", "daily sync", "DAILY"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("%q missing from the output:\n%s", want, out.String())
		}
	}
}

// TestCalRmCalendarFlagAfterUID — regression: --calendar after a positional uid
// used to be silently ignored because flag.Parse stops at the first non-flag,
// and the event was deleted from the default calendar instead of the specified one.
func TestCalRmCalendarFlagAfterUID(t *testing.T) {
	mux := calCLIMux(t)
	gotCalendarID := make(chan string, 1)
	mux.HandleFunc("DELETE /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		gotCalendarID <- r.URL.Query().Get("calendar_id")
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"cal", "rm", "e1", "--calendar", "mycal"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}

	select {
	case got := <-gotCalendarID:
		if got != "mycal" {
			t.Fatalf("calendar_id = %q, want %q", got, "mycal")
		}
	default:
		t.Fatal("server did not receive the DELETE request")
	}
}
