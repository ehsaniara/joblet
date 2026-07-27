package domain

// IsValidNetworkName reports whether name is a valid custom network name.
// A network name becomes a host bridge interface name, and it is passed to
// iptables and ip-link commands, so it must be a plain identifier: 1-63
// characters of [a-zA-Z0-9_-], starting and ending with an alphanumeric.
// The bridge interface name is derived safely from this value (hashed when the
// "joblet-<name>" form would exceed the kernel interface-name limit), so length
// here is bounded only for sanity, not by IFNAMSIZ.
func IsValidNetworkName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}

	// Must start and end with alphanumeric
	first := rune(name[0])
	last := rune(name[len(name)-1])

	return ((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || (first >= '0' && first <= '9')) &&
		((last >= 'a' && last <= 'z') || (last >= 'A' && last <= 'Z') || (last >= '0' && last <= '9'))
}
