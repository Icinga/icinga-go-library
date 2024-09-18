package config

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"testing"
)

func TestParseFlags(t *testing.T) {
	t.Run("Simple flags", func(t *testing.T) {
		originalArgs := os.Args
		defer func() {
			os.Args = originalArgs
		}()

		os.Args = []string{"cmd", "--test-flag=value"}

		type Flags struct {
			TestFlag string `long:"test-flag"`
		}

		var flags Flags
		err := ParseFlags(&flags)
		require.NoError(t, err)
		require.Equal(t, "value", flags.TestFlag)
	})

	t.Run("Nil pointer argument", func(t *testing.T) {
		var flags *any

		err := ParseFlags(flags)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("Nil argument", func(t *testing.T) {
		err := ParseFlags(nil)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("Exit on help flag", func(t *testing.T) {
		// This test case checks the behavior of ParseFlags() when the help flag (e.g. -h) is provided.
		// Since ParseFlags() calls os.Exit() upon encountering the help flag, we need to run this
		// test in a separate subprocess to capture and verify the output without terminating the
		// main test process.
		if os.Getenv("TEST_HELP_FLAG") == "1" {
			// This block runs in the subprocess.
			type Flags struct{}
			var flags Flags

			originalArgs := os.Args
			defer func() {
				os.Args = originalArgs
			}()

			os.Args = []string{"cmd", "-h"}

			if err := ParseFlags(&flags); err != nil {
				panic(err)
			}

			return
		}

		// This block runs in the main test process. It starts this test again in a subprocess with the
		// TEST_HELP_FLAG=1 environment variable provided in order to run the above code block.
		// #nosec G204 -- The subprocess is launched with controlled input for testing purposes.
		// The command and arguments are derived from the test framework and are not influenced by external input.
		cmd := exec.Command(os.Args[0], fmt.Sprintf("-test.run=%s", t.Name()))
		cmd.Env = append(os.Environ(), "TEST_HELP_FLAG=1")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err)
		// When the help flag is provided, ParseFlags() outputs usage information,
		// including "-h, --help Show this help message" (whitespace may vary).
		require.Contains(t, string(out), "-h, --help")
	})
}
