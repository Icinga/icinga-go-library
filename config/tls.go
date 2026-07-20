package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/pkg/errors"
)

// TLSCommon represents common TLS config options that are shared between TLS client and server settings.
type TLSCommon struct {
	// Enable indicates whether TLS is enabled.
	//
	// If false, the other TLS settings are ignored and no TLS configuration is created.
	Enable bool `yaml:"tls" env:"TLS"`

	// Cert is either the path to the client/server TLS certificate file or a raw PEM-encoded string.
	//
	// In TLS client mode, this certificate is sent to the server if requested, and may be used by
	// the server to authenticate the client. In TLS server mode, this certificate is sent to clients
	// during the TLS handshake and is used by clients to authenticate the server. Thus, in TLS server
	// mode, this option is required if TLS is enabled. In either mode, if this option is set, the Key
	// option must also be set to provide the corresponding private key.
	Cert string `yaml:"cert" env:"CERT"`

	// Key is either the path to the private key file corresponding to the TLS cert or a raw PEM-encoded string.
	//
	// If this option is set, the Cert option must also be set to provide the corresponding TLS certificate.
	Key string `yaml:"key" env:"KEY,unset"`

	// Ca is either the path to the CA certificate file or a raw PEM-encoded string representing it.
	//
	// If specified, the CA certificate is used to verify the server's certificate in TLS client mode,
	// or to verify client certificates in TLS server mode. Otherwise, the system's root CA pool is used.
	// This option is ignored if Insecure is true in TLS client mode.
	Ca string `yaml:"ca" env:"CA"`
}

// makeConfig assembles a *[tls.Config] from the [TLSCommon] struct.
//
// It returns a configured *tls.Config or an error if there are issues with the configured TLS settings.
// If TLS is not enabled, it returns nil without an error.
func (tc *TLSCommon) makeConfig() (*tls.Config, error) {
	if !tc.Enable {
		return nil, nil
	}

	hasKeyWithoutCert := tc.Key != "" && tc.Cert == ""
	hasCertWithoutKey := tc.Cert != "" && tc.Key == ""
	hasCertPair := tc.Cert != "" && tc.Key != ""

	if hasKeyWithoutCert {
		return nil, errors.New("private key given, but certificate missing")
	}
	if hasCertWithoutKey {
		return nil, errors.New("certificate given, but private key missing")
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	if hasCertPair {
		certPem, err := loadPemOrFile(tc.Cert)
		if err != nil {
			return nil, errors.Wrap(err, "can't load X.509 certificate")
		}
		keyPem, err := loadPemOrFile(tc.Key)
		if err != nil {
			return nil, errors.Wrap(err, "can't load X.509 private key")
		}

		crt, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, errors.Wrap(err, "can't parse certificate and private key into an X.509 key pair")
		}
		tlsConfig.Certificates = []tls.Certificate{crt}
	}

	return tlsConfig, nil
}

// TLS represents configuration for a TLS client.
// It provides options to enable TLS, specify certificate and key files,
// CA certificate, and whether to skip verification of the server's certificate chain and host name.
// Use the [TLS.MakeConfig] method to assemble a [*tls.Config] from the TLS struct.
type TLS struct {
	TLSCommon `yaml:",inline"`

	// Insecure indicates whether to skip verification of the server's certificate chain and host name.
	// If true, any certificate presented by the server and any host name in that certificate is accepted.
	// In this mode, TLS is susceptible to machine-in-the-middle attacks unless custom verification is used.
	Insecure bool `yaml:"insecure" env:"INSECURE"`
}

// loadPemOrFile either returns a PEM from within the string or treats it as a file, returning its content.
func loadPemOrFile(pemOrFile string) ([]byte, error) {
	block, _ := pem.Decode([]byte(pemOrFile))
	if block != nil {
		return []byte(pemOrFile), nil
	}

	data, err := os.ReadFile(pemOrFile) // #nosec G304 G703 -- inclusion of user-specified file
	if err != nil {
		return nil, err
	}
	return data, nil
}

