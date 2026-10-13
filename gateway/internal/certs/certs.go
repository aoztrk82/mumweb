// Package certs creates and loads Mumble client certificates.
//
// A Mumble identity is a self-signed certificate; the server identifies
// registered users by its hash. The gateway never stores certificates: they
// are generated for the browser, which keeps them and sends them back.
package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"
)

const (
	keyBits  = 2048
	validFor = 20 * 365 * 24 * time.Hour
)

// Generate creates an RSA 2048 self-signed client certificate with the given
// common name, valid for 20 years. It returns the PEM-encoded certificate and
// PKCS#8 private key.
func Generate(commonName string) (certPEM, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return nil, nil, fmt.Errorf("certs: generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, nil, fmt.Errorf("certs: serial: %w", err)
	}
	now := time.Now().Add(-time.Hour) // tolerate clock skew
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now,
		NotAfter:              now.Add(validFor),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("certs: create certificate: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("certs: marshal key: %w", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

// Load parses a PEM certificate and private key into a TLS certificate.
func Load(certPEM, keyPEM []byte) (*tls.Certificate, error) {
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, errors.New("certs: empty certificate or key")
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("certs: %w", err)
	}
	return &cert, nil
}
