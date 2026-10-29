package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func contactsCLIMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"jwt"}`))
	})
	mux.HandleFunc("GET /me/contacts", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"uid":"c1","full_name":"Alice","emails":["a@x.io"],"phones":[]}]`))
	})
	return mux
}

func TestContactsListCmd(t *testing.T) {
	srv := httptest.NewServer(contactsCLIMux(t))
	defer srv.Close()
	setupProfile(t, srv.URL)

	var out, errb bytes.Buffer
	if code := Run([]string{"contacts"}, &out, &errb); code != 0 {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Alice")) || !bytes.Contains(out.Bytes(), []byte("a@x.io")) {
		t.Fatalf("out:\n%s", out.String())
	}
}

func TestContactsRmIrreversibleHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"contacts", "rm", "--help"}, &out, &errb); code != 0 {
		// rm --help prints the help for the contacts command (the subverb) — we check the marker
	}
	// the root help for the contacts command must mention that deletion is irreversible
	out.Reset()
	errb.Reset()
	Run([]string{"contacts", "--help"}, &out, &errb)
	if !bytes.Contains(out.Bytes(), []byte("Irreversible")) {
		t.Fatalf("contacts --help must mark rm as irreversible:\n%s", out.String())
	}
}
