package resource

import (
	"context"
	"errors"

	"github.com/Microsoft/hcsshim/internal/vm"
)

var (
	ErrNoAvailableLocation = errors.New("no available location")
	ErrNotAttached         = errors.New("not attached")
	ErrAlreadyAttached     = errors.New("already attached")
)

type Manager interface {
	// ResourceType returns the type of resource that is being managed.
	ResourceType() vm.Resource

	// Close attempts to close all resources, returning any errors that arose.
	Close(context.Context) error
}
