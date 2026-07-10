package config

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/testutils"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func Test_loadPemOrFile(t *testing.T) {
	cert, _, err := generateCert("cert", generateCertOptions{})
	require.NoError(t, err)
	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})

	certFile, err := os.CreateTemp("", "cert-*.pem")
	require.NoError(t, err)
	defer func(name string) {
		_ = os.Remove(name) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
	}(certFile.Name())
	_, err = certFile.Write(certPem)
	require.NoError(t, err)

	t.Run("Load raw PEM", func(t *testing.T) {
		out, err := loadPemOrFile(string(certPem))
		require.NoError(t, err)
		require.Equal(t, certPem, out)
	})

	t.Run("Load file", func(t *testing.T) {
		out, err := loadPemOrFile(certFile.Name())
		require.NoError(t, err)
		require.Equal(t, certPem, out)
	})

	t.Run("Invalid file", func(t *testing.T) {
		_, err := loadPemOrFile("/dev/null/nonexistent")
		require.Error(t, err)
	})
}

func TestTLS_MakeConfig(t *testing.T) {
	t.Run("TLS disabled", func(t *testing.T) {
		tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: false}}
		config, err := tlsConfig.MakeConfig("icinga.com")
		require.NoError(t, err)
		require.Nil(t, config)
	})

	t.Run("Server name", func(t *testing.T) {
		tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true}}
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
		tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true}, Insecure: true}
		config, err := tlsConfig.MakeConfig("icinga.com")
		require.NoError(t, err)
		require.NotNil(t, config)
		require.True(t, config.InsecureSkipVerify)
	})

	t.Run("Missing client certificate", func(t *testing.T) {
		tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Key: "test.key"}}
		_, err := tlsConfig.MakeConfig("icinga.com")
		require.ErrorContains(t, err, "certificate missing")
	})

	t.Run("Missing private key", func(t *testing.T) {
		tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: "test.crt"}}
		_, err := tlsConfig.MakeConfig("icinga.com")
		require.ErrorContains(t, err, "private key missing")
	})

	t.Run("x509", func(t *testing.T) {
		cert, key, err := generateCert("cert", generateCertOptions{})
		require.NoError(t, err)
		certFile, err := os.CreateTemp("", "cert-*.pem")
		require.NoError(t, err)
		defer func(name string) {
			_ = os.Remove(name) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
		}(certFile.Name())
		err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		require.NoError(t, err)

		keyFile, err := os.CreateTemp("", "key-*.pem")
		require.NoError(t, err)
		defer func(name string) {
			_ = os.Remove(name) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
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
			_ = os.Remove(name) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
		}(caFile.Name())
		err = pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})
		require.NoError(t, err)

		corruptFile, err := os.CreateTemp("", "corrupt-*.pem")
		require.NoError(t, err)
		defer func(name string) {
			_ = os.Remove(name) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
		}(corruptFile.Name())
		err = os.WriteFile(corruptFile.Name(), // #nosec G703 -- name is not user supplied, but from os.CreateTemp
			[]byte("-----BEGIN CORRUPT-----\nOOPS\n-----END CORRUPT-----"),
			0600)
		require.NoError(t, err)

		t.Run("Valid certificate and key", func(t *testing.T) {
			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: certFile.Name(), Key: keyFile.Name()}}
			config, err := tlsConfig.MakeConfig("icinga.com")
			require.NoError(t, err)
			require.NotNil(t, config)
			require.Len(t, config.Certificates, 1)
		})

		t.Run("Valid certificate and key as PEM", func(t *testing.T) {
			certRaw, err := os.ReadFile(certFile.Name()) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
			require.NoError(t, err)
			keyRaw, err := os.ReadFile(keyFile.Name()) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
			require.NoError(t, err)

			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: string(certRaw), Key: string(keyRaw)}}
			config, err := tlsConfig.MakeConfig("icinga.com")
			require.NoError(t, err)
			require.NotNil(t, config)
			require.Len(t, config.Certificates, 1)
		})

		t.Run("Valid certificate and key, mixed file and PEM", func(t *testing.T) {
			keyRaw, err := os.ReadFile(keyFile.Name()) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
			require.NoError(t, err)

			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: certFile.Name(), Key: string(keyRaw)}}
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
				_ = os.Remove(name) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
			}(_keyFile.Name())
			_keyBytes, err := x509.MarshalPKCS8PrivateKey(_key)
			require.NoError(t, err)
			err = pem.Encode(_keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: _keyBytes})
			require.NoError(t, err)

			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: certFile.Name(), Key: _keyFile.Name()}}
			_, err = tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Invalid certificate path", func(t *testing.T) {
			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: "nonexistent.crt", Key: keyFile.Name()}}
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

			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: certFile.Name(), Key: keyFile.Name()}}
			_, err = tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Corrupt certificate", func(t *testing.T) {
			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: corruptFile.Name(), Key: keyFile.Name()}}
			_, err := tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Corrupt certificate as PEM", func(t *testing.T) {
			corruptRaw, err := os.ReadFile(corruptFile.Name()) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
			require.NoError(t, err)
			keyRaw, err := os.ReadFile(keyFile.Name()) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
			require.NoError(t, err)

			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: string(corruptRaw), Key: string(keyRaw)}}
			_, err = tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Invalid key path", func(t *testing.T) {
			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: certFile.Name(), Key: "nonexistent.key"}}
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

			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: certFile.Name(), Key: keyFile.Name()}}
			_, err = tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Corrupt key", func(t *testing.T) {
			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Cert: certFile.Name(), Key: corruptFile.Name()}}
			_, err := tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Valid CA", func(t *testing.T) {
			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Ca: caFile.Name()}}
			config, err := tlsConfig.MakeConfig("icinga.com")
			require.NoError(t, err)
			require.NotNil(t, config)
			require.NotNil(t, config.RootCAs)
		})

		t.Run("Valid CA as PEM", func(t *testing.T) {
			caRaw, err := os.ReadFile(caFile.Name()) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
			require.NoError(t, err)

			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Ca: string(caRaw)}}
			config, err := tlsConfig.MakeConfig("icinga.com")
			require.NoError(t, err)
			require.NotNil(t, config)
			require.NotNil(t, config.RootCAs)
		})

		t.Run("Invalid CA path", func(t *testing.T) {
			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Ca: "nonexistent.ca"}}
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

			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Ca: caFile.Name()}}
			_, err = tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})

		t.Run("Corrupt CA", func(t *testing.T) {
			tlsConfig := &TLS{TLSCommon: TLSCommon{Enable: true, Ca: corruptFile.Name()}}
			_, err := tlsConfig.MakeConfig("icinga.com")
			require.Error(t, err)
		})
	})
}

