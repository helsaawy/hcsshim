package osversion

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func Test_Tags(t *testing.T) {
	//todo: when new LTSC (ie, 2025) is added, update tests
	for _, tt := range []struct {
		build uint16
		tag   string
	}{
		{RS3, "1709"},
		{RS4, "1803"},
		{RS5, "ltsc2019"},
		{V1903, "1903"},
		{V1909, "1909"},
		{V2004, "2004"},
		{LTSC2022, "ltsc2022"},
		{V21H2Win11, "ltsc2022"},
		{V22H2Win11, "ltsc2022"},
	} {
		t.Run(fmt.Sprintf("Build %d", tt.build), func(t *testing.T) {
			tag, err := ContainerTagFromBuild(tt.build)
			if err != nil {
				t.Error(err)
			} else if !strings.EqualFold(tag, tt.tag) {
				t.Errorf("got: %s; wanted %s", tag, tt.tag)
			}
		})
	}

	for _, tt := range []uint16{
		RS1,
		RS2,
		V21H1,
		V21H2Win10,
		V22H2Win10,
	} {
		t.Run(fmt.Sprintf("Build %d", tt), func(t *testing.T) {
			_, err := ContainerTagFromBuild(tt)
			if !errors.Is(err, ErrUnsupportedBuild) {
				t.Errorf("expected not found error, got: %v", err)
			}
		})
	}
}
