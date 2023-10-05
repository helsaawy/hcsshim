//go:build windows && functional
// +build windows,functional

package functional

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"testing"

	ctrdoci "github.com/containerd/containerd/oci"

	"github.com/Microsoft/hcsshim/internal/winapi"
	"github.com/Microsoft/hcsshim/osversion"

	"github.com/Microsoft/hcsshim/test/internal/cmd"
	"github.com/Microsoft/hcsshim/test/internal/container"
	"github.com/Microsoft/hcsshim/test/internal/layers"
	testoci "github.com/Microsoft/hcsshim/test/internal/oci"
	"github.com/Microsoft/hcsshim/test/pkg/require"
)

func TestHostProcess(t *testing.T) {
	requireFeatures(t, featureContainer, featureWCOW, featureHostProcess)
	require.Build(t, osversion.RS5)

	ctx := namespacedContext()
	ls := windowsImageLayers(ctx, t)

	system := `NT AUTHORITY\System`
	localService := `NT AUTHORITY\LocalService`

	t.Run("whoami", func(t *testing.T) {
		username := currentUsername(ctx, t)
		t.Logf("current username: %s", username)

		// theres probably a better way to test for this *shrug*
		isSystem := strings.EqualFold(username, system)

		for _, tt := range []struct {
			name   string
			user   ctrdoci.SpecOpts
			whoiam string
		}{
			// Logging in as the current user may require a password.
			// No guarantee that Administrator, DefaultAccount, or Guest are enabled.
			// Best bet is to login into a service user account, which is only possible if running
			// from `NT AUTHORITY\System`
			{
				name:   "username",
				user:   ctrdoci.WithUser(system),
				whoiam: system,
			},
			{
				name:   "username",
				user:   ctrdoci.WithUser(localService),
				whoiam: localService,
			},
			{
				name:   "inherit",
				user:   testoci.HostProcessInheritUser(),
				whoiam: username,
			},
		} {
			t.Run(tt.name+"_"+tt.whoiam, func(t *testing.T) {
				if strings.HasPrefix(strings.ToLower(tt.whoiam), `nt authority\`) && !isSystem {
					t.Skipf("starting HostProcess with account %q as requires running tests as %q", tt.whoiam, system)
				}

				cID := testName(t, "container")
				scratch := layers.WCOWScratchDir(ctx, t, "")
				spec := testoci.CreateWindowsSpec(ctx, t, cID,
					testoci.DefaultWindowsSpecOpts(cID,
						ctrdoci.WithProcessCommandLine("cmd /c whoami"),
						testoci.WithWindowsLayerFolders(append(ls, scratch)),
						testoci.AsHostProcessContainer(),
						tt.user,
					)...)

				c, _, cleanup := container.Create(ctx, t, nil, spec, cID, hcsOwner)
				t.Cleanup(cleanup)

				io := cmd.NewBufferedIO()
				init := container.StartWithSpec(ctx, t, c, spec.Process, io)
				t.Cleanup(func() {
					container.Kill(ctx, t, c)
					container.Wait(ctx, t, c)
				})

				if e := cmd.Wait(ctx, t, init); e != 0 {
					t.Fatalf("got exit code %d, wanted %d", e, 0)
				}

				io.TestOutput(t, tt.whoiam, nil, true)
			})
		}

		t.Run("newgroup", func(t *testing.T) {
			// CreateProcessAsUser needs SE_INCREASE_QUOTA_NAME and SE_ASSIGNPRIMARYTOKEN_NAME
			// privileges, which we is not guaranteed for Administrators to have.
			// So, if not System or LocalService, skip.
			//
			// https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessasuserw
			if !isSystem {
				t.Skipf("starting HostProcess within a new localgroup requires running tests as %q", system)
			}

			cID := testName(t, "container")

			groupName := testName(t)
			newLocalGroup(ctx, t, groupName)

			scratch := layers.WCOWScratchDir(ctx, t, "")
			spec := testoci.CreateWindowsSpec(ctx, t, cID,
				testoci.DefaultWindowsSpecOpts(cID,
					ctrdoci.WithProcessCommandLine("cmd /c whoami"),
					testoci.WithWindowsLayerFolders(append(ls, scratch)),
					testoci.AsHostProcessContainer(),
					ctrdoci.WithUser(groupName),
				)...)

			c, _, cleanup := container.Create(ctx, t, nil, spec, cID, hcsOwner)
			t.Cleanup(cleanup)

			io := cmd.NewBufferedIO()
			init := container.StartWithSpec(ctx, t, c, spec.Process, io)
			t.Cleanup(func() {
				container.Kill(ctx, t, c)
				container.Wait(ctx, t, c)
			})

			if e := cmd.Wait(ctx, t, init); e != 0 {
				t.Fatalf("got exit code %d, wanted %d", e, 0)
			}

			expectedUser := cID[:winapi.UserNameCharLimit]
			io.TestOutput(t, expectedUser, nil, true)

			checkLocalGroupMember(ctx, t, groupName, expectedUser)
		})
	})

	t.Run("hostname", func(t *testing.T) {
		hostname, err := os.Hostname()
		if err != nil {
			t.Fatalf("could not get hostname: %v", err)
		}
		t.Logf("current hostname: %s", hostname)

		cID := testName(t, "container")

		scratch := layers.WCOWScratchDir(ctx, t, "")
		spec := testoci.CreateWindowsSpec(ctx, t, cID,
			testoci.DefaultWindowsSpecOpts(cID,
				ctrdoci.WithProcessCommandLine("cmd /c whoami"),
				testoci.WithWindowsLayerFolders(append(ls, scratch)),
				testoci.AsHostProcessContainer(),
				testoci.HostProcessInheritUser(),
			)...)

		c, _, cleanup := container.Create(ctx, t, nil, spec, cID, hcsOwner)
		t.Cleanup(cleanup)

		io := cmd.NewBufferedIO()
		init := container.StartWithSpec(ctx, t, c, spec.Process, io)
		t.Cleanup(func() {
			container.Kill(ctx, t, c)
			container.Wait(ctx, t, c)
		})

		if e := cmd.Wait(ctx, t, init); e != 0 {
			t.Fatalf("got exit code %d, wanted %d", e, 0)
		}

		io.TestOutput(t, hostname, nil, true)
	})
}

func newLocalGroup(ctx context.Context, tb testing.TB, name string) {
	c := exec.CommandContext(ctx, "net", "localgroup", name, "/add")
	if output, err := c.CombinedOutput(); err != nil {
		tb.Logf("command %q output: %s", c.String(), strings.TrimSpace(string(output)))
		tb.Fatalf("failed to create localgroup %q with: %v", name, err)
	}
	tb.Logf("created localgroup: %s", name)

	tb.Cleanup(func() {
		deleteLocalGroup(ctx, tb, name)
	})
}

func deleteLocalGroup(ctx context.Context, tb testing.TB, name string) {
	c := exec.CommandContext(ctx, "net", "localgroup", name, "/delete")
	if output, err := c.CombinedOutput(); err != nil {
		tb.Logf("command %q output: %s", c.String(), strings.TrimSpace(string(output)))
		tb.Fatalf("failed to delete localgroup %q: %v", name, err)
	}
	tb.Logf("deleted localgroup: %s", name)
}

// Checks if userName is present in the group `groupName`.
func checkLocalGroupMember(ctx context.Context, tb testing.TB, groupName, userName string) {
	c := exec.CommandContext(ctx, "net", "localgroup", groupName)
	b, err := c.CombinedOutput()
	output := strings.TrimSpace(string(b))
	if err != nil {
		tb.Logf("command %q output: %s", c.String(), output)
		tb.Fatalf("failed to check members for localgroup %q: %v", groupName, err)
	}
	if !strings.Contains(strings.ToLower(output), strings.ToLower(userName)) {
		tb.Logf("command %q output: %s", c.String(), output)
		tb.Fatalf("user %s not present in the local group %s", userName, groupName)
	}
}

func currentUsername(_ context.Context, tb testing.TB) string {
	tb.Helper()

	u, err := user.Current() // cached, so no need to save on lookup
	if err != nil {
		tb.Fatalf("could not lookup current user: %v", err)
	}
	return u.Username
}
