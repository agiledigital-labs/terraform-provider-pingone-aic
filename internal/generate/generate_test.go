package generate

import "testing"

func TestSanitizeIdent(t *testing.T) {
	tests := map[string]string{
		"Get IP":                           "get_ip",
		"AIC-Rhino-Legacy-Probe":           "aic_rhino_legacy_probe",
		"_DealerMfa":                       "dealermfa",
		"OAuth2 Client Authorization Test": "oauth2_client_authorization_test",
		"2FA":                              "n_2fa",
	}
	for in, want := range tests {
		if got := sanitizeIdent(in); got != want {
			t.Fatalf("sanitizeIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDisplayConn(t *testing.T) {
	if displayConn("70e691a5-1e33-4ac3-a356-e7b6d60d92e0") != "success" {
		t.Fatal("success sentinel")
	}
	if displayConn("e301438c-0bd0-429c-ab0c-66126501069a") != "failure" {
		t.Fatal("failure sentinel")
	}
}

func TestHclIdentOrQuote(t *testing.T) {
	if hclIdentOrQuote("ok") != "ok" {
		t.Fatal("ok")
	}
	if hclIdentOrQuote("true") != "true" {
		t.Fatal("true is a valid ident in HCL map keys")
	}
	if hclIdentOrQuote("no-ip") != `"no-ip"` {
		t.Fatalf("got %s", hclIdentOrQuote("no-ip"))
	}
}
