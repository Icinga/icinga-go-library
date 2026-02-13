package database

import (
	"fmt"
	"os"
	"testing"

	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/testutils"
	"github.com/stretchr/testify/require"
)

// minimalYaml is a constant string representing a minimal valid YAML configuration for
// connecting to a PostgreSQL database. PostgreSQL is explicitly chosen here to
// test whether the default type (which is MySQL) is correctly overridden.
const minimalYaml = `
type: pgsql
host: localhost
user: icinga
database: icingadb
password: secret`

// minimalEnv returns a map of environment variables representing a minimal valid configuration for
// connecting to a PostgreSQL database. PostgreSQL is explicitly chosen here to
// test whether the default type (which is MySQL) is correctly overridden.
func minimalEnv() map[string]string {
	return map[string]string{
		"TYPE":     "pgsql",
		"HOST":     "localhost",
		"USER":     "icinga",
		"DATABASE": "icingadb",
		"PASSWORD": "secret",
	}
}

// withMinimalEnv takes a map of environment variables and merges it with the
// minimal environment configuration returned from minimalEnv,
// overriding any existing keys with the provided values.
// It returns the resulting map.
func withMinimalEnv(v map[string]string) map[string]string {
	env := minimalEnv()

	for key, value := range v {
		env[key] = value
	}

	return env
}

