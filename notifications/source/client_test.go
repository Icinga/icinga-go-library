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

	"github.com/google/uuid"
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
					Response[Incident]{Status: ResponseStatusSuccess, Result: Incident{IsMuted: false, ObjectTags: map[string]string{"icinga": "test"}, Severity: event.SeverityCrit}},
					Response[Incident]{Status: ResponseStatusSuccess, Result: Incident{IsMuted: true, ObjectTags: map[string]string{"icinga": "test2"}, Severity: event.SeverityWarning}},
					Response[Incident]{Status: ResponseStatusSuccess, Result: Incident{IsMuted: false, ObjectTags: map[string]string{"icinga": "test3"}, Severity: event.SeverityInfo}},
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
					Response[Incident]{Status: ResponseStatusSuccess, Result: Incident{IsMuted: false, ObjectTags: map[string]string{"icinga": "test"}, Severity: event.SeverityCrit}},
					Response[ErrorState]{Status: ResponseStatusError, Result: ErrorState{Error: "something went wrong"}},
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
					Response[ModifiedIncidentResp]{Status: ResponseStatusSuccess, Result: ModifiedIncidentResp{ObjectTags: map[string]string{"icinga": "test"}}},
					Response[ModifiedIncidentResp]{Status: ResponseStatusSuccess, Result: ModifiedIncidentResp{ObjectTags: map[string]string{"icinga": "test2"}}},
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
					Response[ModifiedIncidentResp]{Status: ResponseStatusSuccess, Result: ModifiedIncidentResp{ObjectTags: map[string]string{"icinga": "test"}}},
					Response[ModifiedIncidentResp]{Status: ResponseStatusError, Result: ModifiedIncidentResp{
						ObjectTags: map[string]string{"icinga": "test2"},
						ErrorState: ErrorState{Error: "something went wrong"}},
					},
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

func TestClientGetNotificationHistory(t *testing.T) {
	t.Parallel()

	uuid1 := types.MakeUUID(uuid.New())
	uuid2 := types.MakeUUID(uuid.New())

	tests := []struct {
		name    string
		since   int64
		handler func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool
		verify  func(t *testing.T, err error, result []NotificationHistory)
	}{
		{
			name:  "Success",
			since: 1,
			handler: func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool {
				entries := []Response[NotificationHistory]{
					{
						Status: ResponseStatusSuccess,
						Result: NotificationHistory{
							EventID:      uuid1,
							TriggeredAt:  types.UnixMilli(time.Unix(0, 1234567890123*int64(time.Millisecond))),
							ContactName:  types.MakeString("first-contact"),
							ChannelName:  types.MakeString("first-channel"),
							EventMessage: types.MakeString("hello"),
							State:        NotificationStateSent,
						},
					},
					{
						Status: ResponseStatusSuccess,
						Result: NotificationHistory{
							EventID:      uuid2,
							TriggeredAt:  types.UnixMilli(time.Unix(0, 1234567890456*int64(time.Millisecond))),
							ContactName:  types.MakeString("second-contact"),
							ChannelName:  types.MakeString("second-channel"),
							EventMessage: types.MakeString("hello"),
							State:        NotificationStateFailed,
						},
					},
				}
				writeResp(t, rw, entries)

				return true
			},
			verify: func(t *testing.T, err error, result []NotificationHistory) {
				require.NoError(t, err)
				require.Len(t, result, 2)

				first := result[0]
				assert.Equal(t, uuid1, first.EventID)
				assert.Equal(t, types.UnixMilli(time.Unix(0, 1234567890123*int64(time.Millisecond))), first.TriggeredAt)
				assert.Equal(t, "first-contact", first.ContactName.String)
				assert.Equal(t, "first-channel", first.ChannelName.String)
				assert.Equal(t, "hello", first.EventMessage.String)
				assert.Equal(t, NotificationStateSent, first.State)

				second := result[1]
				assert.Equal(t, uuid2, second.EventID)
				assert.Equal(t, types.UnixMilli(time.Unix(0, 1234567890456*int64(time.Millisecond))), second.TriggeredAt)
				assert.Equal(t, "second-contact", second.ContactName.String)
				assert.Equal(t, "second-channel", second.ChannelName.String)
				assert.Equal(t, "hello", second.EventMessage.String)
				assert.Equal(t, NotificationStateFailed, second.State)
			},
		},
		{
			name:  "Partial Error",
			since: 1,
			handler: func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool {
				entries := []any{
					Response[NotificationHistory]{Status: ResponseStatusSuccess, Result: NotificationHistory{State: NotificationStateSent}},
					Response[ErrorState]{Status: ResponseStatusError, Result: ErrorState{Error: "something went wrong"}},
				}
				writeResp(t, rw, entries)
				return true
			},
			verify: func(t *testing.T, err error, result []NotificationHistory) {
				require.ErrorIs(t, err, ErrReadPartialResp)
				require.ErrorContains(t, err, "something went wrong")
				assert.Len(t, result, 0)
			},
		},
		{
			name:  "Unexpected Status",
			since: 1,
			handler: func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool {
				rw.WriteHeader(http.StatusInternalServerError)
				return true
			},
			verify: func(t *testing.T, err error, result []NotificationHistory) {
				require.ErrorContains(t, err, "unexpected response from API")
				assert.Len(t, result, 0)
			},
		},
		{
			name:  "invalid since",
			since: 0,
			handler: func(t *testing.T, rw http.ResponseWriter, r *http.Request) bool {
				t.Fatal("request should not have reached the server")
				return true
			},
			verify: func(t *testing.T, err error, result []NotificationHistory) {
				require.ErrorContains(t, err, "since parameter must be a positive integer")
				assert.Len(t, result, 0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reached bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				assert.Equal(t, "test-client", r.Header.Get("User-Agent"))
				if tc.handler(t, w, r) {
					return // headers and status code already written by handler
				}
				w.WriteHeader(http.StatusAccepted)
			}))
			defer srv.Close()

			client, err := NewClient(Config{Url: srv.URL}, "test-client")
			require.NoError(t, err)

			result, err := client.GetNotificationHistory(t.Context(), map[string]string{}, tc.since)
			if tc.since > 0 {
				assert.True(t, reached, "request should have reached the server")
			} else {
				assert.False(t, reached, "request should not have reached the server")
			}
			tc.verify(t, err, result)
		})
	}
}

// writeResp writes each of results as a chunk of a streamed x-ndjson response with a 202 Accepted status,
// mirroring how the Icinga Notifications API streams /incidents and /notification-history responses.
func writeResp[T any](t *testing.T, rw http.ResponseWriter, results []T) {
	rw.Header().Set("Content-Type", "application/x-ndjson")
	rw.WriteHeader(http.StatusAccepted)

	ctrl := http.NewResponseController(rw)
	enc := json.NewEncoder(rw)
	for i := range results {
		require.NoError(t, enc.Encode(&results[i]))
		require.NoError(t, ctrl.Flush())
	}
}
