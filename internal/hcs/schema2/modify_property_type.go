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

// ModifyPropertyType : Service property type modified by HcsModifyServiceSettings
type ModifyPropertyType string

// List of ModifyPropertyType
const (
	ModifyPropertyType_MEMORY                     ModifyPropertyType = "Memory"
	ModifyPropertyType_CPU_GROUP                  ModifyPropertyType = "CpuGroup"
	ModifyPropertyType_CACHE_ALLOCATION           ModifyPropertyType = "CacheAllocation"
	ModifyPropertyType_CACHE_MONITORING           ModifyPropertyType = "CacheMonitoring"
	ModifyPropertyType_CONTAINER_CREDENTIAL_GUARD ModifyPropertyType = "ContainerCredentialGuard"
	ModifyPropertyType_MEMORY_BW_ALLOCATION       ModifyPropertyType = "MemoryBwAllocation"
)