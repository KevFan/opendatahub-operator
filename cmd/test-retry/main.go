package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/opendatahub-io/opendatahub-operator/v2/cmd/test-retry/pkg/cli"
	"github.com/opendatahub-io/opendatahub-operator/v2/cmd/test-retry/pkg/types"
)

func main() {
	rootCmd := cli.NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		// Exit code 3 = every remaining failure is infrastructure-classified;
		// lets CI requeue on a fresh cluster instead of gating the PR.
		var infra *types.InfraOnlyFailureError
		if errors.As(err, &infra) {
			os.Exit(3)
		}
		os.Exit(1)
	}
}
