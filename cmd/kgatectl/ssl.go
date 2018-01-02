package main

// TODO move this to a package

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"time"
)

const (
	// From Kubernetes:
	// ECPrivateKeyBlockType is a possible value for pem.Block.Type.
	ECPrivateKeyBlockType = "EC PRIVATE KEY"
)

func PrivateKeyPEM() (*ecdsa.PrivateKey, []byte) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal("Failed to generate the key: ", err)
	}

	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		log.Fatal("Unable to mashal EC key: ", err)
	}

	buf := &bytes.Buffer{}

	if err := pem.Encode(buf, &pem.Block{
		Type:  ECPrivateKeyBlockType,
		Bytes: b,
	}); err != nil {
		log.Fatal("Failed to write encode key: ", err)
	}

	return key, buf.Bytes()
}

func SelfSignedCertificatePEM(name, role string, ttlYears int, key *ecdsa.PrivateKey) []byte {
	notBefore := time.Now()
	notAfter := notBefore.AddDate(ttlYears, 0, 0).Truncate(24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(0xffffffff))
	if err != nil {
		log.Fatal("Failed to generate serial number: ", err)
	}

	certTemplate := &x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		Subject:               pkix.Name{},
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{},
		BasicConstraintsValid: true,
	}
	parentTemplate := certTemplate // self-signed
	publicKey := key.Public()

	derBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, parentTemplate, publicKey, key)
	if err != nil {
		log.Fatal("Failed to generate certificate: ", err)
	}

	buf := &bytes.Buffer{}

	if err := pem.Encode(buf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatal("Failed to write certificate: ", err)
	}

	return buf.Bytes()
}

func HostCertificatePEM(caData map[string][]byte, ttlYears int, key *ecdsa.PrivateKey, dnsNames ...string) []byte {
	caKey := loadPrivateKey(caData)
	caCrt := loadCertificate(caData)

	notBefore := time.Now()
	notAfter := notBefore.AddDate(ttlYears, 0, 0).Truncate(24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(0xffffffff))
	if err != nil {
		log.Fatal("Failed to generate serial number: ", err)
	}

	certTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		IsCA:         false,
		Subject: pkix.Name{
			CommonName: dnsNames[0],
		},
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
	}
	parentTemplate := caCrt
	publicKey := key.Public()

	derBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, parentTemplate, publicKey, caKey)
	if err != nil {
		log.Fatal("Failed to generate certificate: ", err)
	}

	f := &bytes.Buffer{}
	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatal("Failed to write certificate: ", err)
	}

	return f.Bytes()
}

func loadPrivateKey(secretData map[string][]byte) *ecdsa.PrivateKey {
	keyBytes := secretData["tls.key"]

	p, _ := pem.Decode(keyBytes)
	if p.Type != ECPrivateKeyBlockType {
		log.Fatal("Wrong type : ", p.Type)
	}
	key, err := x509.ParseECPrivateKey(p.Bytes)
	if err != nil {
		log.Fatal("Unable to parse key: ", err)
	}
	return key
}

func loadCertificate(secretData map[string][]byte) *x509.Certificate {
	crtBytes := secretData["tls.crt"]

	p, _ := pem.Decode(crtBytes)
	if p.Type != "CERTIFICATE" {
		log.Fatal("Wrong type: ", p.Type)
	}
	crt, err := x509.ParseCertificate(p.Bytes)
	if err != nil {
		log.Fatal("Unable to parse certificate: ", err)
	}

	return crt
}
