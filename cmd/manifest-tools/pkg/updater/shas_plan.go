package updater

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/opendatahub-io/opendatahub-operator/v2/cmd/manifest-tools/pkg/config"
)

// SHAPlan is a snapshot of manifest refs that can be applied as independent
// pull requests. Failed units are kept in the plan so one upstream failure
// does not prevent the other units from being processed.
type SHAPlan struct {
	Units    []SHAPlanUnit    `json:"units"`
	Failures []SHAPlanFailure `json:"failures"`
}

// SHAPlanUnit groups changes that need to be applied and validated together.
type SHAPlanUnit struct {
	ID        string      `json:"id"`
	Slug      string      `json:"slug"`
	Section   string      `json:"section"`
	Component string      `json:"component,omitempty"`
	Changes   []SHAChange `json:"changes"`
}

// SHAChange records both refs so ApplySHAPlan can reject a stale plan.
type SHAChange struct {
	Section   string `json:"section"`
	Component string `json:"component,omitempty"`
	Platform  string `json:"platform"`
	OldRef    string `json:"oldRef"`
	NewRef    string `json:"newRef"`
}

// SHAPlanFailure identifies an upstream lookup failure within an update unit.
type SHAPlanFailure struct {
	Unit      string `json:"unit"`
	Component string `json:"component,omitempty"`
	Platform  string `json:"platform"`
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Error     string `json:"error"`
}

// PlanSHAs snapshots the latest refs for each logical update unit. A component
// is one unit across both platforms. CCM charts are grouped because their RBAC
// output is generated together; Build-Config is a shared unit.
func PlanSHAs(ctx context.Context, opts SHAsOptions) (SHAPlan, error) {
	cfg, err := config.Load(opts.ConfigFile)
	if err != nil {
		return SHAPlan{}, err
	}

	plan := SHAPlan{Failures: []SHAPlanFailure{}}
	lookupCache := map[string]lookupResult{}

	for _, sec := range allSections(cfg) {
		ccmUnit := SHAPlanUnit{ID: "ccmCharts", Slug: "ccm-charts", Section: "ccmCharts"}
		var ccmFailures []SHAPlanFailure

		componentNames := make([]string, 0, len(sec.components))
		for name := range sec.components {
			componentNames = append(componentNames, name)
		}
		sort.Strings(componentNames)

		for _, componentName := range componentNames {
			comp := sec.components[componentName]
			unit := SHAPlanUnit{
				ID:        shaUnitID(sec.name, componentName),
				Slug:      shaUnitSlug(sec.name, componentName),
				Section:   sec.name,
				Component: componentName,
			}
			var unitFailures []SHAPlanFailure

			for _, platform := range []string{"odh", "rhoai"} {
				pr := comp.PlatformRepo(platform)
				if pr == nil || pr.Ref == "" {
					continue
				}

				currentSHA := config.ExtractSHA(pr.Ref)
				branch := config.ExtractBranch(pr.Ref)
				if currentSHA == "" {
					continue
				}

				ownerRepo := strings.SplitN(pr.Repo, "/", 2)
				if len(ownerRepo) != 2 {
					continue
				}

				latestSHA, lookupErr := lookupLatestSHA(ctx, opts.GH, ownerRepo[0], ownerRepo[1], branch, lookupCache)
				if lookupErr != nil {
					unitFailures = append(unitFailures, SHAPlanFailure{
						Unit:      unit.ID,
						Component: componentName,
						Platform:  platform,
						Repo:      pr.Repo,
						Branch:    branch,
						Error:     lookupErr.Error(),
					})
					continue
				}

				if latestSHA == currentSHA {
					continue
				}

				unit.Changes = append(unit.Changes, SHAChange{
					Section:   sec.name,
					Component: componentName,
					Platform:  platform,
					OldRef:    pr.Ref,
					NewRef:    fmt.Sprintf("%s@%s", branch, latestSHA),
				})
			}

			if len(unitFailures) > 0 {
				if sec.name == "ccmCharts" {
					ccmFailures = append(ccmFailures, unitFailures...)
				} else {
					plan.Failures = append(plan.Failures, unitFailures...)
				}
				continue
			}

			if len(unit.Changes) == 0 {
				continue
			}
			if sec.name == "ccmCharts" {
				ccmUnit.Changes = append(ccmUnit.Changes, unit.Changes...)
				continue
			}
			plan.Units = append(plan.Units, unit)
		}

		if sec.name == "ccmCharts" {
			if len(ccmFailures) > 0 {
				plan.Failures = append(plan.Failures, ccmFailures...)
			} else if len(ccmUnit.Changes) > 0 {
				plan.Units = append(plan.Units, ccmUnit)
			}
		}
	}

	buildConfigUnit, failures := planBuildConfigSHAs(ctx, cfg, opts.GH, lookupCache)
	plan.Failures = append(plan.Failures, failures...)
	if len(buildConfigUnit.Changes) > 0 && len(failures) == 0 {
		plan.Units = append(plan.Units, buildConfigUnit)
	}

	sort.Slice(plan.Units, func(i, j int) bool { return plan.Units[i].ID < plan.Units[j].ID })
	sort.Slice(plan.Failures, func(i, j int) bool {
		if plan.Failures[i].Unit == plan.Failures[j].Unit {
			return plan.Failures[i].Platform < plan.Failures[j].Platform
		}
		return plan.Failures[i].Unit < plan.Failures[j].Unit
	})

	return plan, nil
}

