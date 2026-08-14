package resources

import (
	"context"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
)

// mutateHashed runs a whole-document rule mutation and returns the hash
// used in the confirmation. Access rules and authentication mappings share
// the same identity model (content hash, first match).
func mutateHashed(
	ctx context.Context,
	mutate func(context.Context, func(map[string]any) (map[string]any, client.RuleConfirm, error)) error,
	apply func(map[string]any) (map[string]any, string, client.RuleConfirm, error),
) (string, error) {
	var hash string
	err := mutate(ctx, func(doc map[string]any) (map[string]any, client.RuleConfirm, error) {
		next, h, confirm, err := apply(doc)
		if err != nil {
			return nil, client.RuleConfirm{}, err
		}
		hash = h
		return next, confirm, nil
	})
	return hash, err
}
