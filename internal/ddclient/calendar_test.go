package ddclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func calMux(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/calendars", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"cal1","name":"Personal","color":"#f00","is_default":true,"is_owner":true}]`))
	})
	mux.HandleFunc("GET /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start") == "" || r.URL.Query().Get("end") == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"start and end are required (RFC3339)"}`))
			return
		}
		_, _ = w.Write([]byte(`[{"uid":"e1","summary":"Meeting","start":"2026-07-02T10:00:00Z","end":"2026-07-02T11:00:00Z","all_day":false,"recurring":false,"calendar_id":"cal1"}]`))
	})
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e2"}`))
	})
	mux.HandleFunc("DELETE /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"uid":"e1"}`))
	})
	return mux
}

func TestCalendarsAndEvents(t *testing.T) {
	srv := httptest.NewServer(calMux(t))
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	cals, err := c.ListCalendars(ctx)
	if err != nil || len(cals) != 1 || cals[0].ID != "cal1" || !cals[0].IsDefault {
		t.Fatalf("cals: %+v err=%v", cals, err)
	}
	if _, err := c.ListEvents(ctx, "", ""); !IsAPIStatus(err, 400) {
		t.Fatalf("expected 400 on missing start/end, got %v", err)
	}
	evs, err := c.ListEvents(ctx, "2026-07-01T00:00:00Z", "2026-07-08T00:00:00Z")
	if err != nil || len(evs) != 1 || evs[0].UID != "e1" || evs[0].Summary != "Meeting" || evs[0].CalendarID != "cal1" {
		t.Fatalf("events: %+v err=%v", evs, err)
	}
	uid, err := c.CreateEvent(ctx, EventForm{Summary: "X", Start: "2026-07-03T09:00:00Z", End: "2026-07-03T10:00:00Z"})
	if err != nil || uid != "e2" {
		t.Fatalf("create: %q err=%v", uid, err)
	}
	if err := c.DeleteEvent(ctx, "e1", "cal1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestCheckAlarm(t *testing.T) {
	for in, ok := range map[string]bool{
		"":     true,
		"0":    true,
		"5":    true,
		"30":   true,
		"1440": true,
		"abc":  false,
		"-5":   false,
		"5m":   false,
		" 30":  false,
		"keep": false, // update-only value; not accepted on create
	} {
		err := CheckAlarm(in)
		if ok && err != nil {
			t.Errorf("CheckAlarm(%q) = %v, want nil", in, err)
		}
		if !ok && err == nil {
			t.Errorf("CheckAlarm(%q) = nil, want error", in)
		}
	}
}

func TestCheckFreq(t *testing.T) {
	for in, want := range map[string]string{
		"":        "",
		"daily":   "DAILY",
		"WEEKLY":  "WEEKLY",
		" yearly": "YEARLY",
	} {
		got, err := CheckFreq(in)
		if err != nil || got != want {
			t.Errorf("CheckFreq(%q) = %q, %v; want %q, nil", in, got, err, want)
		}
	}
	for _, bad := range []string{"hourly", "every week", "DAILY;COUNT=3"} {
		if _, err := CheckFreq(bad); err == nil {
			t.Errorf("CheckFreq(%q) = nil error, want an error", bad)
		}
	}
}

// A numeric offset must be converted to UTC: the server stores anything else as a
// floating local time, which shifts the event for every CalDAV client that reads it.
func TestNormalizeTime(t *testing.T) {
	for in, want := range map[string]string{
		"":                          "",
		"2026-08-05T10:00:00Z":      "2026-08-05T10:00:00Z",
		"2026-08-05T10:00:00+03:00": "2026-08-05T07:00:00Z",
		"2026-08-05T13:30:00-02:30": "2026-08-05T16:00:00Z",
	} {
		got, err := NormalizeTime(in)
		if err != nil || got != want {
			t.Errorf("NormalizeTime(%q) = %q, %v; want %q, nil", in, got, err, want)
		}
	}
	// A value without a zone is read in the machine's local time.
	got, err := NormalizeTime("2026-08-05 10:00")
	if err != nil {
		t.Fatalf("NormalizeTime local: %v", err)
	}
	want := time.Date(2026, 8, 5, 10, 0, 0, 0, time.Local).UTC().Format(time.RFC3339)
	if got != want {
		t.Errorf("NormalizeTime(\"2026-08-05 10:00\") = %q, want %q", got, want)
	}
	if _, err := NormalizeTime("next friday"); err == nil {
		t.Error("garbage time must be rejected")
	}
}

// All-day boundaries keep their calendar date: converting them to UTC first would
// move the date by a day for any zone east or west of Greenwich.
func TestNormalizeDate(t *testing.T) {
	for in, want := range map[string]string{
		"2026-08-05":                "2026-08-05T00:00:00Z",
		"2026-08-05T00:00:00+06:00": "2026-08-05T00:00:00Z",
		"2026-08-05T23:00:00-05:00": "2026-08-05T00:00:00Z",
	} {
		got, err := NormalizeDate(in)
		if err != nil || got != want {
			t.Errorf("NormalizeDate(%q) = %q, %v; want %q, nil", in, got, err, want)
		}
	}
}

// CreateEvent must normalize the body it sends and reject values the server would drop.
func TestCreateEventNormalizesBody(t *testing.T) {
	bodies := make(chan map[string]any, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies <- body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e2"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	if _, err := c.CreateEvent(ctx, EventForm{
		Summary: "Call", Start: "2026-08-05T10:00:00+03:00", End: "2026-08-05T11:00:00+03:00",
		Description: "notes", Freq: "weekly", Until: "2026-09-05T10:00:00+03:00",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	body := <-bodies
	if body["start"] != "2026-08-05T07:00:00Z" || body["end"] != "2026-08-05T08:00:00Z" {
		t.Fatalf("times not normalized to UTC: %v", body)
	}
	if body["freq"] != "WEEKLY" || body["until"] != "2026-09-05T00:00:00Z" {
		t.Fatalf("recurrence not normalized: %v", body)
	}
	if body["description"] != "notes" {
		t.Fatalf("description lost: %v", body)
	}

	// An all-day event keeps its dates and gets an explicit next-day end.
	if _, err := c.CreateEvent(ctx, EventForm{
		Summary: "Vacation", Start: "2026-08-01", AllDay: true,
	}); err != nil {
		t.Fatalf("create all-day: %v", err)
	}
	body = <-bodies
	if body["start"] != "2026-08-01T00:00:00Z" || body["end"] != "2026-08-02T00:00:00Z" {
		t.Fatalf("all-day boundaries: %v", body)
	}

	if _, err := c.CreateEvent(ctx, EventForm{Summary: "X", Start: "2026-08-05T10:00:00Z", Freq: "fortnightly"}); err == nil {
		t.Fatal("an unsupported freq must be rejected before the request")
	}
	if _, err := c.CreateEvent(ctx, EventForm{Summary: "X", Start: "2026-08-05T10:00:00Z", Until: "2026-09-05T10:00:00Z"}); err == nil {
		t.Fatal("until without freq must be rejected")
	}
	select {
	case body := <-bodies:
		t.Fatalf("a rejected form must not reach the server: %v", body)
	default:
	}
}

// PatchEvent changes only the fields given and carries the rest over from the server.
func TestPatchEvent(t *testing.T) {
	put := make(chan map[string]any, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"uid":"e1","summary":"Standup","location":"Office","description":"daily sync",
			"start":"2026-08-05T10:00:00+06:00","end":"2026-08-05T10:15:00+06:00","all_day":false,
			"freq":"DAILY","until":"","alarm":"keep","calendar_id":"cal1"}`))
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		put <- body
		_, _ = w.Write([]byte(`{"uid":"e1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")

	location, alarm := "", "15"
	form, err := c.PatchEvent(context.Background(), "e1", EventPatch{Location: &location, Alarm: &alarm})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	body := <-put
	if body["summary"] != "Standup" || body["description"] != "daily sync" || body["freq"] != "DAILY" {
		t.Fatalf("untouched fields must be carried over: %v", body)
	}
	if body["location"] != "" {
		t.Fatalf("an explicit empty value must clear the field: %v", body)
	}
	if body["alarm"] != "15" {
		t.Fatalf("alarm = %v, want \"15\"", body["alarm"])
	}
	// The time comes back from the server in its own zone; the write must be UTC.
	if body["start"] != "2026-08-05T04:00:00Z" {
		t.Fatalf("start = %v, want the UTC form 2026-08-05T04:00:00Z", body["start"])
	}
	if form.Start != "2026-08-05T04:00:00Z" || form.Location != "" {
		t.Fatalf("returned form does not match what was written: %+v", form)
	}
}

// An alarm the server could not map to a preset comes back as "keep" and must survive
// a patch: on create the same value is a mistake and is rejected.
func TestPatchEventKeepsUnknownAlarm(t *testing.T) {
	put := make(chan map[string]any, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"uid":"e1","summary":"Standup","start":"2026-08-05T10:00:00Z","end":"2026-08-05T11:00:00Z","alarm":"keep","calendar_id":"cal1"}`))
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		put <- body
		_, _ = w.Write([]byte(`{"uid":"e1"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")

	summary := "Standup (moved)"
	if _, err := c.PatchEvent(context.Background(), "e1", EventPatch{Summary: &summary}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if body := <-put; body["alarm"] != "keep" {
		t.Fatalf("alarm = %v, want \"keep\"", body["alarm"])
	}
}

func TestUpdateEvent(t *testing.T) {
	srv := httptest.NewServer(calMux(t))
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	uid, err := c.UpdateEvent(ctx, "e1", EventForm{Summary: "Updated", Start: "2026-07-03T09:00:00Z", End: "2026-07-03T10:00:00Z"})
	if err != nil || uid != "e1" {
		t.Fatalf("update: %q err=%v", uid, err)
	}
}

// multiCalMux serves two calendars where event e9 lives in the non-default one, and
// resolves a uid strictly inside the calendar asked for — like the server does.
func multiCalMux(t *testing.T, deleted *[]string, put *[]string) http.Handler {
	t.Helper()
	const homeCal = "cal2"
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/calendars", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"cal1","name":"Personal","is_default":true,"is_owner":true},
			{"id":"cal2","name":"Home","is_default":false,"is_owner":true}]`))
	})
	// calendar_id defaults to cal1 exactly as the server's resolveCal does.
	calOf := func(r *http.Request) string {
		if id := r.URL.Query().Get("calendar_id"); id != "" {
			return id
		}
		return "cal1"
	}
	notFound := func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"event not found"}`))
	}
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("uid") != "e9" || calOf(r) != homeCal {
			notFound(w)
			return
		}
		_, _ = w.Write([]byte(`{"uid":"e9","summary":"Dentist","start":"2026-07-02T10:00:00Z","end":"2026-07-02T11:00:00Z","alarm":"keep","calendar_id":"cal2"}`))
	})
	mux.HandleFunc("DELETE /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("uid") != "e9" || calOf(r) != homeCal {
			notFound(w)
			return
		}
		*deleted = append(*deleted, calOf(r))
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		var f EventForm
		if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
			t.Errorf("put decode: %v", err)
		}
		if f.CalendarID != homeCal {
			notFound(w)
			return
		}
		*put = append(*put, f.Summary)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	return mux
}

