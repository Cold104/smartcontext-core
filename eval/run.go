package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/lifecycle"
	scmcp "github.com/Cold104/smartcontext-core/mcp"
	"github.com/Cold104/smartcontext-core/version"
)

const (
	DefaultTopN         = 5
	DefaultBudgetTokens = 2000
	DefaultPackIters    = 8
	DefaultMinReduction = 0.70
	DefaultMinRecall    = 0.80
)

// TaskResult is one scored pack measurement.
type TaskResult struct {
	ID              string   `json:"id"`
	Suite           string   `json:"suite"`
	Prompt          string   `json:"prompt"`
	Gold            []string `json:"gold"`
	Negative        bool     `json:"negative,omitempty"`
	Selected        []string `json:"selected"`
	Hit             bool     `json:"hit"`
	RecallAtK       float64  `json:"recall_at_k"`
	NaiveTokens     int      `json:"naive_tokens"`
	EstimatedTokens int      `json:"estimated_tokens"`
	BudgetTokens    int      `json:"budget_tokens"`
	BudgetPct       float64  `json:"budget_pct,omitempty"`
	SelectedCount   int      `json:"selected_count"`
	Reduction       float64  `json:"reduction"`
	LatencyMSP50    float64  `json:"latency_ms_p50"`
	LatencyMSP95    float64  `json:"latency_ms_p95"`
	LatencyMSAvg    float64  `json:"latency_ms_avg"`
	NegativeOK      bool     `json:"negative_ok,omitempty"`
}

// SuiteSummary aggregates one corpus suite.
type SuiteSummary struct {
	Name            string       `json:"name"`
	SkillCount      int          `json:"skill_count"`
	Hits            int          `json:"hits"`
	ScoredTasks     int          `json:"scored_tasks"`
	HitRate         float64      `json:"hit_rate"`
	MeanRecallAtK   float64      `json:"mean_recall_at_k"`
	MedianReduction float64      `json:"median_reduction"`
	MeanReduction   float64      `json:"mean_reduction"`
	PackMSP50       float64      `json:"pack_ms_p50"`
	PackMSP95       float64      `json:"pack_ms_p95"`
	PackMSAvg       float64      `json:"pack_ms_avg"`
	DiscoverMSP50   float64      `json:"discover_ms_p50"`
	DiscoverMSP95   float64      `json:"discover_ms_p95"`
	NaiveBaseline   int          `json:"naive_baseline_tokens"`
	Tasks           []TaskResult `json:"tasks"`
	GateOK          bool         `json:"gate_ok"`
	Violations      []string     `json:"violations,omitempty"`
}

// TelemetrySummary captures local lifecycle telemetry exercise.
type TelemetrySummary struct {
	Enabled           bool   `json:"lifecycle_enabled"`
	Home              string `json:"isolated_home"`
	EventsPath        string `json:"events_path"`
	PackEventsWritten int    `json:"pack_events_written"`
	JSONLBytes        int64  `json:"jsonl_bytes"`
	JSONLLines        int    `json:"jsonl_lines"`
	RawTaskLeaked     bool   `json:"raw_task_leaked"`
	MemoryEventCount  int    `json:"memory_event_count"`
	RunLogPath        string `json:"run_telemetry_log,omitempty"`
	RunLogLines       int    `json:"run_telemetry_log_lines"`
	Note              string `json:"note"`
}

// CaseStudySummary is a thin cold/seeded stand-in for Python case studies.
type CaseStudySummary struct {
	ColdEmptyCatalog bool     `json:"cold_empty_catalog"`
	ColdSelected     int      `json:"cold_selected_count"`
	SeededCatalog    int      `json:"seeded_catalog_size"`
	SeededHit        bool     `json:"seeded_pack_hit"`
	SeededSelected   []string `json:"seeded_selected"`
	Note             string   `json:"note"`
}

