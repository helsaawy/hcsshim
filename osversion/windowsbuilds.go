package osversion

import "github.com/Microsoft/hcsshim/internal/os/version"

const (
	// RS1 (version 1607, codename "Redstone 1") corresponds to Windows Server
	// 2016 (ltsc2016) and Windows 10 (Anniversary Update).
	RS1 = version.RS1

	// RS2 (version 1703, codename "Redstone 2") was a client-only update, and
	// corresponds to Windows 10 (Creators Update).
	RS2 = version.RS2

	// RS3 (version 1709, codename "Redstone 3") corresponds to Windows Server
	// 1709 (Semi-Annual Channel (SAC)), and Windows 10 (Fall Creators Update).
	RS3 = version.RS3

	// RS4 (version 1803, codename "Redstone 4") corresponds to Windows Server
	// 1803 (Semi-Annual Channel (SAC)), and Windows 10 (April 2018 Update).
	RS4 = version.RS4

	// RS5 (version 1809, codename "Redstone 5") corresponds to Windows Server
	// 2019 (ltsc2019), and Windows 10 (October 2018 Update).
	RS5 = version.RS5

	// V19H1 (version 1903) corresponds to Windows Server 1903 (semi-annual
	// channel).
	V19H1 = version.V19H1

	// V19H2 (version 1909) corresponds to Windows Server 1909 (semi-annual
	// channel).
	V19H2 = version.V19H2

	// V20H1 (version 2004) corresponds to Windows Server 2004 (semi-annual
	// channel).
	V20H1 = version.V20H1

	// V20H2 corresponds to Windows Server 20H2 (semi-annual channel).
	V20H2 = version.V20H2

	// V21H1 corresponds to Windows Server 21H1 (semi-annual channel).
	V21H1 = version.V20H1

	// V21H2Win10 corresponds to Windows 10 (November 2021 Update).
	V21H2Win10 = version.V21H2Win10

	// V21H2Server corresponds to Windows Server 2022 (ltsc2022).
	V21H2Server = version.V21H2Server

	// V21H2Win11 corresponds to Windows 11 (original release).
	V21H2Win11 = version.V21H2Win11
)
