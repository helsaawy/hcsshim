package name

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnknownOS = errors.New("unknown OS")

type OS string

const (
	Invalid = OS("")
	Windows = OS("windows")
	Linux   = OS("linux")
)

var _osLookup = map[string]OS{
	"windows": Windows,
	"linux":   Linux,
}

func FromString(s string) (OS, error) {
	s = strings.ToLower(s)
	if os, ok := _osLookup[s]; ok {
		return os, nil
	}
	return Invalid, fmt.Errorf("invalid OS name %q: %w", s, ErrUnknownOS)
}

func (os OS) String() string {
	return string(os)
}

func (os *OS) MarshalText() ([]byte, error) {
	return []byte(*os), nil
}

func (os *OS) UnmarshalText(text []byte) (err error) {
	*os, err = FromString(string(text))
	return err
}

// IsLinux returns whether the os is Linux
func IsLinux(os string) bool {
	return Is(os, Linux)
}

// IsWindows returns whether the os is Windows
func IsWindows(os string) bool {
	return Is(os, Windows)
}

// Is returns whether the os string equals the given OS name
func Is(s string, os OS) bool {
	_os, _ := FromString(s)
	return _os == os
}
