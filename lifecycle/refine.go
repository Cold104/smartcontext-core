package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Cold104/smartcontext-core/core"
)

const thinDescriptionChars = 40

// Proposal is a refine queue entry (propose-only; never writes SKILL.md).
type Proposal struct {
	ID          string   `json:"id"`
	SkillName   string   `json:"skill_name"`
	Action      string   `json:"action"`
	Status      string   `json:"status"`
	Reasons     []string `json:"reasons"`
	Path        string   `json:"path,omitempty"`
	CreatedAt   string   `json:"created_at"`
	Description string   `json:"description,omitempty"`
	DraftBody   string   `json:"draft_body,omitempty"`
}

// RefineListResult is the skills_refine_list MCP payload.
type RefineListResult struct {
	Count     int        `json:"count"`
	Status    string     `json:"status_filter"`
	Proposals []Proposal `json:"proposals"`
	QueueDir  string     `json:"queue_dir"`
	Note      string     `json:"note"`
}

// ListProposals lists refine proposals from the local queue (read-only).
func ListProposals(home, status string) RefineListResult {
	if status == "" {
		status = "pending"
	}
	queue := refineQueueDir(home)
	entries, _ := os.ReadDir(queue)
	var rows []Proposal
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(queue, e.Name()))
		if err != nil {
			continue
		}
		var p Proposal
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		if status != "all" && p.Status != status {
			continue
		}
		rows = append(rows, p)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt < rows[j].CreatedAt
	})
	return RefineListResult{
		Count:     len(rows),
		Status:    status,
		Proposals: rows,
		QueueDir:  queue,
		Note:      "Read-only refine proposals; factory apply is consent-gated separately",
	}
}

// ProposeThinSkills enqueues update proposals for skills with thin descriptions.
// Does not write SKILL.md bodies.
func ProposeThinSkills(cat core.Catalog, home, source string) []Proposal {
	if !Enabled() {
		return nil
	}
	queue := refineQueueDir(home)
	var out []Proposal
	for _, e := range cat.Entries {
		desc := strings.TrimSpace(e.Description)
		if len(desc) >= thinDescriptionChars {
			continue
		}
		reasons := []string{"thin_description"}
		if pendingDuplicate(queue, e.Name, reasons, "update") {
			continue
		}
		id := "thin-" + safeFilename(e.Name) + "-" + time.Now().UTC().Format("20060102T150405")
		improved := desc
		if improved != "" {
			improved += " - "
		}
		improved += "Expand this description so pack/select can match developer tasks."
		if len(improved) < thinDescriptionChars {
			improved = e.Name + ": Cursor skill for SmartContext packing and triage."
		}
		p := Proposal{
			ID:          id,
			SkillName:   e.Name,
			Action:      "update",
			Status:      "pending",
			Reasons:     reasons,
			Path:        e.Path,
			CreatedAt:   time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
			Description: e.Description,
			DraftBody: "---\nname: " + e.Name + "\ndescription: " + improved + "\n---\n\n" +
				"# " + e.Name + "\n\n_Draft from refine scan (thin_description). Review before factory apply._\n",
		}
		payload, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			continue
		}
		path := filepath.Join(queue, id+".json")
		if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
			continue
		}
		RecordEvent("refine_propose", map[string]any{
			"id": id, "skill_name": e.Name, "reasons": reasons, "action": "update",
		}, source, home)
		out = append(out, p)
	}
	return out
}

func pendingDuplicate(queue, skillName string, reasons []string, action string) bool {
	entries, err := os.ReadDir(queue)
	if err != nil {
		return false
	}
	want := map[string]bool{}
	for _, r := range reasons {
		want[r] = true
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(queue, e.Name()))
		if err != nil {
			continue
		}
		var p Proposal
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		if p.Status != "pending" || p.SkillName != skillName {
			continue
		}
		if p.Action == "" {
			p.Action = "update"
		}
		if p.Action != action || len(p.Reasons) != len(want) {
			continue
		}
		match := true
		for _, r := range p.Reasons {
			if !want[r] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func safeFilename(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "skill"
	}
	if len(out) > 80 {
		return out[:80]
	}
	return out
}
