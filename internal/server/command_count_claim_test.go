package server

import "testing"

// README, the website, docs/commands.md and snapcraft.yaml describe the
// command surface as "about 1,500" / "1,500+" commands. Moving outside that
// band makes those statements false, so this fails until they are updated.
func TestRegisteredCommandCountMatchesPublishedClaim(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	n := len(s.router.Commands())
	if n < 1450 || n >= 1550 {
		t.Fatalf("%d commands are registered; update the \"1,500\" claims in README.md, docs/commands.md, website/src and snapcraft.yaml, then this bound", n)
	}
}
