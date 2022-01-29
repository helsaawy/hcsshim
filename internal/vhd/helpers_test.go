package vhd

import (
	"fmt"
	"testing"
)

// todo: verify this is utf-8 safe
// todo: add fuzzing for byte/utf8 string testing

func TestBytesToString(t *testing.T) {
	ss := []string{
		"",
		"hey",
		"i just bit you",
		"and this is ",
		"",
		"crazy",
		"but heres a moderatelly long string that has random unicode حروف in it to test your ability to process uft8 سلاسل نصية",
		"so pass this test",
		"können Sie?",
		"世界",
	}

	for i := 1; i < 3; i++ { // number of null terminals to add
		// bytesToString
		for _, s := range ss {
			t.Run(fmt.Sprintf("%s-%s-%d", t.Name(), s, i), func(t *testing.T) {
				b := make([]byte, len(s)+i)
				copy(b, s)

				r := bytesToString(b)
				if r != s {
					t.Fail()
				}
			})
		}

		// bytesToStringArray
		t.Run(fmt.Sprintf("%sArray-%d", t.Name(), i), func(t *testing.T) {
			var l, n int

			for _, s := range ss {
				l += len(s) + i
				if len(s) > 0 {
					n++
				}
			}

			b := make([]byte, l)

			j := 0
			for _, s := range ss {
				copy(b[j:], s)
				j += len(s) + i
			}

			rs := bytesToStringArray(b)

			if len(rs) != n {
				t.Errorf("found %d string, expected %d", len(rs), n)
			}

			for _, r := range rs {
				f := false
				for _, s := range ss {
					if r == s {
						f = true
						break
					}
				}
				if !f {
					t.Errorf("could not find %q in the original string array", r)
				}
			}

		})
	}
}
