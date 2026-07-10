package utils

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
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
	path   string
	issuer *x509.Certificate
	mu     sync.RWMutex
	crl    *x509.RevocationList
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

func (c *CrlChecker) reload() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return fmt.Errorf("cannot read CRL file %q: %w", c.path, err)
	}
	if block, _ := pem.Decode(data); block != nil {
		data = block.Bytes // strip PEM wrapper if present; ParseRevocationList wants DER
	}
	rl, err := x509.ParseRevocationList(data)
	if err != nil {
		return fmt.Errorf("cannot parse CRL: %w", err)
	}
	if err := rl.CheckSignatureFrom(c.issuer); err != nil {
		return fmt.Errorf("CRL signature invalid: %w", err)
	}

	c.mu.Lock()
	c.crl = rl
	c.mu.Unlock()

	return nil
}

// WatchAndReload blocks, watching the CRL file for changes and reloading it whenever a
// Write, Create, Rename, or Remove event is received. It returns when ctx is canceled.
// Reload failures are logged as warnings and do not stop the watcher. Callers typically
// run this in a goroutine: go c.WatchAndReload(ctx, logger).
//
// Atomic file replacements (e.g. mv tmp.crl ca.crl) are handled correctly across platforms:
// on Linux/inotify the replacement fires Remove (IN_DELETE_SELF) on the old inode, while on
// macOS/kqueue it fires Rename. WatchAndReload re-adds the watch path on every event so the
// watcher follows the new inode in either case.
//
// WatchAndReload returns an error only if the initial watcher setup fails; runtime errors are
// surfaced through logger.
func (c *CrlChecker) WatchAndReload(ctx context.Context, logger *zap.SugaredLogger) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("cannot create watcher: %w", err)
	}
	if err := watcher.Add(c.path); err != nil {
		_ = watcher.Close()
		return fmt.Errorf("cannot watch CRL file: %w", err)
	}

	defer func() { _ = watcher.Close() }()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("CRL watcher channel closed unexpectedly")
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
				// Re-add path on every event: on Linux (inotify), atomic file
				// replacement (mv tmp.crl ca.crl) fires IN_DELETE_SELF (Remove) on
				// the old inode, not Rename — the watcher must re-add the path to
				// pick up the new inode. On macOS (kqueue) the same operation fires
				// Rename instead. Handling all four events keeps both platforms working.
				_ = watcher.Add(c.path)
				if err := c.reload(); err != nil {
					logger.Warnw("CRL reload failed", zap.Error(err))
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

// IsRevoked reports whether serial appears in the CRL. If the CRL's NextUpdate time has
// passed, the file is reloaded before the lookup so the check always operates on fresh data.
// An error is returned only if an expired CRL cannot be reloaded from disk.
func (c *CrlChecker) IsRevoked(serial *big.Int) (bool, error) {
	crl := c.getCRL()
	if crl.NextUpdate.Before(time.Now()) {
		return false, fmt.Errorf("CRL is outdated (NextUpdate=%s)", crl.NextUpdate)
	}

	for _, entry := range crl.RevokedCertificateEntries {
		if entry.SerialNumber.Cmp(serial) == 0 {
			return true, nil
		}
	}
	return false, nil
}

// getCRL returns the current CRL under a read lock.
// It is used internally to ensure that the CRL is not modified while being accessed.
func (c *CrlChecker) getCRL() *x509.RevocationList {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.crl
}
