package secrets

import "testing"

func TestParseScope(t *testing.T) {
	cases := []struct {
		in         string
		wantVault  string
		wantSecret string
	}{
		{"commons:infra.staging", "commons", "infra.staging"},
		{"infra.staging", "", "infra.staging"},
		{":infra.staging", "", "infra.staging"},
		{"", "", ""},
		{"commons:", "commons", ""},
		{"infra.staging:weird", "", "infra.staging:weird"},
	}
	for _, c := range cases {
		got := ParseScope(c.in)
		if got.Vault != c.wantVault || got.SecretPath != c.wantSecret {
			t.Errorf("ParseScope(%q) = {%q,%q}, want {%q,%q}",
				c.in, got.Vault, got.SecretPath, c.wantVault, c.wantSecret)
		}
	}
}
