package uvm

import (
	"context"
	"errors"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/gcs"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/oc"
	"go.opencensus.io/trace"
	"golang.org/x/sys/windows"
)

// Close terminates and releases resources associated with the utility VM.
func (uvm *UtilityVM) Close() (err error) {
	ctx, span := trace.StartSpan(context.Background(), "uvm::Close")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute(logfields.UVMID, uvm.id))

	windows.Close(uvm.vmmemProcess)

	if uvm.hcsSystem != nil {
		if err = uvm.shutdownAndWait(ctx); err != nil {
			log.G(ctx).Errorf("uvm shutdown failed: %v", err)
		}
	}

	log.G(ctx).Debug("closing GCS connection")
	if err = uvm.CloseGCSConnection(); err != nil {
		log.G(ctx).Errorf("close GCS connection failed: %s", err)
	}

	// outputListener will only be nil for a Create -> Stop without a Start. In
	// this case we have no goroutine processing output so its safe to close the
	// channel here.
	if uvm.outputListener != nil {
		close(uvm.outputProcessingDone)
		uvm.outputListener.Close()
		uvm.outputListener = nil
	}
	if uvm.hcsSystem != nil {
		return uvm.hcsSystem.Close()
	}
	return nil
}

func (uvm *UtilityVM) shutdownAndWait(ctx context.Context) (err error) {
	wait := uvm.shutdownTimeout > 0
	if wait {
		var cancel context.CancelFunc // predeclare so new ctx overwrites func parameter
		ctx, cancel = context.WithTimeout(ctx, uvm.shutdownTimeout)
		defer cancel()
	}

	if uvm.shutdownGraceful {
		err = uvm.Shutdown(ctx)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to shutdown uvm")
			log.G(ctx).Debug("forcibly terminating uvm")
		}
	}

	if !uvm.shutdownGraceful || err != nil {
		// forcibly terminate
		err = uvm.Terminate(ctx)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to terminate uvm")
		}
	}

	if wait {
		// shutdown or terminate may have triggered an HCS shutdown, but returned still error
		// so wait regardless

		entry := log.G(ctx)
		entry.WithField(logfields.Timeout, uvm.shutdownTimeout.String()).Debug("waiting for uvm to shutdown")

		var werr error
		ch := make(chan struct{})
		go func() {
			werr = uvm.Wait()
			close(ch)
		}()

		select {
		case <-ch:
			err = werr
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				entry = entry.WithField(logfields.Timeout, uvm.shutdownTimeout.String())
				err = hcs.ErrTimeout
			} else {
				err = hcs.ErrCancelled
			}
		}

		if err != nil {
			entry.WithError(err).Error("failed to wait for container shutdown")
			err = fmt.Errorf("uvm wait after shutdown: %w", err)
		}
	}

	return err
}

// Shutdown requests that the utility VM be cleanly shut down.
func (uvm *UtilityVM) Shutdown(ctx context.Context) error {
	log.G(ctx).Info("uvm::Shutdown")
	// requires guest connection to initiate shutdown
	if uvm.gc == nil {
		// return fmt.Errorf("unable to request shutdown: %w", ErrGuestConnectionRequired)
		log.G(ctx).WithError(errGuestConnectionRequired).Error("falling back on shutdown via HCS")
		return uvm.hcsSystem.Shutdown(ctx)
	}

	return uvm.gc.Shutdown(ctx, gcs.NullContainerID)
}

// Terminate requests that the utility VM be forecully terminated.
func (uvm *UtilityVM) Terminate(ctx context.Context) error {
	log.G(ctx).Info("uvm::Terminate")
	return uvm.hcsSystem.Terminate(ctx)
}

// ExitError returns an error if the utility VM has terminated unexpectedly.
func (uvm *UtilityVM) ExitError() error {
	return uvm.hcsSystem.ExitError()
}
