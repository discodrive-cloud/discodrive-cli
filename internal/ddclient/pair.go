package ddclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/discodrive-cloud/discodrive-cli/internal/i18n"
)

// Pairing is the server's response to /pair/init: the polling secret plus the code/URL for the human.
type Pairing struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	Interval        int
	ExpiresIn       int
}

// PairInit starts the device-code flow. When kind is empty the server picks the default ("desktop").
// insecure disables TLS certificate verification (pairing against a self-signed server).
func PairInit(ctx context.Context, serverURL, name, kind string, insecure bool) (Pairing, error) {
	body, _ := json.Marshal(map[string]string{"name": name, "kind": kind})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/pair/init", bytes.NewReader(body))
	if err != nil {
		return Pairing{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient(insecure).Do(req)
	if err != nil {
		return Pairing{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return Pairing{}, readError(resp, "POST /pair/init")
	}
	var out struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
		ExpiresIn       int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Pairing{}, err
	}
	return Pairing(out), nil
}

// PairInteractive runs the full device-pairing: init → print instructions to out → poll —
// and returns the device-token. Shared orchestration for dd-cli login and dd-mcp login.
func PairInteractive(ctx context.Context, serverURL, name, kind string, insecure bool, out io.Writer) (string, error) {
	p, err := PairInit(ctx, serverURL, name, kind, insecure)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(out, i18n.T("pair.open_link"))
	fmt.Fprintf(out, "  %s%s\n", serverURL, p.VerificationURI)
	fmt.Fprintln(out, i18n.T("pair.enter_code", i18n.M{"Code": p.UserCode}))
	fmt.Fprintln(out, i18n.T("pair.waiting"))
	interval := time.Duration(p.Interval) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return PairPoll(ctx, serverURL, p.DeviceCode, interval, insecure)
}

// PairPoll polls /pair/token every interval until the user confirms
// (status=approved) or the code expires/is rejected.
func PairPoll(ctx context.Context, serverURL, deviceCode string, interval time.Duration, insecure bool) (string, error) {
	for {
		body, _ := json.Marshal(map[string]string{"device_code": deviceCode})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/pair/token", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient(insecure).Do(req)
		if err != nil {
			return "", err
		}
		var out struct {
			Status      string `json:"status"`
			DeviceToken string `json:"device_token"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		switch out.Status {
		case "approved":
			return out.DeviceToken, nil
		case "pending":
		default:
			return "", errors.New(i18n.T("err.pair_incomplete", i18n.M{"Status": out.Status}))
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
	}
}
