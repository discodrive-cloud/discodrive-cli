package ddmcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPimDefaultHidesChanging(t *testing.T) {
	got := names(selectTools(append(toolSpecs(), pimToolSpecs()...), false))
	for _, want := range []string{"dd_contacts_list", "dd_calendar_list", "dd_calendar_events", "dd_calendar_create_event", "dd_calendar_get_event", "dd_tasks_list", "dd_tasks_create"} {
		if !got[want] {
			t.Fatalf("read/create tool %q must be available by default", want)
		}
	}
	for _, banned := range []string{"dd_contacts_delete", "dd_calendar_delete_event", "dd_calendar_update_event", "dd_tasks_delete", "dd_tasks_set_done"} {
		if got[banned] {
			t.Fatalf("mutating tool %q must not be available without --allow-changes", banned)
		}
	}
}

func TestPimAllowChangesExposesAll(t *testing.T) {
	got := names(selectTools(append(toolSpecs(), pimToolSpecs()...), true))
	for _, want := range []string{"dd_contacts_delete", "dd_calendar_delete_event", "dd_calendar_update_event", "dd_tasks_delete", "dd_tasks_set_done"} {
		if !got[want] {
			t.Fatalf("mutating tool %q must be available under --allow-changes", want)
		}
	}
}

// callTool runs a full MCP handshake against buildServer(c) and calls one tool.
func callTool(t *testing.T, apiURL, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	return callToolAllow(t, apiURL, name, args, false)
}

// callToolAllow is callTool with the changing tools enabled.
func callToolAllow(t *testing.T, apiURL, name string, args map[string]any, allowChanges bool) *mcp.CallToolResult {
	t.Helper()
	c := ddclient.New(apiURL, "tok")
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return buildServer(c, allowChanges) }, nil))
	defer httpServer.Close()

	ctx := t.Context()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

// contentText flattens a tool result's text content for error messages.
func contentText(res *mcp.CallToolResult) string {
	var out string
	for _, ct := range res.Content {
		if tc, ok := ct.(*mcp.TextContent); ok {
			out += tc.Text + "\n"
		}
	}
	return out
}

