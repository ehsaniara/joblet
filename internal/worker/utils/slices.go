package utils

func CopyStringSlice(src []string) []string {
	if src == nil {
		return nil
	}

	if len(src) == 0 {
		return []string{}
	}

	dst := make([]string, len(src))
	copy(dst, src)

	return dst
}
