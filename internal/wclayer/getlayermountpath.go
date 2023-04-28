//go:build windows

package wclayer

import (
	"context"
	"syscall"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/otel"
	"go.opentelemetry.io/otel/attribute"
)

// GetLayerMountPath will look for a mounted layer with the given path and return
// the path at which that layer can be accessed.  This path may be a volume path
// if the layer is a mounted read-write layer, otherwise it is expected to be the
// folder path at which the layer is stored.
func GetLayerMountPath(ctx context.Context, path string) (_ string, err error) {
	title := "hcsshim::GetLayerMountPath"
	ctx, span := otel.StartSpan(ctx, title)
	defer func() { otel.SetSpanStatusAndEnd(span, err) }()
	span.SetAttributes(attribute.String("path", path))

	var mountPathLength uintptr = 0

	// Call the procedure itself.
	log.G(ctx).Debug("Calling proc (1)")
	err = getLayerMountPath(&stdDriverInfo, path, &mountPathLength, nil)
	if err != nil {
		return "", hcserror.New(err, title, "(first call)")
	}

	// Allocate a mount path of the returned length.
	if mountPathLength == 0 {
		return "", nil
	}
	mountPathp := make([]uint16, mountPathLength)
	mountPathp[0] = 0

	// Call the procedure again
	log.G(ctx).Debug("Calling proc (2)")
	err = getLayerMountPath(&stdDriverInfo, path, &mountPathLength, &mountPathp[0])
	if err != nil {
		return "", hcserror.New(err, title, "(second call)")
	}

	mountPath := syscall.UTF16ToString(mountPathp[0:])
	span.SetAttributes(attribute.String("mountPath", mountPath))
	return mountPath, nil
}
