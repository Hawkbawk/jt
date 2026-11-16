package screen

import "testing"

func TestExpireStatusClearsOnlyForTheLatestNotify(t *testing.T) {
	var c Layout
	c.Notify("copied", StatusTime)
	first := StatusExpiredMsg{seq: c.statusSeq}
	c.Notify("copied", StatusTime)
	second := StatusExpiredMsg{seq: c.statusSeq}

	c.ExpireStatus(first)
	if c.Status != "copied" {
		t.Fatalf("status = %q, want it kept after an outdated timer", c.Status)
	}

	c.ExpireStatus(second)
	if c.Status != "" {
		t.Fatalf("status = %q, want cleared by the latest timer", c.Status)
	}
}
