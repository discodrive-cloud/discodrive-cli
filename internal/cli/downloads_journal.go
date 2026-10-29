package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/discodrive-cloud/discodrive-cli/internal/ddclient"
)

// journalLimit caps the local journal: it exists to find downloads again, not
// to be an archive.
const journalLimit = 50

// downloadEntry is one queued download remembered locally. The server offers no
// listing of downloads at all, so without this file an id typed once is lost.
type downloadEntry struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
	At    string `json:"at,omitempty"`
	// Profile and Server name the login that queued the id. The server keeps ids
	// per account, so only that login can tell whether the id is gone. Entries
	// written before these fields existed have them empty.
	Profile string `json:"profile,omitempty"`
	Server  string `json:"server,omitempty"`
}

// journalOwner is the login a journal entry belongs to: the resolved profile name
// and the server it talks to.
type journalOwner struct {
	Profile string
	Server  string
	// Sole: this is the only profile, so untagged entries from before the
	// journal carried owners can only be its own.
	Sole bool
}

func (e downloadEntry) ownedBy(o journalOwner) bool {
	return e.Profile == o.Profile && e.Server == o.Server
}

func (e downloadEntry) untagged() bool { return e.Profile == "" && e.Server == "" }

// journalOwner resolves the login the command runs as; ok is false when no profile can
// be resolved (the entry is then written untagged).
func (cc *cmdCtx) journalOwner() (journalOwner, bool) {
	cfg, p, err := cc.profile()
	if err != nil {
		return journalOwner{}, false
	}
	return journalOwner{
		Profile: p.Name,
		Server:  normServer(p.ServerURL),
		Sole:    len(cfg.Profiles) == 1,
	}, true
}

// downloadsPath is downloads.json next to cli.json.
func downloadsPath(cc *cmdCtx) (string, error) {
	base := cc.ConfigPath
	if base == "" {
		p, err := ddclient.ConfigPath()
		if err != nil {
			return "", err
		}
		base = p
	}
	return filepath.Join(filepath.Dir(base), "downloads.json"), nil
}

// loadDownloads reads the journal. A missing or corrupt file is an empty journal:
// losing the local list must never block a download.
func loadDownloads(cc *cmdCtx) []downloadEntry {
	path, err := downloadsPath(cc)
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []downloadEntry
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

// saveDownloads writes the journal at 0600, keeping the newest journalLimit
// entries of each owner, so one profile's downloads never evict another's.
func saveDownloads(cc *cmdCtx, items []downloadEntry) error {
	path, err := downloadsPath(cc)
	if err != nil {
		return err
	}
	items = trimPerOwner(items)
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeFileAtomicMode(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(append(b, '\n'))
		return err
	})
}

// trimPerOwner keeps the newest journalLimit entries of every owner (untagged
// entries count as one owner), preserving the order.
func trimPerOwner(items []downloadEntry) []downloadEntry {
	count := map[[2]string]int{}
	for _, it := range items {
		count[[2]string{it.Profile, it.Server}]++
	}
	out := make([]downloadEntry, 0, len(items))
	for _, it := range items {
		k := [2]string{it.Profile, it.Server}
		if count[k] > journalLimit {
			count[k]--
			continue
		}
		out = append(out, it)
	}
	return out
}

// mine reports whether the current login may treat e as its own: tagged with
// it, or untagged (legacy) and possibly its.
func (e downloadEntry) mine(o journalOwner, known bool) bool {
	if !known {
		return e.untagged()
	}
	return e.ownedBy(o) || e.untagged()
}

// rememberDownload appends an entry tagged with the current login, replacing an
// earlier one with the same id of that login (the server returns the existing
// item when the same url is queued twice).
func rememberDownload(cc *cmdCtx, e downloadEntry) error {
	o, known := cc.journalOwner()
	if known {
		e.Profile, e.Server = o.Profile, o.Server
	}
	return updateDownloads(cc, func(items []downloadEntry) []downloadEntry {
		kept := make([]downloadEntry, 0, len(items)+1)
		for _, it := range items {
			if it.ID != e.ID || !it.mine(o, known) {
				kept = append(kept, it)
			}
		}
		return append(kept, e)
	})
}

// forgetDownload drops an id of the current login from the journal.
func forgetDownload(cc *cmdCtx, id string) error {
	o, known := cc.journalOwner()
	return updateDownloads(cc, func(items []downloadEntry) []downloadEntry {
		kept := make([]downloadEntry, 0, len(items))
		for _, it := range items {
			if it.ID != id || !it.mine(o, known) {
				kept = append(kept, it)
			}
		}
		return kept
	})
}

// Serialize the entire read-modify-write transaction across CLI processes.
// The lock file is persistent: unlinking it could let waiters lock different inodes.
func updateDownloads(cc *cmdCtx, update func([]downloadEntry) []downloadEntry) error {
	path, err := downloadsPath(cc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := lockJournal(lock); err != nil {
		return err
	}
	defer unlockJournal(lock)
	return saveDownloads(cc, update(loadDownloads(cc)))
}
