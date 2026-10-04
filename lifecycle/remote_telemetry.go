package lifecycle

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Opt-in remote telemetry (OSS default OFF). See docs/TELEMETRY_CONTRACT.md.
const (
	EnvTelemetryUpload   = "SMARTCONTEXT_TELEMETRY_UPLOAD"
	EnvTelemetryEndpoint = "SMARTCONTEXT_TELEMETRY_ENDPOINT"

	// DefaultTelemetryEndpoint is used when upload is enabled and no override is set.
	// Operators may point SMARTCONTEXT_TELEMETRY_ENDPOINT at a self-hosted collector.
	DefaultTelemetryEndpoint = "https://telemetry.smartcontext.dev/v1/events"

	InstallIDTTL   = 90 * 24 * time.Hour
	uploadTimeout  = 3 * time.Second
	remoteSchemaV1 = "smartcontext.telemetry.v1"
)

var (
	uploadMu        sync.Mutex
	uploadScheduled bool
	uploadWG        sync.WaitGroup

	// httpDo is swappable in tests (default: http.DefaultClient.Do).
	httpDo = func(req *http.Request) (*http.Response, error) {
		return http.DefaultClient.Do(req)
	}
)

// InstallIdentity is the rotating install UUID under ~/.smartcontext/telemetry/.
type InstallIdentity struct {
	ID       string `json:"id"`
	IssuedAt string `json:"issued_at"` // UTC RFC3339
}

