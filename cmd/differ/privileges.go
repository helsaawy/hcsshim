package main

import (
	"fmt"

	"github.com/Microsoft/hcsshim/internal/winapi"
	"golang.org/x/sys/windows"
)

// privilegesToDelete returns a list of all the privleges a token has, except for those
// specified in keep.
//
// The return is a pointer to the first element of a []
func privilegesToDelete(token windows.Token, keep []string) ([]windows.LUIDAndAttributes, error) {
	keepLUIDs := make([]windows.LUID, 0, len(keep))
	for _, s := range keep {
		var l windows.LUID
		if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(s), &l); err != nil {
			return nil, fmt.Errorf("could not lookup privilege %q: %w", s, err)
		}
		keepLUIDs = append(keepLUIDs, l)
	}

	pv, err := winapi.GetTokenPrivileges(token)
	if err != nil {
		return nil, fmt.Errorf("could not get token privileges: %w", err)
	}

	privs := pv.AllPrivileges()
	privDel := make([]windows.LUIDAndAttributes, 0, len(privs))

	for _, a := range privs {
		if deletePriv(&a, keepLUIDs) {
			privDel = append(privDel, a)
		}
	}

	return privDel, nil
}

func deletePriv(p *windows.LUIDAndAttributes, keep []windows.LUID) bool {
	for _, l := range keep {
		if p.Luid == l {
			return false
		}
	}
	return true
}
