package network

import "testing"

func TestBridgeNameForNetwork(t *testing.T) {
	// The default "bridge" network maps to the shared joblet0 bridge.
	if got := bridgeNameForNetwork("bridge"); got != "joblet0" {
		t.Errorf("bridge -> %q, want joblet0", got)
	}

	// Short names keep the readable joblet-<name> form (<= 15 chars).
	cases := map[string]string{
		"tinynet":  "joblet-tinynet",  // 14
		"shortnet": "joblet-shortnet", // 15 (at the limit)
	}
	for name, want := range cases {
		if got := bridgeNameForNetwork(name); got != want {
			t.Errorf("%q -> %q, want %q", name, got, want)
		}
	}

	// Longer names fall back to a hashed form that still fits the interface-name
	// limit, is prefixed "joblet", and is deterministic.
	for _, name := range []string{"probe-network-1", "my-application-network", "a234567890"} {
		got := bridgeNameForNetwork(name)
		if len(got) > maxIfaceNameLen {
			t.Errorf("%q -> %q exceeds interface-name limit (%d > %d)", name, got, len(got), maxIfaceNameLen)
		}
		if got[:7] != "joblet-" {
			t.Errorf("%q -> %q not prefixed joblet-", name, got)
		}
		if got2 := bridgeNameForNetwork(name); got2 != got {
			t.Errorf("%q not deterministic: %q vs %q", name, got, got2)
		}
	}

	// Distinct long names should not collide.
	if bridgeNameForNetwork("my-application-network") == bridgeNameForNetwork("another-long-network") {
		t.Error("expected distinct long network names to map to distinct bridges")
	}
}
