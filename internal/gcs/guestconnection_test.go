//go:build windows

package gcs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/sirupsen/logrus"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/Microsoft/hcsshim/internal/otel"
)

func init() {
	// need a tracer so OTel creates recoding spans and sets their trace/span ID
	if _, err := otel.InitializeProvider(
		tracesdk.WithSpanProcessor(tracesdk.NewSimpleSpanProcessor(nil)),
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
	); err != nil {
		panic(err)
	}
}

const pipePortFmt = `\\.\pipe\gctest-port-%d`

func npipeIoListen(port uint32) (net.Listener, error) {
	return winio.ListenPipe(fmt.Sprintf(pipePortFmt, port), &winio.PipeConfig{
		MessageMode: true,
	})
}

func dialPort(port uint32) (net.Conn, error) {
	return winio.DialPipe(fmt.Sprintf(pipePortFmt, port), nil)
}

func simpleGcs(t *testing.T, rwc io.ReadWriteCloser) {
	t.Helper()
	defer rwc.Close()
	err := simpleGcsLoop(t, rwc)
	if err != nil {
		t.Error(err)
	}
}

func simpleGcsLoop(t *testing.T, rw io.ReadWriter) error {
	t.Helper()
	for {
		id, typ, b, err := readMessage(rw)
		if err != nil {
			if err == io.EOF || err == io.ErrClosedPipe {
				err = nil
			}
			return err
		}
		switch proc := rpcProc(typ &^ msgTypeRequest); proc {
		case rpcNegotiateProtocol:
			err := sendJSON(t, rw, msgTypeResponse|msgType(proc), id, &negotiateProtocolResponse{
				Version: protocolVersion,
				Capabilities: gcsCapabilities{
					RuntimeOsType: "linux",
				},
			})
			if err != nil {
				return err
			}
		case rpcCreate:
			err := sendJSON(t, rw, msgTypeResponse|msgType(proc), id, &containerCreateResponse{})
			if err != nil {
				return err
			}
		case rpcExecuteProcess:
			var req containerExecuteProcess
			var params baseProcessParams
			req.Settings.ProcessParameters.Value = &params
			err := json.Unmarshal(b, &req)
			if err != nil {
				return err
			}
			var stdin, stdout, stderr net.Conn
			if params.CreateStdInPipe {
				stdin, err = dialPort(req.Settings.VsockStdioRelaySettings.StdIn)
				if err != nil {
					return err
				}
				defer stdin.Close()
			}
			if params.CreateStdOutPipe {
				stdout, err = dialPort(req.Settings.VsockStdioRelaySettings.StdOut)
				if err != nil {
					return err
				}
				defer stdout.Close()
			}
			if params.CreateStdErrPipe {
				stderr, err = dialPort(req.Settings.VsockStdioRelaySettings.StdErr)
				if err != nil {
					return err
				}
				defer stderr.Close()
			}
			if stdin != nil && stdout != nil {
				go func() {
					_, err := io.Copy(stdout, stdin)
					if err != nil {
						t.Error(err)
					}
					stdin.Close()
					stdout.Close()
				}()
			}
			err = sendJSON(t, rw, msgTypeResponse|msgType(proc), id, &containerExecuteProcessResponse{
				ProcessID: 42,
			})
			if err != nil {
				return err
			}
		case rpcWaitForProcess:
			// nothing
		case rpcShutdownForced:
			var req RequestBase
			err = json.Unmarshal(b, &req)
			if err != nil {
				return err
			}
			err = sendJSON(t, rw, msgTypeResponse|msgType(proc), id, &responseBase{})
			if err != nil {
				return err
			}
			time.Sleep(50 * time.Millisecond)
			err = sendJSON(t, rw, msgType(msgTypeNotify|notifyContainer), 0, &containerNotification{
				RequestBase: RequestBase{
					ContainerID: req.ContainerID,
				},
			})
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported msg %s", typ)
		}
	}
}

func connectGcs(ctx context.Context, t *testing.T) *GuestConnection {
	t.Helper()
	s, c := pipeConn()
	if ctx != context.Background() && ctx != context.TODO() {
		go func() {
			<-ctx.Done()
			c.Close()
		}()
	}
	go simpleGcs(t, c)
	gcc := &GuestConnectionConfig{
		Conn:     s,
		Log:      logrus.NewEntry(logrus.StandardLogger()),
		IoListen: npipeIoListen,
	}
	gc, err := gcc.Connect(context.Background(), true)
	if err != nil {
		c.Close()
		t.Fatal(err)
	}
	return gc
}

func TestGcsConnect(t *testing.T) {
	gc := connectGcs(context.Background(), t)
	defer gc.Close()
}

