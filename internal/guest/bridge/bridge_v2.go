//go:build linux
// +build linux

package bridge

import (
	"context"
	"encoding/json"
	"syscall"
	"time"

	"github.com/Microsoft/hcsshim/internal/debug"
	"github.com/Microsoft/hcsshim/internal/guest/commonutils"
	"github.com/Microsoft/hcsshim/internal/guest/gcserr"
	"github.com/Microsoft/hcsshim/internal/guest/prot"
	"github.com/Microsoft/hcsshim/internal/guest/runtime/hcsv2"
	"github.com/Microsoft/hcsshim/internal/guest/stdio"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"golang.org/x/sys/unix"
)

// The capabilities of this GCS.
var capabilities = prot.GcsCapabilities{
	SendHostCreateMessage:   false,
	SendHostStartMessage:    false,
	HVSocketConfigOnStartup: false,
	SupportedSchemaVersions: []prot.SchemaVersion{
		{
			Major: 2,
			Minor: 1,
		},
	},
	RuntimeOsType: prot.OsTypeLinux,
	GuestDefinedCapabilities: prot.GcsGuestCapabilities{
		NamespaceAddRequestSupported:  true,
		SignalProcessSupported:        true,
		DumpStacksSupported:           true,
		DeleteContainerStateSupported: true,
	},
}

// negotiateProtocolV2 was introduced in v4 so will not be called with a minimum
// lower than that.
func (b *Bridge) negotiateProtocolV2(r *Request) (_ RequestResponse, err error) {
	_, span := trace.StartSpan(r.Context, "opengcs::bridge::negotiateProtocolV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	var request prot.NegotiateProtocol
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	if request.MaximumVersion < uint32(prot.PvV4) || uint32(prot.PvMax) < request.MinimumVersion {
		return nil, gcserr.NewHresultError(gcserr.HrVmcomputeUnsupportedProtocolVersion)
	}

	min := func(x, y uint32) uint32 {
		if x < y {
			return x
		}
		return y
	}

	major := min(uint32(prot.PvMax), request.MaximumVersion)

	// Set our protocol selected version before return.
	b.protVer = prot.ProtocolVersion(major)

	return &prot.NegotiateProtocolResponse{
		Version:      major,
		Capabilities: capabilities,
	}, nil
}

// createContainerV2 creates a container based on the settings passed in `r`.
//
// This is allowed only for protocol version 4+, schema version 2.1+
func (b *Bridge) createContainerV2(r *Request) (_ RequestResponse, err error) {
	ctx, span := trace.StartSpan(r.Context, "opengcs::bridge::createContainerV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	var request prot.ContainerCreate
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	var settingsV2 prot.VMHostedContainerSettingsV2
	if err := commonutils.UnmarshalJSONWithHresult([]byte(request.ContainerConfig), &settingsV2); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON for ContainerConfig \"%s\"", request.ContainerConfig)
	}

	if settingsV2.SchemaVersion.Cmp(prot.SchemaVersion{Major: 2, Minor: 1}) < 0 {
		return nil, gcserr.WrapHresult(
			errors.Errorf("invalid schema version: %v", settingsV2.SchemaVersion),
			gcserr.HrVmcomputeInvalidJSON)
	}

	c, err := b.hostState.CreateContainer(ctx, request.ContainerID, &settingsV2)
	if err != nil {
		return nil, err
	}
	waitFn := func() prot.NotificationType {
		return c.Wait()
	}

	go func() {
		nt := waitFn()
		notification := &prot.ContainerNotification{
			MessageBase: prot.MessageBase{
				ContainerID: request.ContainerID,
				ActivityID:  request.ActivityID,
			},
			Type:       nt,
			Operation:  prot.AoNone,
			Result:     0,
			ResultInfo: "",
		}
		b.PublishNotification(notification)
	}()

	return &prot.ContainerCreateResponse{}, nil
}

