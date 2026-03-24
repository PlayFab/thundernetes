package controllers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// defaultCertPollInterval is how often the CertificateWatcher checks for cert file changes
	defaultCertPollInterval = 30 * time.Second
)

// CertificateWatcher watches certificate files on disk and reloads them when they change.
// It is designed to work with Kubernetes Secret volume mounts, where kubelet atomically
// rotates the files via symlink swaps.
// CertificateWatcher implements the manager.Runnable interface so it can be added to the controller manager.
type CertificateWatcher struct {
	mu           sync.RWMutex
	currentCert  *tls.Certificate
	caCertPool   *x509.CertPool
	certPath     string
	keyPath      string
	pollInterval time.Duration
	logger       logr.Logger
}

// NewCertificateWatcher creates a new CertificateWatcher that monitors the given cert and key files.
func NewCertificateWatcher(certPath, keyPath string) *CertificateWatcher {
	return &CertificateWatcher{
		certPath:     certPath,
		keyPath:      keyPath,
		pollInterval: defaultCertPollInterval,
		logger:       log.Log.WithName("cert-watcher"),
	}
}

// SetPollInterval sets the polling interval for file change detection.
// Must be called before Start().
func (cw *CertificateWatcher) SetPollInterval(d time.Duration) {
	cw.pollInterval = d
}

// LoadCertificate reads and parses the certificate and key from disk.
// It is safe to call concurrently.
func (cw *CertificateWatcher) LoadCertificate() error {
	certPEM, err := os.ReadFile(cw.certPath)
	if err != nil {
		return fmt.Errorf("reading certificate file %s: %w", cw.certPath, err)
	}
	keyPEM, err := os.ReadFile(cw.keyPath)
	if err != nil {
		return fmt.Errorf("reading key file %s: %w", cw.keyPath, err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("parsing TLS key pair: %w", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(certPEM)

	cw.mu.Lock()
	cw.currentCert = &cert
	cw.caCertPool = caCertPool
	cw.mu.Unlock()

	cw.logger.Info("loaded TLS certificate", "certPath", cw.certPath, "keyPath", cw.keyPath)
	return nil
}

// GetCertificate returns the current TLS certificate.
// It is intended to be used as the tls.Config.GetCertificate callback.
func (cw *CertificateWatcher) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	if cw.currentCert == nil {
		return nil, fmt.Errorf("no certificate loaded")
	}
	return cw.currentCert, nil
}

// GetConfigForClient returns a tls.Config with the current CA cert pool for client certificate verification.
// It is intended to be used as the tls.Config.GetConfigForClient callback.
func (cw *CertificateWatcher) GetConfigForClient(_ *tls.ClientHelloInfo) (*tls.Config, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	if cw.caCertPool == nil {
		return nil, fmt.Errorf("no CA certificate pool loaded")
	}
	return &tls.Config{
		ClientCAs:          cw.caCertPool,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		GetCertificate:     cw.GetCertificate,
		GetConfigForClient: nil, // avoid recursive call
	}, nil
}

// Start implements the manager.Runnable interface.
// It polls the cert files for changes and reloads them when modified.
func (cw *CertificateWatcher) Start(ctx context.Context) error {
	cw.logger.Info("starting certificate watcher", "certPath", cw.certPath, "keyPath", cw.keyPath, "pollInterval", cw.pollInterval)

	ticker := time.NewTicker(cw.pollInterval)
	defer ticker.Stop()

	var lastCertModTime, lastKeyModTime time.Time

	// Initialize modification times
	if info, err := os.Stat(cw.certPath); err == nil {
		lastCertModTime = info.ModTime()
	}
	if info, err := os.Stat(cw.keyPath); err == nil {
		lastKeyModTime = info.ModTime()
	}

	for {
		select {
		case <-ctx.Done():
			cw.logger.Info("stopping certificate watcher")
			return nil
		case <-ticker.C:
			changed := false
			if info, err := os.Stat(cw.certPath); err == nil {
				if !info.ModTime().Equal(lastCertModTime) {
					lastCertModTime = info.ModTime()
					changed = true
				}
			}
			if info, err := os.Stat(cw.keyPath); err == nil {
				if !info.ModTime().Equal(lastKeyModTime) {
					lastKeyModTime = info.ModTime()
					changed = true
				}
			}
			if changed {
				cw.logger.Info("certificate file change detected, reloading")
				if err := cw.LoadCertificate(); err != nil {
					cw.logger.Error(err, "failed to reload certificate, keeping previous certificate")
				}
			}
		}
	}
}
