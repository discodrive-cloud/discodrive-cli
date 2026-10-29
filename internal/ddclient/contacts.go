package ddclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// TypedValue is a typed value (email/phone) in the detailed contact form.
type TypedValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ContactAddr is a contact's postal address.
type ContactAddr struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	Region  string `json:"region"`
	Postal  string `json:"postal"`
	Country string `json:"country"`
}

// ContactListItem is the compact form from GET /me/contacts (emails/phones are values only).
type ContactListItem struct {
	UID      string   `json:"uid"`
	FullName string   `json:"full_name"`
	Emails   []string `json:"emails"`
	Phones   []string `json:"phones"`
}

// Contact is the full contact form (GET by uid; the create/update body). An empty string clears the field.
type Contact struct {
	UID      string       `json:"uid"`
	FullName string       `json:"full_name"`
	Family   string       `json:"family"`
	Given    string       `json:"given"`
	Emails   []TypedValue `json:"emails"`
	Phones   []TypedValue `json:"phones"`
	Org      string       `json:"org"`
	Title    string       `json:"title"`
	Adr      ContactAddr  `json:"adr"`
	Note     string       `json:"note"`
	Bday     string       `json:"bday"`
	HasPhoto bool         `json:"has_photo"`
	PhotoURI string       `json:"photo_uri,omitempty"`
}

// ListContacts returns the compact list of contacts.
func (c *Client) ListContacts(ctx context.Context) ([]ContactListItem, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/contacts")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /me/contacts")
	}
	var out []ContactListItem
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetContact returns the full contact form by uid.
func (c *Client) GetContact(ctx context.Context, uid string) (Contact, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/contacts/"+url.PathEscape(uid))
	if err != nil {
		return Contact{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Contact{}, readError(resp, "GET /me/contacts/{uid}")
	}
	var out Contact
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// CreateContact creates a contact and returns the new uid.
func (c *Client) CreateContact(ctx context.Context, f Contact) (string, error) {
	return c.mutateForID(ctx, http.MethodPost, "/me/contacts", f, http.StatusCreated, "uid")
}

// UpdateContact fully overwrites contact uid and returns the uid.
func (c *Client) UpdateContact(ctx context.Context, uid string, f Contact) (string, error) {
	return c.mutateForID(ctx, http.MethodPut, "/me/contacts/"+url.PathEscape(uid), f, http.StatusOK, "uid")
}

// DeleteContact deletes contact uid.
func (c *Client) DeleteContact(ctx context.Context, uid string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/me/contacts/"+url.PathEscape(uid))
	if err != nil {
		return err
	}
	return expect204(resp, "DELETE /me/contacts/{uid}")
}
