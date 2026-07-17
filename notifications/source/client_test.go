package source

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
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	clientCert, clientKey := generateClientCert(t)
	clientCertPool := x509.NewCertPool()
	require.True(t, clientCertPool.AppendCertsFromPEM([]byte(clientCert)))

	tests := []struct {
		name    string
		server  func(t *testing.T, h http.Handler) *httptest.Server
		handler func(t *testing.T, r *http.Request)
		conf    func(srv *httptest.Server) Config
	}{
		{
			name:   "http",
			server: func(_ *testing.T, h http.Handler) *httptest.Server { return httptest.NewServer(h) },
			handler: func(t *testing.T, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				assert.True(t, ok, "expected basic auth")
				assert.Equal(t, "icinga", user)
				assert.Equal(t, "insecure", pass)
			},
			conf: func(srv *httptest.Server) Config {
				return Config{
					Url:      srv.URL,
					Username: "icinga",
					Password: "insecure",
				}
			},
		},
		{
			name:   "https",
			server: func(_ *testing.T, h http.Handler) *httptest.Server { return httptest.NewTLSServer(h) },
			handler: func(t *testing.T, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				assert.True(t, ok, "expected basic auth")
				assert.Equal(t, "icinga", user)
				assert.Equal(t, "insecure", pass)
			},
			conf: func(srv *httptest.Server) Config {
				ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
				return Config{
					Url:        srv.URL,
					Username:   "icinga",
					Password:   "insecure",
					TlsOptions: config.TLS{TLSCommon: config.TLSCommon{Ca: string(ca)}},
				}
			},
		},
		{
			name: "https client cert",
			server: func(_ *testing.T, h http.Handler) *httptest.Server {
				srv := httptest.NewUnstartedServer(h)
				srv.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCertPool}
				srv.StartTLS()
				return srv
			},
			handler: func(t *testing.T, r *http.Request) {
				assert.Len(t, r.TLS.VerifiedChains, 1, "expected one verified cert")
				assert.Equal(t, r.TLS.VerifiedChains[0][0].Subject.String(), "CN=icinga", "invalid client cert subject")
			},
			conf: func(srv *httptest.Server) Config {
				ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
				return Config{
					Url: srv.URL,
					TlsOptions: config.TLS{TLSCommon: config.TLSCommon{
						Ca:   string(ca),
						Cert: clientCert,
						Key:  clientKey,
					}},
				}
			},
		},
		{
			name: "unix",
			server: func(t *testing.T, h http.Handler) *httptest.Server {
				ln, err := net.Listen("unix", filepath.Join(t.TempDir(), "sock"))
				require.NoError(t, err)
				srv := httptest.NewUnstartedServer(h)
				srv.Listener = ln
				srv.Start()
				return srv
			},
			// No handler as this would require OS specific implementations; total overkill for testing.
			conf: func(srv *httptest.Server) Config { return Config{Url: "unix://" + srv.Listener.Addr().String()} },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var reached bool
			srv := tc.server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				assert.Equal(t, "test-client", r.Header.Get("User-Agent"))
				if tc.handler != nil {
					tc.handler(t, r)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			client, err := NewClient(tc.conf(srv), "test-client")
			require.NoError(t, err)

			_, err = client.ProcessEvent(context.Background(), &event.Event{}, false)
			require.NoError(t, err)
			assert.True(t, reached, "request should have reached the server")
		})
	}
}

// generateClientCert for HTTPS client certificate testing.
func generateClientCert(t *testing.T) (string, string) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(23),
		Subject:               pkix.Name{CommonName: "icinga"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)

	keyDer, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	certPem := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPem := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDer}))

	return certPem, keyPem
}
