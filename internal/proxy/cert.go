package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// LoadOrGenerateCA loads the CA cert/key from dataDir, or generates a new pair.
func LoadOrGenerateCA(dataDir string) (tls.Certificate, error) {
	certPath := filepath.Join(dataDir, "ca.pem")
	keyPath := filepath.Join(dataDir, "ca-key.pem")

	cert, err := loadCA(certPath, keyPath)
	if err == nil {
		return cert, nil
	}
	if !os.IsNotExist(err) {
		return tls.Certificate{}, fmt.Errorf("load CA: %w (delete %s to regenerate)", err, certPath)
	}
	return generateCA(certPath, keyPath)
}

func loadCA(certPath, keyPath string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	return cert, nil
}

func generateCA(certPath, keyPath string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "llm.log CA",
			Organization: []string{"llm.log"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	if err := writePEM(certPath, 0644, "CERTIFICATE", certDER); err != nil {
		return tls.Certificate{}, err
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal key: %w", err)
	}
	if err := writePEM(keyPath, 0600, "EC PRIVATE KEY", keyDER); err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}, nil
}

func writePEM(path string, perm os.FileMode, blockType string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: data}); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Close()
}

// CertPath returns the CA certificate path.
func CertPath(dataDir string) string {
	return filepath.Join(dataDir, "ca.pem")
}
