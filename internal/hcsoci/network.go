package hcsoci

import (
	"context"

	"github.com/Microsoft/hcsshim/internal/hns"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/Microsoft/hcsshim/internal/resources"
	"github.com/Microsoft/hcsshim/internal/uvm"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

func createNetworkNamespace(ctx context.Context, coi *createOptionsInternal, r *resources.Resources) (err error) {
	ctx, span := oc.StartTraceSpan(ctx, "hcsoci::createNetworkNamespace")
	defer func() { oc.SetSpanStatus(span, err); span.End() }()
	span.AddAttributes(trace.StringAttribute(logfields.ContainerID, coi.ID))

	netID, err := hns.CreateNamespace()
	if err != nil {
		return err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"netID":               netID,
		logfields.ContainerID: coi.ID,
	}).Info("created network namespace for container")

	r.SetNetNS(netID)
	r.SetCreatedNetNS(true)

	endpoints := make([]string, 0)
	for _, endpointID := range coi.Spec.Windows.Network.EndpointList {
		err = hns.AddNamespaceEndpoint(netID, endpointID)
		if err != nil {
			return err
		}
		log.G(ctx).WithFields(logrus.Fields{
			"netID":      netID,
			"endpointID": endpointID,
		}).Info("added network endpoint to namespace")
		endpoints = append(endpoints, endpointID)
	}
	r.Add(&uvm.NetworkEndpoints{EndpointIDs: endpoints, Namespace: netID})
	return nil
}
