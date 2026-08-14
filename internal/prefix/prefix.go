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

// ESV IDs must match ^esv-[a-z0-9_-]{1,124}$. The provider prefix
// (default Terraform_) is lowercased, underscores become hyphens, and
// the result is inserted after esv-: esv-test11 → esv-terraform-test11.
func SanitizeESV(prefix string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(prefix) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case r == '_' || r == '-':
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return b.String()
}

func ApplyESV(pfx, id string) string {
	p := SanitizeESV(pfx)
	if p == "" || id == "" {
		return id
	}
	rest, ok := strings.CutPrefix(id, "esv-")
	if !ok {
		return id
	}
	if strings.HasPrefix(rest, p) {
		return id
	}
	return "esv-" + p + rest
}

func StripESV(pfx, id string) string {
	p := SanitizeESV(pfx)
	if p == "" {
		return id
	}
	rest, ok := strings.CutPrefix(id, "esv-")
	if !ok || !strings.HasPrefix(rest, p) {
		return id
	}
	return "esv-" + strings.TrimPrefix(rest, p)
}
