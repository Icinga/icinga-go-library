package config

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"math/big"
	"os"
	"testing"
	"time"
)

func TestTLS_MakeConfig(t *testing.T) {
	t.Run("TLS disabled", func(t *testing.T) {
		tlsConfig := &TLS{Enable: false}
		config, err := tlsConfig.MakeConfig("icinga.com")
		require.NoError(t, err)
		require.Nil(t, config)
	})

	t.Run("Server name", func(t *testing.T) {
		tlsConfig := &TLS{Enable: true}
		config, err := tlsConfig.MakeConfig("icinga.com")
		require.NoError(t, err)
		require.NotNil(t, config)
		require.Equal(t, "icinga.com", config.ServerName)
	})

	t.Run("Empty server name", func(t *testing.T) {
		t.Skip("TODO: Either ServerName or InsecureSkipVerify must be specified in the tls.Config and" +
			" should be verified in MakeConfig.")
	})

	t.Run("Insecure skip verify", func(t *testing.T) {
		tlsConfig := &TLS{Enable: true, Insecure: true}
		config, err := tlsConfig.MakeConfig("icinga.com")
		require.NoError(t, err)
		require.NotNil(t, config)
		require.True(t, config.InsecureSkipVerify)
	})

	t.Run("Missing client certificate", func(t *testing.T) {
		tlsConfig := &TLS{Enable: true, Key: "test.key"}
		_, err := tlsConfig.MakeConfig("icinga.com")
		require.Error(t, err)
	})

	t.Run("Missing private key", func(t *testing.T) {
		tlsConfig := &TLS{Enable: true, Cert: "test.crt"}
		_, err := tlsConfig.MakeConfig("icinga.com")
		require.Error(t, err)
	})

	t.Run("x509", func(t *testing.T) {
		cert, key, err := generateCert("cert", generateCertOptions{})
		require.NoError(t, err)
		certFile, err := os.CreateTemp("", "cert-*.pem")
		require.NoError(t, err)
		defer func(name string) {
			_ = os.Remove(name)
		}(certFile.Name())
		err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		require.NoError(t, err)

		keyFile, err := os.CreateTemp("", "key-*.pem")
		require.NoError(t, err)
		defer func(name string) {
			_ = os.Remove(name)
		}(keyFile.Name())
		keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		err = pem.Encode(keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
		require.NoError(t, err)

		ca, _, err := generateCert("ca", generateCertOptions{ca: true})
		require.NoError(t, err)
		caFile, err := os.CreateTemp("", "ca-*.pem")
		require.NoError(t, err)
		defer func(name string) {
			_ = os.Remove(name)
		}(caFile.Name())
		err = pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})
		require.NoError(t, err)

		corruptFile, err := os.CreateTemp("", "corrupt-*.pem")
		require.NoError(t, err)
		defer func(name string) {
			_ = os.Remove(name)
		}(corruptFile.Name())
		err = os.WriteFile(corruptFile.Name(), []byte("corrupt PEM"), 0600)
		require.NoError(t, err)

		t.Run("Valid certificate and key", func(t *testing.T) {
			tlsConfig := &TLS{Enable: true, Cert: certFile.Name(), Key: keyFile.Name()}
			config, err := tlsConfig.MakeConfig("icinga.com")
			require.NoError(t, err)
			require.NotNil(t, config)
			require.Len(t, config.Certificates, 1)
		})

		t.Run("Mismatched certificate and key", func(t *testing.T) {
			_key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, err)
			_keyFile, err := os.CreateTemp("", "key-*.pem")
			require.NoError(t, err)
			defer func(name string) {
				_ = os.Remove(name)
			}(_keyFile.Name())
			_keyBytes, err := x509.MarshalPKCS8PrivateKey(_key)
			require.NoError(t, err)
			err = pem.Encode(_keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: _keyBytes})
			require.NoError(t, err)

			tlsConfig := &TLS{Enable: true, Cert: certFile.Name(), Key: _keyFile.Name()}
			_, err = tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Invalid certificate path", func(t *testing.T) {
			tlsConfig := &TLS{Enable: true, Cert: "nonexistent.crt", Key: keyFile.Name()}
			_, err := tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Invalid certificate permissions", func(t *testing.T) {
			fileInfo, err := certFile.Stat()
			require.NoError(t, err)
			defer func() {
				err := certFile.Chmod(fileInfo.Mode())
				require.NoError(t, err)
			}()
			err = certFile.Chmod(0000)
			require.NoError(t, err)

			tlsConfig := &TLS{Enable: true, Cert: certFile.Name(), Key: keyFile.Name()}
			_, err = tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Corrupt certificate", func(t *testing.T) {
			tlsConfig := &TLS{Enable: true, Cert: corruptFile.Name(), Key: keyFile.Name()}
			_, err := tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Invalid key path", func(t *testing.T) {
			tlsConfig := &TLS{Enable: true, Cert: certFile.Name(), Key: "nonexistent.key"}
			_, err := tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Invalid key permissions", func(t *testing.T) {
			fileInfo, err := keyFile.Stat()
			require.NoError(t, err)
			defer func() {
				err := keyFile.Chmod(fileInfo.Mode())
				require.NoError(t, err)
			}()
			err = keyFile.Chmod(0000)
			require.NoError(t, err)

			tlsConfig := &TLS{Enable: true, Cert: certFile.Name(), Key: keyFile.Name()}
			_, err = tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Corrupt key", func(t *testing.T) {
			tlsConfig := &TLS{Enable: true, Cert: certFile.Name(), Key: corruptFile.Name()}
			_, err := tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Valid CA", func(t *testing.T) {
			tlsConfig := &TLS{Enable: true, Ca: caFile.Name()}
			config, err := tlsConfig.MakeConfig("icinga.com")
			require.NoError(t, err)
			require.NotNil(t, config)
			require.NotNil(t, config.RootCAs)
		})

		t.Run("Invalid CA path", func(t *testing.T) {
			tlsConfig := &TLS{Enable: true, Ca: "nonexistent.ca"}
			_, err := tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Invalid CA permissions", func(t *testing.T) {
			fileInfo, err := caFile.Stat()
			require.NoError(t, err)
			defer func() {
				err := caFile.Chmod(fileInfo.Mode())
				require.NoError(t, err)
			}()
			err = caFile.Chmod(0000)
			require.NoError(t, err)

			tlsConfig := &TLS{Enable: true, Ca: caFile.Name()}
			_, err = tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Corrupt CA", func(t *testing.T) {
			tlsConfig := &TLS{Enable: true, Ca: corruptFile.Name()}
			_, err := tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})
	})
}

type generateCertOptions struct {
	ca        bool
	issuer    *x509.Certificate
	issuerKey crypto.PrivateKey
}

func generateCert(cn string, options generateCertOptions) (*x509.Certificate, crypto.PrivateKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  options.ca,
	}

	var issuer *x509.Certificate
	var issuerKey crypto.PrivateKey
	if options.issuer != nil {
		if options.issuerKey == nil {
			return nil, nil, errors.New("issuerKey required if issuer set")
		}
		issuer = options.issuer
		issuerKey = options.issuerKey
	} else {
		issuer = template
		issuerKey = privateKey
	}

	der, err := x509.CreateCertificate(rand.Reader, template, issuer, privateKey.Public(), issuerKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}

	return cert, privateKey, nil
}