type lookupResult struct {
	sha string
	err error
}

func lookupLatestSHA(ctx context.Context, gh GitHubClient, owner, repo, branch string, cache map[string]lookupResult) (string, error) {
	key := owner + "/" + repo + "@" + branch
	if result, ok := cache[key]; ok {
		return result.sha, result.err
	}
	sha, err := gh.GetLatestCommitSHA(ctx, owner, repo, branch)
	cache[key] = lookupResult{sha: sha, err: err}
	return sha, err
}

func planBuildConfigSHAs(ctx context.Context, cfg *config.ManifestsConfig, gh GitHubClient, cache map[string]lookupResult) (SHAPlanUnit, []SHAPlanFailure) {
	unit := SHAPlanUnit{ID: "buildConfig", Slug: "build-config", Section: "buildConfig"}
	var failures []SHAPlanFailure
	for _, platform := range []string{"odh", "rhoai"} {
		repo := cfg.BuildConfig.PlatformRepo(platform)
		if repo == nil || repo.Ref == "" {
			continue
		}

		currentSHA := config.ExtractSHA(repo.Ref)
		branch := config.ExtractBranch(repo.Ref)
		if currentSHA == "" {
			continue
		}

		ownerRepo := strings.SplitN(repo.Repo, "/", 2)
		if len(ownerRepo) != 2 {
			continue
		}

		latestSHA, err := lookupLatestSHA(ctx, gh, ownerRepo[0], ownerRepo[1], branch, cache)
		if err != nil {
			failures = append(failures, SHAPlanFailure{
				Unit:     unit.ID,
				Platform: platform,
				Repo:     repo.Repo,
				Branch:   branch,
				Error:    err.Error(),
			})
			continue
		}
		if latestSHA == currentSHA {
			continue
		}

		unit.Changes = append(unit.Changes, SHAChange{
			Section:  "buildConfig",
			Platform: platform,
			OldRef:   repo.Ref,
			NewRef:   fmt.Sprintf("%s@%s", branch, latestSHA),
		})
	}
	return unit, failures
}

func shaUnitID(section, component string) string {
	if section == "ccmCharts" {
		return "ccmCharts"
	}
	return section + "/" + component
}

func shaUnitSlug(section, component string) string {
	if section == "ccmCharts" {
		return "ccm-charts"
	}
	return strings.Trim(strings.ReplaceAll(section+"-"+component, "/", "-"), "-")
}

// ApplySHAPlan applies one unit from a previously captured plan. It checks the
// old refs before editing so a plan cannot silently overwrite a newer main.
func ApplySHAPlan(configFile, unitID string, plan SHAPlan) (bool, error) {
	var selected *SHAPlanUnit
	for i := range plan.Units {
		if plan.Units[i].ID == unitID {
			selected = &plan.Units[i]
			break
		}
	}
	if selected == nil {
		return false, fmt.Errorf("SHA plan has no update unit %q", unitID)
	}

	cfg, err := config.Load(configFile)
	if err != nil {
		return false, err
	}
	nodeDoc, err := config.LoadNode(configFile)
	if err != nil {
		return false, err
	}

	for _, change := range selected.Changes {
		currentRef, err := currentSHARef(cfg, change)
		if err != nil {
			return false, err
		}
		if currentRef != change.OldRef {
			return false, fmt.Errorf("stale SHA plan for %s/%s/%s: expected %q, found %q", change.Section, change.Component, change.Platform, change.OldRef, currentRef)
		}

		if change.Section == "buildConfig" {
			err = nodeDoc.SetBuildConfigRef(change.Platform, change.NewRef)
		} else {
			err = nodeDoc.SetComponentRef(change.Section, change.Component, change.Platform, change.NewRef)
		}
		if err != nil {
			return false, fmt.Errorf("applying SHA update for %s/%s/%s: %w", change.Section, change.Component, change.Platform, err)
		}
	}

	if err := nodeDoc.Save(configFile); err != nil {
		return false, fmt.Errorf("saving config: %w", err)
	}
	return len(selected.Changes) > 0, nil
}

func currentSHARef(cfg *config.ManifestsConfig, change SHAChange) (string, error) {
	if change.Section == "buildConfig" {
		repo := cfg.BuildConfig.PlatformRepo(change.Platform)
		if repo == nil {
			return "", fmt.Errorf("buildConfig.%s not found", change.Platform)
		}
		return repo.Ref, nil
	}

	for _, sec := range allSections(cfg) {
		if sec.name != change.Section {
			continue
		}
		component, ok := sec.components[change.Component]
		if !ok {
			return "", fmt.Errorf("%s.%s not found", change.Section, change.Component)
		}
		repo := component.PlatformRepo(change.Platform)
		if repo == nil {
			return "", fmt.Errorf("%s.%s.%s not found", change.Section, change.Component, change.Platform)
		}
		return repo.Ref, nil
	}
	return "", fmt.Errorf("unknown SHA plan section %q", change.Section)
}
