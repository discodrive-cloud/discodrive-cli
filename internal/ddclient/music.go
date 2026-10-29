package ddclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// MusicFolder is the source folder for music.
type MusicFolder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MusicSettings holds music-domain settings (read-only).
type MusicSettings struct {
	Enabled           bool         `json:"enabled"`
	Folder            *MusicFolder `json:"folder"`
	HasPassword       bool         `json:"hasPassword"`
	TagEditVersioning bool         `json:"tagEditVersioning"`
}

// Radio is an internet radio station. The list and detail forms are identical.
type Radio struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	StreamURL   string `json:"streamUrl"`
	HomepageURL string `json:"homepageUrl"`
}

// Podcast is a podcast subscription (NOT an episode).
type Podcast struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	FeedURL  string `json:"feedUrl"`
	CoverURL string `json:"coverUrl"`
	HasCover bool   `json:"hasCover"`
}

// ScanResult is the response of a fire-and-forget scan (music and books).
type ScanResult struct {
	Scanning bool `json:"scanning"`
}

// GetMusicSettings — GET /me/music.
func (c *Client) GetMusicSettings(ctx context.Context) (MusicSettings, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/music")
	if err != nil {
		return MusicSettings{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return MusicSettings{}, readError(resp, "GET /me/music")
	}
	var out MusicSettings
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return MusicSettings{}, err
	}
	return out, nil
}

// ListMusicFolder returns the contents of the folder designated as the music folder (a file listing).
// The optional subpath descends inward by the names of nested folders. It errors if music is
// disabled or the folder is unset. The full library (search/streaming) is served via Subsonic.
func (c *Client) ListMusicFolder(ctx context.Context, subpath string) ([]Node, error) {
	st, err := c.GetMusicSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !st.Enabled || st.Folder == nil {
		return nil, errors.New(i18n.T("err.music.not_configured"))
	}
	id := st.Folder.ID
	for _, seg := range strings.Split(strings.Trim(subpath, "/"), "/") {
		if seg == "" {
			continue
		}
		kids, err := c.ListChildren(ctx, id)
		if err != nil {
			return nil, err
		}
		next := ""
		for _, k := range kids {
			if k.Name == seg {
				if !k.IsDir {
					return nil, errors.New(i18n.T("err.music.not_folder", i18n.M{"Name": seg}))
				}
				next = k.ID
				break
			}
		}
		if next == "" {
			return nil, errors.New(i18n.T("err.music.no_folder", i18n.M{"Name": seg}))
		}
		id = next
	}
	return c.ListChildren(ctx, id)
}

// ListRadio — GET /me/music/radio.
func (c *Client) ListRadio(ctx context.Context) ([]Radio, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/music/radio")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /me/music/radio")
	}
	var out []Radio
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateRadio — POST /me/music/radio. The server returns the FULL object (201), not {id}.
func (c *Client) CreateRadio(ctx context.Context, r Radio) (Radio, error) {
	body := map[string]string{"name": r.Name, "streamUrl": r.StreamURL, "homepageUrl": r.HomepageURL}
	resp, err := c.doJSON(ctx, http.MethodPost, "/me/music/radio", body)
	if err != nil {
		return Radio{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return Radio{}, readError(resp, "POST /me/music/radio")
	}
	var out Radio
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Radio{}, err
	}
	return out, nil
}

// DeleteRadio — DELETE /me/music/radio/{id}.
func (c *Client) DeleteRadio(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/me/music/radio/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	return expect204(resp, "DELETE /me/music/radio/{id}")
}

// ListPodcasts — GET /me/music/podcasts.
func (c *Client) ListPodcasts(ctx context.Context) ([]Podcast, error) {
	resp, err := c.do(ctx, http.MethodGet, "/me/music/podcasts")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readError(resp, "GET /me/music/podcasts")
	}
	var out []Podcast
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddPodcast — POST /me/music/podcasts {"url":...}. Accepts 200 (already subscribed) OR 201 (created);
// 400 means the feed is unreachable. Returns the full subscription object.
func (c *Client) AddPodcast(ctx context.Context, feedURL string) (Podcast, error) {
	resp, err := c.doJSON(ctx, http.MethodPost, "/me/music/podcasts", map[string]string{"url": feedURL})
	if err != nil {
		return Podcast{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return Podcast{}, readError(resp, "POST /me/music/podcasts")
	}
	var out Podcast
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Podcast{}, err
	}
	return out, nil
}

// DeletePodcast — DELETE /me/music/podcasts/{id}.
func (c *Client) DeletePodcast(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/me/music/podcasts/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	return expect204(resp, "DELETE /me/music/podcasts/{id}")
}

// MusicScan — POST /me/music/scan. Fire-and-forget; true means a scan started, false means no folder.
func (c *Client) MusicScan(ctx context.Context) (bool, error) {
	resp, err := c.do(ctx, http.MethodPost, "/me/music/scan")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, readError(resp, "POST /me/music/scan")
	}
	var out ScanResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return out.Scanning, nil
}
