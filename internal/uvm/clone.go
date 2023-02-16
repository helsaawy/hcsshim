//go:build windows

package uvm

import (
	"context"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/cow"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/vm"
	"github.com/pkg/errors"
)

const (
	hcsComputeSystemSaveType = "AsTemplate"
	// default namespace ID used for all template and clone VMs.
	DefaultCloneNetworkNamespaceID = "89EB8A86-E253-41FD-9800-E6D88EB2E18A"
)

// UVMTemplateConfig is just a wrapper struct that keeps together all the resources that
// need to be saved to create a template.
type UVMTemplateConfig struct {
	// ID of the template vm
	UVMID string
	// Array of all resources that will be required while making a clone from this template
	Resources []vm.Cloneable
	// The OptionsWCOW used for template uvm creation
	CreateOpts OptionsWCOW
}

// Captures all the information that is necessary to properly save this UVM as a template
// and create clones from this template later. The struct returned by this method must be
// later on made available while creating a clone from this template.
func (uvm *UtilityVM) GenerateTemplateConfig() (*UVMTemplateConfig, error) {
	if _, ok := uvm.createOpts.(OptionsWCOW); !ok {
		return nil, fmt.Errorf("template config can only be created for a WCOW uvm")
	}

	// Add all the SCSI Mounts and VSMB shares into the list of clones
	templateConfig := &UVMTemplateConfig{
		UVMID:      uvm.ID(),
		CreateOpts: uvm.createOpts.(OptionsWCOW),
	}

	for _, share := range uvm.vsmb.Shares() {
		templateConfig.Resources = append(templateConfig.Resources, share)
	}

	for _, location := range uvm.scsiLocations {
		for _, scsiMount := range location {
			if scsiMount != nil {
				templateConfig.Resources = append(templateConfig.Resources, scsiMount)
			}
		}
	}

	return templateConfig, nil
}

// Pauses the uvm and then saves it as a template. This uvm can not be restarted or used
// after it is successfully saved.
// uvm must be in the paused state before it can be saved as a template.save call will throw
// an incorrect uvm state exception if uvm is not in the paused state at the time of saving.
func (uvm *UtilityVM) SaveAsTemplate(ctx context.Context) error {
	if err := uvm.Pause(ctx); err != nil {
		return errors.Wrap(err, "error pausing the VM")
	}

	if err := uvm.Save(ctx); err != nil {
		return errors.Wrap(err, "error saving the VM")
	}
	return nil
}

func (uvm *UtilityVM) Save(ctx context.Context) error {
	saveOptions := hcsschema.SaveOptions{
		SaveType: hcsComputeSystemSaveType,
	}
	return uvm.hcsSystem.Save(ctx, saveOptions)
}

// CloneContainer attaches back to a container that is already running inside the UVM
// because of the clone
func (uvm *UtilityVM) CloneContainer(ctx context.Context, id string) (cow.Container, error) {
	if uvm.gc == nil {
		return nil, fmt.Errorf("clone container cannot work without external GCS connection")
	}
	c, err := uvm.gc.CloneContainer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to clone container %s: %s", id, err)
	}
	return c, nil
}
