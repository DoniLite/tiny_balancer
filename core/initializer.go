// Copyright 2025 DoniLite. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	"crypto/tls"
	"log"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/caddyserver/certmagic"
)

var (
	cache  *certmagic.Cache
	logger Logger
)

func init() {
	cache = certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(cert certmagic.Certificate) (*certmagic.Config, error) {
			return certmagic.New(cache, certmagic.Config{
				OnEvent: onCertManagerEvent,
			}), nil
		},
	})
	logger = newLogger()
}

func getEndPointFromEnvConfig(envKey string) string {
	env := os.Getenv(envKey)

	if env == "production" {
		return certmagic.LetsEncryptProductionCA
	}

	return certmagic.LetsEncryptStagingCA

}

func NewProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	return proxy
}

func NewCertManager(cacheDir, email, envKey string) *CertManager {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		log.Printf("cert cache mkdir: %v", err)
	}
	cfg := certmagic.New(cache, certmagic.Config{})
	userACME := certmagic.NewACMEIssuer(cfg, certmagic.ACMEIssuer{
		CA:     getEndPointFromEnvConfig(envKey),
		Email:  email,
		Agreed: true,
	})
	storage := &certmagic.FileStorage{Path: cacheDir}
	cfg.Storage = storage
	cfg.Issuers = []certmagic.Issuer{userACME}
	return &CertManager{cm: cfg, selfStore: make(map[string]*tls.Certificate)}
}

func newLogger() Logger {
	return make(chan any, 100)
}

func GetLogger() Logger {
	if logger == nil {
		logger = newLogger()
		return logger
	}

	return logger
}