// startContainerV2 doesn't have a great correlation to LCOW. On Windows this is
// used to start the container silo. In Linux the container is the process so we
// wait until the exec process of the init process to actually issue the start.
//
// This is allowed only for protocol version 4+, schema version 2.1+
func (b *Bridge) startContainerV2(r *Request) (_ RequestResponse, err error) {
	_, span := trace.StartSpan(r.Context, "opengcs::bridge::startContainerV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	// This is just a noop, but needs to be handled so that an error isn't
	// returned to the HCS.
	var request prot.MessageBase
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	return &prot.MessageResponseBase{}, nil
}

// execProcessV2 is used to execute three types of processes in the guest.
//
// 1. HostProcess. This is a process in the Host pid namespace that runs as
// root. It is signified by either `request.IsExternal` or `request.ContainerID
// == hcsv2.UVMContainerID`.
//
// 2. Container Init process. This is the init process of the created container.
// We use exec for this instead of `StartContainer` because the protocol does
// not pass in the appropriate std pipes for relaying the results until exec.
// Until this is called the container remains in the `created` state.
//
// 3. Container Exec process. This is a process that is run in the container's
// pid namespace.
//
// This is allowed only for protocol version 4+, schema version 2.1+
func (b *Bridge) execProcessV2(r *Request) (_ RequestResponse, err error) {
	ctx, span := trace.StartSpan(r.Context, "opengcs::bridge::execProcessV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	var request prot.ContainerExecuteProcess
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	// The request contains a JSON string field which is equivalent to an
	// ExecuteProcessInfo struct.
	var params prot.ProcessParameters
	if err := commonutils.UnmarshalJSONWithHresult([]byte(request.Settings.ProcessParameters), &params); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON for ProcessParameters \"%s\"", request.Settings.ProcessParameters)
	}

	var conSettings stdio.ConnectionSettings
	if params.CreateStdInPipe {
		conSettings.StdIn = &request.Settings.VsockStdioRelaySettings.StdIn
	}
	if params.CreateStdOutPipe {
		conSettings.StdOut = &request.Settings.VsockStdioRelaySettings.StdOut
	}
	if params.CreateStdErrPipe {
		conSettings.StdErr = &request.Settings.VsockStdioRelaySettings.StdErr
	}

	var pid int
	var c *hcsv2.Container
	if params.IsExternal || request.ContainerID == hcsv2.UVMContainerID {
		pid, err = b.hostState.RunExternalProcess(ctx, params, conSettings)
	} else if c, err = b.hostState.GetContainer(request.ContainerID); err == nil {
		// We found a V2 container. Treat this as a V2 process.
		if params.OCIProcess == nil {
			pid, err = c.Start(ctx, conSettings)
		} else {
			pid, err = c.ExecProcess(ctx, params.OCIProcess, conSettings)
		}
	}

	if err != nil {
		return nil, err
	}
	log.G(ctx).WithField("pid", pid).Debug("created process pid")
	return &prot.ContainerExecuteProcessResponse{
		ProcessID: uint32(pid),
	}, nil
}

// killContainerV2 is a user forced terminate of the container and all processes
// in the container. It is equivalent to sending SIGKILL to the init process and
// all exec'd processes.
//
// This is allowed only for protocol version 4+, schema version 2.1+
func (b *Bridge) killContainerV2(r *Request) (RequestResponse, error) {
	ctx, span := trace.StartSpan(r.Context, "opengcs::bridge::killContainerV2")
	defer span.End()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	return b.signalContainerV2(ctx, span, r, unix.SIGKILL)
}

// shutdownContainerV2 is a user requested shutdown of the container and all
// processes in the container. It is equivalent to sending SIGTERM to the init
// process and all exec'd processes.
//
// This is allowed only for protocol version 4+, schema version 2.1+
func (b *Bridge) shutdownContainerV2(r *Request) (RequestResponse, error) {
	ctx, span := trace.StartSpan(r.Context, "opengcs::bridge::shutdownContainerV2")
	defer span.End()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	return b.signalContainerV2(ctx, span, r, unix.SIGTERM)
}

