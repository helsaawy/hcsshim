package vhd

// miscellaneous byte and string manipulation functions

func to8byteAlignment(s uint) uint {
	// todo: make this generic over integer types
	r := s & 0x7
	if r != 0 {
		s = s ^ r + 0x8
	}
	return s
}

func bytesToString(b []byte) string {
	for i, v := range b {
		if v == 0 {
			b = b[:i]
			break
		}
	}
	return string(b)
}

func bytesToStringArray(b []byte) []string {
	ss := make([]string, 0, 3)

	for i := 0; i < len(b); {
		s := bytesToString(b[i:])
		l := len(s)
		if l > 0 {
			// skip empty strings (ie, repeated null bytes)
			ss = append(ss, s)
		}
		i += l + 1
	}
	return ss
}

func maxUintptr(a, b uintptr) uintptr {
	if a > b {
		return a
	}
	return b
}
