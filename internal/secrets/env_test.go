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

func TestToEnvVars_preserves_case(t *testing.T) {
	tree := map[string]*Node{
		"Mixed_Key1":  {Value: "value1"}, // mixed: preserved as-is
		"UPPER_KEY2":  {Value: "value2"}, // uppercase: preserved
		"my_lower_key": {Value: "value3"}, // lowercase: preserved
	}
	env := ToEnvVars(tree)
	if env["Mixed_Key1"] != "value1" {
		t.Errorf("expected Mixed_Key1=value1, got %v", env)
	}
	if env["UPPER_KEY2"] != "value2" {
		t.Errorf("expected UPPER_KEY2=value2, got %v", env)
	}
	if env["my_lower_key"] != "value3" {
		t.Errorf("expected my_lower_key=value3, got %v", env)
	}
}

func TestToFlatEnvEntries_preserves_case(t *testing.T) {
	tree := map[string]*Node{
		"app": {
			Children: map[string]*Node{
				"Mixed_Key1": {Value: "value1"}, // mixed case nested: preserved
				"UPPER_KEY2": {Value: "value2"}, // uppercase nested: preserved
				"lower_key3": {Value: "value3"}, // lowercase nested: preserved
			},
		},
	}
	got, err := ToFlatEnvEntries(tree, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := got["Mixed_Key1"]; !ok || v.Value != "value1" {
		t.Errorf("expected Mixed_Key1=value1, got %v", got)
	}
	if v, ok := got["UPPER_KEY2"]; !ok || v.Value != "value2" {
		t.Errorf("expected UPPER_KEY2=value2, got %v", got)
	}
	if v, ok := got["lower_key3"]; !ok || v.Value != "value3" {
		t.Errorf("expected lower_key3=value3, got %v", got)
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

func TestToFlatEnvEntries_case_collision(t *testing.T) {
	tree := map[string]*Node{
		"DATABASE_URL": {Value: "postgres://upper"},
		"database_url": {Value: "postgres://lower"},
	}
	_, err := ToFlatEnvEntries(tree, "")
	if err == nil {
		t.Fatal("expected error for case-insensitive collision, got nil")
	}
	ce, ok := err.(*EnvConflictError)
	if !ok {
		t.Fatalf("expected EnvConflictError, got %T", err)
	}
	if len(ce.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(ce.Conflicts))
	}
	if !ce.Conflicts[0].CaseCollision {
		t.Error("expected CaseCollision=true")
	}
}

func TestToFlatEnvEntries_case_collision_nested(t *testing.T) {
	tree := map[string]*Node{
		"app": {
			Children: map[string]*Node{
				"DATABASE_URL": {Value: "postgres://nested"},
			},
		},
		"database_url": {Value: "postgres://top"},
	}
	_, err := ToFlatEnvEntries(tree, "")
	if err == nil {
		t.Fatal("expected error for case-insensitive collision, got nil")
	}
	ce, ok := err.(*EnvConflictError)
	if !ok {
		t.Fatalf("expected EnvConflictError, got %T", err)
	}
	if len(ce.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(ce.Conflicts))
	}
	if !ce.Conflicts[0].CaseCollision {
		t.Error("expected CaseCollision=true")
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

func TestToFlatEnvEntries_deeper_path_shadows_shallower(t *testing.T) {
	// arrange: app.service_account_json and app.other.service_account_json
	// deeper (app.other.service_account_json) must win silently — no collision error
	tree := map[string]*Node{
		"app": {Children: map[string]*Node{
			"service_account_json": {Value: "shallow"},
			"other": {Children: map[string]*Node{
				"service_account_json": {Value: "deep"},
			}},
		}},
	}

	// act
	got, err := ToFlatEnvEntries(tree, "")

	// assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got["service_account_json"].Value != "deep" {
		t.Errorf("expected deeper value to win, got %q", got["service_account_json"].Value)
	}
}

func TestToFlatEnvEntries_deeper_path_shadows_regular_secret(t *testing.T) {
	// arrange: app.database_url and app.production.database_url — same leaf name, different depth
	tree := map[string]*Node{
		"app": {Children: map[string]*Node{
			"database_url": {Value: "base"},
			"production": {Children: map[string]*Node{
				"database_url": {Value: "prod"},
			}},
		}},
	}

	// act
	got, err := ToFlatEnvEntries(tree, "")

	// assert
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got["database_url"].Value != "prod" {
		t.Errorf("expected deeper value to win, got %q", got["database_url"].Value)
	}
}

func TestToFlatEnvEntries_same_depth_unrelated_is_collision(t *testing.T) {
	// arrange: app.one.token and app.two.token — same depth, unrelated paths → collision
	tree := map[string]*Node{
		"app": {Children: map[string]*Node{
			"one": {Children: map[string]*Node{
				"token": {Value: "aaa"},
			}},
			"two": {Children: map[string]*Node{
				"token": {Value: "bbb"},
			}},
		}},
	}

	// act
	_, err := ToFlatEnvEntries(tree, "")

	// assert
	if err == nil {
		t.Fatal("expected collision error for same-depth unrelated paths")
	}
	ce, ok := err.(*EnvConflictError)
	if !ok {
		t.Fatalf("expected EnvConflictError, got %T", err)
	}
	if len(ce.Conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(ce.Conflicts))
	}
}

func TestToFlatEnvEntriesScoped_overlay(t *testing.T) {
	tree := mkTree()
	roots, _ := ResolveScopes(tree, []Scope{{SecretPath: "group.sub1"}})
	got, err := ToFlatEnvEntriesScoped(roots)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["key1"]; !ok {
		t.Error("expected key1 from vault1.group.sub1")
	}
	if _, ok := got["key2"]; !ok {
		t.Error("expected key2 from vault2.group.sub1")
	}
	if len(got) != 2 {
		t.Errorf("expected exactly 2 leaves (no sub2 leak), got %v", got)
	}
}

func TestToFlatEnvEntriesScoped_collision(t *testing.T) {
	leaf := func(v string) *Node { return &Node{Value: v} }
	tree := map[string]*Node{
		"vault1": {Children: map[string]*Node{"group": {Children: map[string]*Node{
			"sub1": {Children: map[string]*Node{"key1": leaf("value1")}}}}}},
		"vault2": {Children: map[string]*Node{"group": {Children: map[string]*Node{
			"sub1": {Children: map[string]*Node{"key1": leaf("value2")}}}}}},
	}
	roots, _ := ResolveScopes(tree, []Scope{{SecretPath: "group.sub1"}})
	if _, err := ToFlatEnvEntriesScoped(roots); err == nil {
		t.Fatal("expected collision on key1 across two vaults")
	}
}

func TestToFlatEnvEntriesScoped_nested_no_dataloss(t *testing.T) {
	leaf := func(v string) *Node { return &Node{Value: v} }
	// dois roots com sub-mapa intermediário de mesmo nome "db" mas folhas distintas
	tree := map[string]*Node{
		"vault1": {Children: map[string]*Node{"group": {Children: map[string]*Node{
			"sub1": {Children: map[string]*Node{
				"db": {Children: map[string]*Node{"HOST_C": leaf("c")}}}}}}}},
		"vault2": {Children: map[string]*Node{"group": {Children: map[string]*Node{
			"sub1": {Children: map[string]*Node{
				"db": {Children: map[string]*Node{"HOST_T": leaf("t")}}}}}}}},
	}
	roots, _ := ResolveScopes(tree, []Scope{{SecretPath: "group.sub1"}})
	got, err := ToFlatEnvEntriesScoped(roots)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["HOST_C"]; !ok {
		t.Error("HOST_C lost — shallow merge dropped a nested leaf")
	}
	if _, ok := got["HOST_T"]; !ok {
		t.Error("HOST_T lost — shallow merge dropped a nested leaf")
	}
}
