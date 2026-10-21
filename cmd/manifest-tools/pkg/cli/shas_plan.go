package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/opendatahub-io/opendatahub-operator/v2/cmd/manifest-tools/pkg/github"
	"github.com/opendatahub-io/opendatahub-operator/v2/cmd/manifest-tools/pkg/updater"
)

type shaPlanMatrix struct {
	Include []shaPlanMatrixUnit `json:"include"`
}

type shaPlanMatrixUnit struct {
	Unit      string `json:"unit"`
	Slug      string `json:"slug"`
	Section   string `json:"section"`
	Component string `json:"component,omitempty"`
}

func newPlanSHAsCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan-shas",
		Short: "Create a snapshot plan for independent manifest SHA updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			gh, err := github.NewClient()
			if err != nil {
				return err
			}

			plan, err := updater.PlanSHAs(cmd.Context(), updater.SHAsOptions{
				ConfigFile: root.configFile,
				GH:         gh,
			})
			if err != nil {
				return err
			}

			planJSON, err := json.Marshal(plan)
			if err != nil {
				return fmt.Errorf("encoding SHA plan: %w", err)
			}

			matrix := shaPlanMatrix{}
			for _, unit := range plan.Units {
				matrix.Include = append(matrix.Include, shaPlanMatrixUnit{
					Unit:      unit.ID,
					Slug:      unit.Slug,
					Section:   unit.Section,
					Component: unit.Component,
				})
			}
			if len(matrix.Include) == 0 {
				// GitHub Actions requires at least one matrix entry. The update job
				// is skipped when has-updates is false.
				matrix.Include = append(matrix.Include, shaPlanMatrixUnit{Unit: "__no_updates__"})
			}
			matrixJSON, err := json.Marshal(matrix)
			if err != nil {
				return fmt.Errorf("encoding SHA plan matrix: %w", err)
			}
			failuresJSON, err := json.Marshal(plan.Failures)
			if err != nil {
				return fmt.Errorf("encoding SHA plan failures: %w", err)
			}

			writeGitHubOutput("plan", string(planJSON))
			writeGitHubOutput("matrix", string(matrixJSON))
			writeGitHubOutput("has_updates", fmt.Sprintf("%t", len(plan.Units) > 0))
			writeGitHubOutput("failures", string(failuresJSON))
			if os.Getenv("GITHUB_OUTPUT") == "" {
				fmt.Println(string(planJSON))
			}
			return nil
		},
	}
	return cmd
}

func newApplySHAPlanCommand(root *rootOptions) *cobra.Command {
	var planFile string
	var unitID string
	cmd := &cobra.Command{
		Use:   "apply-sha-plan",
		Short: "Apply one unit from a previously captured manifest SHA plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(planFile)
			if err != nil {
				return fmt.Errorf("reading SHA plan: %w", err)
			}
			var plan updater.SHAPlan
			if err := json.Unmarshal(data, &plan); err != nil {
				return fmt.Errorf("parsing SHA plan: %w", err)
			}

			updated, err := updater.ApplySHAPlan(root.configFile, unitID, plan)
			if err != nil {
				return err
			}
			writeGitHubOutput("updates-needed", fmt.Sprintf("%t", updated))
			return nil
		},
	}
	cmd.Flags().StringVar(&planFile, "plan-file", "", "Path to the JSON SHA plan")
	cmd.Flags().StringVar(&unitID, "unit", "", "Update unit ID to apply")
	_ = cmd.MarkFlagRequired("plan-file")
	_ = cmd.MarkFlagRequired("unit")
	return cmd
}
