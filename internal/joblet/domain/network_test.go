package domain

import "testing"

func TestIsValidNetworkName(t *testing.T) {
	valid := []string{"a", "app", "app-net", "app_net", "backend1", "n123", "MyNet"}
	for _, name := range valid {
		if !IsValidNetworkName(name) {
			t.Errorf("expected %q to be a valid network name", name)
		}
	}

	invalid := []string{
		"",                       // empty
		"-net",                   // leading separator
		"net-",                   // trailing separator
		"_net",                   // leading underscore
		"a/b",                    // slash
		"../etc",                 // traversal
		"a b",                    // space
		"a;b",                    // shell metacharacter
		"net$",                   // symbol
		string(make([]byte, 64)), // too long / non-alnum
	}
	for _, name := range invalid {
		if IsValidNetworkName(name) {
			t.Errorf("expected %q to be an invalid network name", name)
		}
	}
}
