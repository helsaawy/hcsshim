package name

import (
	"encoding/json"
	"fmt"
	"testing"
)

var oses = []OS{Linux, Windows}

func TestString(t *testing.T) {
	for _, os := range oses {
		t.Run(os.String(), func(t *testing.T) {
			s := os.String()
			_os, err := FromString(s)
			if err != nil {
				t.Fatalf("could convert %q back to OS from string: %v", s, err)
			}
			if _os != os {
				t.Fatalf("got %v, wanted %v", _os, os)
			}
		})
	}
}

func TestMarshal(t *testing.T) {
	for _, os := range oses {
		t.Run(os.String(), func(t *testing.T) {
			b, err := json.Marshal(os)
			if err != nil {
				t.Fatalf("could not marshal: %v", err)
			}
			want := fmt.Sprintf("%q", os.String())
			if string(b) != want {
				t.Fatalf("got %q, wanted %q", b, want)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	for _, os := range oses {
		t.Run(os.String(), func(t *testing.T) {
			b, err := json.Marshal(os)
			if err != nil {
				t.Fatalf("could not marshal: %v", err)
			}

			var _os OS
			if err = json.Unmarshal(b, &_os); err != nil {
				t.Fatalf("could not unmarshal %q: %v", b, err)
			}
			if _os != os {
				t.Fatalf("got %v, wanted %v", _os, os)
			}
		})
	}
}
