package cmd

import (
	"reflect"
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

func subvaultTree() map[string]*secrets.Node {
	return map[string]*secrets.Node{
		"myfish": {Children: map[string]*secrets.Node{
			"secrets": {Children: map[string]*secrets.Node{
				"dev": {Children: map[string]*secrets.Node{
					"DATABASE_URL": {Value: "d"},
				}},
			}},
		}},
		"myfish::restricted": {Children: map[string]*secrets.Node{
			"secrets": {Children: map[string]*secrets.Node{
				"production": {Children: map[string]*secrets.Node{
					"DATABASE_URL": {Value: "p"},
				}},
			}},
		}},
	}
}

func TestCompleteScopes_subvault_cases(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"myfish::", []string{"myfish::restricted:"}},
		{"", []string{"myfish:", "myfish::restricted:"}},
		{"myfish:", []string{"myfish:secrets", "myfish:secrets.dev", "myfish:secrets.dev.DATABASE_URL"}},
		{"myfish::restricted:sec", []string{"myfish::restricted:secrets", "myfish::restricted:secrets.production", "myfish::restricted:secrets.production.DATABASE_URL"}},
	}
	for _, c := range cases {
		// act
		got, _ := completeScopes(subvaultTree(), c.in)
		// assert
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("completeScopes(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
