package ddmcp

import (
	"context"
	"errors"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- tool input/output structs ---

type contactsListOutput struct {
	Contacts []ddclient.ContactListItem `json:"contacts"`
}
type calListOutput struct {
	Calendars []ddclient.Calendar `json:"calendars"`
}
type eventsInput struct {
	Start string `json:"start" jsonschema:"window start RFC3339 (required)"`
	End   string `json:"end" jsonschema:"window end RFC3339 (required)"`
}
type eventsOutput struct {
	Events []ddclient.Occurrence `json:"events"`
}
type createEventInput struct {
	TZ          string `json:"tz,omitempty" jsonschema:"IANA time zone, e.g. Europe/Berlin; recurring events default to the local zone of the machine running dd-mcp"`
	Summary     string `json:"summary" jsonschema:"event title"`
	Start       string `json:"start" jsonschema:"start RFC3339 WITH an explicit zone or Z (e.g. 2026-08-05T10:00:00+06:00); a value without a zone is read in the local time of the machine running dd-mcp"`
	End         string `json:"end,omitempty" jsonschema:"end RFC3339 (optional; default: start + 1 hour)"`
	Location    string `json:"location,omitempty" jsonschema:"location (optional)"`
	Description string `json:"description,omitempty" jsonschema:"description / notes (optional)"`
	AllDay      bool   `json:"all_day,omitempty" jsonschema:"all-day event: only the date part of start/end is used (optional)"`
	Freq        string `json:"freq,omitempty" jsonschema:"repeat: DAILY|WEEKLY|MONTHLY|YEARLY; empty = no repeat (optional)"`
	Until       string `json:"until,omitempty" jsonschema:"repeat until RFC3339; requires freq (optional)"`
	Alarm       string `json:"alarm,omitempty" jsonschema:"reminder: minutes before start as integer string (0 = at start; presets 0|5|15|30|60|1440); empty = no reminder (optional)"`
	CalendarID  string `json:"calendar_id,omitempty" jsonschema:"calendar id (optional)"`
}

// eventRefInput points at a single event: the tools that take a uid also accept the
// calendar it lives in, because the server resolves a uid inside one calendar at a time.
type eventRefInput struct {
	UID        string `json:"uid" jsonschema:"event identifier"`
	CalendarID string `json:"calendar_id,omitempty" jsonschema:"calendar id (optional; without it the uid is looked up in the default calendar first and then in the others)"`
}
type eventFormOutput struct {
	Event ddclient.EventForm `json:"event"`
}

// updateEventInput mirrors createEventInput with pointers: an omitted field is left
// as it is, a field set to "" clears the property.
type updateEventInput struct {
	TZ          *string `json:"tz,omitempty" jsonschema:"IANA time zone for an event without an existing zone (optional)"`
	UID         string  `json:"uid" jsonschema:"event identifier"`
	Summary     *string `json:"summary,omitempty" jsonschema:"event title (optional)"`
	Start       *string `json:"start,omitempty" jsonschema:"start RFC3339 with an explicit zone or Z (optional)"`
	End         *string `json:"end,omitempty" jsonschema:"end RFC3339 (optional)"`
	Location    *string `json:"location,omitempty" jsonschema:"location; \"\" clears it (optional)"`
	Description *string `json:"description,omitempty" jsonschema:"description / notes; \"\" clears it (optional)"`
	AllDay      *bool   `json:"all_day,omitempty" jsonschema:"all-day event (optional)"`
	Freq        *string `json:"freq,omitempty" jsonschema:"repeat: DAILY|WEEKLY|MONTHLY|YEARLY; \"\" removes the repeat (optional)"`
	Until       *string `json:"until,omitempty" jsonschema:"repeat until RFC3339 (optional)"`
	Alarm       *string `json:"alarm,omitempty" jsonschema:"reminder in minutes before start; \"\" removes it; \"keep\" leaves the existing reminders untouched (optional)"`
	CalendarID  string  `json:"calendar_id,omitempty" jsonschema:"calendar id (optional)"`
}
type tasksListOutput struct {
	Tasks []ddclient.Task `json:"tasks"`
}
type createTaskInput struct {
	Summary  string `json:"summary" jsonschema:"task title"`
	Due      string `json:"due" jsonschema:"due date RFC3339 (optional)"`
	Priority int    `json:"priority" jsonschema:"priority 1..9 (1 = highest; 0 = none)"`
}
type uidInput struct {
	UID string `json:"uid" jsonschema:"identifier"`
}
type setDoneInput struct {
	UID       string `json:"uid" jsonschema:"task identifier"`
	Completed bool   `json:"completed" jsonschema:"true = done, false = unmark"`
}
type idOutput struct {
	ID string `json:"id"`
}

// pimToolSpecs returns the tools for the contacts, calendar and tasks domains.
func pimToolSpecs() []toolSpec {
	return []toolSpec{
		{Name: "dd_contacts_list", Description: i18n.T("tool.dd_contacts_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, contactsListOutput, error) {
				items, err := c.ListContacts(ctx)
				if err != nil {
					return nil, contactsListOutput{}, err
				}
				return nil, contactsListOutput{Contacts: items}, nil
			})
		}},
		{Name: "dd_calendar_list", Description: i18n.T("tool.dd_calendar_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, calListOutput, error) {
				cals, err := c.ListCalendars(ctx)
				if err != nil {
					return nil, calListOutput{}, err
				}
				return nil, calListOutput{Calendars: cals}, nil
			})
		}},
		{Name: "dd_calendar_events", Description: i18n.T("tool.dd_calendar_events.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in eventsInput) (*mcp.CallToolResult, eventsOutput, error) {
				evs, err := c.ListEvents(ctx, in.Start, in.End)
				if err != nil {
					return nil, eventsOutput{}, err
				}
				return nil, eventsOutput{Events: evs}, nil
			})
		}},
		{Name: "dd_calendar_create_event", Description: i18n.T("tool.dd_calendar_create_event.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in createEventInput) (*mcp.CallToolResult, idOutput, error) {
				if err := ddclient.CheckAlarm(in.Alarm); err != nil {
					return nil, idOutput{}, err
				}
				uid, err := c.CreateEvent(ctx, ddclient.EventForm{
					Summary: in.Summary, Start: in.Start, End: in.End, Location: in.Location,
					Description: in.Description, AllDay: in.AllDay, Freq: in.Freq, Until: in.Until,
					Alarm: in.Alarm, CalendarID: in.CalendarID, TZ: in.TZ,
				})
				if err != nil {
					return nil, idOutput{}, err
				}
				return nil, idOutput{ID: uid}, nil
			})
		}},
		{Name: "dd_calendar_get_event", Description: i18n.T("tool.dd_calendar_get_event.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in eventRefInput) (*mcp.CallToolResult, eventFormOutput, error) {
				ev, err := c.GetEvent(ctx, in.UID, in.CalendarID)
				if err != nil {
					return nil, eventFormOutput{}, err
				}
				return nil, eventFormOutput{Event: ev}, nil
			})
		}},
		{Name: "dd_tasks_list", Description: i18n.T("tool.dd_tasks_list.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, tasksListOutput, error) {
				items, err := c.ListTasks(ctx)
				if err != nil {
					return nil, tasksListOutput{}, err
				}
				return nil, tasksListOutput{Tasks: items}, nil
			})
		}},
		{Name: "dd_tasks_create", Description: i18n.T("tool.dd_tasks_create.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in createTaskInput) (*mcp.CallToolResult, idOutput, error) {
				uid, err := c.CreateTask(ctx, ddclient.Task{Summary: in.Summary, Due: in.Due, Priority: in.Priority})
				if err != nil {
					return nil, idOutput{}, err
				}
				return nil, idOutput{ID: uid}, nil
			})
		}},
		{Name: "dd_contacts_delete", Changing: true, Description: i18n.T("tool.dd_contacts_delete.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in uidInput) (*mcp.CallToolResult, okOutput, error) {
				if err := c.DeleteContact(ctx, in.UID); err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_contacts_delete.msg.deleted", i18n.M{"UID": in.UID})}, nil
			})
		}},
		{Name: "dd_calendar_update_event", Changing: true, Description: i18n.T("tool.dd_calendar_update_event.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in updateEventInput) (*mcp.CallToolResult, eventFormOutput, error) {
				patch := ddclient.EventPatch{
					TZ:         in.TZ,
					CalendarID: in.CalendarID, Summary: in.Summary, Location: in.Location,
					Description: in.Description, Start: in.Start, End: in.End,
					AllDay: in.AllDay, Freq: in.Freq, Until: in.Until, Alarm: in.Alarm,
				}
				if patch.Empty() {
					return nil, eventFormOutput{}, errors.New(i18n.T("tool.dd_calendar_update_event.msg.no_fields"))
				}
				ev, err := c.PatchEvent(ctx, in.UID, patch)
				if err != nil {
					return nil, eventFormOutput{}, err
				}
				return nil, eventFormOutput{Event: ev}, nil
			})
		}},
		{Name: "dd_calendar_delete_event", Changing: true, Description: i18n.T("tool.dd_calendar_delete_event.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in eventRefInput) (*mcp.CallToolResult, okOutput, error) {
				if err := c.DeleteEvent(ctx, in.UID, in.CalendarID); err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_calendar_delete_event.msg.deleted", i18n.M{"UID": in.UID})}, nil
			})
		}},
		{Name: "dd_tasks_delete", Changing: true, Description: i18n.T("tool.dd_tasks_delete.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in uidInput) (*mcp.CallToolResult, okOutput, error) {
				if err := c.DeleteTask(ctx, in.UID); err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_tasks_delete.msg.deleted", i18n.M{"UID": in.UID})}, nil
			})
		}},
		{Name: "dd_tasks_set_done", Changing: true, Description: i18n.T("tool.dd_tasks_set_done.desc"), register: func(s *mcp.Server, c *ddclient.Client, t *mcp.Tool) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in setDoneInput) (*mcp.CallToolResult, okOutput, error) {
				if _, err := c.SetTaskDone(ctx, in.UID, in.Completed); err != nil {
					return nil, okOutput{}, err
				}
				return nil, okOutput{OK: true, Note: i18n.T("tool.dd_tasks_set_done.msg.status", i18n.M{"UID": in.UID, "Completed": in.Completed})}, nil
			})
		}},
	}
}
