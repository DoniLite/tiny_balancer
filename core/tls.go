package core

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

func (m *CertManager) isLocalHostLike(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "localhost") || strings.HasSuffix(name, ".test")
}

func (m *CertManager) GetCertificate(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if chi == nil || chi.ServerName == "" {
		return nil, errors.New("missing SNI")
	}
	name := strings.ToLower(chi.ServerName)
	if m.isLocalHostLike(name) {
		// self-signed for localhost-like
		m.mu.RLock()
		if cert, ok := m.selfStore[name]; ok {
			m.mu.RUnlock()
			return cert, nil
		}
		m.mu.RUnlock()
		cert, err := generateSelfSigned(name)
		if err != nil {
			return nil, err
		}
		m.mu.Lock()
		m.selfStore[name] = cert
		m.mu.Unlock()
		return cert, nil
	}
	// Public domain: Let certmagic manage/renew
	if err := m.cm.ManageSync(context.Background(), []string{name}); err != nil {
		return nil, fmt.Errorf("certmagic ManageSync: %w", err)
	}
	return m.cm.GetCertificate(chi)
}

func generateSelfSigned(host string) (*tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host, Organization: []string{"Mogoly Local CA"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &pair, nil
}
