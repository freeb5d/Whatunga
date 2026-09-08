// This file implements the Poller interface (see poller.go) for HPE
// servers' iLO management processors, using the Redfish API — the
// modern, vendor-neutral REST/JSON standard for server management
// that iLO 4 (with a firmware update), iLO 5, and iLO 6 all expose.
//
// Unlike RouterOS's binary API, Redfish is just HTTPS + JSON, so no
// hand-rolled wire protocol is needed here — the standard library's
// net/http is enough. iLO's TLS certificate is self-signed by default
// on most deployments, so InsecureSkipVerify is used; if you've
// installed a real certificate on your iLOs, set Insecure: false in
// the poller options.
package monitor

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ILOPoller polls one HPE iLO management processor over Redfish.
type ILOPoller struct {
	device   string
	baseURL  string // e.g. "https://10.0.0.5"
	username string
	password string
	client   *http.Client
}

// NewILOPoller returns a Poller for an HPE iLO at address (host or
// host:port; https is assumed). insecureSkipVerify should stay true
// unless you've replaced iLO's default self-signed certificate.
func NewILOPoller(deviceName, address, username, password string, insecureSkipVerify bool, timeout time.Duration) *ILOPoller {
	return &ILOPoller{
		device:   deviceName,
		baseURL:  "https://" + address,
		username: username,
		password: password,
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify}, //nolint:gosec // iLO ships with a self-signed cert by default
			},
		},
	}
}

// Close is a no-op for ILOPoller — each request is a fresh HTTPS
// call, there's no persistent connection to release.
func (p *ILOPoller) Close() error { return nil }

// redfishSystem mirrors the small subset of a Redfish ComputerSystem
// resource that Whatunga cares about. Real responses have many more
// fields (Redfish schemas are large); we only decode what we use.
type redfishSystem struct {
	Model        string `json:"Model"`
	PowerState   string `json:"PowerState"`
	BiosVersion  string `json:"BiosVersion"`
	Status       struct {
		Health string `json:"Health"`
	} `json:"Status"`
	MemorySummary struct {
		TotalSystemMemoryGiB float64 `json:"TotalSystemMemoryGiB"`
	} `json:"MemorySummary"`
}

// redfishSystemCollection is the response from /redfish/v1/Systems/,
// used to discover the actual system resource path (usually "1" but
// not guaranteed across every HPE model).
type redfishSystemCollection struct {
	Members []struct {
		ODataID string `json:"@odata.id"`
	} `json:"Members"`
}

// Snapshot fetches the server's ComputerSystem resource from Redfish
// and maps it onto Whatunga's common Snapshot shape. There are no
// network Interfaces to report here — iLO monitors server health, not
// a router's interface table — so Interfaces is always empty.
func (p *ILOPoller) Snapshot() (Snapshot, error) {
	systemPath, err := p.discoverSystemPath()
	if err != nil {
		return Snapshot{}, fmt.Errorf("monitor: ilo: discovering system resource: %w", err)
	}

	var sys redfishSystem
	if err := p.getJSON(systemPath, &sys); err != nil {
		return Snapshot{}, fmt.Errorf("monitor: ilo: fetching system resource: %w", err)
	}

	return Snapshot{
		Device: p.device,
		Kind:   KindILO,
		System: SystemStatus{
			Version:      sys.BiosVersion,
			BoardName:    sys.Model,
			Health:       sys.Status.Health,
			PowerState:   sys.PowerState,
			CPULoadKnown: false, // standard Redfish doesn't expose a live CPU utilization percentage
			TotalMemKB:   int64(sys.MemorySummary.TotalSystemMemoryGiB * 1024 * 1024),
			CheckedAt:    time.Now().UTC(),
		},
		Interfaces: nil,
	}, nil
}

// discoverSystemPath finds the first ComputerSystem resource's path.
// Most HPE servers expose exactly one at /redfish/v1/Systems/1/, but
// walking the collection instead of hard-coding "1" is more robust
// across firmware/model differences.
func (p *ILOPoller) discoverSystemPath() (string, error) {
	var collection redfishSystemCollection
	if err := p.getJSON("/redfish/v1/Systems/", &collection); err != nil {
		return "", err
	}
	if len(collection.Members) == 0 {
		return "", fmt.Errorf("no ComputerSystem members found")
	}
	return collection.Members[0].ODataID, nil
}

func (p *ILOPoller) getJSON(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.username, p.password)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, path)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
