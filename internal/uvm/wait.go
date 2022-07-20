//go:build windows

package uvm

import (
	logfields "github.com/Microsoft/hcsshim/internal/log/fields"
	"github.com/sirupsen/logrus"
)

// Wait waits synchronously for a utility VM to terminate.
func (uvm *UtilityVM) Wait() error {
	err := uvm.hcsSystem.Wait()

	logrus.WithField(logfields.UVMID, uvm.id).Debug("uvm exited, waiting for output processing to complete")
	if uvm.outputProcessingDone != nil {
		<-uvm.outputProcessingDone
	}

	return err
}
