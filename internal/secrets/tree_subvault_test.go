package secrets

import "testing"

func defaultRootTree() *Tree {
	return NewTree(map[string]interface{}{
		"myfish::default": map[string]interface{}{
			"secrets": map[string]interface{}{
				"dev": map[string]interface{}{"X": "1"},
			},
		},
	})
}

func TestTree_Kind_resolves_default_root_alias(t *testing.T) {
	// arrange
	tr := defaultRootTree()
	// act + assert
	if k := tr.Kind("myfish.secrets.dev.X"); k != KindLeaf {
		t.Errorf("leaf: got %v", k)
	}
	if k := tr.Kind("myfish.secrets.dev"); k != KindGroup {
		t.Errorf("group: got %v", k)
	}
	if k := tr.Kind("myfish.secrets.dev.Y"); k != KindAbsent {
		t.Errorf("absent: got %v", k)
	}
}

func TestTree_Set_preserves_default_root(t *testing.T) {
	// arrange
	tr := defaultRootTree()
	// act
	tr.Set("myfish.secrets.dev.Y", "2")
	// assert
	root := tr.Root()
	if _, ok := root["myfish"]; ok {
		t.Errorf("Set created a second root: %v", root)
	}
	dev := root["myfish::default"].(map[string]interface{})["secrets"].(map[string]interface{})["dev"].(map[string]interface{})
	if dev["X"] != "1" || dev["Y"] != "2" {
		t.Errorf("got %v", dev)
	}
}

func TestTree_Unset_preserves_default_root(t *testing.T) {
	// arrange
	tr := defaultRootTree()
	// act
	tr.Unset("myfish.secrets.dev.X")
	// assert
	if k := tr.Kind("myfish.secrets.dev.X"); k != KindAbsent {
		t.Errorf("expected absent, got %v", k)
	}
	if _, ok := tr.Root()["myfish"]; ok {
		t.Errorf("Unset created a second root: %v", tr.Root())
	}
}

func TestTree_short_root_still_works(t *testing.T) {
	// arrange
	tr := NewTree(map[string]interface{}{
		"myfish": map[string]interface{}{"secrets": map[string]interface{}{"X": "1"}},
	})
	// act
	tr.Set("myfish.secrets.Y", "2")
	// assert
	if k := tr.Kind("myfish.secrets.Y"); k != KindLeaf {
		t.Errorf("got %v", k)
	}
	if _, ok := tr.Root()["myfish::default"]; ok {
		t.Errorf("unexpected alias root: %v", tr.Root())
	}
}
