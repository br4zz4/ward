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
		"company_sectors_one_staging_database_url": "postgres://staging",
		"company_sectors_one_staging_redis_url":    "redis://staging",
	}
	for k, want := range cases {
		if got := env[k]; got != want {
			t.Errorf("%s: got %q, want %q", k, got, want)
		}
	}
}

func TestToEnvVars_nested_preserves_case(t *testing.T) {
	tree := map[string]*Node{
		"myApp": {
			Children: map[string]*Node{
				"dbURL": {Value: "postgres://x"},
			},
		},
	}
	env := ToEnvVars(tree)
	if _, ok := env["myApp_dbURL"]; !ok {
		t.Errorf("expected myApp_dbURL, got keys: %v", env)
	}
}

func TestToEnvVars_toplevel_preserves_case(t *testing.T) {
	tree := map[string]*Node{
		"TF_VAR_aws_region":              {Value: "us-east-1"},
		"AWS_MANAGEMENT_ACCESS_KEY_ID":   {Value: "AKIA123"},
		"my_lower_key":                   {Value: "value"},
	}
	env := ToEnvVars(tree)
	if env["TF_VAR_aws_region"] != "us-east-1" {
		t.Errorf("expected TF_VAR_aws_region=us-east-1, got %v", env)
	}
	if env["AWS_MANAGEMENT_ACCESS_KEY_ID"] != "AKIA123" {
		t.Errorf("expected AWS_MANAGEMENT_ACCESS_KEY_ID=AKIA123, got %v", env)
	}
	if env["my_lower_key"] != "value" {
		t.Errorf("expected my_lower_key=value, got %v", env)
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

	got, err := ToFlatEnvEntries(tree, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v, ok := got["database_url"]; !ok || v.Value != "postgres://localhost/myapp" {
		t.Errorf("expected database_url=postgres://localhost/myapp, got %v", got)
	}
	if v, ok := got["redis_url"]; !ok || v.Value != "redis://localhost:6379" {
		t.Errorf("expected redis_url=redis://localhost:6379, got %v", got)
	}
	if _, ok := got["myapp_database_url"]; ok {
		t.Error("should not have prefixed key myapp_database_url")
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

	got, err := ToFlatEnvEntries(tree, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got["url"]; !ok {
		t.Error("expected url")
	}
	if _, ok := got["port"]; !ok {
		t.Error("expected port")
	}
	if _, ok := got["token"]; !ok {
		t.Error("expected token")
	}
	if _, ok := got["app_db_url"]; ok {
		t.Error("should not have prefixed key app_db_url")
	}
}

func TestToFlatEnvEntries_empty(t *testing.T) {
	got, err := ToFlatEnvEntries(map[string]*Node{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map")
	}
}
