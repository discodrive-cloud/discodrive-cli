package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
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
}

// ListTasks returns the tasks (the server sorts them: incomplete first, then by due date, then by name).
func (c *Client) ListTasks(ctx context.Context) ([]Task, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/tasks")
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
