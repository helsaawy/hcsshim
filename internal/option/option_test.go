package option

import (
	"encoding/json"
	"testing"
)

type A struct {
	B  Option[bool] `json:",omitempty"`
	I  Option[int]
	Ii Option[int] `json:",omitempty"`
}

func TestEncoding(t *testing.T) {
	for _, a := range []A{
		{
			B:  Some(false),
			I:  Some(3),
			Ii: Some(3),
		},
		{},
	} {
		t.Logf("%v", a)

		b, err := json.Marshal(a)
		if err != nil {
			t.Error(err)
		} else {
			t.Log(string(b))
		}
	}
}
