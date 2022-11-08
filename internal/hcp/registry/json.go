package registry

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	sdkpacker "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer/packer"
)

// JSONMetadataRegistry is a HCP handler made to process legacy JSON templates
type JSONMetadataRegistry struct {
	configuration *packer.Core
	bucket        *Bucket
}

func NewJSONMetadataRegistry(config *packer.Core) (*JSONMetadataRegistry, hcl.Diagnostics) {
	bucket, diags := createConfiguredBucket(
		filepath.Dir(config.Template.Path),
		withPackerEnvConfiguration,
	)

	if diags.HasErrors() {
		return nil, diags
	}

	// Get all builds slated within config ignoring any only or exclude flags.
	for _, b := range config.BuildNames([]string{}, []string{}) {
		hcpName, err := config.HCPName(b)
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "No HCP name available",
				Detail: fmt.Sprintf("Packer failed to get an HCP-compatible "+
					"build name for builder %q: %s.\nThis is an "+
					"internal error, which stems from a bug in Packer. "+
					"Please open a Github issue for the team to look at.",
					b, err),
			})
			continue
		}

		bucket.RegisterBuildForComponent(hcpName)
	}

	return &JSONMetadataRegistry{
		configuration: config,
		bucket:        bucket,
	}, nil
}

// PopulateIteration creates the metadata on HCP for a build
func (h *JSONMetadataRegistry) PopulateIteration(ctx context.Context) error {
	err := h.bucket.Validate()
	if err != nil {
		return err
	}
	err = h.bucket.Initialize(ctx)
	if err != nil {
		return err
	}

	err = h.bucket.populateIteration(ctx)
	if err != nil {
		return err
	}

	return nil
}

// StartBuild is invoked when one build for the configuration is starting to be processed
func (h *JSONMetadataRegistry) StartBuild(ctx context.Context, buildName string) error {
	build, err := h.configuration.HCPName(buildName)
	if err != nil {
		return fmt.Errorf("failed to get build %q: %s", buildName, err)
	}
	return h.bucket.startBuild(ctx, build)
}

// CompleteBuild is invoked when one build for the configuration has finished
func (h *JSONMetadataRegistry) CompleteBuild(
	ctx context.Context,
	buildName string,
	artifacts []sdkpacker.Artifact,
	buildErr error,
) ([]sdkpacker.Artifact, error) {
	build, err := h.configuration.HCPName(buildName)
	if err != nil {
		return nil, fmt.Errorf("failed to get build %q: %s", buildName, err)
	}
	return h.bucket.completeBuild(ctx, build, artifacts, buildErr)
}
