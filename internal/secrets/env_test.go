package secrets

import (
	"testing"
)

func TestToEnvVars_basic(t *testing.T) {
	tree := map[string]*Node{
		"company": {
			Children: map[string]*Node{
				"sectors": {
					Children: map[string]*Node{
						"one": {
							Children: map[string]*Node{
								"staging": {
									Children: map[string]*Node{
										"database_url": {Value: "postgres://staging"},
										"redis_url":    {Value: "redis://staging"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	env := ToEnvVars(tree)

	cases := map[string]string{
		"COMPANY_SECTORS_ONE_STAGING_DATABASE_URL": "postgres://staging",
		"COMPANY_SECTORS_ONE_STAGING_REDIS_URL":    "redis://staging",
	}
	for k, want := range cases {
		if got := env[k]; got != want {
			t.Errorf("%s: got %q, want %q", k, got, want)
		}
	}
}

func TestToEnvVars_uppercase(t *testing.T) {
	tree := map[string]*Node{
		"myApp": {
			Children: map[string]*Node{
				"dbURL": {Value: "postgres://x"},
			},
		},
	}
	env := ToEnvVars(tree)
	if _, ok := env["MYAPP_DBURL"]; !ok {
		t.Errorf("expected MYAPP_DBURL, got keys: %v", env)
	}
}

func TestToEnvVars_empty(t *testing.T) {
	env := ToEnvVars(map[string]*Node{})
	if len(env) != 0 {
		t.Errorf("expected empty map")
	}
}
