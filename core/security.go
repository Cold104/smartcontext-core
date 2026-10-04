package core

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ScrubReport is the pack-time security overlay (secret values never retained).
type ScrubReport struct {
	Enabled             bool             `json:"enabled"`
	ScannedAt           string           `json:"scanned_at"`
	TotalRedactions     int              `json:"total_redactions"`
	RulesApplied        []string         `json:"rules_applied,omitempty"`
	Events              []RedactionEvent `json:"events,omitempty"`
	TotalInjectionFlags int              `json:"total_injection_flags"`
	InjectionFlags      []InjectionFlag  `json:"injection_flags,omitempty"`
	Note                string           `json:"note,omitempty"`
}

// RedactionEvent reports one secret rule hit without storing the secret value.
type RedactionEvent struct {
	RuleID      string `json:"rule_id"`
	Description string `json:"description"`
	Placeholder string `json:"placeholder"`
	Occurrences int    `json:"occurrences"`
	EntryName   string `json:"entry_name,omitempty"`
	Path        string `json:"path,omitempty"`
}

// InjectionFlag is a report-only prompt-injection signal.
type InjectionFlag struct {
	RuleID      string `json:"rule_id"`
	Description string `json:"description"`
	Occurrences int    `json:"occurrences"`
	EntryName   string `json:"entry_name,omitempty"`
	Path        string `json:"path,omitempty"`
}

type secretRule struct {
	id          string
	description string
	pattern     *regexp.Regexp
	assigned    bool // redact capture group 1 only
}

type injectionRule struct {
	id          string
	description string
	pattern     *regexp.Regexp
}

