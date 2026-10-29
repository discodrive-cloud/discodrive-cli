package ddclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReviewPreserveComplexRecurrence(t *testing.T) {
	_, err := prepareForm(EventForm{Start: "2026-10-01T10:00:00Z", Freq: "keep"}, true)
	if err != nil {
		t.Fatalf("server freq=keep rejected on unchanged recurrence: %v", err)
	}
}
func TestReviewUntilRemainsCalendarDate(t *testing.T) {
	old := time.Local
	time.Local = time.FixedZone("Asia/Bishkek", 6*3600)
	defer func() { time.Local = old }()
	f, err := prepareForm(EventForm{Start: "2026-10-01T10:00:00+06:00", Freq: "DAILY", Until: "2026-10-10"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if f.Until != "2026-10-10T00:00:00Z" {
		t.Fatalf("until changed to %s", f.Until)
	}
}

func TestReviewRecurringEventCarriesNamedTimezone(t *testing.T) {
	old := time.Local
	zone, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	time.Local = zone
	defer func() { time.Local = old }()
	t.Setenv("TZ", "Europe/Berlin")
	var got map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"token":"jwt"}`) })
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(201)
		io.WriteString(w, `{"uid":"e1"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	_, err = New(srv.URL, "device").CreateEvent(context.Background(), EventForm{Summary: "Meeting", Start: "2026-10-20T10:00", Freq: "WEEKLY"})
	if err != nil {
		t.Fatal(err)
	}
	if got["tz"] != "Europe/Berlin" || got["start"] != "2026-10-20T08:00:00Z" {
		t.Fatalf("body=%v", got)
	}
}

func TestPatchEventPreservesServerRecurrenceAndTimezone(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"token":"jwt"}`) })
	mux.HandleFunc("GET /me/calendar/events/e1", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"uid":"e1","summary":"Old","start":"2026-10-20T10:00:00+02:00","end":"2026-10-20T11:00:00+02:00","freq":"keep","tz":"Europe/Berlin","calendar_id":"c1","until":"2026-12-20T00:00:00Z","alarm":"keep"}`)
	})
	var got EventForm
	mux.HandleFunc("PUT /me/calendar/events/e1", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		io.WriteString(w, `{"uid":"e1"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	title := "New"
	_, err := New(srv.URL, "device").PatchEvent(context.Background(), "e1", EventPatch{Summary: &title})
	if err != nil {
		t.Fatal(err)
	}
	if got.Freq != "keep" || got.TZ != "Europe/Berlin" || got.Until != "2026-12-20T00:00:00Z" || got.Summary != "New" || got.Alarm != "keep" {
		t.Fatalf("body=%+v", got)
	}
}

func TestExplicitTimezoneInterpretsNaiveTimeAndRejectsInvalidZone(t *testing.T) {
	f, err := prepareForm(EventForm{Start: "2026-10-20T10:00", Freq: "WEEKLY", TZ: "Europe/Berlin"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if f.Start != "2026-10-20T08:00:00Z" || f.TZ != "Europe/Berlin" {
		t.Fatalf("form=%+v", f)
	}
	_, err = prepareForm(EventForm{Start: "2026-10-20T10:00", Freq: "WEEKLY", TZ: "Invalid/Zone"}, false)
	if err == nil {
		t.Fatal("invalid timezone accepted")
	}
}
