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

// ExitInitiator : Initiator of an exit (guest, management client, etc.)
type ExitInitiator string

// List of ExitInitiator
const (
	ExitInitiator_NONE     ExitInitiator = "None"
	ExitInitiator_GUEST_OS ExitInitiator = "GuestOS"
	ExitInitiator_CLIENT   ExitInitiator = "Client"
	ExitInitiator_INTERNAL ExitInitiator = "Internal"
	ExitInitiator_UNKNOWN  ExitInitiator = "Unknown"
)