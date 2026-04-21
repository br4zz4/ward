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

func TestToFlatEnvEntries_basic(t *testing.T) {
	tree := map[string]*Node{
		"myapp": {
			Children: map[string]*Node{
				"database_url": {Value: "postgres://localhost/myapp"},
				"redis_url":    {Value: "redis://localhost:6379"},
			},
		},
	}

	got, err := ToFlatEnvEntries(tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v, ok := got["DATABASE_URL"]; !ok || v.Value != "postgres://localhost/myapp" {
		t.Errorf("expected DATABASE_URL=postgres://localhost/myapp, got %v", got)
	}
	if v, ok := got["REDIS_URL"]; !ok || v.Value != "redis://localhost:6379" {
		t.Errorf("expected REDIS_URL=redis://localhost:6379, got %v", got)
	}
	if _, ok := got["MYAPP_DATABASE_URL"]; ok {
		t.Error("should not have prefixed key MYAPP_DATABASE_URL")
	}
}

func TestToFlatEnvEntries_nested(t *testing.T) {
	tree := map[string]*Node{
		"app": {
			Children: map[string]*Node{
				"db": {
					Children: map[string]*Node{
						"url":  {Value: "postgres://x"},
						"port": {Value: "5432"},
					},
				},
				"token": {Value: "abc"},
			},
		},
	}

	got, err := ToFlatEnvEntries(tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got["URL"]; !ok {
		t.Error("expected URL")
	}
	if _, ok := got["PORT"]; !ok {
		t.Error("expected PORT")
	}
	if _, ok := got["TOKEN"]; !ok {
		t.Error("expected TOKEN")
	}
	if _, ok := got["APP_DB_URL"]; ok {
		t.Error("should not have prefixed key APP_DB_URL")
	}
}

func TestToFlatEnvEntries_empty(t *testing.T) {
	got, err := ToFlatEnvEntries(map[string]*Node{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map")
	}
}