// Report is the full corpus + telemetry eval capture.
type Report struct {
	Version          string           `json:"version"`
	CapturedAt       string           `json:"captured_at"`
	HostNote         string           `json:"host_note"`
	PublicationGate  string           `json:"publication_gate"`
	TopN             int              `json:"top_n"`
	BudgetTokens     int              `json:"budget_tokens"`
	PackIters        int              `json:"pack_iters"`
	MinReduction     float64          `json:"min_reduction_required"`
	MinRecall        float64          `json:"min_recall_required"`
	TokenSuite       SuiteSummary     `json:"token_suite"`
	RoleSuite        SuiteSummary     `json:"role_suite"`
	MCPDiscoverMSP50 float64          `json:"mcp_discover_ms_p50"`
	MCPDiscoverMSP95 float64          `json:"mcp_discover_ms_p95"`
	MCPPackMSP50     float64          `json:"mcp_pack_ms_p50"`
	MCPPackMSP95     float64          `json:"mcp_pack_ms_p95"`
	MCPToolNames     []string         `json:"mcp_tool_names"`
	CaseStudy        CaseStudySummary `json:"case_study"`
	Telemetry        TelemetrySummary `json:"telemetry"`
	PythonCited      map[string]any   `json:"python_cited_evidence"`
	Gaps             []string         `json:"gaps_vs_python"`
}

// Options configures a corpus/telemetry run.
type Options struct {
	GoRoot       string
	BudgetTokens int
	TopN         int
	PackIters    int
	MinReduction float64
	MinRecall    float64
	WriteDocs    bool
	TelemetryLog string // optional path for run-local JSONL pack log
}

// Run executes token + role corpus measurements, MCP latency, case study, telemetry.
func Run(opts Options) (Report, error) {
	if opts.BudgetTokens <= 0 {
		opts.BudgetTokens = DefaultBudgetTokens
	}
	if opts.TopN <= 0 {
		opts.TopN = DefaultTopN
	}
	if opts.PackIters <= 0 {
		opts.PackIters = DefaultPackIters
	}
	if opts.MinReduction <= 0 {
		opts.MinReduction = DefaultMinReduction
	}
	if opts.MinRecall <= 0 {
		opts.MinRecall = DefaultMinRecall
	}
	if opts.GoRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Report{}, err
		}
		opts.GoRoot = wd
		if filepath.Base(wd) != "go" {
			cand := filepath.Join(wd, "go")
			if st, e := os.Stat(cand); e == nil && st.IsDir() {
				opts.GoRoot = cand
			}
		}
	}

	work, err := os.MkdirTemp("", "sc-corpus-telemetry-*")
	if err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(work)

	home := filepath.Join(work, "sc-home")
	_ = os.Setenv("SMARTCONTEXT_HOME", home)
	_ = os.Unsetenv("SMARTCONTEXT_ROOT")
	_ = os.Unsetenv("SMARTCONTEXT_FRIEND_ROOT")
	// Lifecycle on (default); do not force production opt-out.
	_ = os.Unsetenv("SMARTCONTEXT_LIFECYCLE")
	// Corpus gold tasks must not inherit sticky names across prompts.
	_ = os.Setenv("SMARTCONTEXT_STICKY_SESSION", "0")
	lifecycle.ResetMemoryEvents()
	lifecycle.EnsureLayout(home)

	rep := Report{
		Version:         version.Version,
		CapturedAt:      time.Now().UTC().Format(time.RFC3339),
		HostNote:        "Windows local corpus+telemetry eval; isolated SMARTCONTEXT_HOME + SKILL_ROOTS",
		PublicationGate: "CLOSED — local measurement only; no Python parity / publication claim",
		TopN:            opts.TopN,
		BudgetTokens:    opts.BudgetTokens,
		PackIters:       opts.PackIters,
		MinReduction:    opts.MinReduction,
		MinRecall:       opts.MinRecall,
		PythonCited: map[string]any{
			"source":                    "docs/requirements.md (product docs; not re-run here)",
			"recorded_median_reduction": 0.863,
			"recorded_mean_recall_at_5": 0.927,
			"gates":                     "median reduction >= 70%; mean recall@5 >= 80%",
			"note":                      "Cited only; Python baseline not re-executed in this harness run.",
		},
		Gaps: []string{
			"Python refund-receipt / metrics_summary aggregates (load_aggregates, export_refund_receipt) — Go has local JSONL RecordEvent only; no receipt builder",
			"Python case studies refine_scan + factory apply + Canvas CTA — Go case study is cold empty vs seeded pack only",
			"Python include_security_report=False for token gate — Go pack markdown always includes scrub report lines (methodology delta)",
			"Python .cursor/rules/*.mdc in token fixture — Go discover is SKILL.md-only in this harness",
			"Diehard held-out ladder / Lathe stack-eval ingest — out of scope",
			"Core micro-bench Budget.for_ci latency gates — ported (go/docs/LATENCY.md; TestLatencyBudgets + CI benches)",
			"Remote / upload telemetry lifecycle — intentionally not required; local JSONL only",
		},
	}

	var runLog *os.File
	if opts.TelemetryLog != "" {
		if err := os.MkdirAll(filepath.Dir(opts.TelemetryLog), 0o755); err != nil {
			return Report{}, err
		}
		runLog, err = os.Create(opts.TelemetryLog)
		if err != nil {
			return Report{}, err
		}
		defer runLog.Close()
		rep.Telemetry.RunLogPath = opts.TelemetryLog
	}

	tokenRoot := filepath.Join(work, "token-fixture")
	nSkills, err := WriteTokenFixture(tokenRoot, 12)
	if err != nil {
		return Report{}, err
	}
	rep.TokenSuite, err = runSuite("token_benchmark", tokenRoot, nSkills, StandardTasks(), opts, runLog)
	if err != nil {
		return Report{}, err
	}

	roleRoot := filepath.Join(work, "role-fixture")
	nRoles, err := WriteRoleFixture(roleRoot, 12)
	if err != nil {
		return Report{}, err
	}
	roleTasks := make([]TaskSpec, 0, len(Roles()))
	for _, r := range Roles() {
		roleTasks = append(roleTasks, TaskSpec{ID: r.ID, Prompt: r.Prompt, Gold: r.Gold})
	}
	rep.RoleSuite, err = runSuite("role_token_benchmark", roleRoot, nRoles, roleTasks, opts, runLog)
	if err != nil {
		return Report{}, err
	}

	// MCP latency against token fixture (lifecycle path via MCP).
	_ = os.Setenv("SMARTCONTEXT_SKILL_ROOTS", tokenRoot)
	tools, discP50, discP95, packP50, packP95 := probeMCP(tokenRoot, opts)
	rep.MCPToolNames = tools
	rep.MCPDiscoverMSP50 = discP50
	rep.MCPDiscoverMSP95 = discP95
	rep.MCPPackMSP50 = packP50
	rep.MCPPackMSP95 = packP95

	rep.CaseStudy = runCaseStudy(work)
	rep.Telemetry = mergeTelemetry(rep.Telemetry, home, "secret corpus telemetry task text that must not leak")

	if opts.WriteDocs {
		docsDir := filepath.Join(opts.GoRoot, "docs")
		if err := os.MkdirAll(docsDir, 0o755); err != nil {
			return Report{}, err
		}
		jsonPath := filepath.Join(docsDir, "corpus_telemetry_eval.json")
		mdPath := filepath.Join(docsDir, "CORPUS_TELEMETRY_EVAL.md")
		b, _ := json.MarshalIndent(rep, "", "  ")
		if err := os.WriteFile(jsonPath, b, 0o644); err != nil {
			return Report{}, err
		}
		if err := os.WriteFile(mdPath, []byte(RenderMarkdown(rep)), 0o644); err != nil {
			return Report{}, err
		}
	}

	return rep, nil
}

