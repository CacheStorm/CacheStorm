package installscripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	installerSh  = "../../scripts/install.sh"
	installerPS1 = "../../scripts/install.ps1"

	// selectHookEnv tells the shell installer to run only its release
	// selection and print the chosen tag. The installers are piped into a
	// shell (`curl … | bash`), so they cannot source a helper file; this
	// documented entry point is how the selection logic is driven from a
	// test without running an install.
	selectHookEnv = "CACHESTORM_TEST_SELECT_LATEST"
)

// backDatedFixture is the trap this check exists for: v0.1.27 was
// published LAST (it was tagged and released long after v0.3.1), so the
// releases/latest endpoint — which returns the most recently PUBLISHED
// release — hands out the February-2026 binaries while v0.3.1 is the
// version users should get. The installer must pick by version.
//
// v0.4.0 is a draft and v0.3.1-rc.1 a prerelease, so neither may win.
const backDatedFixture = `[
  {
    "tag_name": "v0.3.1-rc.1",
    "published_at": "2026-10-18T13:19:56Z",
    "draft": false,
    "prerelease": true
  },
  {
    "tag_name": "v0.3.1",
    "published_at": "2026-10-22T16:18:09Z",
    "draft": false,
    "prerelease": false
  },
  {
    "tag_name": "v0.4.0",
    "published_at": "2026-11-01T09:00:00Z",
    "draft": true,
    "prerelease": false
  },
  {
    "tag_name": "v0.1.27",
    "published_at": "2026-10-22T21:33:05Z",
    "draft": false,
    "prerelease": false
  }
]`

// orderedFixture is the control: here the highest version is also the
// most recently published one, so version-based and date-based selection
// agree and the installer must still pick it.
const orderedFixture = `[
  {
    "tag_name": "v0.2.0",
    "published_at": "2026-03-21T00:00:00Z",
    "draft": false,
    "prerelease": false
  },
  {
    "tag_name": "v0.3.1",
    "published_at": "2026-10-22T16:18:09Z",
    "draft": false,
    "prerelease": false
  }
]`

// TestInstallersDoNotResolveLatestByPublishDate is the contract the
// installers must satisfy: they may not resolve their download target
// through the releases/latest endpoint, because that endpoint answers
// with the most recently PUBLISHED release, not the highest version. A
// version published late — a back-dated release — would then be offered
// to every user running the one-click installer.
func TestInstallersDoNotResolveLatestByPublishDate(t *testing.T) {
	for _, path := range []string{installerSh, installerPS1} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		if strings.Contains(string(data), "releases/latest") {
			t.Errorf("%s resolves its download target through the releases/latest endpoint, "+
				"which returns the most recently published release rather than the highest "+
				"version: a back-dated release would be installed instead of the newest one",
				path)
		}
	}
}

// TestShellInstallerSelectsTheHighestVersion drives the shell installer's
// selection against the back-dated fixture and requires the highest
// version, not the newest one.
func TestShellInstallerSelectsTheHighestVersion(t *testing.T) {
	got := runShellSelection(t, backDatedFixture)
	if got != "v0.3.1" {
		t.Errorf("the shell installer selected %q, want %q: the release published last "+
			"(v0.1.27) must not be installed in place of the highest version", got, "v0.3.1")
	}
}

// TestShellInstallerSelectionIgnoresDraftsAndPrereleases checks that a
// draft and a prerelease never win the selection, even when they carry a
// higher version than the release users should get.
func TestShellInstallerSelectionIgnoresDraftsAndPrereleases(t *testing.T) {
	fixture := `[
  {
    "tag_name": "v0.9.0",
    "published_at": "2026-12-01T00:00:00Z",
    "draft": false,
    "prerelease": true
  },
  {
    "tag_name": "v0.8.0",
    "published_at": "2026-12-01T00:00:00Z",
    "draft": true,
    "prerelease": false
  },
  {
    "tag_name": "v0.3.1",
    "published_at": "2026-10-22T16:18:09Z",
    "draft": false,
    "prerelease": false
  }
]`
	got := runShellSelection(t, fixture)
	if got != "v0.3.1" {
		t.Errorf("the shell installer selected %q, want %q: a draft and a prerelease must not win", got, "v0.3.1")
	}
}

// TestShellInstallerSelectionControl is the control case: with a list
// where the highest version is also the most recently published, the
// selection must still return it.
func TestShellInstallerSelectionControl(t *testing.T) {
	got := runShellSelection(t, orderedFixture)
	if got != "v0.3.1" {
		t.Errorf("the shell installer selected %q, want %q", got, "v0.3.1")
	}
}

// TestPowerShellInstallerSelectsTheHighestVersion runs the same contract
// against the Windows installer. It needs PowerShell, which the
// repository's CI runners have but a developer machine may not.
func TestPowerShellInstallerSelectsTheHighestVersion(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("pwsh is not available on the CI runner, so the PowerShell installer's selection would go unchecked: %v", err)
		}
		t.Skip("pwsh is not installed here; the PowerShell installer is exercised by the same check on the CI runners, which ship it")
	}
	got := runPowerShellSelection(t, pwsh, backDatedFixture)
	if got != "v0.3.1" {
		t.Errorf("the PowerShell installer selected %q, want %q: the release published last "+
			"(v0.1.27) must not be installed in place of the highest version", got, "v0.3.1")
	}
}

func runShellSelection(t *testing.T, fixture string) string {
	t.Helper()
	path := writeFixture(t, fixture)
	cmd := exec.Command("bash", installerSh)
	cmd.Env = append(os.Environ(), selectHookEnv+"="+path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the shell installer's selection: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func runPowerShellSelection(t *testing.T, pwsh, fixture string) string {
	t.Helper()
	path := writeFixture(t, fixture)
	cmd := exec.Command(pwsh, "-NoProfile", "-File", installerPS1, "-SelectOnly", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the PowerShell installer's selection: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func writeFixture(t *testing.T, fixture string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "releases.json")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return path
}
