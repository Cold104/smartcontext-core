// Package eval hosts reusable corpus / telemetry measurement harnesses
// modeled on Python token_benchmark + role_token_benchmark (EVAL_HYGIENE).
package eval

import (
	"fmt"
	"os"
	"path/filepath"
)

// TaskSpec mirrors Python token_benchmark.TaskSpec / STANDARD_TASKS.
type TaskSpec struct {
	ID       string
	Prompt   string
	Gold     []string
	Negative bool
}

// RoleSpec mirrors Python role_token_benchmark.RoleSpec.
type RoleSpec struct {
	ID      string
	Persona string
	Role    string
	Apps    []string
	Prompt  string
	Gold    []string
}

// StandardTasks mirrors src/smartcontext/core/token_benchmark.py STANDARD_TASKS.
func StandardTasks() []TaskSpec {
	return []TaskSpec{
		{ID: "hermes-team", Prompt: "launch Hermes engineering team containers docker persona indexer", Gold: []string{"hermes-agent-team", "docker-personas"}},
		{ID: "skill-index", Prompt: "index Cursor skills and rules into a catalog with frontmatter", Gold: []string{"skill-catalog", "frontmatter-parse"}},
		{ID: "windows-paths", Prompt: "Windows PowerShell paths F drive quoting for Python scripts", Gold: []string{"windows-dev", "powershell-paths"}},
		{ID: "token-ingest", Prompt: "reduce token usage when ingesting code into Cursor chat context", Gold: []string{"token-pack", "context-budget"}},
		{ID: "cycle-review", Prompt: "evaluate Hermes cycle with lint smoke review before DONE merge", Gold: []string{"cycle-review", "merge-gate"}},
	}
}

// ExtendedTasks adds distractor + negative cases used by Go POC eval.
func ExtendedTasks() []TaskSpec {
	base := StandardTasks()
	return append(base,
		TaskSpec{ID: "distractor-cooking", Prompt: "sourdough bread recipes and kitchen timer techniques", Gold: []string{"unrelated-cooking"}},
		TaskSpec{ID: "negative-astronomy", Prompt: "orbital mechanics of exoplanet transit photometry", Negative: true},
	)
}

