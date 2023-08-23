//go:build windows

package uvm

import (
	"context"
	"fmt"

	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/option"
	"github.com/Microsoft/hcsshim/pkg/annotations"
	"github.com/Microsoft/hcsshim/pkg/ctrdtaskapi"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func (uvm *UtilityVM) Update(ctx context.Context, data interface{}, annots map[string]string) error {
	var memoryLimitInBytes option.Option[uint64]
	var processorLimits option.Option[hcsschema.ProcessorLimits]

	switch resources := data.(type) {
	case *specs.WindowsResources:
		if resources.Memory != nil {
			memoryLimitInBytes = option.Some(*resources.Memory.Limit)
		}
		if resources.CPU != nil {
			processorLimits = &hcsschema.ProcessorLimits{}
			if resources.CPU.Maximum != nil {
				processorLimits.Limit = uint64(*resources.CPU.Maximum)
			}
			if resources.CPU.Shares != nil {
				processorLimits.Weight = uint64(*resources.CPU.Shares)
			}
		}
	case *specs.LinuxResources:
		if resources.Memory != nil {
			memoryLimitInBytes = option.Some(uint64(*resources.Memory.Limit))
		}
		if resources.CPU != nil {
			processorLimits = &hcsschema.ProcessorLimits{}
			if resources.CPU.Quota != nil {
				processorLimits.Limit = uint64(*resources.CPU.Quota)
			}
			if resources.CPU.Shares != nil {
				processorLimits.Weight = uint64(*resources.CPU.Shares)
			}
		}
	case *ctrdtaskapi.PolicyFragment:
		return uvm.InjectPolicyFragment(ctx, resources)
	default:
		return fmt.Errorf("invalid resource: %+v", resources)
	}

	if option.IsSome(memoryLimitInBytes) {
		if err := uvm.UpdateMemory(ctx, option.Unwrap(memoryLimitInBytes)); err != nil {
			return err
		}
	}
	if option.IsSome(processorLimits) {
		if err := uvm.UpdateCPULimits(ctx, processorLimits); err != nil {
			return err
		}
	}

	// Check if an annotation was sent to update cpugroup membership
	if cpuGroupID, ok := annots[annotations.CPUGroupID]; ok {
		if err := uvm.SetCPUGroup(ctx, cpuGroupID); err != nil {
			return err
		}
	}

	return nil
}