func TestGcsCreateContainer(t *testing.T) {
	gc := connectGcs(context.Background(), t)
	defer gc.Close()
	c, err := gc.CreateContainer(context.Background(), "foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func TestGcsWaitContainer(t *testing.T) {
	gc := connectGcs(context.Background(), t)
	defer gc.Close()
	c, err := gc.CreateContainer(context.Background(), "foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	err = c.Terminate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = c.Wait()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGcsWaitContainerBridgeTerminated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gc := connectGcs(ctx, t)
	c, err := gc.CreateContainer(context.Background(), "foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	cancel() // close the GCS connection
	err = c.Wait()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGcsCreateProcess(t *testing.T) {
	gc := connectGcs(context.Background(), t)
	defer gc.Close()
	p, err := gc.CreateProcess(context.Background(), &baseProcessParams{
		CreateStdInPipe:  true,
		CreateStdOutPipe: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	stdin, stdout, _ := p.Stdio()
	_, err = stdin.Write(([]byte)("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	err = p.CloseStdin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello world" {
		t.Errorf("unexpected: %q", string(b))
	}
}

func TestGcsWaitProcessBridgeTerminated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gc := connectGcs(ctx, t)
	defer gc.Close()
	p, err := gc.CreateProcess(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	cancel()
	err = p.Wait()
	if err == nil || !strings.Contains(err.Error(), "bridge closed") {
		t.Fatal("unexpected: ", err)
	}
}

var requestBaseTests = []struct {
	n      string
	update func(*testing.T, RequestBase) RequestBase
}{
	{
		"id",
		func(t *testing.T, r RequestBase) RequestBase {
			t.Helper()
			return r
		},
	},
	{
		"json",
		func(t *testing.T, r RequestBase) RequestBase {
			t.Helper()

			b, err := json.Marshal(r)
			if err != nil {
				t.Fatalf("requestBase JSON marshal: %v", err)
			}
			rr := RequestBase{}
			if err := json.Unmarshal(b, &rr); err != nil {
				t.Fatalf("requestBase JSON unmarshal")
			}
			return rr
		},
	},
}

func Test_makeRequestNoSpan(t *testing.T) {
	n := t.Name()
	r := NewRequestBase(context.Background(), n)

	for _, tt := range requestBaseTests {
		t.Run(tt.n, func(t *testing.T) {
			r = tt.update(t, r)
			if r.ContainerID != n {
				t.Fatalf("expected ContainerID: %q, got: %q", n, r.ContainerID)
			}
			if r.ActivityID != (guid.GUID{}) {
				t.Fatalf("expected ActivityID empty, got: %q", r.ActivityID.String())
			}
			if len(r.OTelCarrier) != 0 {
				t.Fatal("expected empty trace context")
			}
		})
	}
}

func Test_makeRequestWithSpan(t *testing.T) {
	ctx, span := otel.StartSpan(context.Background(), t.Name())
	defer span.End()
	n := t.Name()
	r := NewRequestBase(ctx, n)

	for _, tt := range requestBaseTests {
		t.Run(tt.n, func(t *testing.T) {
			r = tt.update(t, r)
			if r.ContainerID != n {
				t.Fatalf("expected ContainerID: %q, got: %q", n, r.ContainerID)
			}

			if len(r.OTelCarrier) == 0 {
				t.Fatal("expected non-empty trace context")
			}

			if r.ActivityID != (guid.GUID{}) {
				t.Fatalf("expected ActivityID empty, got: %q", r.ActivityID.String())
			}

			// use a new context as the base, without the span from above
			// the extracted span context will have remote set, so update the original to match to
			// equality can succeed
			sc := span.SpanContext().WithRemote(true)
			extractedSC := trace.SpanContextFromContext(r.ExtractContext(context.Background()))
			if !sc.Equal(extractedSC) {
				t.Fatalf("extracted SpanContext %v != %v", extractedSC, sc)
			}
		})
	}
}

func Test_makeRequestWithSpan_TraceStateEmptyEntries(t *testing.T) {
	// Start a remote context span so we can forward trace state.
	parent := trace.SpanContext{}.WithTraceState(trace.TraceState{})
	ctx, span := otel.StartSpanWithRemoteParent(context.Background(), t.Name(), parent)
	defer span.End()
	r := NewRequestBase(ctx, t.Name())

	for _, tt := range requestBaseTests {
		t.Run(tt.n, func(t *testing.T) {
			r = tt.update(t, r)

			if len(r.OTelCarrier) == 0 {
				t.Fatal("expected non-empty trace context")
			}
			sc2 := trace.SpanContextFromContext(r.ExtractContext(context.Background()))
			if ts2 := sc2.TraceState(); ts2.Len() != 0 {
				t.Fatalf("expected encoded TraceState: '', got: %q", ts2)
			}
		})
	}
}

func Test_makeRequestWithSpan_TraceStateEntries(t *testing.T) {
	// Start a remote context span so we can forward trace state.
	ts, err := trace.TraceState{}.Insert("test", "also a test")
	if err != nil {
		t.Fatalf("failed to make test Tracestate")
	}
	parent := trace.SpanContext{}.WithTraceState(ts)
	ctx, span := otel.StartSpanWithRemoteParent(context.Background(), t.Name(), parent)
	defer span.End()
	r := NewRequestBase(ctx, t.Name())

	for _, tt := range requestBaseTests {
		t.Run(tt.n, func(t *testing.T) {
			r = tt.update(t, r)
			if len(r.OTelCarrier) == 0 {
				t.Fatal("expected non-empty trace context")
			}
			sc2 := trace.SpanContextFromContext(r.ExtractContext(context.Background()))
			if ts2 := sc2.TraceState(); ts2.String() != ts.String() {
				t.Fatalf("expected encoded TraceState: %q, got: %q", ts.String(), ts2.String())
			}
		})
	}
}