func TestServerTLS_MakeConfig(t *testing.T) {
	t.Parallel()

	t.Run("TLS disabled", func(t *testing.T) {
		tlsConfig := &ServerTLS{TLSCommon: TLSCommon{Enable: false}}
		config, err := tlsConfig.MakeConfig()
		require.NoError(t, err)
		require.Nil(t, config)
	})

	t.Run("X509", func(t *testing.T) {
		ca, _, err := generateCert("ca", generateCertOptions{ca: true})
		require.NoError(t, err)
		var caBytes bytes.Buffer
		err = pem.Encode(&caBytes, &pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})
		require.NoError(t, err)

		cert, key, err := generateCert("cert", generateCertOptions{})
		require.NoError(t, err)
		var certBytes bytes.Buffer
		require.NoError(t, pem.Encode(&certBytes, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))

		_keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		var keyBytes bytes.Buffer
		require.NoError(t, pem.Encode(&keyBytes, &pem.Block{Type: "PRIVATE KEY", Bytes: _keyBytes}))

		t.Run("Valid CA and Server Cert", func(t *testing.T) {
			tlsConfig := &ServerTLS{TLSCommon: TLSCommon{
				Enable: true,
				Cert:   certBytes.String(),
				Key:    keyBytes.String(),
				Ca:     caBytes.String(),
			}}
			config, err := tlsConfig.MakeConfig()
			require.NoError(t, err)
			require.NotNil(t, config)
			require.NotNil(t, config.ClientCAs)
			require.Len(t, config.Certificates, 1)
		})

		t.Run("Invalid CA", func(t *testing.T) {
			tlsConfig := &ServerTLS{TLSCommon: TLSCommon{
				Enable: true,
				Cert:   certBytes.String(),
				Key:    keyBytes.String(),
				Ca:     "invalid-ca",
			}}
			_, err := tlsConfig.MakeConfig()
			require.ErrorContains(t, err, "can't load X.509 CA certificate")
		})

		t.Run("Without Server Cert", func(t *testing.T) {
			tlsConfig := &ServerTLS{TLSCommon: TLSCommon{Enable: true, Ca: caBytes.String()}}
			_, err := tlsConfig.MakeConfig()
			require.ErrorContains(t, err, "TLS is enabled but no certificate/key pair is configured")
		})
	})
}

