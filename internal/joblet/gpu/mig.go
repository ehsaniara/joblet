package gpu

import (
	"regexp"
	"strings"
)

// MIG (Multi-Instance GPU) support. When a physical GPU is in MIG mode it is
// partitioned into hardware-isolated instances, each with dedicated memory and
// compute. Those instances - not the whole card - are the unit joblet allocates,
// because they are the unit of isolation.

// nvidia-smi -L lines look like:
//
//	GPU 0: NVIDIA A100-SXM4-40GB (UUID: GPU-8f...)
//	  MIG 1g.5gb      Device  0: (UUID: MIG-3eb...)
//	  MIG 2g.10gb     Device  1: (UUID: MIG-9c...)
var (
	parentGPURe = regexp.MustCompile(`^GPU (\d+):`)
	migDeviceRe = regexp.MustCompile(`MIG\s+(\S+)\s+Device\s+(\d+):\s+\(UUID:\s+(MIG-[0-9a-fA-F-]+)\)`)
)

// ParseMIGDevices parses `nvidia-smi -L` output into the MIG instances it lists.
// Each becomes a GPU entry with IsMIG set and a synthetic Index (parent*100 +
// device id) so it slots into the normal allocation pool. Returns nil when the
// output lists no MIG instances (i.e. no GPU is in MIG mode).
func ParseMIGDevices(nvidiaSmiL string) []*GPU {
	var out []*GPU
	parent := 0
	for _, line := range strings.Split(nvidiaSmiL, "\n") {
		if m := parentGPURe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			parent = atoiSafe(m[1])
			continue
		}
		m := migDeviceRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		profile, devID, uuid := m[1], atoiSafe(m[2]), m[3]
		out = append(out, &GPU{
			Index:   parent*100 + devID,
			UUID:    uuid,
			MIGUUID: uuid,
			Name:    "MIG " + profile,
			IsMIG:   true,
		})
	}
	return out
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}
