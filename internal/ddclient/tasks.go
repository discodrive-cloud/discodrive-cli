package ddclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Task is a VTODO task (the create/update body; the shape in the list and by uid is the same).
// priority: 0 — none, 1 — highest, 9 — lowest. An empty string in due clears the due date.
type Task struct {
	UID       string `json:"uid"`
	Summary   string `json:"summary"`
	Notes     string `json:"notes,omitempty"`
	Due       string `json:"due,omitempty"`
	DueAllDay bool   `json:"due_all_day"`
	Priority  int    `json:"priority"`
	Completed bool   `json:"completed"`
	// ListID is the task's list (a Reminders list). On create it picks the list ("" = the
	// default one); on update a different list moves the task, "" leaves it where it is.
	ListID string `json:"list_id,omitempty"`
}

// TaskList is a task list — a list in Apple Reminders. Lists other users shared with
// you have IsOwner=false and the owner's email.
type TaskList struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	IsDefault  bool   `json:"is_default"`
	IsOwner    bool   `json:"is_owner"`
	OwnerEmail string `json:"owner_email,omitempty"`
}

// ListTaskLists returns the task lists: your own (the default one first among them in the
// server's order) and the ones shared with you.
func (c *Client) ListTaskLists(ctx context.Context) ([]TaskList, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/task-lists")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /me/task-lists")
	}
	var out []TaskList
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// ErrTaskListNotFound — no task list has that id or name.
var ErrTaskListNotFound = errors.New("task list not found")

// ResolveTaskList turns a list id or name (case-insensitive) into the list. A name that
// several lists share is ambiguous: the error names their ids.
func (c *Client) ResolveTaskList(ctx context.Context, ref string) (TaskList, error) {
	ref = strings.TrimSpace(ref)
	lists, err := c.ListTaskLists(ctx)
	if err != nil {
		return TaskList{}, err
	}
	var byName []TaskList
	for _, l := range lists {
		if l.ID == ref {
			return l, nil
		}
		if strings.EqualFold(l.Name, ref) {
			byName = append(byName, l)
		}
	}
	switch len(byName) {
	case 0:
		return TaskList{}, fmt.Errorf("%w: %q", ErrTaskListNotFound, ref)
	case 1:
		return byName[0], nil
	}
	ids := make([]string, 0, len(byName))
	for _, l := range byName {
		who := l.OwnerEmail
		if l.IsOwner {
			who = "own"
		}
		ids = append(ids, l.ID+" ("+who+")")
	}
	return TaskList{}, fmt.Errorf("several task lists are named %q, use the id: %s", ref, strings.Join(ids, ", "))
}

// ListTasks returns the tasks of one list (listID) or of all lists ("") — the server sorts
// them: incomplete first, then by due date, then by name.
func (c *Client) ListTasks(ctx context.Context, listID string) ([]Task, error) {
	path := "/me/tasks"
	if listID != "" {
		path += "?list=" + url.QueryEscape(listID)
	}
	resp, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /me/tasks")
	}
	var out []Task
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTask returns the task by uid.
func (c *Client) GetTask(ctx context.Context, uid string) (Task, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/tasks/"+url.PathEscape(uid))
	if err != nil {
		return Task{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Task{}, readError(resp, "GET /me/tasks/{uid}")
	}
	var out Task
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// CreateTask creates a task and returns the new uid.
func (c *Client) CreateTask(ctx context.Context, f Task) (string, error) {
	return c.mutateForID(ctx, http.MethodPost, "/me/tasks", f, http.StatusCreated, "uid")
}

// UpdateTask fully overwrites task uid (including the completed status!) and returns the uid.
func (c *Client) UpdateTask(ctx context.Context, uid string, f Task) (string, error) {
	return c.mutateForID(ctx, http.MethodPut, "/me/tasks/"+url.PathEscape(uid), f, http.StatusOK, "uid")
}

// SetTaskDone sets the task's completion status (this is a set, not a toggle).
func (c *Client) SetTaskDone(ctx context.Context, uid string, completed bool) (string, error) {
	return c.mutateForID(ctx, http.MethodPut, "/me/tasks/"+url.PathEscape(uid)+"/done",
		map[string]bool{"completed": completed}, http.StatusOK, "uid")
}

// DeleteTask deletes task uid.
func (c *Client) DeleteTask(ctx context.Context, uid string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/me/tasks/"+url.PathEscape(uid))
	if err != nil {
		return err
	}
	return expect204(resp, "DELETE /me/tasks/{uid}")
}
