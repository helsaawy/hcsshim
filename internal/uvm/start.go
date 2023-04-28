//go:build windows

package uvm

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim/internal/gcs"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcs/schema1"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/protocol/guestrequest"
	"github.com/Microsoft/hcsshim/internal/protocol/guestresource"
)

// entropyBytes is the number of bytes of random data to send to a Linux UVM
// during boot to seed the CRNG. There is not much point in making this too
// large since the random data collected from the host is likely computed from a
// relatively small key (256 bits?), so additional bytes would not actually
// increase the entropy of the guest's pool. However, send enough to convince
// containers that there is a large amount of entropy since this idea is
// generally misunderstood.
const entropyBytes = 512

func isDisconnectError(err error) bool {
	return hcs.IsAny(err, windows.WSAECONNABORTED, windows.WSAECONNRESET)
}

// When using an external GCS connection it is necessary to send a ModifySettings request
// for HvSockt so that the GCS can setup some registry keys that are required for running
// containers inside the UVM. In non external GCS connection scenarios this is done by the
// HCS immediately after the GCS connection is done. Since, we are using the external GCS
// connection we should do that setup here after we connect with the GCS.
// This only applies for WCOW
func (uvm *UtilityVM) configureHvSocketForGCS(ctx context.Context) (err error) {
	if uvm.OS() != "windows" {
		return nil
	}

	hvsocketAddress := &hcsschema.HvSocketAddress{
		LocalAddress:  uvm.runtimeID.String(),
		ParentAddress: gcs.WindowsGcsHvHostID.String(),
	}

	conSetupReq := &hcsschema.ModifySettingRequest{
		GuestRequest: guestrequest.ModificationRequest{
			RequestType:  guestrequest.RequestTypeUpdate,
			ResourceType: guestresource.ResourceTypeHvSocket,
			Settings:     hvsocketAddress,
		},
	}

	if err = uvm.modify(ctx, conSetupReq); err != nil {
		return fmt.Errorf("failed to configure HVSOCK for external GCS: %s", err)
	}

	return nil
}

