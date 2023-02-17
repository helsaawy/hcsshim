//go:build windows

package plan9

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcs/resourcepaths"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/protocol/guestrequest"
	"github.com/Microsoft/hcsshim/internal/protocol/guestresource"
	"github.com/Microsoft/hcsshim/internal/uvm/resource"
	"github.com/Microsoft/hcsshim/osversion"
)

const plan9Port = 564

// Manager tracks and manages Plan9 file shares.
type Manager struct {
	// noWritableFileShares disables mounting any writable plan9 shares.
	// This prevents the guest from modifying files and directories shared into it.
	//
	// Readonly for manager's lifespan
	noWritableFileShares bool

	// mu locks the following fields
	mu sync.RWMutex

	// parent uVM; used to send modify requests over GCS
	host resource.Host
	// Every plan9 share has a counter used for its ID in the ResourceURI and name
	counter uint64

	shares map[string]*Share
}

var _ resource.Manager[*Share] = &Manager{}

func NewManager(host resource.Host, noWritableFileShares bool) *Manager {
	return &Manager{
		host:                 host,
		shares:               make(map[string]*Share),
		noWritableFileShares: noWritableFileShares,
	}
}

func (m *Manager) Host() (resource.Host, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := m.validate(); err != nil {
		return nil, err
	}

	return m.host, nil
}

// AddPlan9 adds a Plan9 share to a utility VM.
func (m *Manager) Add(ctx context.Context, hostPath string, guestPath string, readOnly bool, restrict bool, allowedNames []string) (*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.entry(ctx).WithField(logfields.Path, hostPath)
	entry.Trace("Adding vSMB share")

	if restrict && osversion.Build() < osversion.V19H1 {
		return nil, errors.New("single-file mappings are not supported on this build of Windows")
	}
	if guestPath == "" {
		return nil, fmt.Errorf("uvmPath must be passed to AddPlan9")
	}
	if !readOnly && m.noWritableFileShares {
		return nil, fmt.Errorf("adding writable shares is denied: %w", hcs.ErrOperationDenied)
	}

	// TODO: JTERRY75 - These are marked private in the schema. For now use them
	// but when there are public variants we need to switch to them.
	const (
		shareFlagsReadOnly           int32 = 0x00000001
		shareFlagsLinuxMetadata      int32 = 0x00000004
		shareFlagsCaseSensitive      int32 = 0x00000008
		shareFlagsRestrictFileAccess int32 = 0x00000080
	)

	// TODO: JTERRY75 - `shareFlagsCaseSensitive` only works if the Windows
	// `hostPath` supports case sensitivity. We need to detect this case before
	// forwarding this flag in all cases.
	flags := shareFlagsLinuxMetadata // | shareFlagsCaseSensitive
	if readOnly {
		flags |= shareFlagsReadOnly
	}
	if restrict {
		flags |= shareFlagsRestrictFileAccess
	}

	index := m.counter
	m.counter++
	name := strconv.FormatUint(index, 10)

	modification := &hcsschema.ModifySettingRequest{
		RequestType: guestrequest.RequestTypeAdd,
		Settings: hcsschema.Plan9Share{
			Name:         name,
			AccessName:   name,
			Path:         hostPath,
			Port:         plan9Port,
			Flags:        flags,
			AllowedFiles: allowedNames,
		},
		ResourcePath: resourcepaths.Plan9ShareResourcePath,
		GuestRequest: guestrequest.ModificationRequest{
			ResourceType: guestresource.ResourceTypeMappedDirectory,
			RequestType:  guestrequest.RequestTypeAdd,
			Settings: guestresource.LCOWMappedDirectory{
				MountPath: guestPath,
				ShareName: name,
				Port:      plan9Port,
				ReadOnly:  readOnly,
			},
		},
	}

	if err := m.host.Modify(ctx, modification); err != nil {
		return nil, err
	}

	r := &Share{
		m:         m,
		hostPath:  hostPath,
		name:      name,
		guestPath: guestPath,
	}
	m.shares[name] = r
	return r, nil
}

// Close releases any Plan9 shares still remaining and clears the manager's internal state.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.entry(ctx)
	entry.Trace("Closing Plan9 manager")

	if err := m.validate(); err != nil {
		return fmt.Errorf("plan9 manager was already closed: %w", hcs.ErrAlreadyClosed)
	}

	// todo: multierror to aggregate and return removeShare() errors
	for n, share := range m.shares {
		if err := m.remove(ctx, share); err != nil {
			entry.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				logfields.Name:  n,
				logfields.Path:  share.hostPath,
			}).Warning("could not remove Plan9 share")
		}
	}
	m.shares = nil
	m.host = nil
	return nil
}

// Remove removes a Plan9 share.
func (m *Manager) Remove(ctx context.Context, share *Share) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.validate(); err != nil {
		return err
	}

	return m.remove(ctx, share)

}

// must hold m.mu before calling.
func (m *Manager) remove(ctx context.Context, r *Share) error {
	modification := &hcsschema.ModifySettingRequest{
		RequestType: guestrequest.RequestTypeRemove,
		Settings: hcsschema.Plan9Share{
			Name:       r.name,
			AccessName: r.name,
			Port:       plan9Port,
		},
		ResourcePath: resourcepaths.Plan9ShareResourcePath,
		GuestRequest: guestrequest.ModificationRequest{
			ResourceType: guestresource.ResourceTypeMappedDirectory,
			RequestType:  guestrequest.RequestTypeRemove,
			Settings: guestresource.LCOWMappedDirectory{
				MountPath: r.guestPath,
				ShareName: r.name,
				Port:      plan9Port,
			},
		},
	}
	if err := m.host.Modify(ctx, modification); err != nil {
		return fmt.Errorf("failed to remove plan9 share %s from %s: %+v: %s", r.name, m.host.ID(), modification, err)
	}
	delete(m.shares, r.name)
	return r.close(ctx)
}

// List returns all the Plan9 shares the manager currently holds.
func (m *Manager) List(ctx context.Context) ([]*Share, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := m.validate(); err != nil {
		return nil, err
	}

	m.entry(ctx).Trace("Listing Plan9 shares")

	return m.list(), nil
}

// must hold m.mu before calling.
func (m *Manager) list() []*Share {
	rs := make([]*Share, 0, len(m.shares))
	for _, r := range m.shares {
		rs = append(rs, r)
	}
	return rs
}

// must hold m.mu before calling.
func (m *Manager) entry(ctx context.Context) *logrus.Entry {
	e := log.G(ctx)
	if m.invalid() {
		return e
	}
	return e.WithFields(logrus.Fields{
		logfields.UVMID: m.host.ID(),
	})
}

// must hold m.mu before calling.
func (m *Manager) validate() error {
	if m.invalid() {
		return fmt.Errorf("plan9 manager not initialized or already closed: %w", resource.ErrInvalidManagerState)
	}
	return nil
}

// must hold m.mu before calling.
func (m *Manager) invalid() bool { return m == nil || m.host == nil }