// RemotePackSample is one aggregate sample for remote upload (no raw task / bodies / paths).
type RemotePackSample struct {
	Type            string `json:"type"`
	Timestamp       string `json:"timestamp"`
	Version         string `json:"version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	LatencyBucket   string `json:"latency_bucket"`
	SelectedCount   int    `json:"selected_count"`
	BudgetPreset    string `json:"budget_preset,omitempty"`
	BudgetTokens    int    `json:"budget_tokens"`
	CacheFriendly   bool   `json:"cache_friendly"`
	EmptyCatalog    bool   `json:"empty_catalog"`
	ScrubFlagCount  int    `json:"scrub_flag_count"`
	CatalogSize     int    `json:"catalog_size"`
	RootCount       int    `json:"root_count"`
	OffloadRecalled bool   `json:"offload_recalled,omitempty"`
}

// remoteBatch is the HTTPS POST body.
type remoteBatch struct {
	Schema    string             `json:"schema"`
	InstallID string             `json:"install_id"`
	SentAt    string             `json:"sent_at"`
	Events    []RemotePackSample `json:"events"`
}

// TelemetryStatus is the CLI / diagnostics snapshot.
type TelemetryStatus struct {
	LifecycleEnabled    bool   `json:"lifecycle_enabled"`
	UploadEnabled       bool   `json:"upload_enabled"`
	Endpoint            string `json:"endpoint"`
	EndpointOverride    bool   `json:"endpoint_override"`
	InstallID           string `json:"install_id,omitempty"`
	InstallIssuedAt     string `json:"install_issued_at,omitempty"`
	InstallExpiresAt    string `json:"install_expires_at,omitempty"`
	InstallExpired      bool   `json:"install_expired"`
	RemoteBufferPending int    `json:"remote_buffer_pending"`
	LocalEventsPath     string `json:"local_events_path"`
	InstallIDPath       string `json:"install_id_path"`
	RemoteBufferPath    string `json:"remote_buffer_path"`
	Note                string `json:"note"`
}

// UploadEnabled reports whether opt-in remote upload is on (default OFF).
// Requires lifecycle enabled and SMARTCONTEXT_TELEMETRY_UPLOAD=1|true|yes|on|enabled.
func UploadEnabled() bool {
	if !Enabled() {
		return false
	}
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(EnvTelemetryUpload)))
	switch raw {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

// TelemetryEndpoint returns the configured upload URL (override or default).
func TelemetryEndpoint() string {
	if ep := strings.TrimSpace(os.Getenv(EnvTelemetryEndpoint)); ep != "" {
		return ep
	}
	return DefaultTelemetryEndpoint
}

func installIDPath(home string) string {
	return filepath.Join(EnsureLayout(home), "telemetry", "install_id.json")
}

func remoteBufferPath(home string) string {
	return filepath.Join(EnsureLayout(home), "telemetry", "remote_pending.jsonl")
}

// newInstallUUID returns a random UUID v4 string (no external dependency).
func newInstallUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32]), nil
}

// LoadOrRotateInstallID returns a valid install UUID, rotating when missing or past TTL.
func LoadOrRotateInstallID(home string) (InstallIdentity, error) {
	path := installIDPath(home)
	if data, err := os.ReadFile(path); err == nil {
		var id InstallIdentity
		if json.Unmarshal(data, &id) == nil && id.ID != "" && id.IssuedAt != "" {
			if issued, err := time.Parse(time.RFC3339, id.IssuedAt); err == nil {
				if time.Since(issued) < InstallIDTTL {
					return id, nil
				}
			}
		}
	}
	return rotateInstallID(home)
}

func rotateInstallID(home string) (InstallIdentity, error) {
	uid, err := newInstallUUID()
	if err != nil {
		return InstallIdentity{}, err
	}
	id := InstallIdentity{
		ID:       uid,
		IssuedAt: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
	}
	raw, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return InstallIdentity{}, err
	}
	if err := os.WriteFile(installIDPath(home), append(raw, '\n'), 0o644); err != nil {
		return InstallIdentity{}, err
	}
	return id, nil
}

// ResetTelemetry wipes the install UUID and remote pending buffer (severs historical linkage).
// Local events.jsonl is left intact (operator-owned).
func ResetTelemetry(home string) error {
	root := EnsureLayout(home)
	_ = os.Remove(filepath.Join(root, "telemetry", "install_id.json"))
	_ = os.Remove(filepath.Join(root, "telemetry", "remote_pending.jsonl"))
	return nil
}

// GetTelemetryStatus reports upload gate + install identity state for CLI.
func GetTelemetryStatus(home string) TelemetryStatus {
	root := Home(home)
	st := TelemetryStatus{
		LifecycleEnabled: Enabled(),
		UploadEnabled:    UploadEnabled(),
		Endpoint:         TelemetryEndpoint(),
		EndpointOverride: strings.TrimSpace(os.Getenv(EnvTelemetryEndpoint)) != "",
		LocalEventsPath:  filepath.Join(root, "telemetry", "events.jsonl"),
		InstallIDPath:    filepath.Join(root, "telemetry", "install_id.json"),
		RemoteBufferPath: filepath.Join(root, "telemetry", "remote_pending.jsonl"),
		Note:             "Remote upload default OFF; enable with SMARTCONTEXT_TELEMETRY_UPLOAD=1",
	}
	if data, err := os.ReadFile(st.InstallIDPath); err == nil {
		var id InstallIdentity
		if json.Unmarshal(data, &id) == nil {
			st.InstallID = id.ID
			st.InstallIssuedAt = id.IssuedAt
			if issued, err := time.Parse(time.RFC3339, id.IssuedAt); err == nil {
				exp := issued.Add(InstallIDTTL)
				st.InstallExpiresAt = exp.UTC().Format(time.RFC3339)
				st.InstallExpired = time.Now().After(exp)
			}
		}
	}
	if data, err := os.ReadFile(st.RemoteBufferPath); err == nil && len(data) > 0 {
		st.RemoteBufferPending = countJSONLLines(data)
	}
	if !st.LifecycleEnabled {
		st.Note = "Lifecycle disabled (SMARTCONTEXT_LIFECYCLE); local and remote telemetry off"
	} else if st.UploadEnabled {
		st.Note = "Remote upload enabled (fail-open background POST); pack path never waits on network"
	}
	return st
}

func countJSONLLines(data []byte) int {
	n := 0
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			n++
		}
	}
	return n
}

// LatencyBucket maps wall-time ms into coarse aggregate buckets (no exact fingerprint).
func LatencyBucket(ms float64) string {
	switch {
	case ms < 1:
		return "<1"
	case ms < 5:
		return "1-5"
	case ms < 20:
		return "5-20"
	case ms < 100:
		return "20-100"
	default:
		return "100+"
	}
}

// packEventToRemoteSample converts a local pack event into a remote-safe aggregate.
// Omits task_hash, stable_prefix_hash, and any path/body fields.
func packEventToRemoteSample(ev PackEvent) RemotePackSample {
	return RemotePackSample{
		Type:           "pack_agg",
		Timestamp:      ev.Timestamp,
		Version:        ev.Version,
		OS:             ev.OS,
		Arch:           ev.Arch,
		LatencyBucket:  LatencyBucket(ev.LatencyMS),
		SelectedCount:  ev.SelectedCount,
		BudgetPreset:   ev.BudgetPreset,
		BudgetTokens:   ev.BudgetTokens,
		CacheFriendly:  ev.CacheFriendly,
		EmptyCatalog:   ev.EmptyCatalog,
		ScrubFlagCount: ev.ScrubFlagCount,
		CatalogSize:    ev.CatalogSize,
		RootCount:      ev.RootCount,
	}
}

// enqueueRemotePackSample appends an aggregate sample and schedules a fail-open upload.
// Never blocks on network; safe to call from the pack path.
func enqueueRemotePackSample(ev PackEvent, home string) {
	if !UploadEnabled() {
		return
	}
	sample := packEventToRemoteSample(ev)
	line, err := json.Marshal(sample)
	if err != nil {
		return
	}
	path := remoteBufferPath(home)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
	_ = f.Close()
	scheduleRemoteUpload(home)
}

// scheduleRemoteUpload starts at most one background flush goroutine (fail-open).
func scheduleRemoteUpload(home string) {
	uploadMu.Lock()
	if uploadScheduled {
		uploadMu.Unlock()
		return
	}
	uploadScheduled = true
	uploadMu.Unlock()

	uploadWG.Add(1)
	go func() {
		defer func() {
			_ = recover() // fail-open: never surface upload panics to pack
			uploadMu.Lock()
			uploadScheduled = false
			uploadMu.Unlock()
			uploadWG.Done()
		}()
		_ = flushRemotePending(home)
	}()
}

// flushRemotePending POSTs the pending buffer with a hard timeout. Errors are ignored.
func flushRemotePending(home string) error {
	path := remoteBufferPath(home)
	data, err := os.ReadFile(path)
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return err
	}
	var events []RemotePackSample
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var s RemotePackSample
		if json.Unmarshal(line, &s) != nil {
			continue
		}
		events = append(events, s)
	}
	if len(events) == 0 {
		_ = os.Remove(path)
		return nil
	}

	id, err := LoadOrRotateInstallID(home)
	if err != nil {
		return err
	}
	batch := remoteBatch{
		Schema:    remoteSchemaV1,
		InstallID: id.ID,
		SentAt:    time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		Events:    events,
	}
	body, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TelemetryEndpoint(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "smartcontext-telemetry/1")

	resp, err := httpDo(req)
	if err != nil {
		return err // leave buffer for a later attempt
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telemetry upload status %d", resp.StatusCode)
	}
	// Success: clear pending buffer (fail-open if remove fails).
	_ = os.Remove(path)
	return nil
}

// WaitRemoteUploads blocks until in-flight background uploads finish (tests only).
func WaitRemoteUploads() {
	uploadWG.Wait()
}

// ResetRemoteUploadState clears scheduler flags between tests.
func ResetRemoteUploadState() {
	WaitRemoteUploads()
	uploadMu.Lock()
	uploadScheduled = false
	uploadMu.Unlock()
}
