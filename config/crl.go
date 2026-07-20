package config

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// CrlChecker loads a Certificate Revocation List (CRL) from a file, verifies its signature
// against a trusted CA certificate, and exposes [CrlChecker.IsRevoked] for serial number
// lookups. The in-memory CRL is protected by a read/write mutex so that background reloads
// via [CrlChecker.WatchAndReload] are safe to run concurrently with ongoing checks.
type CrlChecker struct {
	mu         sync.RWMutex
	path       string
	issuer     *x509.Certificate
	crlExpiry  time.Time
	crlRevoked map[string]struct{}
}

// NewCRLChecker creates a [CrlChecker] by loading the CRL at path and verifying its signature
// against issuer. The file may be PEM- or DER-encoded. An error is returned if the file cannot
// be read, is not a valid CRL, or its signature does not match issuer.
func NewCRLChecker(path string, issuer *x509.Certificate) (*CrlChecker, error) {
	c := &CrlChecker{path: path, issuer: issuer}
	if err := c.reload(); err != nil {
		return nil, err
	}
	return c, nil
}

// reload reads the CRL file, verifies its signature against the issuer, and updates the in-memory CRL.
func (c *CrlChecker) reload() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return fmt.Errorf("cannot read CRL file: %w", err)
	}
	if block, _ := pem.Decode(data); block != nil {
		data = block.Bytes // strip PEM wrapper if present; ParseRevocationList wants DER
	}
	rl, err := x509.ParseRevocationList(data)
	if err != nil {
		return fmt.Errorf("cannot parse CRL: %w", err)
	}

	err = rl.CheckSignatureFrom(c.issuer)
	if err != nil {
		return fmt.Errorf("CRL signature invalid: %w", err)
	}

	revoked := make(map[string]struct{})
	for _, entry := range rl.RevokedCertificateEntries {
		revoked[entry.SerialNumber.Text(16)] = struct{}{}
	}

	c.mu.Lock()
	c.crlExpiry = rl.NextUpdate
	c.crlRevoked = revoked
	c.mu.Unlock()

	return nil
}

// WatchAndReload blocks, watching the CRL file for changes and reloading it whenever a
// Write, Create, or Rename event is received. It returns when ctx is canceled.
// Reload failures are logged as warnings and do not stop the watcher.
func (c *CrlChecker) WatchAndReload(ctx context.Context, logger *zap.SugaredLogger) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("cannot create watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()
	if err := watcher.Add(filepath.Dir(c.path)); err != nil {
		return fmt.Errorf("cannot watch CRL file: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("CRL watcher channel closed unexpectedly")
			}

			if filepath.Base(event.Name) != filepath.Base(c.path) {
				continue
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				if err := c.reload(); err != nil {
					logger.Warnw("CRL reload failed", zap.Error(err))
				} else {
					logger.Info("CRL reloaded successfully")
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("CRL watcher error channel closed unexpectedly")
			}
			logger.Warnw("CRL watcher error", zap.Error(err))
		}
	}
}

// IsRevoked reports whether serial appears in the CRL.
//
// It returns an error if the CRL is outdated (NextUpdate in the past).
func (c *CrlChecker) IsRevoked(serial *big.Int) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if time.Now().After(c.crlExpiry) {
		return false, fmt.Errorf("CRL is outdated (NextUpdate=%s)", c.crlExpiry)
	}

	_, revoked := c.crlRevoked[serial.Text(16)]
	return revoked, nil
}