// RoleSkills mirrors Python ROLE_SKILLS (name, description).
func RoleSkills() map[string][][2]string {
	return map[string][][2]string{
		"data_analytics": {
			{"dbt-analytics-engineering", "Build and modify dbt models, write tests, explore Snowflake data sources"},
			{"dbt-semantic-layer", "Create MetricFlow metrics and dimensions for governed KPIs"},
			{"looker-explore-router", "Route business questions to the correct Looker explore and LookML model"},
			{"amplitude-analyze-chart", "Design and analyze Amplitude charts, funnels, and retention"},
			{"snowflake-cost-aware-sql", "Write cost-aware Snowflake SQL with warehouse sizing"},
			{"data-contracts-dq", "Data contracts, backfills, and data-quality governance"},
		},
		"product_management": {
			{"amplitude-weekly-product-review", "Automate a weekly product review from Amplitude signals"},
			{"analyze-experiment", "Design and interpret A/B experiments with statistical rigor"},
			{"opportunity-scoring", "Score product opportunities by ROI from multi-source evidence"},
			{"launchdarkly-flag-rollout", "Plan LaunchDarkly flag exposure and staged rollout checklists"},
			{"figma-spec-handoff", "Turn Figma designs into a spec handoff for engineering"},
			{"pendo-friction-themes", "Cluster Pendo feedback into prioritized friction themes"},
		},
		"software_development": {
			{"typescript-javascript", "TypeScript and JavaScript patterns for services and apps"},
			{"python-development", "Modern Python patterns, typing, and packaging"},
			{"nestjs-module-patterns", "NestJS module, provider, and health-check patterns"},
			{"aspire-service-defaults", ".NET Aspire service defaults and orchestration"},
			{"django-orm-migrations", "Django ORM models, migrations, and smoke tests"},
			{"write-tests", "Write unit and integration tests before shipping"},
		},
		"networking": {
			{"networking-transport", "Networking and transport non-negotiables for gRPC and services"},
			{"istio-traffic-management", "Istio VirtualService, mTLS, and traffic policy management"},
			{"haproxy-acl-tuning", "HAProxy ACL rules, rate limits, and load-balancer tuning"},
			{"cisco-aci-epg-contracts", "Cisco ACI EPG, contracts, and tenant fabric design"},
			{"grpc-deadline-budgets", "gRPC deadline and retry budget configuration"},
			{"service-mesh-mtls", "Service mesh mutual TLS enforcement across workloads"},
		},
		"security": {
			{"aws-iam", "AWS IAM roles, policies, and OIDC federation for CI"},
			{"zero-trust", "Zero Trust identity, network, and workload architecture"},
			{"iam-security-advisor", "Principal-level IAM, IGA, and PKI decisions"},
			{"codebase-security-audit", "Multi-layer secrets, SAST, SCA, and IaC audit"},
			{"prisma-cloud-misconfig", "Map Prisma Cloud CSPM misconfigurations to fixes"},
			{"crowdstrike-detection-triage", "Triage CrowdStrike EDR detections and draft notes"},
		},
		"devops": {
			{"pipeline-review", "Review CI/CD pipelines for reliability and efficiency"},
			{"pulumi-preview-review", "Analyze Pulumi preview diffs and infrastructure design"},
			{"docker-review", "Review Dockerfiles and container build best practices"},
			{"gitlab-ci-oidc", "GitLab CI pipelines with OIDC and secure job design"},
			{"spinnaker-pipeline-templates", "Spinnaker pipeline templates and canary gates"},
			{"release-readiness", "Validate production deployment readiness"},
		},
		"devsecops": {
			{"devops-security", "DevSecOps pipeline integration: SAST, DAST, container scanning"},
			{"checkov-policy-suppressions", "Triage Checkov IaC findings and manage suppressions"},
			{"owasp-zap-baseline", "Run OWASP ZAP baseline scans in merge-request pipelines"},
			{"sonarqube-quality-gate", "Configure SonarQube quality gates and hotspots"},
			{"supply-chain-sbom", "SBOM generation, signing, and provenance for supply chain"},
			{"policy-as-code-opa", "Policy-as-code with OPA and Gatekeeper"},
		},
		"platform_engineering": {
			{"platform-engineering", "Build internal developer platforms and golden paths"},
			{"crossplane-composition", "Author Crossplane compositions and XRDs"},
			{"port-blueprint", "Define Port.io blueprints and scorecards for the IDP"},
			{"kubernetes-ops", "Deploy, scale, and troubleshoot Kubernetes workloads"},
			{"argocd-gitops", "GitOps deployments and reconciliation with Argo CD"},
			{"golden-path-scaffold", "Scaffold golden-path templates for service creation"},
		},
	}
}

// Roles mirrors Python role_token_benchmark.ROLES.
func Roles() []RoleSpec {
	return []RoleSpec{
		{ID: "data_analytics", Persona: "Priya", Role: "Data analytics engineer", Apps: []string{"Amplitude", "Looker", "Snowflake"},
			Prompt: "Explain churn KPI from Snowflake via Looker explore and draft an Amplitude chart definition using the dbt semantic layer",
			Gold:   []string{"dbt-semantic-layer", "looker-explore-router", "amplitude-analyze-chart"}},
		{ID: "product_management", Persona: "Morgan", Role: "Product manager", Apps: []string{"LaunchDarkly", "Figma", "Pendo"},
			Prompt: "Weekly product review: LaunchDarkly flag exposure, Pendo friction themes, and Figma spec handoff for checkout",
			Gold:   []string{"amplitude-weekly-product-review", "launchdarkly-flag-rollout", "pendo-friction-themes"}},
		{ID: "software_development", Persona: "Dana", Role: "Software developer", Apps: []string{"NestJS", ".NET Aspire", "Django"},
			Prompt: "Add a shared health-check endpoint pattern for NestJS and .NET Aspire and mirror a Django smoke test",
			Gold:   []string{"nestjs-module-patterns", "aspire-service-defaults", "django-orm-migrations"}},
		{ID: "networking", Persona: "Kai", Role: "Network engineer", Apps: []string{"HAProxy", "Istio", "Cisco ACI"},
			Prompt: "Tighten Istio mTLS for payments, mirror rate limits in HAProxy, and note the Cisco ACI contract impact",
			Gold:   []string{"istio-traffic-management", "haproxy-acl-tuning", "cisco-aci-epg-contracts"}},
		{ID: "security", Persona: "Samira", Role: "Security engineer", Apps: []string{"AWS IAM", "CrowdStrike", "Prisma Cloud"},
			Prompt: "Review an AWS IAM role for CI OIDC, map Prisma Cloud misconfigurations, and draft a CrowdStrike detection note",
			Gold:   []string{"aws-iam", "prisma-cloud-misconfig", "crowdstrike-detection-triage"}},
		{ID: "devops", Persona: "Jordan", Role: "DevOps engineer", Apps: []string{"Spinnaker", "Pulumi", "GitLab CI"},
			Prompt: "GitLab CI job is flaky on the Pulumi preview step; compare against the Spinnaker canary gate for release readiness",
			Gold:   []string{"gitlab-ci-oidc", "pulumi-preview-review", "spinnaker-pipeline-templates"}},
		{ID: "devsecops", Persona: "Riley", Role: "DevSecOps engineer", Apps: []string{"Checkov", "OWASP ZAP", "SonarQube"},
			Prompt: "Triage a Checkov S3 failure, map it to a SonarQube hotspot, and plan an OWASP ZAP baseline in the merge-request pipeline",
			Gold:   []string{"checkov-policy-suppressions", "sonarqube-quality-gate", "owasp-zap-baseline"}},
		{ID: "platform_engineering", Persona: "Avery", Role: "Platform engineer", Apps: []string{"Kubernetes", "Port.io", "Crossplane"},
			Prompt: "Add a Crossplane XRD for Redis, expose a Port.io blueprint, and document Kubernetes resource quotas for tenants",
			Gold:   []string{"crossplane-composition", "port-blueprint", "kubernetes-ops"}},
	}
}