// MakeConfig assembles a [*tls.Config] from the TLS struct and the provided serverName.
// It returns a configured *tls.Config or an error if there are issues with the provided TLS settings.
// If TLS is not enabled (t.Enable is false), it returns nil without an error.
func (t *TLS) MakeConfig(serverName string) (*tls.Config, error) {
	tlsConfig, err := t.makeConfig()
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		if t.Insecure {
			tlsConfig.InsecureSkipVerify = true
		} else if t.Ca != "" {
			caPem, err := loadPemOrFile(t.Ca)
			if err != nil {
				return nil, errors.Wrap(err, "can't load X.509 CA certificate")
			}

			tlsConfig.RootCAs = x509.NewCertPool()
			if !tlsConfig.RootCAs.AppendCertsFromPEM(caPem) {
				return nil, errors.New("can't parse CA file")
			}
		}

		tlsConfig.ServerName = serverName
	}

	return tlsConfig, nil
}

// TlsClientAuthType is a wrapper around [tls.ClientAuthType] that implements [encoding.TextUnmarshaler] and yaml.InterfaceUnmarshaler.
type TlsClientAuthType tls.ClientAuthType

// UnmarshalText implements encoding.TextUnmarshaler to allow parsing ClientAuth from environment variables.
//
// This is required by the env library, which is used to parse environment variables into the configuration struct.
func (o *TlsClientAuthType) UnmarshalText(text []byte) error {
	switch str := string(text); str {
	case "NoClientCert":
		*o = TlsClientAuthType(tls.NoClientCert)
	case "RequestClientCert":
		*o = TlsClientAuthType(tls.RequestClientCert)
	case "RequireAnyClientCert":
		*o = TlsClientAuthType(tls.RequireAnyClientCert)
	case "VerifyClientCertIfGiven":
		*o = TlsClientAuthType(tls.VerifyClientCertIfGiven)
	case "RequireAndVerifyClientCert":
		*o = TlsClientAuthType(tls.RequireAndVerifyClientCert)
	default:
		return errors.Errorf("invalid ClientAuth value: %q", str)
	}
	return nil
}

// UnmarshalYAML implements yaml.InterfaceUnmarshaler to allow Options to be parsed go-yaml.
func (o *TlsClientAuthType) UnmarshalYAML(unmarshal func(any) error) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}
	return o.UnmarshalText([]byte(str))
}

// ServerTLS represents all required TLS configuration options for a TLS server.
//
// It embeds [TLSCommon] to include the common TLS settings, and adds the ClientAuth field to specify
// the TLS client authentication policy the server will follow for client authentication, with the default
// being [tls.NoClientCert].
//
// Use the [ServerTLS.MakeConfig] method to assemble a *[tls.Config] from the [ServerTLS] struct.
type ServerTLS struct {
	TLSCommon `yaml:",inline"`

	// ClientAuth specifies the policy the server will follow for TLS Client Authentication.
	//
	// If empty, the default is [tls.NoClientCert], meaning that the server will not request a certificate from
	// clients and will not verify any certificates if they are sent. This is the most common mode for TLS servers,
	// unless client authentication is explicitly required to restrict access to the server to specific clients.
	//
	// Valid values are all the [tls.ClientAuthType] options typed out as strings:
	// "NoClientCert", "RequestClientCert", "RequireAnyClientCert", "VerifyClientCertIfGiven", and "RequireAndVerifyClientCert".
	ClientAuth TlsClientAuthType `yaml:"client_auth" env:"CLIENT_AUTH" default:"NoClientCert"`

	// CrlFile is the path to the Certificate Revocation List (CRL) file.
	//
	// If specified, the CRL is used to check for revoked certificates in TLS.
	// The CRL must be signed by the CA specified in the Ca option. If the CRL file is not found or cannot be loaded,
	// an error is returned during TLS configuration.
	// This option is ignored if TLS is not enabled. If the Ca option is not set it returns an error.
	//
	// Note that populating this field alone does not enable CRL checking.
	// [TLSCommon.InitRevocationChecking] must be called after assembling the [tls.Config]
	// to wire up the actual revocation check.
	CrlFile string `yaml:"crl_file" env:"CRL_FILE"`
}

