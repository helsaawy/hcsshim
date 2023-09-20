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

// Configuration for a process dump using MiniDumpWriteDump
type ProcessDump struct {
	Type_ *ProcessDumpType `json:"Type,omitempty"`
	// Custom MINIDUMP_TYPE flags used if Type is ProcessDumpType::Custom
	CustomDumpFlags uint32 `json:"CustomDumpFlags,omitempty"`
	// Path to create the dump file. The file must not exist.
	DumpFileName string `json:"DumpFileName,omitempty"`
}