// signalContainerV2 is not a handler func. This is because the actual signal is
// implied based on the message type of either `killContainerV2` or
// `shutdownContainerV2`.
func (b *Bridge) signalContainerV2(ctx context.Context, span *trace.Span, r *Request, signal syscall.Signal) (_ RequestResponse, err error) {
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(
		trace.StringAttribute("cid", r.ContainerID),
		trace.Int64Attribute("signal", int64(signal)))

	var request prot.MessageBase
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	// If this is targeting the UVM send the request to the host itself.
	if request.ContainerID == hcsv2.UVMContainerID {
		// We are asking to shutdown the UVM itself
		// Terminates (forcefull shutdowns) are handled by HCS direct, so don't handle here
		if signal != unix.SIGTERM {
			log.G(ctx).Errorf("invalid signal for uvm: %v", signal)
		}
		log.G(ctx).Debug("shutting down linux uvm")
		// This is a destructive call. We do not respond to the HCS
		b.quitChan <- true
		b.hostState.Shutdown(ctx)
	} else {
		c, err := b.hostState.GetContainer(request.ContainerID)
		if err != nil {
			return nil, err
		}

		err = c.Kill(ctx, signal)
		if err != nil {
			return nil, err
		}
	}

	return &prot.MessageResponseBase{}, nil
}

func (b *Bridge) signalProcessV2(r *Request) (_ RequestResponse, err error) {
	ctx, span := trace.StartSpan(r.Context, "opengcs::bridge::signalProcessV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	var request prot.ContainerSignalProcess
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	span.AddAttributes(
		trace.Int64Attribute("pid", int64(request.ProcessID)),
		trace.Int64Attribute("signal", int64(request.Options.Signal)))

	c, err := b.hostState.GetContainer(request.ContainerID)
	if err != nil {
		return nil, err
	}

	p, err := c.GetProcess(request.ProcessID)
	if err != nil {
		return nil, err
	}

	var signal syscall.Signal
	if request.Options.Signal == 0 {
		signal = unix.SIGKILL
	} else {
		signal = syscall.Signal(request.Options.Signal)
	}
	if err := p.Kill(ctx, signal); err != nil {
		return nil, err
	}

	return &prot.MessageResponseBase{}, nil
}

func (b *Bridge) getPropertiesV2(r *Request) (_ RequestResponse, err error) {
	ctx, span := trace.StartSpan(r.Context, "opengcs::bridge::getPropertiesV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	var request prot.ContainerGetProperties
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	properties := &prot.PropertiesV2{}

	var query prot.PropertyQuery
	if len(request.Query) != 0 {
		if err := json.Unmarshal([]byte(request.Query), &query); err != nil {
			e := gcserr.WrapHresult(err, gcserr.HrVmcomputeInvalidJSON)
			return nil, errors.Wrapf(e, "The query could not be unmarshaled: '%s'", query)
		}
	}

	if request.ContainerID == hcsv2.UVMContainerID {
		return nil, errors.New("getPropertiesV2 is not supported against the UVM")
	}

	c, err := b.hostState.GetContainer(request.ContainerID)
	if err != nil {
		return nil, err
	}

	for _, requestedProperty := range query.PropertyTypes {
		if requestedProperty == prot.PtProcessList {
			pids, err := c.GetAllProcessPids(ctx)
			if err != nil {
				return nil, err
			}
			properties.ProcessList = make([]prot.ProcessDetails, len(pids))
			for i, pid := range pids {
				properties.ProcessList[i].ProcessID = uint32(pid)
			}
		} else if requestedProperty == prot.PtStatistics {
			cgroupMetrics, err := c.GetStats(ctx)
			if err != nil {
				return nil, err
			}
			properties.Metrics = cgroupMetrics
		}
	}

	propertyJSON := []byte("{}")
	if properties != nil {
		var err error
		propertyJSON, err = json.Marshal(properties)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%+v\"", properties)
		}
	}

	return &prot.ContainerGetPropertiesResponse{
		Properties: string(propertyJSON),
	}, nil
}

