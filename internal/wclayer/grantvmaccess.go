//go:build windows

package wclayer

import (
	"context"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/otel"
	"go.opentelemetry.io/otel/attribute"
)

// GrantVmAccess adds access to a file for a given VM
func GrantVmAccess(ctx context.Context, vmid string, filepath string) (err error) {
	title := "hcsshim::GrantVmAccess"
	ctx, span := otel.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(
		attribute.String("vm-id", vmid),
		attribute.String("path", filepath))

	err = grantVmAccess(vmid, filepath)
	if err != nil {
		return hcserror.New(err, title, "")
	}
	return nil
}
