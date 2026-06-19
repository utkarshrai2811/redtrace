// Package cert generates and manages the RedTrace certificate authority and the
// per-host leaf certificates used to intercept TLS traffic.
package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	caCertFile = "redtrace-ca.pem"
	caKeyFile  = "redtrace-ca-key.pem"

	caKeyBits   = 2048
	leafKeyBits = 2048

	caValidity = 10 * 365 * 24 * time.Hour
)

// Authority is the RedTrace certificate authority. It holds the CA certificate
// and private key and mints short-lived leaf certificates for intercepted
// hosts, caching them in memory. Authority is safe for concurrent use.
type Authority struct {
	caCert *x509.Certificate
	caKey  *rsa.PrivateKey

	// leafKey is a single private key reused across all leaf certificates. Only
	// the certificate (and its SANs) differ per host, which keeps interception
	// fast without weakening the model — the key never leaves this process.
	leafKey *rsa.PrivateKey

	mu    sync.RWMutex
	cache map[string]*tlsCertEntry
}

// LoadOrCreateAuthority loads the CA from dir, generating and persisting a new
// one on first run. The directory is created with 0700 permissions and the CA
// private key is written with 0600 permissions.
func LoadOrCreateAuthority(dir string) (*Authority, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create ca dir: %w", err)
	}

	certPath := filepath.Join(dir, caCertFile)
	keyPath := filepath.Join(dir, caKeyFile)

	cert, key, err := loadCA(certPath, keyPath)
	switch {
	case err == nil:
		// loaded existing CA
	case errors.Is(err, os.ErrNotExist):
		cert, key, err = generateCA()
		if err != nil {
			return nil, fmt.Errorf("generate ca: %w", err)
		}
		if err := persistCA(certPath, keyPath, cert, key); err != nil {
			return nil, fmt.Errorf("persist ca: %w", err)
		}
	default:
		return nil, fmt.Errorf("load ca: %w", err)
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, leafKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate leaf key: %w", err)
	}

	return &Authority{
		caCert:  cert,
		caKey:   key,
		leafKey: leafKey,
		cache:   make(map[string]*tlsCertEntry),
	}, nil
}

// CACertPEM returns the PEM-encoded CA certificate, suitable for installing into
// an OS or browser trust store.
func (a *Authority) CACertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.caCert.Raw})
}

func loadCA(certPath, keyPath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(certPath) //nolint:gosec // G304: path is derived from the configured CA directory, not user input
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(keyPath) //nolint:gosec // G304: path is derived from the configured CA directory, not user input
	if err != nil {
		return nil, nil, err
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, errors.New("invalid CA certificate PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, errors.New("invalid CA key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA key: %w", err)
	}

	return cert, key, nil
}

func generateCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, caKeyBits)
	if err != nil {
		return nil, nil, err
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "RedTrace CA",
			Organization: []string{"RedTrace"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func persistCA(certPath, keyPath string, cert *x509.Certificate, key *rsa.PrivateKey) error {
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	// The CA certificate is public material and is intentionally world-readable
	// so other tools and trust stores can import it.
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil { //nolint:gosec // G306: the CA cert is public and intentionally world-readable
		return err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return err
	}
	return nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}
