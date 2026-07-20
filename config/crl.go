package config

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// CrlChecker loads a Certificate Revocation List (CRL) from a file, verifies its signature
// against a trusted CA certificate, and exposes [CrlChecker.IsRevoked] for serial number
// lookups. The in-memory CRL is protected by a read/write mutex so that background reloads
// via [CrlChecker.WatchAndReload] are safe to run concurrently with ongoing checks.
type CrlChecker struct {
	path    string
	issuers []*x509.Certificate
	mu      sync.RWMutex
	crl     *x509.RevocationList
}

// NewCRLChecker creates a [CrlChecker] by loading the CRL at path and verifying its signature
// against issuer. The file may be PEM- or DER-encoded. An error is returned if the file cannot
// be read, is not a valid CRL, or its signature does not match issuer.
//
// If a CA bundle is used to sign the CRL, it must contain exactly one certificate, the issuer of the CRL.
// Multiple certificates in the bundle will result in an error.
func NewCRLChecker(path string, issuers ...*x509.Certificate) (*CrlChecker, error) {
	if len(issuers) != 1 {
		return nil, fmt.Errorf("expected exactly one issuer certificate, got %d", len(issuers))
	}

	c := &CrlChecker{path: path, issuers: issuers}
	if err := c.reload(); err != nil {
		return nil, err
	}
	return c, nil
}

// reload reads the CRL file, verifies its signature against the issuer, and updates the in-memory CRL.
func (c *CrlChecker) reload() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		c.mu.RLock()
		defer c.mu.RUnlock()
		if c.crl == nil {
			// avoid nil pointer panic if CrlChecker is created without the factory method
			return fmt.Errorf("cannot read initial CRL file %q: %w", c.path, err)
		}
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("cannot read CRL file %q: %w", c.path, err)
		}
		return nil
	}
	if block, _ := pem.Decode(data); block != nil {
		data = block.Bytes // strip PEM wrapper if present; ParseRevocationList wants DER
	}
	rl, err := x509.ParseRevocationList(data)
	if err != nil {
		return fmt.Errorf("cannot parse CRL: %w", err)
	}

	if len(c.issuers) == 0 {
		return fmt.Errorf("no issuer certificates provided to verify CRL signature")
	} else if len(c.issuers) > 1 {
		return fmt.Errorf("multiple issuer certificates provided; expected exactly one")
	}

	if signatureErr := rl.CheckSignatureFrom(c.issuers[0]); signatureErr != nil {
		return fmt.Errorf("CRL signature invalid: %w", signatureErr)
	}

	c.mu.Lock()
	c.crl = rl
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
// It returns an error if the CRL is outdated (NextUpdate in the past) or if the CRL cannot be accessed.
func (c *CrlChecker) IsRevoked(serial *big.Int) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.crl == nil {
		return false, fmt.Errorf("CRL is not loaded")
	}

	if c.crl.NextUpdate.Before(time.Now()) {
		return false, fmt.Errorf("CRL is outdated (NextUpdate=%s)", c.crl.NextUpdate)
	}

	for _, entry := range c.crl.RevokedCertificateEntries {
		if entry.SerialNumber.Cmp(serial) == 0 {
			return true, nil
		}
	}

	return false, nil
}