// Without calendar_id the server only looks in the default calendar; the client has to
// find the uid in the others instead of reporting a 404.
func TestEventUIDResolvedAcrossCalendars(t *testing.T) {
	var deleted, put []string
	srv := httptest.NewServer(multiCalMux(t, &deleted, &put))
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	ev, err := c.GetEvent(ctx, "e9", "")
	if err != nil || ev.Summary != "Dentist" || ev.CalendarID != "cal2" {
		t.Fatalf("get without calendar_id: %+v err=%v", ev, err)
	}
	if _, err := c.PatchEvent(ctx, "e9", EventPatch{Summary: strptr("Dentist 2")}); err != nil {
		t.Fatalf("patch without calendar_id: %v", err)
	}
	if len(put) != 1 || put[0] != "Dentist 2" {
		t.Fatalf("put = %v, want one write of the new summary", put)
	}
	if err := c.DeleteEvent(ctx, "e9", ""); err != nil {
		t.Fatalf("delete without calendar_id: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "cal2" {
		t.Fatalf("deleted = %v, want one delete in cal2", deleted)
	}
	// A uid that is in no calendar still fails, and with the server's own 404.
	if _, err := c.GetEvent(ctx, "nope", ""); !IsNotFound(err) {
		t.Fatalf("get missing uid: %v, want 404", err)
	}
	if err := c.DeleteEvent(ctx, "nope", ""); !IsNotFound(err) {
		t.Fatalf("delete missing uid: %v, want 404", err)
	}
	// An explicit calendar is taken at face value: no search, no delete elsewhere.
	if err := c.DeleteEvent(ctx, "e9", "cal1"); !IsNotFound(err) {
		t.Fatalf("delete with explicit wrong calendar: %v, want 404", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("deleted = %v, want no extra delete", deleted)
	}
}

func strptr(s string) *string { return &s }

// A uid is unique only per calendar on the server (UNIQUE (calendar_id, uid)), so the
// same uid can sit in two non-default calendars — e.g. an own copy of an invitation and
// a calendar shared by the organizer. Without calendar_id the client must not pick one
// of them: reading reports the ambiguity, and nothing is written or deleted.
func TestEventUIDAmbiguousAcrossCalendars(t *testing.T) {
	var deleted, put []string
	holders := map[string]bool{"cal2": true, "cal3": true}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/calendars", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"cal1","name":"Personal","is_default":true,"is_owner":true},
			{"id":"cal2","name":"Home","is_default":false,"is_owner":true},
			{"id":"cal3","name":"Team","is_default":false,"is_owner":false}]`))
	})
	calOf := func(r *http.Request) string {
		if id := r.URL.Query().Get("calendar_id"); id != "" {
			return id
		}
		return "cal1"
	}
	notFound := func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"event not found"}`))
	}
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		cal := calOf(r)
		if r.PathValue("uid") != "e9" || !holders[cal] {
			notFound(w)
			return
		}
		_, _ = w.Write([]byte(`{"uid":"e9","summary":"Sync","start":"2026-07-02T10:00:00Z","end":"2026-07-02T11:00:00Z","alarm":"keep","calendar_id":"` + cal + `"}`))
	})
	mux.HandleFunc("DELETE /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("uid") != "e9" || !holders[calOf(r)] {
			notFound(w)
			return
		}
		deleted = append(deleted, calOf(r))
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		var f EventForm
		_ = json.NewDecoder(r.Body).Decode(&f)
		put = append(put, f.CalendarID)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	wantAmbiguous := func(what string, err error) {
		t.Helper()
		var amb *AmbiguousEventError
		if !errors.As(err, &amb) {
			t.Fatalf("%s: err = %v, want AmbiguousEventError", what, err)
		}
		if len(amb.CalendarIDs) != 2 || amb.CalendarIDs[0] != "cal2" || amb.CalendarIDs[1] != "cal3" {
			t.Fatalf("%s: calendars = %v, want [cal2 cal3]", what, amb.CalendarIDs)
		}
		if !strings.Contains(err.Error(), "cal2") || !strings.Contains(err.Error(), "cal3") {
			t.Fatalf("%s: message %q does not name the calendars", what, err.Error())
		}
	}
	_, err := c.GetEvent(ctx, "e9", "")
	wantAmbiguous("get", err)
	_, err = c.PatchEvent(ctx, "e9", EventPatch{Summary: strptr("changed")})
	wantAmbiguous("patch", err)
	wantAmbiguous("delete", c.DeleteEvent(ctx, "e9", ""))
	if len(deleted) != 0 || len(put) != 0 {
		t.Fatalf("mutated an ambiguous uid: deleted=%v put=%v", deleted, put)
	}
	// Naming the calendar resolves it.
	if err := c.DeleteEvent(ctx, "e9", "cal3"); err != nil || len(deleted) != 1 || deleted[0] != "cal3" {
		t.Fatalf("explicit delete: err=%v deleted=%v", err, deleted)
	}
}

