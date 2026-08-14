// Package prefix applies the provider-level resource_prefix to AIC names.
// Terraform config always uses the unprefixed logical name; the prefix is
// added on the wire so terraform-managed copies do not collide with the
// originals they were generated from.
package prefix

import "strings"

func Apply(prefix, name string) string {
	if prefix == "" || name == "" {
		return name
	}
	if strings.HasPrefix(name, prefix) {
		return name
	}
	return prefix + name
}

func Strip(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return strings.TrimPrefix(name, prefix)
}

func Has(prefix, name string) bool {
	return prefix != "" && strings.HasPrefix(name, prefix)
}
