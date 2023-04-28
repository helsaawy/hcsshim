//go:build windows

package main

import (
	"context"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	task "github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/errdefs"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Microsoft/hcsshim/internal/extendedtask"
	"github.com/Microsoft/hcsshim/internal/shimdiag"
)

type ServiceOptions struct {
	Events    publisher
	TID       string
	IsSandbox bool
}

type ServiceOption func(*ServiceOptions)

func WithEventPublisher(e publisher) ServiceOption {
	return func(o *ServiceOptions) {
		o.Events = e
	}
}
func WithTID(tid string) ServiceOption {
	return func(o *ServiceOptions) {
		o.TID = tid
	}
}
func WithIsSandbox(s bool) ServiceOption {
	return func(o *ServiceOptions) {
		o.IsSandbox = s
	}
}

type service struct {
	events publisher
	// tid is the original task id to be served. This can either be a single
	// task or represent the POD sandbox task id. The first call to Create MUST
	// match this id or the shim is considered to be invalid.
	//
	// This MUST be treated as readonly for the lifetime of the shim.
	tid string
	// isSandbox specifies if `tid` is a POD sandbox. If `false` the shim will
	// reject all calls to `Create` where `tid` does not match. If `true`
	// multiple calls to `Create` are allowed as long as the workload containers
	// all have the same parent task id.
	//
	// This MUST be treated as readonly for the lifetime of the shim.
	isSandbox bool

	// taskOrPod is either the `pod` this shim is tracking if `isSandbox ==
	// true` or it is the `task` this shim is tracking. If no call to `Create`
	// has taken place yet `taskOrPod.Load()` MUST return `nil`.
	taskOrPod atomic.Value

	// cl is the create lock. Since each shim MUST only track a single task or
	// POD, `cl` is used to create the task or POD sandbox.
	//
	// It SHOULD NOT be taken when creating tasks in a POD sandbox, as that can happen
	// concurrently.
	cl sync.Mutex

	// shutdown is closed to signal a shutdown request is received
	shutdown chan struct{}
	// shutdownOnce is responsible for closing `shutdown` and any other necessary cleanup
	shutdownOnce sync.Once
	// gracefulShutdown dictates whether to shutdown gracefully and clean up resources
	// or exit immediately
	gracefulShutdown bool
}

var _ task.TaskService = &service{}

func NewService(o ...ServiceOption) (*service, error) {
	var opts ServiceOptions
	for _, op := range o {
		op(&opts)
	}

	svc := &service{
		events:    opts.Events,
		tid:       opts.TID,
		isSandbox: opts.IsSandbox,
		shutdown:  make(chan struct{}),
	}
	return svc, nil
}

func (s *service) State(ctx context.Context, req *task.StateRequest) (*task.StateResponse, error) {
	r, e := s.stateInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Create(ctx context.Context, req *task.CreateTaskRequest) (*task.CreateTaskResponse, error) {

	r, e := s.createInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Start(ctx context.Context, req *task.StartRequest) (*task.StartResponse, error) {
	r, e := s.startInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Delete(ctx context.Context, req *task.DeleteRequest) (*task.DeleteResponse, error) {
	r, e := s.deleteInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Pids(ctx context.Context, req *task.PidsRequest) (*task.PidsResponse, error) {
	r, e := s.pidsInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Pause(ctx context.Context, req *task.PauseRequest) (*emptypb.Empty, error) {
	r, e := s.pauseInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Resume(ctx context.Context, req *task.ResumeRequest) (*emptypb.Empty, error) {
	r, e := s.resumeInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Checkpoint(ctx context.Context, req *task.CheckpointTaskRequest) (*emptypb.Empty, error) {
	r, e := s.checkpointInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Kill(ctx context.Context, req *task.KillRequest) (*emptypb.Empty, error) {
	r, e := s.killInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Exec(ctx context.Context, req *task.ExecProcessRequest) (*emptypb.Empty, error) {
	r, e := s.execInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) DiagExecInHost(ctx context.Context, req *shimdiag.ExecProcessRequest) (*shimdiag.ExecProcessResponse, error) {
	r, e := s.diagExecInHostInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) DiagShare(ctx context.Context, req *shimdiag.ShareRequest) (*shimdiag.ShareResponse, error) {
	r, e := s.diagShareInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) DiagTasks(ctx context.Context, req *shimdiag.TasksRequest) (*shimdiag.TasksResponse, error) {
	r, e := s.diagTasksInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) ResizePty(ctx context.Context, req *task.ResizePtyRequest) (*emptypb.Empty, error) {
	r, e := s.resizePtyInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) CloseIO(ctx context.Context, req *task.CloseIORequest) (*emptypb.Empty, error) {
	r, e := s.closeIOInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Update(ctx context.Context, req *task.UpdateTaskRequest) (*emptypb.Empty, error) {
	r, e := s.updateInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Wait(ctx context.Context, req *task.WaitRequest) (*task.WaitResponse, error) {
	r, e := s.waitInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Stats(ctx context.Context, req *task.StatsRequest) (*task.StatsResponse, error) {
	r, e := s.statsInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Connect(ctx context.Context, req *task.ConnectRequest) (*task.ConnectResponse, error) {
	r, e := s.connectInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Shutdown(ctx context.Context, req *task.ShutdownRequest) (*emptypb.Empty, error) {
	r, e := s.shutdownInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) DiagStacks(ctx context.Context, req *shimdiag.StacksRequest) (*shimdiag.StacksResponse, error) {
	if s == nil {
		return nil, nil
	}

	buf := make([]byte, 4096)
	for {
		buf = buf[:runtime.Stack(buf, true)]
		if len(buf) < cap(buf) {
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	resp := &shimdiag.StacksResponse{Stacks: string(buf)}

	t, _ := s.getTask(s.tid)
	if t != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		resp.GuestStacks = t.DumpGuestStacks(ctx)
	}
	return resp, nil
}

func (s *service) DiagPid(ctx context.Context, req *shimdiag.PidRequest) (*shimdiag.PidResponse, error) {
	if s == nil {
		return nil, nil
	}
	return &shimdiag.PidResponse{
		Pid: int32(os.Getpid()),
	}, nil
}

func (s *service) ComputeProcessorInfo(
	ctx context.Context,
	req *extendedtask.ComputeProcessorInfoRequest,
) (*extendedtask.ComputeProcessorInfoResponse, error) {
	r, e := s.computeProcessorInfoInternal(ctx, req)
	return r, errdefs.ToGRPC(e)
}

func (s *service) Done() <-chan struct{} {
	return s.shutdown
}

func (s *service) IsShutdown() bool {
	select {
	case <-s.shutdown:
		return true
	default:
		return false
	}
}