func isolateRoots(root string) {
	_ = os.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	_ = os.Unsetenv("SMARTCONTEXT_ROOT")
	_ = os.Unsetenv("SMARTCONTEXT_FRIEND_ROOT")
}

func runSuite(name, root string, skillCount int, tasks []TaskSpec, opts Options, runLog *os.File) (SuiteSummary, error) {
	isolateRoots(root)
	sum := SuiteSummary{Name: name, SkillCount: skillCount}

	discoverMS := make([]float64, 0, opts.PackIters)
	var lastCat core.Catalog
	for i := 0; i < opts.PackIters; i++ {
		start := time.Now()
		lastCat = core.BuildCatalog(nil)
		discoverMS = append(discoverMS, msSince(start))
	}
	sort.Float64s(discoverMS)
	sum.DiscoverMSP50 = Percentile(discoverMS, 50)
	sum.DiscoverMSP95 = Percentile(discoverMS, 95)
	if len(lastCat.Entries) != skillCount {
		return sum, fmt.Errorf("%s: discover count=%d want %d", name, len(lastCat.Entries), skillCount)
	}

	// Naive baseline once (mirrors Python naive_full_corpus_pack).
	// Use budget_pct=100 so NaiveTokens is computed; absolute tiers (medium/balanced)
	// intentionally leave NaiveTokens=0 and must not be used here.
	naiveRes, err := core.ResolveBudget(lastCat.Entries, "(all tasks share corpus)", "", 100, 0)
	if err != nil {
		return sum, err
	}
	sum.NaiveBaseline = naiveRes.NaiveTokens

	var allPackMS []float64
	var reductions, recalls []float64
	var hits, scored int

	for _, task := range tasks {
		taskMS := make([]float64, 0, opts.PackIters)
		var last core.PackResult
		for i := 0; i < opts.PackIters; i++ {
			start := time.Now()
			// Absolute budget like Python token_benchmark; bodies on for reduction.
			last = lifecycle.PackContext(core.PackRequest{
				Task:          task.Prompt,
				TopN:          opts.TopN,
				BudgetTokens:  opts.BudgetTokens,
				IncludeBodies: true,
			})
			taskMS = append(taskMS, msSince(start))
		}
		allPackMS = append(allPackMS, taskMS...)
		sort.Float64s(taskMS)

		tr := TaskResult{
			ID:              task.ID,
			Suite:           name,
			Prompt:          task.Prompt,
			Gold:            task.Gold,
			Negative:        task.Negative,
			Selected:        last.SelectedNames,
			SelectedCount:   last.SelectedCount,
			NaiveTokens:     sum.NaiveBaseline,
			EstimatedTokens: last.EstimatedTokens,
			BudgetTokens:    last.BudgetTokens,
			BudgetPct:       last.BudgetPct,
			LatencyMSP50:    Percentile(taskMS, 50),
			LatencyMSP95:    Percentile(taskMS, 95),
			LatencyMSAvg:    Mean(taskMS),
			Reduction:       Reduction(sum.NaiveBaseline, last.EstimatedTokens),
		}

		if runLog != nil {
			line, _ := json.Marshal(map[string]any{
				"ts":               time.Now().UTC().Format(time.RFC3339),
				"type":             "pack_measure",
				"suite":            name,
				"task_id":          task.ID,
				"task_hash":        lifecycle.HashTask(task.Prompt),
				"selected_names":   last.SelectedNames,
				"selected_count":   last.SelectedCount,
				"naive_tokens":     sum.NaiveBaseline,
				"estimated_tokens": last.EstimatedTokens,
				"budget_tokens":    last.BudgetTokens,
				"reduction":        tr.Reduction,
				"recall_at_k":      RecallAtK(last.SelectedNames, task.Gold, opts.TopN),
				"latency_ms_p50":   tr.LatencyMSP50,
			})
			_, _ = runLog.Write(append(line, '\n'))
		}

		if task.Negative {
			ok := true
			if len(last.Items) > 0 && last.Items[0].Score > 0 {
				lead := strings.ToLower(last.Items[0].Name)
				if lead == "windows-dev" || lead == "token-pack" || lead == "hermes-agent-team" {
					ok = false
				}
			}
			tr.NegativeOK = ok
			tr.Hit = ok
			sum.Tasks = append(sum.Tasks, tr)
			continue
		}

		scored++
		tr.Hit = HitAny(last.SelectedNames, task.Gold)
		tr.RecallAtK = RecallAtK(last.SelectedNames, task.Gold, opts.TopN)
		recalls = append(recalls, tr.RecallAtK)
		reductions = append(reductions, tr.Reduction)
		if tr.Hit {
			hits++
		}
		sum.Tasks = append(sum.Tasks, tr)
	}

	sort.Float64s(allPackMS)
	sum.PackMSP50 = Percentile(allPackMS, 50)
	sum.PackMSP95 = Percentile(allPackMS, 95)
	sum.PackMSAvg = Mean(allPackMS)
	sum.Hits = hits
	sum.ScoredTasks = scored
	if scored > 0 {
		sum.HitRate = float64(hits) / float64(scored)
		sum.MeanRecallAtK = Mean(recalls)
	}
	sum.MedianReduction = Median(reductions)
	sum.MeanReduction = Mean(reductions)

	if sum.MedianReduction < opts.MinReduction {
		sum.Violations = append(sum.Violations,
			fmt.Sprintf("median reduction %.1f%% below required %.0f%%", sum.MedianReduction*100, opts.MinReduction*100))
	}
	if sum.MeanRecallAtK < opts.MinRecall {
		sum.Violations = append(sum.Violations,
			fmt.Sprintf("mean recall@%d %.1f%% below required %.0f%%", opts.TopN, sum.MeanRecallAtK*100, opts.MinRecall*100))
	}
	sum.GateOK = len(sum.Violations) == 0
	return sum, nil
}

