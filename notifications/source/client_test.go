package source

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	t.Parallel()

	clientCert, clientKey := generateClientCert(t)
	clientCertPool := x509.NewCertPool()
	require.True(t, clientCertPool.AppendCertsFromPEM([]byte(clientCert)))

	writeResp := func(t *testing.T, rw http.ResponseWriter, results []any) {
		rw.Header().Set("Content-Type", "application/x-ndjson")
		rw.WriteHeader(http.StatusAccepted)

		ctrl := http.NewResponseController(rw)
		enc := json.NewEncoder(rw)
		for _, result := range results {
			require.NoError(t, enc.Encode(result))
			require.NoError(t, ctrl.Flush())
		}
	}

	tests := []struct {
		name    string
		server  func(t *testing.T, h http.Handler) *httptest.Server
		handler func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool
		conf    func(srv *httptest.Server) Config
		verify  func(t *testing.T, err error, result []Incident)
	}{
		{
			name:   "http",
			server: func(_ *testing.T, h http.Handler) *httptest.Server { return httptest.NewServer(h) },
			handler: func(t *testing.T, _ http.ResponseWriter, r *http.Request) bool {
				user, pass, ok := r.BasicAuth()
				assert.True(t, ok, "expected basic auth")
				assert.Equal(t, "icinga", user)
				assert.Equal(t, "insecure", pass)
				return false
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
			handler: func(t *testing.T, _ http.ResponseWriter, r *http.Request) bool {
				user, pass, ok := r.BasicAuth()
				assert.True(t, ok, "expected basic auth")
				assert.Equal(t, "icinga", user)
				assert.Equal(t, "insecure", pass)
				return false
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
			handler: func(t *testing.T, _ http.ResponseWriter, r *http.Request) bool {
				assert.Len(t, r.TLS.VerifiedChains, 1, "expected one verified cert")
				assert.Equal(t, r.TLS.VerifiedChains[0][0].Subject.String(), "CN=icinga", "invalid client cert subject")
				return false
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
		{
			name:   "GetIncidents",
			server: func(t *testing.T, h http.Handler) *httptest.Server { return httptest.NewServer(h) },
			handler: func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool {
				incidents := []any{
					Incident{IsMuted: false, ObjectTags: map[string]string{"icinga": "test"}, Severity: event.SeverityCrit},
					Incident{IsMuted: true, ObjectTags: map[string]string{"icinga": "test2"}, Severity: event.SeverityWarning},
					Incident{IsMuted: false, ObjectTags: map[string]string{"icinga": "test3"}, Severity: event.SeverityInfo},
				}
				writeResp(t, rw, incidents)
				return true
			},
			conf: func(srv *httptest.Server) Config { return Config{Url: srv.URL} },
			verify: func(t *testing.T, err error, result []Incident) {
				require.NoError(t, err)
				assert.Len(t, result, 3)
				for i, incident := range result {
					switch i {
					case 0:
						assert.Equal(t, event.SeverityCrit, incident.Severity)
						assert.Equal(t, map[string]string{"icinga": "test"}, incident.ObjectTags)
					case 1:
						assert.Equal(t, event.SeverityWarning, incident.Severity)
						assert.Equal(t, map[string]string{"icinga": "test2"}, incident.ObjectTags)
					case 2:
						assert.Equal(t, event.SeverityInfo, incident.Severity)
						assert.Equal(t, map[string]string{"icinga": "test3"}, incident.ObjectTags)
					}
				}
			},
		},
		{
			name:   "GetIncidents Fail",
			server: func(t *testing.T, h http.Handler) *httptest.Server { return httptest.NewServer(h) },
			handler: func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool {
				incidents := []any{
					Incident{IsMuted: false, ObjectTags: map[string]string{"icinga": "test"}, Severity: event.SeverityCrit},
					Incident{ErrorState: ErrorState{Error: "something went wrong"}},
				}
				writeResp(t, rw, incidents)
				return true
			},
			conf: func(srv *httptest.Server) Config { return Config{Url: srv.URL} },
			verify: func(t *testing.T, err error, result []Incident) {
				require.ErrorIs(t, err, ErrReadPartialResp)
				require.ErrorContains(t, err, "something went wrong")
				assert.Len(t, result, 0)
			},
		},
		{
			name:   "ModifyIncidents",
			server: func(t *testing.T, h http.Handler) *httptest.Server { return httptest.NewServer(h) },
			handler: func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool {
				results := []any{
					ModifiedIncidentResp{ObjectTags: map[string]string{"icinga": "test"}},
					ModifiedIncidentResp{ObjectTags: map[string]string{"icinga": "test2"}},
				}
				writeResp(t, rw, results)
				return true
			},
			conf:   func(srv *httptest.Server) Config { return Config{Url: srv.URL} },
			verify: func(t *testing.T, err error, _ []Incident) { require.NoError(t, err) },
		},
		{
			name:   "ModifyIncidents Fail",
			server: func(t *testing.T, h http.Handler) *httptest.Server { return httptest.NewServer(h) },
			handler: func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool {
				results := []any{
					ModifiedIncidentResp{ObjectTags: map[string]string{"icinga": "test"}},
					ModifiedIncidentResp{ObjectTags: map[string]string{"icinga": "test2"}, ErrorState: ErrorState{Error: "something went wrong"}},
				}
				writeResp(t, rw, results)
				return true
			},
			conf: func(srv *httptest.Server) Config { return Config{Url: srv.URL} },
			verify: func(t *testing.T, err error, _ []Incident) {
				var merr *ModifyError
				require.True(t, errors.As(err, &merr))
				assert.Len(t, merr.Results(), 1)
				res := merr.Results()[0]
				assert.Equal(t, map[string]string{"icinga": "test2"}, res.ObjectTags)
				assert.Equal(t, "something went wrong", res.Error)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reached bool
			srv := tc.server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				assert.Equal(t, "test-client", r.Header.Get("User-Agent"))
				if tc.handler != nil {
					if tc.handler(t, w, r) {
						return // headers and status code already written by handler
					}
				}
				w.WriteHeader(http.StatusAccepted)
			}))
			defer srv.Close()

			client, err := NewClient(tc.conf(srv), "test-client")
			require.NoError(t, err)

			filter := map[string]any{"something": "unused"}
			var incidents []Incident
			if strings.HasPrefix(tc.name, "ModifyIncidents") {
				err = client.ModifyIncidents(t.Context(), ModifiableIncidentAttrs{Close: types.MakeBool(true)}, filter)
			} else {
				incidents, err = client.GetIncidents(t.Context(), filter)
			}
			assert.True(t, reached, "request should have reached the server")
			if tc.verify != nil {
				tc.verify(t, err, incidents)
			} else {
				require.NoError(t, err)
			}
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