// Validate checks the [ServerTLS] configuration for consistency and returns an error if the configuration is invalid.
func (st *ServerTLS) Validate() error {
	if !st.Enable {
		return nil
	}

	switch cat := tls.ClientAuthType(st.ClientAuth); cat {
	case tls.NoClientCert, tls.RequestClientCert, tls.RequireAnyClientCert:
		if st.CrlFile != "" {
			return errors.New("CRL file given, but given ClientAuth mode doesn't verify client certificates")
		}
		// These ClientAuth types do not require a CA certificate to be configured,
		// since the server will not verify client certificates in these modes.
		return nil
	case tls.VerifyClientCertIfGiven, tls.RequireAndVerifyClientCert:
		if st.Ca == "" {
			return errors.Errorf("ClientAuth value %q requires a CA certificate to be configured", cat)
		}
		return nil
	default:
		return errors.Errorf("invalid ClientAuth value: %q", cat)
	}
}

// MakeConfig assembles a *[tls.Config] from the [ServerTLS] struct.
//
// It returns a configured *tls.Config or an error if there are issues with the provided TLS settings.
// If TLS is not enabled (st.Enable is false), it returns nil without an error.
func (st *ServerTLS) MakeConfig() (*tls.Config, error) {
	if st.CrlFile != "" && st.Ca == "" {
		return nil, errors.New("CRL file given, but CA certificate missing")
	}
	tlsConfig, err := st.makeConfig()
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		if len(tlsConfig.Certificates) == 0 {
			return nil, errors.New("TLS is enabled but no certificate/key pair is configured")
		}

		if st.Ca != "" {
			caPem, err := loadPemOrFile(st.Ca)
			if err != nil {
				return nil, errors.Wrap(err, "can't load X.509 CA certificate")
			}

			tlsConfig.ClientCAs = x509.NewCertPool()
			if !tlsConfig.ClientCAs.AppendCertsFromPEM(caPem) {
				return nil, errors.New("can't parse CA file")
			}
		}

		tlsConfig.ClientAuth = tls.ClientAuthType(st.ClientAuth)
	}

	return tlsConfig, nil
}

// InitRevocationChecking wires CRL-based client certificate revocation checking into tlsConfig.
//
// It loads the CA certificate from [TLSCommon.Ca] and the CRL from [ServerTLS.CrlFile], verifies
// the CRL's signature against the CA, and installs a [tls.Config.VerifyConnection] hook that
// rejects any client certificate whose serial number appears in the CRL. If a VerifyConnection
// hook is already set on tlsConfig, the existing hook is called first and its error, if any,
// takes precedence.
//
// The returned [CrlChecker] owns the loaded CRL and must be kept alive for the duration
// of the server's lifetime. Callers are responsible for calling [CrlChecker.WatchAndReload]
// to pick up CRL rotations at runtime.
//
// InitRevocationChecking returns an error if tlsConfig, the CA certificate or CRL cannot be loaded or verified.
func (st *ServerTLS) InitRevocationChecking(tlsConfig *tls.Config) (*CrlChecker, error) {
	if tlsConfig == nil || st.CrlFile == "" || st.Ca == "" {
		return nil, errors.New("tls config for crl is required")
	}

	caPem, err := loadPemOrFile(st.Ca)
	if err != nil {
		return nil, err
	}

	var caCerts []*x509.Certificate
	rest := caPem
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		caCerts = append(caCerts, cert)
	}
	if len(caCerts) == 0 {
		return nil, errors.New("failed to decode X.509 certificate")
	}

	checker, err := NewCRLChecker(st.CrlFile, caCerts...)
	if err != nil {
		return nil, fmt.Errorf("cannot load CRL: %w", err)
	}

	existing := tlsConfig.VerifyConnection
	tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
		if existing != nil {
			if err := existing(cs); err != nil {
				return err
			}
		}

		if len(cs.VerifiedChains) == 0 {
			return nil // no client cert presented - skip
		}

		leaf := cs.VerifiedChains[0][0]
		revoked, err := checker.IsRevoked(leaf.SerialNumber)
		if err != nil {
			return fmt.Errorf("CRL check failed: %w", err)
		} else if revoked {
			return fmt.Errorf("client certificate revoked (serial %s)", leaf.SerialNumber)
		}

		return nil
	}

	return checker, nil
}
