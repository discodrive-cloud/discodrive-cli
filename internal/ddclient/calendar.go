package ddclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// Calendar is a calendar collection (GET /me/calendars).
type Calendar struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	IsDefault  bool   `json:"is_default"`
	IsOwner    bool   `json:"is_owner"`
	OwnerEmail string `json:"owner_email,omitempty"`
}

// Occurrence is an expanded event instance from the list endpoint.
type Occurrence struct {
	UID          string `json:"uid"`
	RecurrenceID string `json:"recurrence_id,omitempty"`
	Summary      string `json:"summary"`
	Location     string `json:"location,omitempty"`
	Start        string `json:"start"`
	End          string `json:"end"`
	AllDay       bool   `json:"all_day"`
	Recurring    bool   `json:"recurring"`
	CalendarID   string `json:"calendar_id"`
}

// EventForm is the master event form (GET by uid; the create/update body). An empty string clears the field.
type EventForm struct {
	UID         string `json:"uid"`
	Summary     string `json:"summary"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Start       string `json:"start"`
	End         string `json:"end"`
	AllDay      bool   `json:"all_day"`
	Freq        string `json:"freq"`
	Until       string `json:"until"`
	Alarm       string `json:"alarm"`
	CalendarID  string `json:"calendar_id"`
	TZ          string `json:"tz,omitempty"`
}

// CheckAlarm validates a create-form alarm value: "" (no reminder) or minutes
// before start as a non-negative integer. The server silently drops anything
// else, so the client rejects it explicitly instead.
func CheckAlarm(alarm string) error {
	if alarm == "" {
		return nil
	}
	if n, err := strconv.Atoi(alarm); err != nil || n < 0 {
		return errors.New(i18n.T("err.alarm_invalid", i18n.M{"Value": alarm}))
	}
	return nil
}

// Recurrence presets accepted by the server's RRULE builder.
var recurrenceFreqs = []string{"DAILY", "WEEKLY", "MONTHLY", "YEARLY"}

// CheckFreq validates a recurrence value and returns it upper-cased.
// "" means "no recurrence". Anything the server cannot turn into an RRULE is
// dropped silently there, so it is rejected here instead.
func CheckFreq(freq string) (string, error) {
	if freq == "" {
		return "", nil
	}
	up := strings.ToUpper(strings.TrimSpace(freq))
	if slices.Contains(recurrenceFreqs, up) {
		return up, nil
	}
	return "", errors.New(i18n.T("err.freq_invalid", i18n.M{
		"Value": freq, "Allowed": strings.ToLower(strings.Join(recurrenceFreqs, ", ")),
	}))
}

// Layouts accepted for event times. Values without a zone are read in the
// machine's local time; RFC3339 carries its own offset.
var timeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

func parseInstant(v string) (time.Time, error) {
	return parseInstantIn(v, time.Local)
}

func parseInstantIn(v string, loc *time.Location) (time.Time, error) {
	s := strings.TrimSpace(v)
	for _, layout := range timeLayouts {
		if layout == time.RFC3339 {
			if t, err := time.Parse(layout, s); err == nil {
				return t, nil
			}
			continue
		}
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New(i18n.T("err.time_invalid", i18n.M{"Value": v}))
}

// NormalizeTime converts an event time to UTC RFC3339 ("...Z"), which is the only
// form the server writes into iCalendar as an unambiguous instant: a value carrying
// a numeric offset ends up stored as a floating local time (and an empty TZID
// parameter), so the event drifts by the offset and CalDAV clients — the macOS
// Calendar among them — read it in their own zone. An empty value stays empty.
func NormalizeTime(v string) (string, error) {
	if strings.TrimSpace(v) == "" {
		return "", nil
	}
	t, err := parseInstant(v)
	if err != nil {
		return "", err
	}
	return t.UTC().Format(time.RFC3339), nil
}

// NormalizeDate keeps the calendar date of an all-day boundary and pins it to
// midnight UTC: the server writes DTSTART/DTEND as DATE values formatted in the
// value's own zone, so converting the date to UTC first would shift it a day.
func NormalizeDate(v string) (string, error) {
	if strings.TrimSpace(v) == "" {
		return "", nil
	}
	t, err := parseInstant(v)
	if err != nil {
		return "", err
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), nil
}

// prepareForm validates and normalizes a create/update body. allowKeep permits the
// update-only alarm value "keep" (leave the existing reminders untouched).
func prepareForm(f EventForm, allowKeep bool) (EventForm, error) {
	if !(allowKeep && f.Alarm == "keep") {
		if err := CheckAlarm(f.Alarm); err != nil {
			return f, err
		}
	}
	var err error
	if !(allowKeep && f.Freq == "keep") {
		if f.Freq, err = CheckFreq(f.Freq); err != nil {
			return f, err
		}
	}
	if !allowKeep && f.Freq != "" && !f.AllDay && f.TZ == "" {
		f.TZ = localTimezone()
		if f.TZ == "" {
			return f, errors.New(i18n.T("err.timezone_required"))
		}
	}
	loc := time.Local
	if f.TZ != "" {
		if f.TZ == "Local" {
			return f, errors.New(i18n.T("err.timezone_invalid", i18n.M{"Value": f.TZ}))
		}
		loc, err = time.LoadLocation(f.TZ)
		if err != nil {
			return f, errors.New(i18n.T("err.timezone_invalid", i18n.M{"Value": f.TZ}))
		}
	}
	if f.Until != "" && f.Freq == "" {
		return f, errors.New(i18n.T("err.until_without_freq"))
	}
	norm := func(v string) (string, error) {
		if strings.TrimSpace(v) == "" {
			return "", nil
		}
		t, err := parseInstantIn(v, loc)
		if err != nil {
			return "", err
		}
		return t.UTC().Format(time.RFC3339), nil
	}
	if f.AllDay {
		norm = NormalizeDate
	}
	if f.Start, err = norm(f.Start); err != nil {
		return f, err
	}
	if f.End, err = norm(f.End); err != nil {
		return f, err
	}
	if f.Until, err = NormalizeDate(f.Until); err != nil {
		return f, err
	}
	// An all-day event needs an explicit next-day DTEND: the server's fallback
	// (start + 1 hour) collapses to the same DATE and the event loses its end.
	if f.AllDay && f.End == "" && f.Start != "" {
		st, perr := time.Parse(time.RFC3339, f.Start)
		if perr != nil {
			return f, perr
		}
		f.End = st.AddDate(0, 0, 1).Format(time.RFC3339)
	}
	return f, nil
}

// ListCalendars returns the user's VEVENT calendars (their own plus shared ones).
func (c *Client) ListCalendars(ctx context.Context) ([]Calendar, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/calendars")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /me/calendars")
	}
	var out []Calendar
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateCalendar creates a calendar and returns its id.
func (c *Client) CreateCalendar(ctx context.Context, name, color string) (string, error) {
	return c.mutateForID(ctx, http.MethodPost, "/me/calendars",
		map[string]string{"name": name, "color": color}, http.StatusCreated, "id")
}

// ListEvents returns events within the time window [start, end] (both required).
// Times are normalized to UTC; the server rejects anything that is not RFC3339.
func (c *Client) ListEvents(ctx context.Context, start, end string) ([]Occurrence, error) {
	start, err := NormalizeTime(start)
	if err != nil {
		return nil, err
	}
	end, err = NormalizeTime(end)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("start", start)
	q.Set("end", end)
	resp, err := c.do(ctx, http.MethodGet, "/me/calendar/events?"+q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /me/calendar/events")
	}
	var out []Occurrence
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetEvent returns the master event form by uid. calendarID is optional: without it
// the server looks in the default calendar only, so a miss is retried across the
// remaining calendars. A uid is unique only per calendar on the server (the same uid
// can sit in an own and a shared calendar), so when several calendars hold it the
// lookup fails with *AmbiguousEventError instead of picking one.
func (c *Client) GetEvent(ctx context.Context, uid, calendarID string) (EventForm, error) {
	f, err := c.getEvent(ctx, uid, calendarID)
	if err != nil && calendarID == "" && IsNotFound(err) {
		calID, ferr := c.findEventCalendar(ctx, uid)
		if ferr != nil {
			return EventForm{}, ferr
		}
		if calID != "" {
			return c.getEvent(ctx, uid, calID)
		}
	}
	return f, err
}

// AmbiguousEventError reports a uid that was not in the default calendar but is in
// more than one of the others: the caller has to name the calendar.
type AmbiguousEventError struct {
	UID         string
	CalendarIDs []string
}

func (e *AmbiguousEventError) Error() string {
	return i18n.T("err.event_ambiguous", i18n.M{"UID": e.UID, "Calendars": strings.Join(e.CalendarIDs, ", ")})
}

// findEventCalendar returns the id of the only non-default calendar holding uid, or ""
// if none does (including when the calendar list itself cannot be read): the caller
// then keeps the original 404. More than one holder is an *AmbiguousEventError. Only
// a 404 means "not in this calendar": any other failure while probing a calendar is
// returned as is, since that calendar may hold the uid too.
func (c *Client) findEventCalendar(ctx context.Context, uid string) (string, error) {
	cals, err := c.ListCalendars(ctx)
	if err != nil {
		return "", nil
	}
	var found []string
	for _, cal := range cals {
		if cal.IsDefault || cal.ID == "" {
			continue
		}
		_, err := c.getEvent(ctx, uid, cal.ID)
		switch {
		case err == nil:
			found = append(found, cal.ID)
		case IsNotFound(err):
			// not in this calendar
		default:
			// Unknown whether this calendar holds the uid: picking another one
			// could act on the wrong event, so abort with the real error.
			return "", err
		}
	}
	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return found[0], nil
	default:
		return "", &AmbiguousEventError{UID: uid, CalendarIDs: found}
	}
}

func (c *Client) getEvent(ctx context.Context, uid, calendarID string) (EventForm, error) {
	path := "/me/calendar/events/" + url.PathEscape(uid)
	if calendarID != "" {
		path += "?calendar_id=" + url.QueryEscape(calendarID)
	}
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return EventForm{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return EventForm{}, readError(resp, "GET /me/calendar/events/{uid}")
	}
	var out EventForm
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// CreateEvent creates an event and returns the new uid. CalendarID in the form selects the calendar.
func (c *Client) CreateEvent(ctx context.Context, f EventForm) (string, error) {
	f, err := prepareForm(f, false)
	if err != nil {
		return "", err
	}
	return c.mutateForID(ctx, http.MethodPost, "/me/calendar/events", f, http.StatusCreated, "uid")
}

// UpdateEvent overwrites the master event uid and returns the uid. Every field of
// the form is written, so an empty one clears the property; PatchEvent changes a
// subset instead.
func (c *Client) UpdateEvent(ctx context.Context, uid string, f EventForm) (string, error) {
	f, err := prepareForm(f, true)
	if err != nil {
		return "", err
	}
	return c.mutateForID(ctx, http.MethodPut, "/me/calendar/events/"+url.PathEscape(uid), f, http.StatusOK, "uid")
}

// EventPatch is a partial event change: a nil field is left as it is, a non-nil
// empty string clears the property.
type EventPatch struct {
	TZ          *string
	CalendarID  string
	Summary     *string
	Location    *string
	Description *string
	Start       *string
	End         *string
	AllDay      *bool
	Freq        *string
	Until       *string
	Alarm       *string
}

// Empty reports whether the patch changes nothing.
func (p EventPatch) Empty() bool {
	return p.TZ == nil && p.Summary == nil && p.Location == nil && p.Description == nil &&
		p.Start == nil && p.End == nil && p.AllDay == nil &&
		p.Freq == nil && p.Until == nil && p.Alarm == nil
}

// PatchEvent reads the master form of uid, applies the patch and writes it back,
// returning the form that was sent. The server has no partial update: PUT replaces
// the whole event, so the untouched fields have to be carried over by the client.
func (c *Client) PatchEvent(ctx context.Context, uid string, p EventPatch) (EventForm, error) {
	f, err := c.GetEvent(ctx, uid, p.CalendarID)
	if err != nil {
		return EventForm{}, err
	}
	// Reminders that the server could not express as a preset come back as "keep",
	// which tells it to leave the existing VALARMs alone on write.
	setString(&f.TZ, p.TZ)
	setString(&f.Summary, p.Summary)
	setString(&f.Location, p.Location)
	setString(&f.Description, p.Description)
	setString(&f.Start, p.Start)
	setString(&f.End, p.End)
	setString(&f.Freq, p.Freq)
	setString(&f.Until, p.Until)
	setString(&f.Alarm, p.Alarm)
	if p.AllDay != nil {
		f.AllDay = *p.AllDay
	}
	if f.Freq == "" {
		f.Until = ""
	}
	// prepareForm runs again inside UpdateEvent; it is idempotent, and calling it
	// here is what makes the returned form show what was actually written.
	if f, err = prepareForm(f, true); err != nil {
		return EventForm{}, err
	}
	if _, err := c.UpdateEvent(ctx, uid, f); err != nil {
		return EventForm{}, err
	}
	return f, nil
}

func setString(dst *string, src *string) {
	if src != nil {
		*dst = *src
	}
}

// DeleteEvent deletes event uid. calendarID is optional: as with GetEvent, a miss in
// the default calendar is retried in the one other calendar that holds the uid, and
// refused with *AmbiguousEventError when several do.
func (c *Client) DeleteEvent(ctx context.Context, uid, calendarID string) error {
	err := c.deleteEvent(ctx, uid, calendarID)
	if err != nil && calendarID == "" && IsNotFound(err) {
		calID, ferr := c.findEventCalendar(ctx, uid)
		if ferr != nil {
			return ferr
		}
		if calID != "" {
			return c.deleteEvent(ctx, uid, calID)
		}
	}
	return err
}

func (c *Client) deleteEvent(ctx context.Context, uid, calendarID string) error {
	path := "/me/calendar/events/" + url.PathEscape(uid)
	if calendarID != "" {
		path += "?calendar_id=" + url.QueryEscape(calendarID)
	}
	resp, err := c.do(ctx, http.MethodDelete, path)
	if err != nil {
		return err
	}
	return expect204(resp, "DELETE /me/calendar/events/{uid}")
}
