package certs

import (
	"crypto/x509"
	"testing"
	"time"
)

func TestGenerateAndLoad(t *testing.T) {
	certPEM, keyPEM, err := Generate("Ahmet")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := Load(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Subject.CommonName != "Ahmet" {
		t.Fatalf("CN = %q", leaf.Subject.CommonName)
	}
	if leaf.NotAfter.Before(time.Now().Add(19 * 365 * 24 * time.Hour)) {
		t.Fatalf("expires too early: %v", leaf.NotAfter)
	}
}

func TestLoadRejectsMismatchedKey(t *testing.T) {
	c1, _, _ := Generate("a")
	_, k2, _ := Generate("b")
	if _, err := Load(c1, k2); err == nil {
		t.Fatal("mismatched key accepted")
	}
	if _, err := Load(nil, nil); err == nil {
		t.Fatal("empty input accepted")
	}
}
