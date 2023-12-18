//go:build windows && functional
// +build windows,functional

package functional

import (
	"context"
	"strings"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/Microsoft/hcsshim/internal/protocol/guestrequest"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/pkg/ctrdtaskapi"

	"github.com/Microsoft/hcsshim/test/internal/util"
	"github.com/Microsoft/hcsshim/test/pkg/require"
	testuvm "github.com/Microsoft/hcsshim/test/pkg/uvm"
)

func TestUVM_Update_Resources(t *testing.T) {
	requireFeatures(t, featureUVM)
	requireAnyFeature(t, featureLCOW, featureWCOW)
	require.Build(t, osversion.RS5)

	ctx := util.Context(context.Background(), t)

	type config struct {
		feature    string
		createOpts func(_ context.Context, tb testing.TB) any
		name       string
		resource   any
		valid      bool
	}

	configs := make([]config, 0)
	for _, c := range []config{
		{
			name:     "Valid_LinuxResources",
			resource: &specs.LinuxResources{},
			valid:    true,
		},
		{
			name:     "Valid_WindowsResources",
			resource: &specs.WindowsResources{},
			valid:    true,
		},
		{
			name:     "Valid_PolicyFragment",
			resource: &ctrdtaskapi.PolicyFragment{},
			valid:    true,
		},
		{
			name:     "Invalid_Mount",
			resource: &specs.Mount{},
			valid:    false,
		},
		{
			name:     "Invalid_LCOWNetwork",
			resource: &guestrequest.NetworkModifyRequest{},
			valid:    false,
		},
	} {
		for _, cc := range []struct {
			feature    string
			createOpts func(context.Context, testing.TB) any
		}{
			{
				feature: featureLCOW,
				//nolint: thelper
				createOpts: func(_ context.Context, tb testing.TB) any { return defaultLCOWOptions(ctx, tb) },
			},
			{
				feature: featureWCOW,
				//nolint: thelper
				createOpts: func(ctx context.Context, tb testing.TB) any { return defaultWCOWOptions(ctx, tb) },
			},
		} {
			conf := c
			conf.feature = cc.feature
			conf.createOpts = cc.createOpts
			configs = append(configs, conf)
		}
	}

	for _, tc := range configs {
		t.Run(tc.feature+"_"+tc.name, func(t *testing.T) {
			requireFeatures(t, tc.feature)

			vm, cleanup := testuvm.Create(ctx, t, tc.createOpts(ctx, t))
			testuvm.Start(ctx, t, vm)
			defer cleanup(ctx)

			if err := vm.Update(ctx, tc.resource, nil); err != nil {
				if tc.valid {
					if strings.Contains(err.Error(), "invalid resource") {
						t.Fatalf("failed to update %s UVM constraints: %v", tc.feature, err)
					} else {
						t.Logf("ignored error: %v", err)
					}
				}
			} else if !tc.valid {
				t.Fatalf("expected error updating %s UVM constraints", tc.feature)
			}
		})
	}
}
