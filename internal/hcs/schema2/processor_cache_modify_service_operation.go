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

// ProcessorCacheModifyServiceOperation : Enumeration of different supported service processor cache modification requests
type ProcessorCacheModifyServiceOperation string

// List of ProcessorCache_ModifyServiceOperation
const (
	ProcessorCacheModifyServiceOperation_SET_COS_BITMASK            ProcessorCacheModifyServiceOperation = "SetCosBitmask"
	ProcessorCacheModifyServiceOperation_SET_ROOT_COS               ProcessorCacheModifyServiceOperation = "SetRootCos"
	ProcessorCacheModifyServiceOperation_SET_ROOT_RMID              ProcessorCacheModifyServiceOperation = "SetRootRmid"
	ProcessorCacheModifyServiceOperation_SET_MBA_COS_THROTTLE_VALUE ProcessorCacheModifyServiceOperation = "SetMbaCosThrottleValue"
)