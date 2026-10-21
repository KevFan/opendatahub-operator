package cli

import (
	"github.com/spf13/cobra"

	"github.com/opendatahub-io/opendatahub-operator/v2/cmd/manifest-tools/pkg/resolver"
)

func newResolveCommand(root *rootOptions) *cobra.Command {
	var manifestsDir string
	var relatedImagesConfigFile string
	var csvImportRegistries []string
	var component string
	var csvOnly bool

	cmd := &cobra.Command{
		Use:   "resolve-digests",
		Short: "Resolve image digests and update manifests-config.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := resolver.Resolve(cmd.Context(), resolver.Options{
				ConfigFile:              root.configFile,
				RelatedImagesConfigFile: relatedImagesConfigFile,
				ManifestsDir:            manifestsDir,
				CSVImportRegistries:     csvImportRegistries,
				Component:               component,
				CSVOnly:                 csvOnly,
			})
			return err
		},
	}

	cmd.Flags().StringVar(&manifestsDir, "manifests-dir", "opt/manifests", "Downloaded manifests directory")
	cmd.Flags().StringVar(&relatedImagesConfigFile, "related-images-config", "component-params-env.yaml", "Platform exceptions configuration")
	cmd.Flags().StringSliceVar(&csvImportRegistries, "csv-import-registries", []string{"quay.io/rhoai/"}, "Only import new CSV images from these registry prefixes (empty = all)")
	cmd.Flags().StringVar(&component, "component", "", "Resolve image overrides for one component key")
	cmd.Flags().BoolVar(&csvOnly, "csv-only", false, "Resolve Build-Config CSV image overrides only")

	return cmd
}