func TestServerTLS_Validate(t *testing.T) {
	t.Parallel()

	st := &ServerTLS{
		TLSCommon: TLSCommon{
			Enable: true,
		},
		ClientAuth: TlsClientAuthType(tls.RequestClientCert),
	}
	require.NoError(t, st.Validate())

	st.Enable = false
	st.ClientAuth = TlsClientAuthType(tls.VerifyClientCertIfGiven)
	require.NoError(t, st.Validate())

	st.Enable = true
	require.Error(t, st.Validate())
	st.ClientAuth = TlsClientAuthType(tls.RequireAndVerifyClientCert)
	require.Error(t, st.Validate())

	st.Ca = "/nonexistent.ca"
	require.NoError(t, st.Validate())
}

func TestTlsClientAuthType(t *testing.T) {
	t.Parallel()

	t.Run("UnmarshalText", func(t *testing.T) {
		t.Parallel()

		var st ServerTLS
		require.NoError(t, FromEnv(&st, EnvOptions{Environment: map[string]string{"CLIENT_AUTH": "NoClientCert"}}))
		require.Equal(t, tls.NoClientCert, tls.ClientAuthType(st.ClientAuth))
		require.NoError(t, FromEnv(&st, EnvOptions{Environment: map[string]string{"CLIENT_AUTH": "RequestClientCert"}}))
		require.Equal(t, tls.RequestClientCert, tls.ClientAuthType(st.ClientAuth))

		st = ServerTLS{}
		require.Error(t, FromEnv(&st, EnvOptions{Environment: map[string]string{"CLIENT_AUTH": "InvalidValue"}}))
		require.Equal(t, tls.NoClientCert, tls.ClientAuthType(st.ClientAuth))
	})

	t.Run("UnmarshalYAML", func(t *testing.T) {
		t.Parallel()

		var st ServerTLS
		var err error
		testutils.WithYAMLFile(t, `client_auth: NoClientCert`, func(file *os.File) { err = FromYAMLFile(file.Name(), &st) })
		require.NoError(t, err)
		require.Equal(t, tls.NoClientCert, tls.ClientAuthType(st.ClientAuth))

		testutils.WithYAMLFile(t, `client_auth: RequestClientCert`, func(file *os.File) { err = FromYAMLFile(file.Name(), &st) })
		require.NoError(t, err)
		require.Equal(t, tls.RequestClientCert, tls.ClientAuthType(st.ClientAuth))

		st = ServerTLS{}
		testutils.WithYAMLFile(t, `client_auth: InvalidValue`, func(file *os.File) { err = FromYAMLFile(file.Name(), &st) })
		require.Error(t, err)
		require.Equal(t, tls.NoClientCert, tls.ClientAuthType(st.ClientAuth))
	})
}

