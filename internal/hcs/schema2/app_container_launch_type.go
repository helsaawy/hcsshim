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

type AppContainerLaunchType string

// List of AppContainerLaunchType
const (
	AppContainerLaunchType_DEFAULT_                      AppContainerLaunchType = "Default"
	AppContainerLaunchType_NONE                          AppContainerLaunchType = "None"
	AppContainerLaunchType_APP_CONTAINER                 AppContainerLaunchType = "AppContainer"
	AppContainerLaunchType_LESS_PRIVILEGED_APP_CONTAINER AppContainerLaunchType = "LessPrivilegedAppContainer"
)