var secretRules = []secretRule{
	{
		id:          "aws_access_key_id",
		description: "AWS access key ID (AKIA…)",
		pattern:     regexp.MustCompile(`\b(AKIA[0-9A-Z]{16})\b`),
	},
	{
		id:          "github_pat",
		description: "GitHub personal access token (ghp_…)",
		pattern:     regexp.MustCompile(`\b(ghp_[A-Za-z0-9]{36})\b`),
	},
	{
		id:          "slack_token",
		description: "Slack bot/user token (xox…)",
		pattern:     regexp.MustCompile(`\b(xox[baprs]-[A-Za-z0-9-]{10,})\b`),
	},
	{
		id:          "private_key_block",
		description: "PEM/OpenSSH private key block",
		pattern:     regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----[\s\S]*?-----END (?:RSA |EC |OPENSSH )?PRIVATE KEY-----`),
	},
	{
		id:          "assigned_api_secret",
		description: "Quoted API/secret/token assignment",
		pattern:     regexp.MustCompile(`(?i)\b(?:api[_-]?key|secret[_-]?key|access[_-]?token|auth[_-]?token)\s*[:=]\s*['"]([A-Za-z0-9_\-/.+=]{20,})['"]`),
		assigned:    true,
	},
}

var injectionRules = []injectionRule{
	{
		id:          "ignore_previous_instructions",
		description: "Override attempt: ignore/disregard/forget prior instructions",
		pattern:     regexp.MustCompile(`(?i)\b(?:ignore|disregard|forget)\b[^.\n]{0,40}?\b(?:all\s+|the\s+)?(?:previous|prior|above|earlier|preceding)\b[^.\n]{0,24}?\b(?:instructions?|prompts?|rules?|context|messages?)\b`),
	},
	{
		id:          "reveal_system_prompt",
		description: "Exfiltration attempt: reveal/print system prompt, instructions, or secrets",
		pattern:     regexp.MustCompile(`(?i)\b(?:reveal|print|show|repeat|output|leak|exfiltrate|send)\b[^.\n]{0,40}?\b(?:system\s+prompt|your\s+(?:system\s+)?instructions?|hidden\s+prompt|api[_\s-]?keys?|secrets?|credentials?)\b`),
	},
	{
		id:          "role_override",
		description: "Role override / jailbreak framing",
		pattern:     regexp.MustCompile(`(?i)\b(?:you\s+are\s+now|from\s+now\s+on\s+you|act\s+as|pretend\s+to\s+be)\b[^.\n]{0,48}?\b(?:developer\s+mode|jailbreak|unrestricted|do\s+anything|DAN)\b`),
	},
}

func placeholderFor(ruleID string) string {
	return "[REDACTED:" + ruleID + "]"
}

func emptyScrubReport(enabled bool) ScrubReport {
	rules := make([]string, 0, len(secretRules))
	if enabled {
		for _, r := range secretRules {
			rules = append(rules, r.id)
		}
	}
	return ScrubReport{
		Enabled:      enabled,
		ScannedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		RulesApplied: rules,
		Note:         "pack-time secret redaction + report-only injection scan",
	}
}

func redactSecrets(text, entryName, path string) (string, []RedactionEvent) {
	if text == "" {
		return text, nil
	}
	scrubbed := text
	var events []RedactionEvent
	for _, rule := range secretRules {
		if !rule.pattern.MatchString(scrubbed) {
			continue
		}
		ph := placeholderFor(rule.id)
		count := 0
		if rule.assigned {
			scrubbed = rule.pattern.ReplaceAllStringFunc(scrubbed, func(m string) string {
				sub := rule.pattern.FindStringSubmatchIndex(m)
				if len(sub) < 4 {
					return m
				}
				count++
				return m[:sub[2]] + ph + m[sub[3]:]
			})
		} else {
			locs := rule.pattern.FindAllStringIndex(scrubbed, -1)
			count = len(locs)
			if count > 0 {
				scrubbed = rule.pattern.ReplaceAllString(scrubbed, ph)
			}
		}
		if count > 0 {
			events = append(events, RedactionEvent{
				RuleID:      rule.id,
				Description: rule.description,
				Placeholder: ph,
				Occurrences: count,
				EntryName:   entryName,
				Path:        path,
			})
		}
	}
	return scrubbed, events
}

func scanInjection(text, entryName, path string) []InjectionFlag {
	if text == "" {
		return nil
	}
	var flags []InjectionFlag
	for _, rule := range injectionRules {
		locs := rule.pattern.FindAllStringIndex(text, -1)
		if n := len(locs); n > 0 {
			flags = append(flags, InjectionFlag{
				RuleID:      rule.id,
				Description: rule.description,
				Occurrences: n,
				EntryName:   entryName,
				Path:        path,
			})
		}
	}
	return flags
}

func mergeScrubReports(dst *ScrubReport, src ScrubReport) {
	dst.Events = append(dst.Events, src.Events...)
	dst.TotalRedactions += src.TotalRedactions
	dst.InjectionFlags = append(dst.InjectionFlags, src.InjectionFlags...)
	dst.TotalInjectionFlags += src.TotalInjectionFlags
}

// ScrubBody redacts high-confidence secrets and flags injection markers (report-only).
func ScrubBody(body, entryName, path string, redact bool) (string, ScrubReport) {
	report := emptyScrubReport(redact)
	if body == "" {
		return body, report
	}
	out := body
	if redact {
		var events []RedactionEvent
		out, events = redactSecrets(body, entryName, path)
		report.Events = events
		for _, e := range events {
			report.TotalRedactions += e.Occurrences
		}
	}
	flags := scanInjection(body, entryName, path)
	report.InjectionFlags = flags
	for _, f := range flags {
		report.TotalInjectionFlags += f.Occurrences
	}
	return out, report
}

func formatSecurityMarkdown(report ScrubReport) string {
	var b strings.Builder
	b.WriteString("## Pack security report\n")
	b.WriteString(fmt.Sprintf("Redaction enabled: %v\n", report.Enabled))
	b.WriteString(fmt.Sprintf("Scanned at: %s\n", report.ScannedAt))
	b.WriteString(fmt.Sprintf("Total redactions: %d\n", report.TotalRedactions))
	if !report.Enabled {
		b.WriteString("Secret scanning was disabled for this pack.\n")
	} else if len(report.Events) == 0 {
		b.WriteString("No high-confidence secrets matched pack-time rules.\n")
	} else {
		b.WriteString("Redactions (rule id only; secret values not retained):\n")
		for _, event := range report.Events {
			where := event.EntryName
			if where == "" {
				where = event.Path
			}
			if where == "" {
				where = "(unknown)"
			}
			b.WriteString(fmt.Sprintf("- `%s` ×%d in `%s` → `%s`\n",
				event.RuleID, event.Occurrences, where, event.Placeholder))
		}
	}
	b.WriteString(fmt.Sprintf("Prompt-injection signals: %d\n", report.TotalInjectionFlags))
	if len(report.InjectionFlags) > 0 {
		b.WriteString("Untrusted content flagged (report-only; content NOT modified — review before trusting):\n")
		for _, flag := range report.InjectionFlags {
			where := flag.EntryName
			if where == "" {
				where = flag.Path
			}
			if where == "" {
				where = "(unknown)"
			}
			b.WriteString(fmt.Sprintf("- `%s` ×%d in `%s`\n", flag.RuleID, flag.Occurrences, where))
		}
	}
	return b.String()
}
