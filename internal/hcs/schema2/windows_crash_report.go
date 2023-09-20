// Autogenerated code; DO NOT EDIT.

// Schema retrieved from branch 'fe_release' and build '20348.1.210507-1500'.

/*
 * Schema Open API
 *
 * No description provided (generated by Swagger Codegen https://github.com/swagger-api/swagger-codegen)
 *
 * API version: 2.4
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package hcsschema

// Windows specific crash information
type WindowsCrashReport struct {
	// Path to a Windows memory dump file. This will contain the same path as the configured in the GuestCrashReporting device. This field is not valid unless the FinalPhase is Complete.
	DumpFile string `json:"DumpFile,omitempty"`
	// Major version as reported by the guest OS.
	OSMajorVersion uint32 `json:"OsMajorVersion,omitempty"`
	// Minor version as reported by the guest OS.
	OSMinorVersion uint32 `json:"OsMinorVersion,omitempty"`
	// Build number as reported by the guest OS.
	OSBuildNumber uint32 `json:"OsBuildNumber,omitempty"`
	// Service pack major version as reported by the guest OS.
	OSServicePackMajorVersion uint32 `json:"OsServicePackMajorVersion,omitempty"`
	// Service pack minor version as reported by the guest OS.
	OSServicePackMinorVersion uint32 `json:"OsServicePackMinorVersion,omitempty"`
	// Suite mask as reported by the guest OS.
	OSSuiteMask uint32 `json:"OsSuiteMask,omitempty"`
	// Product type as reported by the guest OS.
	OSProductType uint32 `json:"OsProductType,omitempty"`
	// Status of the crash dump. S_OK indicates success, other HRESULT values on error.
	Status     int32              `json:"Status,omitempty"`
	FinalPhase *WindowsCrashPhase `json:"FinalPhase,omitempty"`
}