func TestTLSCommon_InitRevocationChecking(t *testing.T) {
	ca, caKey, err := generateCert("ca", generateCertOptions{ca: true})
	require.NoError(t, err)

	var caPem bytes.Buffer
	require.NoError(t, pem.Encode(&caPem, &pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw}))
	caStr := caPem.String()

	revokedCert, _, err := generateCert("revoked", generateCertOptions{issuer: ca, issuerKey: caKey})
	require.NoError(t, err)
	validCert, _, err := generateCert("valid", generateCertOptions{issuer: ca, issuerKey: caKey})
	require.NoError(t, err)

	emptyCrlFile := generateCRL(t, ca, caKey)
	revokedCrlFile := generateCRL(t, ca, caKey, revokedCert.SerialNumber)

	t.Run("Nil tlsConfig", func(t *testing.T) {
		tc := &TLSCommon{Ca: caStr, CrlFile: emptyCrlFile}
		checker, err := tc.InitRevocationChecking(nil)
		require.NoError(t, err)
		require.Nil(t, checker)
	})

	t.Run("Empty CrlFile", func(t *testing.T) {
		tc := &TLSCommon{Ca: caStr}
		checker, err := tc.InitRevocationChecking(&tls.Config{})
		require.NoError(t, err)
		require.Nil(t, checker)
	})

	t.Run("Empty Ca", func(t *testing.T) {
		tc := &TLSCommon{CrlFile: emptyCrlFile}
		checker, err := tc.InitRevocationChecking(&tls.Config{})
		require.NoError(t, err)
		require.Nil(t, checker)
	})

	t.Run("Invalid Ca", func(t *testing.T) {
		tc := &TLSCommon{Ca: "/nonexistent/ca.pem", CrlFile: emptyCrlFile}
		_, err := tc.InitRevocationChecking(&tls.Config{})
		require.Error(t, err)
	})

	t.Run("Nonexistent CRL file", func(t *testing.T) {
		tc := &TLSCommon{Ca: caStr, CrlFile: "/nonexistent/crl.pem"}
		_, err := tc.InitRevocationChecking(&tls.Config{})
		require.ErrorContains(t, err, "cannot load CRL")
	})

	t.Run("Valid CA and Valid CRL", func(t *testing.T) {
		tc := &TLSCommon{Ca: caStr, CrlFile: emptyCrlFile}
		tlsConf := &tls.Config{}
		checker, err := tc.InitRevocationChecking(tlsConf)
		require.NoError(t, err)
		require.NotNil(t, checker)
		require.NotNil(t, tlsConf.VerifyConnection)
	})

	t.Run("VerifyConnection", func(t *testing.T) {
		t.Run("Empty VerifiedChains", func(t *testing.T) {
			tc := &TLSCommon{Ca: caStr, CrlFile: emptyCrlFile}
			tlsConf := &tls.Config{}
			_, err := tc.InitRevocationChecking(tlsConf)
			require.NoError(t, err)
			require.NoError(t, tlsConf.VerifyConnection(tls.ConnectionState{}))
		})

		t.Run("Valid Cert", func(t *testing.T) {
			tc := &TLSCommon{Ca: caStr, CrlFile: revokedCrlFile}
			tlsConf := &tls.Config{}
			_, err := tc.InitRevocationChecking(tlsConf)
			require.NoError(t, err)
			err = tlsConf.VerifyConnection(tls.ConnectionState{
				VerifiedChains: [][]*x509.Certificate{{validCert}},
			})
			require.NoError(t, err)
		})

		t.Run("Revoked Cert", func(t *testing.T) {
			tc := &TLSCommon{Ca: caStr, CrlFile: revokedCrlFile}
			tlsConf := &tls.Config{}
			_, err := tc.InitRevocationChecking(tlsConf)
			require.NoError(t, err)
			err = tlsConf.VerifyConnection(tls.ConnectionState{
				VerifiedChains: [][]*x509.Certificate{{revokedCert}},
			})
			require.ErrorContains(t, err, "client certificate revoked")
		})

		t.Run("Chains existing VerifyConnection", func(t *testing.T) {
			tc := &TLSCommon{Ca: caStr, CrlFile: emptyCrlFile}
			sentinel := errors.New("existing verify error")
			tlsConf := &tls.Config{
				VerifyConnection: func(_ tls.ConnectionState) error { return sentinel },
			}
			_, err := tc.InitRevocationChecking(tlsConf)
			require.NoError(t, err)
			err = tlsConf.VerifyConnection(tls.ConnectionState{
				VerifiedChains: [][]*x509.Certificate{{validCert}},
			})
			require.ErrorIs(t, err, sentinel)
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

	keyUsage := x509.KeyUsageCertSign
	if options.ca {
		keyUsage |= x509.KeyUsageCRLSign
	}

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              keyUsage,
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

func generateCRL(t *testing.T, ca *x509.Certificate, caKey crypto.PrivateKey, revokedSerials ...*big.Int) string {
	entries := make([]x509.RevocationListEntry, 0, len(revokedSerials))
	for _, serial := range revokedSerials {
		entries = append(entries, x509.RevocationListEntry{
			SerialNumber:   serial,
			RevocationTime: time.Now().Add(-1 * time.Hour),
		})
	}

	template := &x509.RevocationList{
		Number:                    big.NewInt(1),
		ThisUpdate:                time.Now().Add(-1 * time.Hour),
		NextUpdate:                time.Now().Add(24 * time.Hour),
		RevokedCertificateEntries: entries,
	}

	signer, ok := caKey.(crypto.Signer)
	require.True(t, ok, "caKey must implement crypto.Signer")
	der, err := x509.CreateRevocationList(rand.Reader, template, ca, signer)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "X509 CRL", Bytes: der}))

	f, err := os.CreateTemp("", "crl-*.pem")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.Write(buf.Bytes())
	require.NoError(t, err)
	_ = f.Close()

	return f.Name()
}