func skillMD(name, desc string, extraParagraphs int) string {
	var filler string
	for i := 1; i <= extraParagraphs; i++ {
		filler += fmt.Sprintf(
			"\n\nParagraph %d: detailed guidance for %s covering workflow steps, edge cases, and examples that would normally bloat a Cursor prompt.",
			i, name,
		)
	}
	return fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s%s\n", name, desc, name, desc, filler)
}

func writeSkillFile(dir, name, desc string, extraParagraphs int) error {
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD(name, desc, extraParagraphs)), 0o644)
}

// WriteTokenFixture mirrors Python generate_token_fixture (skills + filler paragraphs).
// Layout: root/skills/<name>/SKILL.md (discoverable under SMARTCONTEXT_SKILL_ROOTS=root).
func WriteTokenFixture(root string, extraParagraphs int) (int, error) {
	if extraParagraphs <= 0 {
		extraParagraphs = 12
	}
	skillsRoot := filepath.Join(root, "skills")
	defs := [][2]string{
		{"hermes-agent-team", "Launch Hermes engineering team Docker persona containers"},
		{"docker-personas", "Docker resource limits for indexer implementer critic docs"},
		{"skill-catalog", "Index Cursor SKILL.md files into a unified skill rule catalog"},
		{"frontmatter-parse", "Parse YAML frontmatter name and description from skills"},
		{"windows-dev", "Windows PowerShell development patterns on F drive paths"},
		{"powershell-paths", "Quoting and path joining for Python on Windows shells"},
		{"token-pack", "Pack only necessary skill context under a token budget for Cursor"},
		{"context-budget", "Estimate tokens and truncate bodies to cut prompt spend"},
		{"cycle-review", "Hermes cycle evaluation lint smoke review gate before DONE"},
		{"merge-gate", "Merge sandbox to main only after critic and review PASS"},
		{"unrelated-cooking", "Recipes for sourdough bread and kitchen timers"},
		{"unrelated-gardening", "Seasonal planting calendar for tomatoes and herbs"},
		{"unrelated-travel", "Packing lists for international flights and hotels"},
		{"unrelated-finance", "Personal budgeting spreadsheets and tax filing tips"},
		{"unrelated-music", "Guitar chord progressions and practice metronomes"},
	}
	for _, d := range defs {
		if err := writeSkillFile(skillsRoot, d[0], d[1], extraParagraphs); err != nil {
			return 0, err
		}
	}
	return len(defs), nil
}

// WriteRoleFixture mirrors Python generate_role_fixture (shared multi-role catalog).
func WriteRoleFixture(root string, extraParagraphs int) (int, error) {
	if extraParagraphs <= 0 {
		extraParagraphs = 12
	}
	skillsRoot := filepath.Join(root, "skills")
	count := 0
	for _, skills := range RoleSkills() {
		for _, d := range skills {
			if err := writeSkillFile(skillsRoot, d[0], d[1], extraParagraphs); err != nil {
				return 0, err
			}
			count++
		}
	}
	return count, nil
}
