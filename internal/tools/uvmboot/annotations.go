//go:build windows

package main

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
)

// parseAnnotations parses the (base64) annotation flags.
func parseAnnotationFlags(ctx context.Context, cCtx *cli.Context) map[string]string {
	vs := cCtx.StringSlice(annotationsArgName)
	b64s := cCtx.StringSlice(annotationsBase64ArgName)
	n := len(vs) + len(b64s)
	if n == 0 {
		// we don't want a nil map to be passed in the OCI spec, so return an empty one
		return map[string]string{}
	}

	annots := make(map[string]string, n)
	for _, x := range []struct {
		args     []string
		isBase64 bool
	}{
		{vs, false},
		{b64s, true},
	} {
		for _, s := range x.args {
			updateAnnotation(ctx, annots, s, x.isBase64)
		}
	}

	return annots
}

// updateAnnotation parses an individual annotation arg and, if valid, updates the annotations map.
func updateAnnotation(ctx context.Context, annotations map[string]string, arg string, isBase64 bool) {
	entry := log.G(ctx).WithFields(logrus.Fields{
		"flag-value": arg,
		"is-base64":  isBase64,
	})
	entry.Debug("parsing annotation")

	k, v, found := strings.Cut(arg, "=")
	if !found {
		entry.WithField(logrus.ErrorKey, "missing `=` in annotation").Warn("invald annotation flag argument")
		return
	}
	if k == "" || v == "" {
		entry.WithField(logrus.ErrorKey, "empty annotation key or value").Warnf("invald annotation flag argument")
		return
	}

	if isBase64 {
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			entry.WithError(err).Warn("invalid base64-encoded annotation value")
			return
		}
		v = string(b)
	}

	entry = entry.WithFields(logrus.Fields{
		logfields.Key:   k,
		logfields.Value: v,
	})
	entry.Debugf("parsed annotation")

	if vv, ok := annotations[k]; ok {
		entry.WithField(logfields.Value+"-existing", vv).Warn("overriding existing annotation")
	}
	annotations[k] = v
}