func (b *Bridge) waitOnProcessV2(r *Request) (_ RequestResponse, err error) {
	_, span := trace.StartSpan(r.Context, "opengcs::bridge::waitOnProcessV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	var request prot.ContainerWaitForProcess
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	span.AddAttributes(
		trace.Int64Attribute("pid", int64(request.ProcessID)),
		trace.Int64Attribute("timeout-ms", int64(request.TimeoutInMs)))

	var exitCodeChan <-chan int
	var doneChan chan<- bool

	if request.ContainerID == hcsv2.UVMContainerID {
		p, err := b.hostState.GetExternalProcess(int(request.ProcessID))
		if err != nil {
			return nil, err
		}
		exitCodeChan, doneChan = p.Wait()
	} else {
		c, err := b.hostState.GetContainer(request.ContainerID)
		if err != nil {
			return nil, err
		}
		p, err := c.GetProcess(request.ProcessID)
		if err != nil {
			return nil, err
		}
		exitCodeChan, doneChan = p.Wait()
	}

	// If we timed out or if we got the exit code. Acknowledge we no longer want to wait.
	defer close(doneChan)

	var tc <-chan time.Time
	if request.TimeoutInMs != prot.InfiniteWaitTimeout {
		t := time.NewTimer(time.Duration(request.TimeoutInMs) * time.Millisecond)
		defer t.Stop()
		tc = t.C
	}
	select {
	case exitCode := <-exitCodeChan:
		return &prot.ContainerWaitForProcessResponse{
			ExitCode: uint32(exitCode),
		}, nil
	case <-tc:
		return nil, gcserr.NewHresultError(gcserr.HvVmcomputeTimeout)
	}
}

func (b *Bridge) resizeConsoleV2(r *Request) (_ RequestResponse, err error) {
	ctx, span := trace.StartSpan(r.Context, "opengcs::bridge::resizeConsoleV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	var request prot.ContainerResizeConsole
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	span.AddAttributes(
		trace.Int64Attribute("pid", int64(request.ProcessID)),
		trace.Int64Attribute("height", int64(request.Height)),
		trace.Int64Attribute("width", int64(request.Width)))

	c, err := b.hostState.GetContainer(request.ContainerID)
	if err != nil {
		return nil, err
	}

	p, err := c.GetProcess(request.ProcessID)
	if err != nil {
		return nil, err
	}

	err = p.ResizeConsole(ctx, request.Height, request.Width)
	if err != nil {
		return nil, err
	}

	return &prot.MessageResponseBase{}, nil
}

func (b *Bridge) modifySettingsV2(r *Request) (_ RequestResponse, err error) {
	ctx, span := trace.StartSpan(r.Context, "opengcs::bridge::modifySettingsV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	request, err := prot.UnmarshalContainerModifySettings(r.Message)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	err = b.hostState.ModifySettings(ctx, request.ContainerID, request.Request.(*prot.ModifySettingRequest))
	if err != nil {
		return nil, err
	}

	return &prot.MessageResponseBase{}, nil
}

func (b *Bridge) dumpStacksV2(r *Request) (_ RequestResponse, err error) {
	_, span := trace.StartSpan(r.Context, "opengcs::bridge::dumpStacksV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()

	stacks := debug.DumpStacks()

	return &prot.DumpStacksResponse{
		GuestStacks: stacks,
	}, nil
}

func (b *Bridge) deleteContainerStateV2(r *Request) (_ RequestResponse, err error) {
	ctx, span := trace.StartSpan(r.Context, "opengcs::bridge::deleteContainerStateV2")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()

	span.AddAttributes(trace.StringAttribute("cid", r.ContainerID))

	var request prot.MessageBase
	if err := commonutils.UnmarshalJSONWithHresult(r.Message, &request); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal JSON in message \"%s\"", r.Message)
	}

	c, err := b.hostState.GetContainer(request.ContainerID)
	if err != nil {
		return nil, err
	}

	if err := c.Delete(ctx); err != nil {
		return nil, err
	}

	b.hostState.RemoveContainer(request.ContainerID)
	return &prot.MessageResponseBase{}, nil
}
