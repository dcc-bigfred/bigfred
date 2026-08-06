package cli

import (
	"testing"

	"github.com/sirupsen/logrus"
)

// Android still launches loco-server with --mdns=false after the in-process
// advertiser was removed. The flags must parse as deprecated no-ops.
func TestDeprecatedMDNSFlagsAccepted(t *testing.T) {
	t.Parallel()
	cmd := NewRootCommand(logrus.New())
	cmd.SetArgs([]string{"--mdns=false", "--mdns-host=phone"})
	if err := cmd.ParseFlags([]string{"--mdns=false", "--mdns-host=phone"}); err != nil {
		t.Fatalf("deprecated mDNS flags must remain accepted: %v", err)
	}
	mdns, err := cmd.Flags().GetBool("mdns")
	if err != nil {
		t.Fatalf("mdns flag: %v", err)
	}
	if mdns {
		t.Fatalf("expected --mdns=false, got true")
	}
	host, err := cmd.Flags().GetString("mdns-host")
	if err != nil {
		t.Fatalf("mdns-host flag: %v", err)
	}
	if host != "phone" {
		t.Fatalf("mdns-host: got %q", host)
	}
}
