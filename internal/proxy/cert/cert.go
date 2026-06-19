package cert

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"strings"
	"time"
)

// leafValidity bounds how long a minted leaf certificate is valid. Leaves are
// regenerated when they fall within leafRenewBefore of expiry.
const (
	leafValidity    = 365 * 24 * time.Hour
	leafRenewBefore = 24 * time.Hour
)

type tlsCertEntry struct {
	cert    *tls.Certificate
	expires time.Time
}

// GetCertificate is a tls.Config.GetCertificate callback. It returns a leaf
// certificate for the host requested in the ClientHello, generating and caching
// one on demand.
func (a *Authority) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := hello.ServerName
	if host == "" {
		// No SNI: fall back to the connection's local address host.
		if hello.Conn != nil {
			if h, _, err := net.SplitHostPort(hello.Conn.LocalAddr().String()); err == nil {
				host = h
			}
		}
	}
	if host == "" {
		return nil, fmt.Errorf("cannot determine host for certificate")
	}
	return a.CertForHost(host)
}

// CertForHost returns a leaf certificate valid for host, signed by the CA. Hosts
// are cached; an entry close to expiry is regenerated.
func (a *Authority) CertForHost(host string) (*tls.Certificate, error) {
	host = normalizeHost(host)

	a.mu.RLock()
	entry, ok := a.cache[host]
	a.mu.RUnlock()
	if ok && time.Until(entry.expires) > leafRenewBefore {
		return entry.cert, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	// Re-check after acquiring the write lock.
	if entry, ok := a.cache[host]; ok && time.Until(entry.expires) > leafRenewBefore {
		return entry.cert, nil
	}

	cert, expires, err := a.mintLeaf(host)
	if err != nil {
		return nil, err
	}
	a.cache[host] = &tlsCertEntry{cert: cert, expires: expires}
	return cert, nil
}

func (a *Authority) mintLeaf(host string) (*tls.Certificate, time.Time, error) {
	serial, err := randomSerial()
	if err != nil {
		return nil, time.Time{}, err
	}

	now := time.Now()
	notAfter := now.Add(leafValidity)

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, a.caCert, &a.leafKey.PublicKey, a.caKey)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("create leaf certificate: %w", err)
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{der, a.caCert.Raw},
		PrivateKey:  a.leafKey,
		Leaf:        tmpl,
	}
	return tlsCert, notAfter, nil
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host
}
