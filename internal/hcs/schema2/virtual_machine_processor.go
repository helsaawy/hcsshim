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

type VirtualMachineProcessor struct {
	Count       uint32 `json:"Count,omitempty"`
	Limit       uint64 `json:"Limit,omitempty"`
	Weight      uint64 `json:"Weight,omitempty"`
	Reservation uint64 `json:"Reservation,omitempty"`
	// Provides the target maximum CPU frequency, in MHz, for a virtual machine.
	MaximumFrequencyMHz            uint32 `json:"MaximumFrequencyMHz,omitempty"`
	ExposeVirtualizationExtensions bool   `json:"ExposeVirtualizationExtensions,omitempty"`
	EnablePerfmonPmu               bool   `json:"EnablePerfmonPmu,omitempty"`
	EnablePerfmonPebs              bool   `json:"EnablePerfmonPebs,omitempty"`
	EnablePerfmonLbr               bool   `json:"EnablePerfmonLbr,omitempty"`
	EnablePerfmonIpt               bool   `json:"EnablePerfmonIpt,omitempty"`
	SynchronizeHostFeatures        bool   `json:"SynchronizeHostFeatures,omitempty"`
	EnableSchedulerAssist          bool   `json:"EnableSchedulerAssist,omitempty"`
	DefaultVpCpuPriority           uint32 `json:"DefaultVpCpuPriority,omitempty"`
	EnableProcessorIdling          bool   `json:"EnableProcessorIdling,omitempty"`
	// Useful to enable if you want to protect against the Intel Processor Machine Check Error vulnerability (CVE-2018-12207). For instance, if you have some virtual machines that you trust that won't cause denial of service on the virtualization hosts and some that you don't trust. Additionally, disabling this may improve guest performance for some workloads. This feature does nothing on non-Intel machines or on Intel machines that are not vulnerable to CVE-2018-12207.
	EnablePageShattering bool `json:"EnablePageShattering,omitempty"`
	// Hides the presence of speculation controls commonly used by guest operating systems as part of side channel vulnerability mitigations. Additionally, these mitigations are often detrimental to guest operating system performance
	DisableSpeculationControls bool                 `json:"DisableSpeculationControls,omitempty"`
	ProcessorFeatureSet        *ProcessorFeatureSet `json:"ProcessorFeatureSet,omitempty"`
	CpuGroup                   *CpuGroup            `json:"CpuGroup,omitempty"`
}