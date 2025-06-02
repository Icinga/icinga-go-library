package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"github.com/pkg/errors"
	"os"
)

// TLS represents configuration for a TLS client.
// It provides options to enable TLS, specify certificate and key files,
// CA certificate, and whether to skip verification of the server's certificate chain and host name.
// Use the [TLS.MakeConfig] method to assemble a [*tls.Config] from the TLS struct.
//
// Example usage:
//
//	func main() {
//		tlsConfig := &config.TLS{
//			Enable:   true,
//			Cert:     "path/to/cert.pem",
//			Key:      "path/to/key.pem",
//			Ca:       "path/to/ca.pem",
//			Insecure: false,
//		}
//
//		cfg, err := tlsConfig.MakeConfig("example.com")
//		if err != nil {
//			log.Fatalf("error creating TLS config: %v", err)
//		}
//
//		// ...
//	}
type TLS struct {
	// Enable indicates whether TLS is enabled.
	Enable bool `yaml:"tls" env:"TLS"`

	// Cert is either the path to the TLS certificate file or a raw PEM-encoded string representing it.
	// If provided, Key must also be specified.
	Cert string `yaml:"cert" env:"CERT"`

	// Key is either the path to the TLS key file or a raw PEM-encoded string representing it.
	// If specified, Cert must also be provided.
	Key string `yaml:"key" env:"KEY,unset"`

	// Ca is either the path to the CA certificate file or a raw PEM-encoded string representing it.
	Ca string `yaml:"ca" env:"CA"`

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

	data, err := os.ReadFile(pemOrFile) // #nosec G304 -- inclusion of user-specified file
	if err != nil {
		return nil, err
	}
	return data, nil
}

// MakeConfig assembles a [*tls.Config] from the TLS struct and the provided serverName.
// It returns a configured *tls.Config or an error if there are issues with the provided TLS settings.
// If TLS is not enabled (t.Enable is false), it returns nil without an error.
func (t *TLS) MakeConfig(serverName string) (*tls.Config, error) {
	if !t.Enable {
		return nil, nil
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	hasKeyWithoutCert := t.Key != "" && t.Cert == ""
	hasCertWithoutKey := t.Cert != "" && t.Key == ""
	hasClientCert := t.Cert != "" && t.Key != ""

	if hasKeyWithoutCert {
		return nil, errors.New("private key given, but client certificate missing")
	}
	if hasCertWithoutKey {
		return nil, errors.New("client certificate given, but private key missing")
	}
	if hasClientCert {
		certPem, err := loadPemOrFile(t.Cert)
		if err != nil {
			return nil, errors.Wrap(err, "can't load X.509 client certificate")
		}
		keyPem, err := loadPemOrFile(t.Key)
		if err != nil {
			return nil, errors.Wrap(err, "can't load X.509 private key")
		}

		crt, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, errors.Wrap(err, "can't parse client certificate and private key into an X.509 key pair")
		}
		tlsConfig.Certificates = []tls.Certificate{crt}
	}

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

	return tlsConfig, nil
}
