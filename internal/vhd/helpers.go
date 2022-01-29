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
