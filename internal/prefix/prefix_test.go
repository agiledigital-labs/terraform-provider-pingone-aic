package prefix

import "testing"

func TestApply(t *testing.T) {
	tests := []struct {
		prefix, name, want string
	}{
		{"Terraform_", "GetIP", "Terraform_GetIP"},
		{"Terraform_", "Terraform_GetIP", "Terraform_GetIP"},
		{"", "GetIP", "GetIP"},
		{"Terraform_", "", ""},
	}
	for _, tt := range tests {
		if got := Apply(tt.prefix, tt.name); got != tt.want {
			t.Fatalf("Apply(%q, %q) = %q, want %q", tt.prefix, tt.name, got, tt.want)
		}
	}
}

func TestStrip(t *testing.T) {
	if got := Strip("Terraform_", "Terraform_GetIP"); got != "GetIP" {
		t.Fatalf("Strip = %q", got)
	}
	if got := Strip("Terraform_", "GetIP"); got != "GetIP" {
		t.Fatalf("Strip unprefixed = %q", got)
	}
}
