//go:build windows

package hcsoci

import (
	"context"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/resources"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/sirupsen/logrus"
)

func createNetworkNamespace(ctx context.Context, coi *createOptionsInternal, r *resources.Resources) error {
	op := "hcsoci::createNetworkNamespace"
	entry := log.G(ctx).WithField(logfields.ContainerID, coi.ID)
	entry.Trace(op)
	defer func() {
		entry.Trace(op + " finished adding network endpoints")
	}()

	ns, err := hcn.NewNamespace("").Create()
	if err != nil {
		return err
	}

	entry.WithFields(logrus.Fields{
		"netID": ns.Id,
	}).Debug("created network namespace for container")

	r.SetNetNS(ns.Id)
	r.SetCreatedNetNS(true)

	endpoints := make([]string, 0)
	for _, endpointID := range coi.Spec.Windows.Network.EndpointList {
		err = hcn.AddNamespaceEndpoint(ns.Id, endpointID)
		if err != nil {
			return err
		}
		entry.WithFields(logrus.Fields{
			"netID":      ns.Id,
			"endpointID": endpointID,
		}).Debug("added network endpoint to namespace")
		endpoints = append(endpoints, endpointID)
	}
	r.Add(&uvm.NetworkEndpoints{EndpointIDs: endpoints, Namespace: ns.Id})
	return nil
}
