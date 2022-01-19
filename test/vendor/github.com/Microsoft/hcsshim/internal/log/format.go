package log

import (
	"fmt"
	"net"
	"reflect"
	"time"
)

const TimeFormat = time.RFC3339Nano

func FormatTime(t time.Time) string {
	return t.Format(TimeFormat)
}

// FormatIO formats net.Conn and other types that have an `Addr()` or `Name()`
func FormatIO(v interface{}) string {
	s := reflect.TypeOf(v).String()

	switch t := v.(type) {
	case net.Conn:
		s += "(" + FormatAddr(t.LocalAddr()) + ")"
	case interface{ Addr() net.Addr }:
		s += "(" + FormatAddr(t.Addr()) + ")"
	case interface{ Name() string }:
		s += "(" + t.Name() + ")"
	default:
		return FormatAny(t)
	}

	return s
}

func FormatAddr(a net.Addr) string {
	return a.Network() + "://" + a.String()
}

func FormatAny(v interface{}) string {
	switch t := v.(type) {
	case fmt.GoStringer:
		return t.GoString()
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%+v", v)
	}
}