// While scanning for the uid, only a 404 means "not in this calendar". Any other
// failure (5xx, 403, network) leaves the answer unknown: the calendar may hold the
// uid too, so picking another one could act on the wrong event. It must abort.
func TestEventUIDScanAbortsOnNon404(t *testing.T) {
	var deleted, put []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/calendars", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"cal1","name":"Personal","is_default":true,"is_owner":true},
			{"id":"calA","name":"Flaky","is_default":false,"is_owner":true},
			{"id":"calB","name":"Home","is_default":false,"is_owner":true}]`))
	})
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("calendar_id") {
		case "calA":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal error"}`))
		case "calB":
			_, _ = w.Write([]byte(`{"uid":"e9","summary":"Sync","start":"2026-07-02T10:00:00Z","end":"2026-07-02T11:00:00Z","alarm":"keep","calendar_id":"calB"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"event not found"}`))
		}
	})
	mux.HandleFunc("DELETE /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("calendar_id") == "" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"event not found"}`))
			return
		}
		deleted = append(deleted, r.URL.Query().Get("calendar_id"))
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		put = append(put, r.URL.Path)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(srv.URL, "kfd")
	ctx := context.Background()

	if err := c.DeleteEvent(ctx, "e9", ""); !IsAPIStatus(err, http.StatusInternalServerError) {
		t.Fatalf("delete: err = %v, want the 500 from calA", err)
	}
	if _, err := c.PatchEvent(ctx, "e9", EventPatch{Summary: strptr("x")}); !IsAPIStatus(err, http.StatusInternalServerError) {
		t.Fatalf("patch: err = %v, want the 500 from calA", err)
	}
	if _, err := c.GetEvent(ctx, "e9", ""); !IsAPIStatus(err, http.StatusInternalServerError) {
		t.Fatalf("get: err = %v, want the 500 from calA", err)
	}
	if len(deleted) != 0 || len(put) != 0 {
		t.Fatalf("acted on calB despite calA being unknown: deleted=%v put=%v", deleted, put)
	}
}
