package cert

import (
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreateAuthority_GeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()

	a, err := LoadOrCreateAuthority(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateAuthority: %v", err)
	}
	if a.caCert == nil || a.caKey == nil {
		t.Fatal("authority missing CA material")
	}
	if !a.caCert.IsCA {
		t.Error("CA certificate is not marked IsCA")
	}

	keyPath := filepath.Join(dir, caKeyFile)
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("CA key perms = %o, want 600", perm)
		}
	}

	// Loading again must reuse the same CA, not regenerate it.
	b, err := LoadOrCreateAuthority(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !b.caCert.Equal(a.caCert) {
		t.Error("reloaded CA differs from persisted CA")
	}
}

func TestCertForHost_SignedByCAAndCached(t *testing.T) {
	a, err := LoadOrCreateAuthority(t.TempDir())
	if err != nil {
		t.Fatalf("authority: %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(a.caCert)

	tests := []struct {
		name    string
		host    string
		wantIP  bool
		wantDNS string
	}{
		{name: "dns host", host: "example.com", wantDNS: "example.com"},
		{name: "host with port", host: "example.com:443", wantDNS: "example.com"},
		{name: "ip host", host: "127.0.0.1", wantIP: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := a.CertForHost(tt.host)
			if err != nil {
				t.Fatalf("CertForHost(%q): %v", tt.host, err)
			}

			leaf, err := x509.ParseCertificate(c.Certificate[0])
			if err != nil {
				t.Fatalf("parse leaf: %v", err)
			}

			if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
				t.Errorf("leaf does not verify against CA: %v", err)
			}

			if tt.wantIP {
				if len(leaf.IPAddresses) == 0 || !leaf.IPAddresses[0].Equal(net.ParseIP("127.0.0.1")) {
					t.Errorf("expected IP SAN, got %v", leaf.IPAddresses)
				}
			}
			if tt.wantDNS != "" {
				if err := leaf.VerifyHostname(tt.wantDNS); err != nil {
					t.Errorf("VerifyHostname(%q): %v", tt.wantDNS, err)
				}
			}
		})
	}

	// Cache: same host returns the identical pointer.
	c1, _ := a.CertForHost("cached.example")
	c2, _ := a.CertForHost("cached.example")
	if c1 != c2 {
		t.Error("expected cached certificate to be reused")
	}
}