// Start synchronously starts the utility VM.
func (uvm *UtilityVM) Start(ctx context.Context) (err error) {
	// save parent context, without timeout to use in terminate
	pCtx := ctx
	ctx, cancel := context.WithTimeout(pCtx, 2*time.Minute)
	g, gctx := errgroup.WithContext(ctx)
	defer func() {
		_ = g.Wait()
	}()
	defer cancel()

	// create exitCh ahead of time to prevent race conditions between writing
	// initalizing the channel and waiting on it during acceptAndClose
	uvm.exitCh = make(chan struct{})

	// Prepare to provide entropy to the init process in the background. This
	// must be done in a goroutine since, when using the internal bridge, the
	// call to Start() will block until the GCS launches, and this cannot occur
	// until the host accepts and closes the entropy connection.
	if uvm.entropyListener != nil {
		g.Go(func() error {
			conn, err := uvm.acceptAndClose(gctx, uvm.entropyListener)
			uvm.entropyListener = nil
			if err != nil {
				return fmt.Errorf("failed to connect to entropy socket: %s", err)
			}
			defer conn.Close()
			_, err = io.CopyN(conn, rand.Reader, entropyBytes)
			if err != nil {
				return fmt.Errorf("failed to write entropy: %s", err)
			}
			return nil
		})
	}

	if uvm.outputListener != nil {
		g.Go(func() error {
			conn, err := uvm.acceptAndClose(gctx, uvm.outputListener)
			uvm.outputListener = nil
			if err != nil {
				close(uvm.outputProcessingDone)
				return fmt.Errorf("failed to connect to log socket: %s", err)
			}
			go func() {
				uvm.outputHandler(conn)
				close(uvm.outputProcessingDone)
			}()
			return nil
		})
	}

	err = uvm.hcsSystem.Start(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			// use parent context, to prevent 2 minute timout (set above) from overridding terminate operation's
			// timeout and erroring out prematurely
			_ = uvm.hcsSystem.Terminate(pCtx)
			_ = uvm.hcsSystem.Wait()
		}
	}()

	// Start waiting on the utility VM.
	go func() {
		err := uvm.hcsSystem.Wait()
		if err == nil {
			err = uvm.hcsSystem.ExitError()
		}
		uvm.exitErr = err
		close(uvm.exitCh)
	}()

	// Collect any errors from writing entropy or establishing the log
	// connection.
	if err = g.Wait(); err != nil {
		return err
	}

	if uvm.gcListener != nil {
		// Accept the GCS connection.
		conn, err := uvm.acceptAndClose(ctx, uvm.gcListener)
		uvm.gcListener = nil
		if err != nil {
			return fmt.Errorf("failed to connect to GCS: %s", err)
		}

		var initGuestState *gcs.InitialGuestState
		if uvm.OS() == "windows" {
			// Default to setting the time zone in the UVM to the hosts time zone unless the client asked to avoid this behavior. If so, assign
			// to UTC.
			if uvm.noInheritHostTimezone {
				initGuestState = &gcs.InitialGuestState{
					Timezone: utcTimezone,
				}
			} else {
				tz, err := getTimezone()
				if err != nil {
					return err
				}
				initGuestState = &gcs.InitialGuestState{
					Timezone: tz,
				}
			}
		}
		// Start the GCS protocol.
		gcc := &gcs.GuestConnectionConfig{
			Conn:           conn,
			Log:            log.G(ctx).WithField(logfields.UVMID, uvm.id),
			IoListen:       gcs.HvsockIoListen(uvm.runtimeID),
			InitGuestState: initGuestState,
		}
		uvm.gc, err = gcc.Connect(ctx, !uvm.IsClone)
		if err != nil {
			return err
		}
		uvm.guestCaps = *uvm.gc.Capabilities()
		uvm.protocol = uvm.gc.Protocol()

		// initial setup required for external GCS connection
		if err = uvm.configureHvSocketForGCS(ctx); err != nil {
			return fmt.Errorf("failed to do initial GCS setup: %s", err)
		}
	} else {
		// Cache the guest connection properties.
		properties, err := uvm.hcsSystem.Properties(ctx, schema1.PropertyTypeGuestConnection)
		if err != nil {
			return err
		}
		uvm.guestCaps = properties.GuestConnectionInfo.GuestDefinedCapabilities
		uvm.protocol = properties.GuestConnectionInfo.ProtocolVersion
	}

	if uvm.confidentialUVMOptions != nil && uvm.OS() == "linux" {
		copts := []ConfidentialUVMOpt{
			WithSecurityPolicy(uvm.confidentialUVMOptions.SecurityPolicy),
			WithSecurityPolicyEnforcer(uvm.confidentialUVMOptions.SecurityPolicyEnforcer),
			WithUVMReferenceInfo(defaultLCOWOSBootFilesPath(), uvm.confidentialUVMOptions.UVMReferenceInfoFile),
		}
		if err := uvm.SetConfidentialUVMOptions(ctx, copts...); err != nil {
			return err
		}
	}

	return nil
}

// acceptAndClose accepts a connection and then closes a listener. If the
// context becomes done or the utility VM terminates, the operation will be
// cancelled (but the listener will still be closed).
func (uvm *UtilityVM) acceptAndClose(ctx context.Context, l net.Listener) (net.Conn, error) {
	var conn net.Conn
	ch := make(chan error)
	go func() {
		var err error
		conn, err = l.Accept()
		ch <- err
	}()
	select {
	case err := <-ch:
		l.Close()
		return conn, err
	case <-ctx.Done():
	case <-uvm.exitCh:
	}
	l.Close()
	err := <-ch
	if err == nil {
		return conn, err
	}
	// Prefer context error to VM error to accept error in order to return the
	// most useful error.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if uvm.exitErr != nil {
		return nil, uvm.exitErr
	}
	return nil, err
}