func TestConfig(t *testing.T) {
	var defaultOptions Options
	require.NoError(t, defaults.Set(&defaultOptions), "setting default options")

	passwordFile, cleanupPasswordFile := testutils.PasswordFile(t, "secret")
	defer cleanupPasswordFile()

	configTests := []testutils.TestCase[Config, testutils.ConfigTestData]{
		{
			Name: "Unknown database type",
			Data: testutils.ConfigTestData{
				Yaml: `type: invalid`,
				Env:  map[string]string{"TYPE": "invalid"},
			},
			Error: testutils.ErrorContains(`unknown database type "invalid"`),
		},
		{
			Name: "Database host missing",
			Data: testutils.ConfigTestData{
				Yaml: `type: pgsql`,
				Env:  map[string]string{"TYPE": "pgsql"},
			},
			Error: testutils.ErrorContains("database host missing"),
		},
		{
			Name: "Database user missing",
			Data: testutils.ConfigTestData{
				Yaml: `
type: pgsql
host: localhost`,
				Env: map[string]string{
					"TYPE": "pgsql",
					"HOST": "localhost",
				},
			},
			Error: testutils.ErrorContains("database user missing"),
		},
		{
			Name: "Database name missing",
			Data: testutils.ConfigTestData{
				Yaml: `
type: pgsql
host: localhost
user: icinga`,
				Env: map[string]string{
					"TYPE": "pgsql",
					"HOST": "localhost",
					"USER": "icinga",
				},
			},
			Error: testutils.ErrorContains("database name missing"),
		},
		{
			Name: "Minimal config",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml,
				Env:  minimalEnv(),
			},
			Expected: Config{
				Type:     "pgsql",
				Host:     "localhost",
				User:     "icinga",
				Database: "icingadb",
				Password: "secret",
				Options:  defaultOptions,
			},
		},
		{
			Name: "Minimal config with password file",
			Data: testutils.ConfigTestData{
				Yaml: fmt.Sprintf(`
type: pgsql
host: localhost
user: icinga
database: icingadb
password_file: %s`, passwordFile),
				Env: map[string]string{
					"TYPE":          "pgsql",
					"HOST":          "localhost",
					"USER":          "icinga",
					"DATABASE":      "icingadb",
					"PASSWORD_FILE": passwordFile,
				},
			},
			Expected: Config{
				Type:         "pgsql",
				Host:         "localhost",
				User:         "icinga",
				Database:     "icingadb",
				Password:     "secret",
				PasswordFile: passwordFile,
				Options:      defaultOptions,
			},
		},
		{
			Name: "Retain defaults",
			Data: testutils.ConfigTestData{
				Yaml: `
host: localhost
user: icinga
database: icinga`,
				Env: map[string]string{
					"HOST":     "localhost",
					"USER":     "icinga",
					"DATABASE": "icinga",
				},
			},
			Expected: Config{
				Type:     "mysql", // Default
				Host:     "localhost",
				User:     "icinga",
				Database: "icinga",
				Options:  defaultOptions,
			},
		},
		{
			Name: "TLS",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml + `
tls: true
cert: cert.pem
key: key.pem
ca: ca.pem`,
				Env: withMinimalEnv(map[string]string{
					"TLS":  "1",
					"CERT": "cert.pem",
					"KEY":  "key.pem",
					"CA":   "ca.pem",
				}),
			},
			Expected: Config{
				Type:     "pgsql",
				Host:     "localhost",
				User:     "icinga",
				Database: "icingadb",
				Password: "secret",
				Options:  defaultOptions,
				TlsOptions: config.TLS{
					Enable: true,
					Cert:   "cert.pem",
					Key:    "key.pem",
					Ca:     "ca.pem",
				},
			},
		},
		{
			Name: "TLS with raw PEM",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml + `
tls: true
cert: |-
  -----BEGIN CERTIFICATE-----
  MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
  DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
  EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
  7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
  5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
  BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
  NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
  Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
  6MF9+Yw1Yy0t
  -----END CERTIFICATE-----
key: |-
  -----BEGIN EC PRIVATE KEY-----
  MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
  AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
  EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
  -----END EC PRIVATE KEY-----
ca: |-
  -----BEGIN CERTIFICATE-----
  MIICSTCCAfOgAwIBAgIUcmQfIJAvbxdVm0PFanS4FWH71Z0wDQYJKoZIhvcNAQEL
  BQAweTELMAkGA1UEBhMCREUxEjAQBgNVBAgMCUZyYW5jb25pYTESMBAGA1UEBwwJ
  TnVyZW1iZXJnMUIwQAYDVQQKDDlIb25lc3QgTWFya3VzJyBVc2VkIE51Y2xlYXIg
  UG93ZXIgUGxhbnRzIGFuZCBDZXJ0aWZpY2F0ZXMwHhcNMjUwMzA1MDk0ODIwWhcN
  MjUwMzA2MDk0ODIwWjB5MQswCQYDVQQGEwJERTESMBAGA1UECAwJRnJhbmNvbmlh
  MRIwEAYDVQQHDAlOdXJlbWJlcmcxQjBABgNVBAoMOUhvbmVzdCBNYXJrdXMnIFVz
  ZWQgTnVjbGVhciBQb3dlciBQbGFudHMgYW5kIENlcnRpZmljYXRlczBcMA0GCSqG
  SIb3DQEBAQUAA0sAMEgCQQCeEGX2IolvELSUjC1DqvJRbTs4DKwE8ZZHDAGrc5K9
  DFrLKvkwgfv3g9R2NJE5o/A5vBLq22IDCFdI26M6t10HAgMBAAGjUzBRMB0GA1Ud
  DgQWBBQn+dCzVtAzYOGC8tIi9JLmRbWI7jAfBgNVHSMEGDAWgBQn+dCzVtAzYOGC
  8tIi9JLmRbWI7jAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA0EAlA27
  ti1NKC+o+iZtyU8I/32aPaFme1+eQNIxvqXfw49jSM/FyDjhfZ0XlAxmK6tzF3mM
  LJZsYbxapLeyWoA05Q==
  -----END CERTIFICATE-----
`,
				Env: withMinimalEnv(map[string]string{ // #nosec G101 -- demo private key
					"TLS": "1",
					"CERT": `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`,
					"KEY": `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`,
					"CA": `-----BEGIN CERTIFICATE-----
MIICSTCCAfOgAwIBAgIUcmQfIJAvbxdVm0PFanS4FWH71Z0wDQYJKoZIhvcNAQEL
BQAweTELMAkGA1UEBhMCREUxEjAQBgNVBAgMCUZyYW5jb25pYTESMBAGA1UEBwwJ
TnVyZW1iZXJnMUIwQAYDVQQKDDlIb25lc3QgTWFya3VzJyBVc2VkIE51Y2xlYXIg
UG93ZXIgUGxhbnRzIGFuZCBDZXJ0aWZpY2F0ZXMwHhcNMjUwMzA1MDk0ODIwWhcN
MjUwMzA2MDk0ODIwWjB5MQswCQYDVQQGEwJERTESMBAGA1UECAwJRnJhbmNvbmlh
MRIwEAYDVQQHDAlOdXJlbWJlcmcxQjBABgNVBAoMOUhvbmVzdCBNYXJrdXMnIFVz
ZWQgTnVjbGVhciBQb3dlciBQbGFudHMgYW5kIENlcnRpZmljYXRlczBcMA0GCSqG
SIb3DQEBAQUAA0sAMEgCQQCeEGX2IolvELSUjC1DqvJRbTs4DKwE8ZZHDAGrc5K9
DFrLKvkwgfv3g9R2NJE5o/A5vBLq22IDCFdI26M6t10HAgMBAAGjUzBRMB0GA1Ud
DgQWBBQn+dCzVtAzYOGC8tIi9JLmRbWI7jAfBgNVHSMEGDAWgBQn+dCzVtAzYOGC
8tIi9JLmRbWI7jAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA0EAlA27
ti1NKC+o+iZtyU8I/32aPaFme1+eQNIxvqXfw49jSM/FyDjhfZ0XlAxmK6tzF3mM
LJZsYbxapLeyWoA05Q==
-----END CERTIFICATE-----`,
				}),
			},
			Expected: Config{
				Type:     "pgsql",
				Host:     "localhost",
				User:     "icinga",
				Database: "icingadb",
				Password: "secret",
				Options:  defaultOptions,
				TlsOptions: config.TLS{ // #nosec G101 -- demo private key
					Enable: true,
					Cert: `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`,
					Key: `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`,
					Ca: `-----BEGIN CERTIFICATE-----
MIICSTCCAfOgAwIBAgIUcmQfIJAvbxdVm0PFanS4FWH71Z0wDQYJKoZIhvcNAQEL
BQAweTELMAkGA1UEBhMCREUxEjAQBgNVBAgMCUZyYW5jb25pYTESMBAGA1UEBwwJ
TnVyZW1iZXJnMUIwQAYDVQQKDDlIb25lc3QgTWFya3VzJyBVc2VkIE51Y2xlYXIg
UG93ZXIgUGxhbnRzIGFuZCBDZXJ0aWZpY2F0ZXMwHhcNMjUwMzA1MDk0ODIwWhcN
MjUwMzA2MDk0ODIwWjB5MQswCQYDVQQGEwJERTESMBAGA1UECAwJRnJhbmNvbmlh
MRIwEAYDVQQHDAlOdXJlbWJlcmcxQjBABgNVBAoMOUhvbmVzdCBNYXJrdXMnIFVz
ZWQgTnVjbGVhciBQb3dlciBQbGFudHMgYW5kIENlcnRpZmljYXRlczBcMA0GCSqG
SIb3DQEBAQUAA0sAMEgCQQCeEGX2IolvELSUjC1DqvJRbTs4DKwE8ZZHDAGrc5K9
DFrLKvkwgfv3g9R2NJE5o/A5vBLq22IDCFdI26M6t10HAgMBAAGjUzBRMB0GA1Ud
DgQWBBQn+dCzVtAzYOGC8tIi9JLmRbWI7jAfBgNVHSMEGDAWgBQn+dCzVtAzYOGC
8tIi9JLmRbWI7jAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA0EAlA27
ti1NKC+o+iZtyU8I/32aPaFme1+eQNIxvqXfw49jSM/FyDjhfZ0XlAxmK6tzF3mM
LJZsYbxapLeyWoA05Q==
-----END CERTIFICATE-----`,
				},
			},
		},
		{
			Name: "max_connections cannot be 0",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml + `
options:
  max_connections: 0`,
				Env: withMinimalEnv(map[string]string{"OPTIONS_MAX_CONNECTIONS": "0"}),
			},
			Error: testutils.ErrorContains("max_connections cannot be 0"),
		},
		{
			Name: "max_connections_per_table must be at least 1",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml + `
options:
  max_connections_per_table: 0`,
				Env: withMinimalEnv(map[string]string{"OPTIONS_MAX_CONNECTIONS_PER_TABLE": "0"}),
			},
			Error: testutils.ErrorContains("max_connections_per_table must be at least 1"),
		},
		{
			Name: "max_placeholders_per_statement must be at least 1",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml + `
options:
  max_placeholders_per_statement: 0`,
				Env: withMinimalEnv(map[string]string{"OPTIONS_MAX_PLACEHOLDERS_PER_STATEMENT": "0"}),
			},
			Error: testutils.ErrorContains("max_placeholders_per_statement must be at least 1"),
		},
		{
			Name: "max_rows_per_transaction must be at least 1",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml + `
options:
  max_rows_per_transaction: 0`,
				Env: withMinimalEnv(map[string]string{"OPTIONS_MAX_ROWS_PER_TRANSACTION": "0"}),
			},
			Error: testutils.ErrorContains("max_rows_per_transaction must be at least 1"),
		},
		{
			Name: "wsrep_sync_wait can only be set to a number between 0 and 15",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml + `
options:
  wsrep_sync_wait: 16`,
				Env: withMinimalEnv(map[string]string{"OPTIONS_WSREP_SYNC_WAIT": "16"}),
			},
			Error: testutils.ErrorContains("wsrep_sync_wait can only be set to a number between 0 and 15"),
		},
		{
			Name: "Options retain defaults",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml + `
options:
  max_connections: 8
  max_connections_per_table: 4`,
				Env: withMinimalEnv(map[string]string{
					"OPTIONS_MAX_CONNECTIONS":           "8",
					"OPTIONS_MAX_CONNECTIONS_PER_TABLE": "4",
				}),
			},
			Expected: Config{
				Type:     "pgsql",
				Host:     "localhost",
				User:     "icinga",
				Database: "icingadb",
				Password: "secret",
				Options: Options{
					MaxConnections:              8,
					MaxConnectionsPerTable:      4,
					MaxPlaceholdersPerStatement: defaultOptions.MaxPlaceholdersPerStatement,
					MaxRowsPerTransaction:       defaultOptions.MaxRowsPerTransaction,
					WsrepSyncWait:               defaultOptions.WsrepSyncWait,
				},
			},
		},
		{
			Name: "Options",
			Data: testutils.ConfigTestData{
				Yaml: minimalYaml + `
options:
  max_connections: 8
  max_connections_per_table: 4
  max_placeholders_per_statement: 4096
  max_rows_per_transaction: 2048
  wsrep_sync_wait: 15`,
				Env: withMinimalEnv(map[string]string{
					"OPTIONS_MAX_CONNECTIONS":                "8",
					"OPTIONS_MAX_CONNECTIONS_PER_TABLE":      "4",
					"OPTIONS_MAX_PLACEHOLDERS_PER_STATEMENT": "4096",
					"OPTIONS_MAX_ROWS_PER_TRANSACTION":       "2048",
					"OPTIONS_WSREP_SYNC_WAIT":                "15",
				}),
			},
			Expected: Config{
				Type:     "pgsql",
				Host:     "localhost",
				User:     "icinga",
				Database: "icingadb",
				Password: "secret",
				Options: Options{
					MaxConnections:              8,
					MaxConnectionsPerTable:      4,
					MaxPlaceholdersPerStatement: 4096,
					MaxRowsPerTransaction:       2048,
					WsrepSyncWait:               15,
				},
			},
		},
	}

	t.Run("FromEnv", func(t *testing.T) {
		for _, tc := range configTests {
			t.Run(tc.Name, tc.F(func(data testutils.ConfigTestData) (Config, error) {
				var actual Config

				err := config.FromEnv(&actual, config.EnvOptions{Environment: data.Env})

				return actual, err
			}))
		}
	})

	t.Run("FromYAMLFile", func(t *testing.T) {
		for _, tc := range configTests {
			t.Run(tc.Name+"/FromYAMLFile", tc.F(func(data testutils.ConfigTestData) (Config, error) {
				var actual Config

				var err error
				testutils.WithYAMLFile(t, data.Yaml, func(file *os.File) {
					err = config.FromYAMLFile(file.Name(), &actual)
				})

				return actual, err
			}))
		}
	})
}