func runCaseStudy(work string) CaseStudySummary {
	empty := filepath.Join(work, "case-empty")
	_ = os.MkdirAll(empty, 0o755)
	isolateRoots(empty)
	cold := lifecycle.PackContext(core.PackRequest{Task: "wire cursor to lathe", TopN: 5, IncludeBodies: true})

	seeded := filepath.Join(work, "case-seeded")
	_ = os.MkdirAll(filepath.Join(seeded, "skills"), 0o755)
	_ = writeSkillFile(filepath.Join(seeded, "skills"), "windows-dev",
		"Windows PowerShell development patterns on F drive for SmartContext", 4)
	isolateRoots(seeded)
	warm := lifecycle.PackContext(core.PackRequest{
		Task: "Windows PowerShell SmartContext development on F drive", TopN: 3, IncludeBodies: true,
	})

	return CaseStudySummary{
		ColdEmptyCatalog: cold.EmptyCatalog,
		ColdSelected:     cold.SelectedCount,
		SeededCatalog:    len(core.BuildCatalog(nil).Entries),
		SeededHit:        HitAny(warm.SelectedNames, []string{"windows-dev"}),
		SeededSelected:   warm.SelectedNames,
		Note:             "Thin stand-in for docs/CASE_STUDIES_SKILLS.md A/B; no factory/refine/Canvas",
	}
}

