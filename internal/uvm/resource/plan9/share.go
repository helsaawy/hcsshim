//go:build windows

package plan9

import (
	"context"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/uvm/resource"
)

// Share contains the host path for a Plan9 mount.
//
// Do not modify unless holding parent controller lock, m.mu.
type Share struct {
	// manager for this share
	//
	// * Do not set to nil. *
	// Even after closing, still need to access m.mu to check fields.
	m *Manager

	hostPath  string
	name      string
	guestPath string
}

var _ resource.Resource = &Share{}

func (*Share) Type() resource.Type { return resource.Plan9 }

// Release frees the resources of the corresponding Plan9 mount.
func (r *Share) Release(ctx context.Context) error {
	if err := r.m.Remove(ctx, r); err != nil {
		return fmt.Errorf("failed to release Plan9 share: %w", err)
	}
	return nil
}

// close closes a plan9 share.
// It should only be called after the manager has removed it from its host
//
// caller should hold the manager's lock.
func (r *Share) close(context.Context) error {
	if r.m == nil {
		return fmt.Errorf("vSMB share already closed: %w", hcs.ErrAlreadyClosed)
	}
	r.hostPath = ""
	r.name = ""
	r.guestPath = ""
	return nil
}
