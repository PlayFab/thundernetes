package controllers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestCert creates a self-signed certificate and returns PEM-encoded cert and key bytes.
func generateTestCert(t *testing.T, cn string) ([]byte, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

// writeCertFiles writes cert and key PEM bytes to the specified directory.
func writeCertFiles(t *testing.T, dir string, certPEM, keyPEM []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.crt"), certPEM, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.key"), keyPEM, 0600))
}

func TestCertificateWatcher_InitialLoad(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := generateTestCert(t, "initial-load-test")
	writeCertFiles(t, dir, certPEM, keyPEM)

	cw := NewCertificateWatcher(
		filepath.Join(dir, "tls.crt"),
		filepath.Join(dir, "tls.key"),
	)

	err := cw.LoadCertificate()
	require.NoError(t, err)

	// Verify GetCertificate returns a valid cert
	cert, err := cw.GetCertificate(nil)
	require.NoError(t, err)
	assert.NotNil(t, cert)

	// Verify GetConfigForClient returns a valid config
	cfg, err := cw.GetConfigForClient(nil)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	assert.NotNil(t, cfg.ClientCAs)
}

func TestCertificateWatcher_GetCertificateBeforeLoad(t *testing.T) {
	cw := NewCertificateWatcher("/nonexistent/tls.crt", "/nonexistent/tls.key")

	// GetCertificate should return an error if no cert has been loaded
	cert, err := cw.GetCertificate(nil)
	assert.Error(t, err)
	assert.Nil(t, cert)

	// GetConfigForClient should also return an error
	cfg, err := cw.GetConfigForClient(nil)
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestCertificateWatcher_MissingFiles(t *testing.T) {
	cw := NewCertificateWatcher("/nonexistent/tls.crt", "/nonexistent/tls.key")

	err := cw.LoadCertificate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading certificate file")
}

func TestCertificateWatcher_MissingKeyFile(t *testing.T) {
	dir := t.TempDir()
	certPEM, _ := generateTestCert(t, "missing-key-test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.crt"), certPEM, 0600))

	cw := NewCertificateWatcher(
		filepath.Join(dir, "tls.crt"),
		filepath.Join(dir, "tls.key"), // does not exist
	)

	err := cw.LoadCertificate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading key file")
}

func TestCertificateWatcher_InvalidCertContent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.crt"), []byte("not a cert"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.key"), []byte("not a key"), 0600))

	cw := NewCertificateWatcher(
		filepath.Join(dir, "tls.crt"),
		filepath.Join(dir, "tls.key"),
	)

	err := cw.LoadCertificate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing TLS key pair")
}

func TestCertificateWatcher_Reload(t *testing.T) {
	dir := t.TempDir()

	// Write initial cert
	certPEM1, keyPEM1 := generateTestCert(t, "cert-v1")
	writeCertFiles(t, dir, certPEM1, keyPEM1)

	cw := NewCertificateWatcher(
		filepath.Join(dir, "tls.crt"),
		filepath.Join(dir, "tls.key"),
	)

	err := cw.LoadCertificate()
	require.NoError(t, err)

	cert1, err := cw.GetCertificate(nil)
	require.NoError(t, err)

	// Write a new cert (different CN)
	certPEM2, keyPEM2 := generateTestCert(t, "cert-v2")
	writeCertFiles(t, dir, certPEM2, keyPEM2)

	// Reload
	err = cw.LoadCertificate()
	require.NoError(t, err)

	cert2, err := cw.GetCertificate(nil)
	require.NoError(t, err)

	// The two certs should be different
	assert.NotEqual(t, cert1.Certificate, cert2.Certificate)
}

func TestCertificateWatcher_StartDetectsFileChange(t *testing.T) {
	dir := t.TempDir()

	// Write initial cert
	certPEM1, keyPEM1 := generateTestCert(t, "start-test-v1")
	writeCertFiles(t, dir, certPEM1, keyPEM1)

	cw := NewCertificateWatcher(
		filepath.Join(dir, "tls.crt"),
		filepath.Join(dir, "tls.key"),
	)
	cw.SetPollInterval(50 * time.Millisecond) // fast polling for tests

	err := cw.LoadCertificate()
	require.NoError(t, err)

	cert1, err := cw.GetCertificate(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the watcher in a goroutine
	go func() {
		_ = cw.Start(ctx)
	}()

	// Wait a bit, then write a new cert
	time.Sleep(100 * time.Millisecond)
	certPEM2, keyPEM2 := generateTestCert(t, "start-test-v2")
	writeCertFiles(t, dir, certPEM2, keyPEM2)

	// Wait for the watcher to detect the change
	assert.Eventually(t, func() bool {
		cert2, err := cw.GetCertificate(nil)
		if err != nil {
			return false
		}
		return !certsEqual(cert1, cert2)
	}, 2*time.Second, 50*time.Millisecond, "watcher should detect cert file change and reload")

	cancel()
}

func TestCertificateWatcher_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := generateTestCert(t, "concurrent-test")
	writeCertFiles(t, dir, certPEM, keyPEM)

	cw := NewCertificateWatcher(
		filepath.Join(dir, "tls.crt"),
		filepath.Join(dir, "tls.key"),
	)

	err := cw.LoadCertificate()
	require.NoError(t, err)

	var wg sync.WaitGroup
	const concurrency = 50

	// Multiple readers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cert, err := cw.GetCertificate(nil)
				assert.NoError(t, err)
				assert.NotNil(t, cert)

				cfg, err := cw.GetConfigForClient(nil)
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		}()
	}

	// Concurrent reloaders
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				err := cw.LoadCertificate()
				assert.NoError(t, err)
			}
		}()
	}

	wg.Wait()
}

func TestCertificateWatcher_ReloadKeepsPreviousOnError(t *testing.T) {
	dir := t.TempDir()

	// Write valid cert
	certPEM, keyPEM := generateTestCert(t, "keep-previous-test")
	writeCertFiles(t, dir, certPEM, keyPEM)

	cw := NewCertificateWatcher(
		filepath.Join(dir, "tls.crt"),
		filepath.Join(dir, "tls.key"),
	)

	err := cw.LoadCertificate()
	require.NoError(t, err)

	cert1, err := cw.GetCertificate(nil)
	require.NoError(t, err)

	// Now write invalid cert content
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.crt"), []byte("invalid"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tls.key"), []byte("invalid"), 0600))

	// LoadCertificate should fail
	err = cw.LoadCertificate()
	assert.Error(t, err)

	// Previous cert should still be available
	cert2, err := cw.GetCertificate(nil)
	require.NoError(t, err)
	assert.Equal(t, cert1.Certificate, cert2.Certificate)
}

// certsEqual checks if two TLS certificates have the same raw certificate data.
func certsEqual(a, b *tls.Certificate) bool {
	if len(a.Certificate) != len(b.Certificate) {
		return false
	}
	for i := range a.Certificate {
		if len(a.Certificate[i]) != len(b.Certificate[i]) {
			return false
		}
		for j := range a.Certificate[i] {
			if a.Certificate[i][j] != b.Certificate[i][j] {
				return false
			}
		}
	}
	return true
}