func mergeTelemetry(base TelemetrySummary, home, secretTask string) TelemetrySummary {
	base.Enabled = lifecycle.Enabled()
	base.Home = home
	base.EventsPath = filepath.Join(home, "telemetry", "events.jsonl")

	lifecycle.ResetMemoryEvents()
	// Extra explicit packs for telemetry exercise (isolated home already used by suites).
	for i := 0; i < 3; i++ {
		lifecycle.RecordEvent("pack", map[string]any{
			"task_hash":        lifecycle.HashTask(secretTask),
			"selected_count":   2,
			"selected_names":   []string{"windows-dev", "token-pack"},
			"budget_tokens":    DefaultBudgetTokens,
			"estimated_tokens": 400 + i*10,
			"naive_tokens":     5000,
		}, lifecycle.SourceMCP, home)
	}
	base.MemoryEventCount = len(lifecycle.MemoryEvents())

	info, err := os.Stat(base.EventsPath)
	if err == nil {
		base.JSONLBytes = info.Size()
	}
	raw, err := os.ReadFile(base.EventsPath)
	if err == nil {
		base.JSONLLines = strings.Count(string(raw), "\n")
		base.RawTaskLeaked = strings.Contains(string(raw), secretTask)
		// Count pack-type lines
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, `"type":"pack"`) {
				base.PackEventsWritten++
			}
		}
	}
	base.Note = "Local JSONL under SMARTCONTEXT_HOME/telemetry; no upload. Pack suites also emit via lifecycle.PackContext."
	if base.RunLogPath != "" {
		if b, err := os.ReadFile(base.RunLogPath); err == nil {
			base.RunLogLines = strings.Count(string(b), "\n")
		}
	}
	return base
}

func probeMCP(fixtureRoot string, opts Options) (tools []string, discP50, discP95, packP50, packP95 float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	_ = os.Setenv("SMARTCONTEXT_SKILL_ROOTS", fixtureRoot)

	server := scmcp.NewServer()
	client := mcp.NewClient(&mcp.Implementation{Name: "corpus-telemetry-eval", Version: "0.0.1"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		return nil, 0, 0, 0, 0
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		return nil, 0, 0, 0, 0
	}
	defer cs.Close()

	listed, err := cs.ListTools(ctx, nil)
	if err == nil {
		for _, t := range listed.Tools {
			tools = append(tools, t.Name)
		}
	}

	discMS := make([]float64, 0, opts.PackIters)
	for i := 0; i < opts.PackIters; i++ {
		start := time.Now()
		_, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "discover_skills", Arguments: map[string]any{}})
		if err == nil {
			discMS = append(discMS, msSince(start))
		}
	}
	packMS := make([]float64, 0, opts.PackIters)
	for i := 0; i < opts.PackIters; i++ {
		start := time.Now()
		_, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name: "pack_context",
			Arguments: map[string]any{
				"task":           "Windows PowerShell SmartContext development on F drive paths",
				"top_n":          opts.TopN,
				"budget_tokens":  opts.BudgetTokens,
				"include_bodies": true,
			},
		})
		if err == nil {
			packMS = append(packMS, msSince(start))
		}
	}
	sort.Float64s(discMS)
	sort.Float64s(packMS)
	return tools, Percentile(discMS, 50), Percentile(discMS, 95), Percentile(packMS, 50), Percentile(packMS, 95)
}

func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
}