// The alarm from the tool input must reach the server's POST body.
func TestCreateEventAlarmReachesServer(t *testing.T) {
	gotBody := make(chan map[string]any, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		gotBody <- body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	api := httptest.NewServer(mux)
	defer api.Close()

	res := callTool(t, api.URL, "dd_calendar_create_event", map[string]any{
		"summary": "Sync call", "start": "2026-07-21T10:00:00Z", "alarm": "30",
	})
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	select {
	case body := <-gotBody:
		if body["alarm"] != "30" {
			t.Fatalf("alarm in the request body = %v, want \"30\"; body=%v", body["alarm"], body)
		}
	default:
		t.Fatal("server did not receive POST /me/calendar/events")
	}
}

// The extended fields must reach the server, with the times normalized to UTC:
// a time carrying a numeric offset is stored by the server as a floating local time.
func TestCreateEventExtendedFields(t *testing.T) {
	t.Setenv("TZ", "Europe/Berlin") // recurring events need an IANA zone; Windows exposes none
	gotBody := make(chan map[string]any, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		gotBody <- body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	api := httptest.NewServer(mux)
	defer api.Close()

	res := callTool(t, api.URL, "dd_calendar_create_event", map[string]any{
		"summary": "Standup", "start": "2026-07-21T10:00:00+03:00",
		"description": "daily sync", "location": "Zoom", "freq": "WEEKLY",
		"until": "2026-09-21T10:00:00+03:00", "alarm": "15",
	})
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	body := <-gotBody
	for field, want := range map[string]any{
		"description": "daily sync", "location": "Zoom", "freq": "WEEKLY",
		"start": "2026-07-21T07:00:00Z", "until": "2026-09-21T00:00:00Z",
	} {
		if body[field] != want {
			t.Errorf("%s = %v, want %v", field, body[field], want)
		}
	}
}

func TestCreateEventAllDay(t *testing.T) {
	gotBody := make(chan map[string]any, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		gotBody <- body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	api := httptest.NewServer(mux)
	defer api.Close()

	res := callTool(t, api.URL, "dd_calendar_create_event", map[string]any{
		"summary": "Vacation", "start": "2026-08-01T00:00:00+06:00", "all_day": true,
	})
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	body := <-gotBody
	if body["all_day"] != true || body["start"] != "2026-08-01T00:00:00Z" || body["end"] != "2026-08-02T00:00:00Z" {
		t.Fatalf("all-day body: %v", body)
	}
}

// dd_calendar_update_event writes only the fields given and carries the rest over.
func TestUpdateEventPartial(t *testing.T) {
	gotBody := make(chan map[string]any, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"uid":"e1","summary":"Standup","location":"Office","description":"daily sync",
			"start":"2026-07-21T10:00:00Z","end":"2026-07-21T10:15:00Z","alarm":"5","calendar_id":"cal1"}`))
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		gotBody <- body
		_, _ = w.Write([]byte(`{"uid":"e1"}`))
	})
	api := httptest.NewServer(mux)
	defer api.Close()

	res := callToolAllow(t, api.URL, "dd_calendar_update_event", map[string]any{
		"uid": "e1", "description": "", "alarm": "30",
	}, true)
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	body := <-gotBody
	if body["summary"] != "Standup" || body["location"] != "Office" {
		t.Fatalf("untouched fields must be carried over: %v", body)
	}
	if body["description"] != "" || body["alarm"] != "30" {
		t.Fatalf("changed fields: %v", body)
	}
}

// An update with no field at all must fail instead of rewriting the event with itself.
func TestUpdateEventWithoutFields(t *testing.T) {
	written := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("PUT /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		written <- struct{}{}
		_, _ = w.Write([]byte(`{"uid":"e1"}`))
	})
	api := httptest.NewServer(mux)
	defer api.Close()

	res := callToolAllow(t, api.URL, "dd_calendar_update_event", map[string]any{"uid": "e1"}, true)
	if !res.IsError {
		t.Fatal("expected a tool error when no field is given")
	}
	select {
	case <-written:
		t.Fatal("nothing must be written when no field is given")
	default:
	}
}

func TestGetEventReturnsForm(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"uid":"e1","summary":"Standup","description":"daily sync","alarm":"5","calendar_id":"cal1"}`))
	})
	api := httptest.NewServer(mux)
	defer api.Close()

	res := callTool(t, api.URL, "dd_calendar_get_event", map[string]any{"uid": "e1"})
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	if out := contentText(res); !strings.Contains(out, "daily sync") {
		t.Fatalf("the description must be part of the result:\n%s", out)
	}
}

// A garbage alarm value must fail loudly, without hitting the server
// (the server silently ignores unparseable alarms).
func TestCreateEventAlarmInvalid(t *testing.T) {
	posted := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("POST /me/calendar/events", func(w http.ResponseWriter, r *http.Request) {
		posted <- struct{}{}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uid":"e9"}`))
	})
	api := httptest.NewServer(mux)
	defer api.Close()

	res := callTool(t, api.URL, "dd_calendar_create_event", map[string]any{
		"summary": "Sync call", "start": "2026-07-21T10:00:00Z", "alarm": "half an hour",
	})
	if !res.IsError {
		t.Fatal("expected a tool error for alarm=\"half an hour\"")
	}
	select {
	case <-posted:
		t.Fatal("a request with a garbage alarm must not reach the server")
	default:
	}
}

// dd_calendar_delete_event must pass calendar_id on: the server resolves a uid inside
// one calendar, so without it a delete outside the default calendar 404s.
func TestDeleteEventCalendarID(t *testing.T) {
	gotQuery := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("DELETE /me/calendar/events/{uid}", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("calendar_id")
		if id != "cal2" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"event not found"}`))
			return
		}
		gotQuery <- id
		w.WriteHeader(http.StatusNoContent)
	})
	api := httptest.NewServer(mux)
	defer api.Close()

	res := callToolAllow(t, api.URL, "dd_calendar_delete_event",
		map[string]any{"uid": "e9", "calendar_id": "cal2"}, true)
	if res.IsError {
		t.Fatalf("tool returned an error: %s", contentText(res))
	}
	select {
	case id := <-gotQuery:
		if id != "cal2" {
			t.Fatalf("calendar_id = %q, want cal2", id)
		}
	default:
		t.Fatal("server did not receive the delete in cal2")
	}
}
