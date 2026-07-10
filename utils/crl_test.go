package utils

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func testCA(t *testing.T) (*x509.Certificate, crypto.PrivateKey) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

func buildCRL(t *testing.T, ca *x509.Certificate, caKey crypto.PrivateKey, nextUpdate time.Time, revokedSerials ...*big.Int) []byte {
	entries := make([]x509.RevocationListEntry, 0, len(revokedSerials))
	for _, s := range revokedSerials {
		entries = append(entries, x509.RevocationListEntry{
			SerialNumber:   s,
			RevocationTime: time.Now().Add(-time.Hour),
		})
	}
	template := &x509.RevocationList{
		Number:                    big.NewInt(1),
		ThisUpdate:                time.Now().Add(-time.Hour),
		NextUpdate:                nextUpdate,
		RevokedCertificateEntries: entries,
	}
	signer, ok := caKey.(crypto.Signer)
	require.True(t, ok, "caKey must implement crypto.Signer")
	der, err := x509.CreateRevocationList(rand.Reader, template, ca, signer)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "X509 CRL", Bytes: der}))
	return buf.Bytes()
}

func createCRLFile(t *testing.T, ca *x509.Certificate, caKey crypto.PrivateKey, nextUpdate time.Time, revokedSerials ...*big.Int) string {
	f, err := os.CreateTemp("", "crl-*.pem")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.Write(buildCRL(t, ca, caKey, nextUpdate, revokedSerials...))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// atomicReplaceCRL writes a new CRL to a temp file then renames it over path,
// mirroring the production mv-based CRL update that exercises the fsnotify re-add branch.
func atomicReplaceCRL(t *testing.T, path string, ca *x509.Certificate, caKey crypto.PrivateKey, nextUpdate time.Time, revokedSerials ...*big.Int) {
	tmp, err := os.CreateTemp("", "crl-new-*.pem")
	require.NoError(t, err)
	_, err = tmp.Write(buildCRL(t, ca, caKey, nextUpdate, revokedSerials...))
	require.NoError(t, err)
	require.NoError(t, tmp.Close())
	require.NoError(t, os.Rename(tmp.Name(), path))
}

func TestNewCRLChecker(t *testing.T) {
	ca, caKey := testCA(t)
	future := time.Now().Add(24 * time.Hour)

	t.Run("Valid PEM CRL", func(t *testing.T) {
		path := createCRLFile(t, ca, caKey, future)
		checker, err := NewCRLChecker(path, ca)
		require.NoError(t, err)
		require.NotNil(t, checker)
	})

	t.Run("Valid DER CRL", func(t *testing.T) {
		template := &x509.RevocationList{
			Number:     big.NewInt(1),
			ThisUpdate: time.Now().Add(-time.Hour),
			NextUpdate: future,
		}
		signer, ok := caKey.(crypto.Signer)
		require.True(t, ok, "caKey must implement crypto.Signer")
		der, err := x509.CreateRevocationList(rand.Reader, template, ca, signer)
		require.NoError(t, err)
		f, err := os.CreateTemp("", "crl-*.der")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Remove(f.Name()) })
		_, err = f.Write(der)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		checker, err := NewCRLChecker(f.Name(), ca)
		require.NoError(t, err)
		require.NotNil(t, checker)
	})

	t.Run("Nonexistent file", func(t *testing.T) {
		_, err := NewCRLChecker("/nonexistent/crl.pem", ca)
		require.ErrorContains(t, err, "cannot read CRL file")
	})

	t.Run("Corrupt data", func(t *testing.T) {
		f, err := os.CreateTemp("", "crl-*.pem")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Remove(f.Name()) })
		_, err = f.Write([]byte("not a crl"))
		require.NoError(t, err)
		require.NoError(t, f.Close())

		_, err = NewCRLChecker(f.Name(), ca)
		require.ErrorContains(t, err, "cannot parse CRL")
	})

	t.Run("Wrong CA signature", func(t *testing.T) {
		otherCA, otherKey := testCA(t)
		path := createCRLFile(t, otherCA, otherKey, future)
		_, err := NewCRLChecker(path, ca)
		require.ErrorContains(t, err, "CRL signature invalid")
	})
}

func TestCRLChecker_IsRevoked(t *testing.T) {
	ca, caKey := testCA(t)
	revokedSerial := big.NewInt(42)
	validSerial := big.NewInt(43)
	future := time.Now().Add(24 * time.Hour)

	path := createCRLFile(t, ca, caKey, future, revokedSerial)
	checker, err := NewCRLChecker(path, ca)
	require.NoError(t, err)

	t.Run("Revoked serial", func(t *testing.T) {
		revoked, err := checker.IsRevoked(revokedSerial)
		require.NoError(t, err)
		require.True(t, revoked)
	})

	t.Run("Non-revoked serial", func(t *testing.T) {
		revoked, err := checker.IsRevoked(validSerial)
		require.NoError(t, err)
		require.False(t, revoked)
	})

	t.Run("Expired CRL", func(t *testing.T) {
		// Must be after ThisUpdate (-1h) but before now so the expiry branch triggers.
		past := time.Now().Add(-30 * time.Minute)

		// Build checker with expired CRL that contains revokedSerial.
		expiredPath := createCRLFile(t, ca, caKey, past, revokedSerial)
		expiredChecker, err := NewCRLChecker(expiredPath, ca)
		require.NoError(t, err)

		// Replace with an expired CRL that no longer lists revokedSerial.
		atomicReplaceCRL(t, expiredPath, ca, caKey, past)

		// IsRevoked detects expiry, throws an error even though the serial is in the CRL.
		_, err = expiredChecker.IsRevoked(revokedSerial)
		require.ErrorContains(t, err, "CRL is outdated (NextUpdate=")
	})
}

func TestCRLChecker_WatchAndReload(t *testing.T) {
	ca, caKey := testCA(t)
	revokedSerial := big.NewInt(42)
	future := time.Now().Add(24 * time.Hour)
	logger := zaptest.NewLogger(t).Sugar()

	t.Run("Reloads on atomic file replacement", func(t *testing.T) {
		path := createCRLFile(t, ca, caKey, future, revokedSerial)
		checker, err := NewCRLChecker(path, ca)
		require.NoError(t, err)

		revoked, err := checker.IsRevoked(revokedSerial)
		require.NoError(t, err)
		require.True(t, revoked)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		go func() {
			if err := checker.WatchAndReload(ctx, logger); err != nil {
				require.ErrorIs(t, err, context.Canceled)
			}
		}()

		// Simulate production CRL rotation via atomic rename.
		atomicReplaceCRL(t, path, ca, caKey, future)

		require.Eventually(t, func() bool {
			revoked, err := checker.IsRevoked(revokedSerial)
			logger.Debug("Checking if revoked serial ", revoked)
			return err == nil && !revoked
		}, 20*time.Second, 100*time.Millisecond)
	})
}
