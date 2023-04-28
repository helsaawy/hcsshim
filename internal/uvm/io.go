//go:build windows

package uvm

// deal with parsing logs and spans from LCOW uVM

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/otel"
	"github.com/Microsoft/hcsshim/internal/otel/exporters/jsonwriter"
)

// TODO: switch over to protobufs full

var errInvalidInput = errors.New("invalid input")

// parses logrus entries and OTel spans serialized over stderr from LCOW uVM and re-emits them
func parseLCOWOutput(vmid string) func(r io.Reader) {
	return func(r io.Reader) {
		j := json.NewDecoder(r)
		x := gcsLogOrSpan{
			vmid: vmid,
		}

		for {
			x.reset()
			if err := j.Decode(&x); err != nil {
				// Something went wrong. Read the rest of the data as a single
				// string and log it at once -- it's probably a GCS panic stack.
				if !errors.Is(err, io.EOF) && !isDisconnectError(err) {
					logrus.WithFields(logrus.Fields{
						logfields.UVMID: vmid,
						logrus.ErrorKey: err,
					}).Error("gcs log read")
				}
				rest, _ := io.ReadAll(io.MultiReader(j.Buffered(), r))
				rest = bytes.TrimSpace(rest)
				if len(rest) != 0 {
					logrus.WithFields(logrus.Fields{
						logfields.UVMID: vmid,
						"stderr":        string(rest),
					}).Error("gcs terminated")
				}
				break
			}
			x.emit()
		}
	}
}

// decode either a log entry or span from JSON
type gcsLogOrSpan struct {
	vmid string
	e    gcsLogEntry
	s    jsonwriter.Span
	// !WARNING: keep in sync with the registered exporter in the GCS
}

func (x *gcsLogOrSpan) emit() {
	if x.e.Message != "" {
		// create a new logrus entry
		x.e.Fields[logfields.UVMID] = x.vmid
		x.e.Fields["vm.time"] = x.e.Time
		// WithFields will duplicate L, and create a copy of the fields
		log.L.WithFields(x.e.Fields).Log(x.e.Level, x.e.Message)
		return
	}

	// if its not a log entry, it should be a span
	otel.ExportSpan(x.s.Snapshot())
}

// empty out underlying storage
func (x *gcsLogOrSpan) reset() {
	// we only want to re-use attr and fields
	fs := x.e.Fields
	as := x.s.Attributes
	// empty storage but keep the underlying capacity
	for k := range fs {
		delete(fs, k)
	}
	x.e = gcsLogEntry{Fields: fs}
	x.s = jsonwriter.Span{Attributes: as[:0]}
}

func (x *gcsLogOrSpan) UnmarshalJSON(b []byte) error {
	// try the log entry first
	err := x.e.UnmarshalJSON(b)

	// if err is nil or something other than errInvalidInput, return
	if err == nil {
		return nil
	} else if !errors.Is(err, errInvalidInput) {
		return fmt.Errorf("unmarshal %T from %q: %w", x, string(b), err)
	}

	// data is not a LogEntry, it should be a Span then
	if err := json.Unmarshal(b, &x.s); err != nil {
		return fmt.Errorf("unmarshal %T fields from %q: %w", x.s, string(b), err)
	}
	if !x.s.Valid() {
		return fmt.Errorf("parsed invalid span (%#+v) from %q", x.s, string(b))
	}
	return nil
}

type gcsLogEntry struct {
	gcsLogEntryStandard
	Fields map[string]interface{}
}

// TODO: Change the GCS log format to include type information
// (e.g. by using a different encoding such as protobuf).
func (e *gcsLogEntry) UnmarshalJSON(b []byte) error {
	// Default the log level to info.
	e.Level = logrus.InfoLevel
	if err := json.Unmarshal(b, &e.gcsLogEntryStandard); err != nil {
		return fmt.Errorf("unmarshal %T from %q: %w", e, string(b), err)
	}
	if e.Message == "" {
		return errInvalidInput
	}
	if err := json.Unmarshal(b, &e.Fields); err != nil {
		return fmt.Errorf("unmarshal %T fields from %q: %w", e, string(b), err)
	}
	// Do not allow fatal or panic level errors to propagate.
	if e.Level < logrus.ErrorLevel {
		e.Level = logrus.ErrorLevel
	}
	// Clear special fields.
	delete(e.Fields, "time")
	delete(e.Fields, "level")
	delete(e.Fields, "msg")
	// Normalize floats to integers.
	for k, v := range e.Fields {
		if d, ok := v.(float64); ok && float64(int64(d)) == d {
			e.Fields[k] = int64(d)
		}
	}
	return nil
}

type gcsLogEntryStandard struct {
	Time    time.Time    `json:"time"`
	Level   logrus.Level `json:"level"`
	Message string       `json:"msg"`
}
