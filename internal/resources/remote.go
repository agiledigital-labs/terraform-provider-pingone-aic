package resources

import (
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// remoteName is the wire name a name-keyed resource actually lives under.
// AM keys trees, OAuth2 clients, ESVs, managed types, IDM endpoints,
// schedules and internal roles by name and cannot rename, so once we have
// recorded an id or remote_name we must keep using it: resource_prefix is
// provider-level and never triggers replacement. Recomputing the name from
// the current prefix on update would PUT a second object and orphan the first.
//
// fallback is the create-time name (prefix.Apply or prefix.ApplyESV). Both
// helpers are already idempotent.
func remoteName(id, remote types.String, fallback func() string) string {
	for _, v := range []types.String{id, remote} {
		if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
			return v.ValueString()
		}
	}
	return fallback()
}

func applyRemote(id, remote types.String, pfx, logical string) string {
	return remoteName(id, remote, func() string { return prefix.Apply(pfx, logical) })
}

func applyESVRemote(id, remote types.String, pfx, logical string) string {
	return remoteName(id, remote, func() string { return prefix.ApplyESV(pfx, logical) })
}

func persistedRemote(id, remote types.String) string {
	return remoteName(id, remote, func() string { return "" })
}
