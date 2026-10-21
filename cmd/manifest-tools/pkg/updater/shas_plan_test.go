package updater

import (
	"os"
	"strings"
	"testing"

	"github.com/opendatahub-io/opendatahub-operator/v2/cmd/manifest-tools/pkg/config"
)

func TestPlanSHAs_ComponentUnitRequiresBothPlatformLookups(t *testing.T) {
	configPath := writeTestConfig(t, `components:
  ray:
    odh:
      repo: opendatahub-io/ray
      ref: main@1111111111111111111111111111111111111111
    rhoai:
      repo: red-hat-data-services/ray
      ref: rhoai-3.6@2222222222222222222222222222222222222222
ccmCharts: {}
componentCharts: {}
`)
	gh := &mockGitHub{shas: map[string]string{
		"opendatahub-io/ray/main": "3333333333333333333333333333333333333333",
		// The RHOAI ref is intentionally absent to simulate a failed lookup.
	}}

	plan, err := PlanSHAs(t.Context(), SHAsOptions{ConfigFile: configPath, GH: gh})
	if err != nil {
		t.Fatalf("PlanSHAs() error: %v", err)
	}
	if len(plan.Units) != 0 {
		t.Fatalf("PlanSHAs() returned units %v; want no partial ray update", plan.Units)
	}
	if len(plan.Failures) != 1 {
		t.Fatalf("PlanSHAs() returned %d failures, want 1: %v", len(plan.Failures), plan.Failures)
	}
	if plan.Failures[0].Unit != "components/ray" || plan.Failures[0].Platform != "rhoai" {
		t.Fatalf("failure = %#v, want ray RHOAI lookup failure", plan.Failures[0])
	}
}

func TestPlanSHAs_CCMFailureSuppressesWholeCCMUnitOnly(t *testing.T) {
	configPath := writeTestConfig(t, `components:
  ray:
    odh:
      repo: opendatahub-io/ray
      ref: main@1111111111111111111111111111111111111111
ccmCharts:
  lws-operator:
    odh:
      repo: opendatahub-io/odh-gitops
      ref: main@2222222222222222222222222222222222222222
  sail-operator:
    odh:
      repo: red-hat-data-services/odh-gitops
      ref: main@4444444444444444444444444444444444444444
componentCharts: {}
`)
	gh := &mockGitHub{shas: map[string]string{
		"opendatahub-io/ray/main":        "3333333333333333333333333333333333333333",
		"opendatahub-io/odh-gitops/main": "5555555555555555555555555555555555555555",
		// The shared CCM ref lookup for sail-operator fails.
	}}

	plan, err := PlanSHAs(t.Context(), SHAsOptions{ConfigFile: configPath, GH: gh})
	if err != nil {
		t.Fatalf("PlanSHAs() error: %v", err)
	}
	if len(plan.Failures) != 1 || plan.Failures[0].Unit != "ccmCharts" {
		t.Fatalf("PlanSHAs() failures = %#v, want one ccmCharts failure", plan.Failures)
	}
	if len(plan.Units) != 1 || plan.Units[0].ID != "components/ray" {
		t.Fatalf("PlanSHAs() units = %#v, want only independent ray unit", plan.Units)
	}
}

func TestApplySHAPlan_RejectsStaleOldRefWithoutWriting(t *testing.T) {
	const oldRef = "main@1111111111111111111111111111111111111111"
	const latestSHA = "2222222222222222222222222222222222222222"
	const changedMainRef = "main@3333333333333333333333333333333333333333"
	configPath := writeTestConfig(t, `components:
  ray:
    odh:
      repo: opendatahub-io/ray
      ref: `+oldRef+`
ccmCharts: {}
componentCharts: {}
`)

	plan, err := PlanSHAs(t.Context(), SHAsOptions{
		ConfigFile: configPath,
		GH: &mockGitHub{shas: map[string]string{
			"opendatahub-io/ray/main": latestSHA,
		}},
	})
	if err != nil {
		t.Fatalf("PlanSHAs() error: %v", err)
	}

	// Simulate main changing this ref after the plan was captured.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), oldRef, changedMainRef, 1))
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	updated, err := ApplySHAPlan(configPath, "components/ray", plan)
	if err == nil || !strings.Contains(err.Error(), "stale SHA plan") {
		t.Fatalf("ApplySHAPlan() error = %v, want stale-plan error", err)
	}
	if updated {
		t.Fatal("ApplySHAPlan() reported an update for a stale plan")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Components["ray"].ODH.Ref; got != changedMainRef {
		t.Fatalf("ref after rejected apply = %q, want unchanged %q", got, changedMainRef)
	}
}
