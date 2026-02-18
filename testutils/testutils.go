// Package testutils provides utilities for testing, including generic test case structures
// and helper functions for error checking and temporary file handling.
//
// This package is designed to simplify the process of writing tests by providing reusable
// components that handle common testing scenarios, such as comparing expected and actual results,
// checking for specific error conditions, and managing temporary files.
package testutils

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

// TestCase represents a generic test case structure.
// It is parameterized by T, the type of the expected result, and D, the type of the test data.
// This struct is useful for defining test cases with expected outcomes and associated data.
type TestCase[T any, D any] struct {
	// Name is the identifier for the test case, used for reporting purposes.
	Name string
	// Expected is the anticipated result of the test. It should be left empty if an error is expected.
	Expected T
	// Data contains the input or configuration for the test case.
	Data D
	// Error is a function that checks the error returned by the test function, if an error is anticipated.
	Error func(*testing.T, error)
}

// F returns a test function that executes the logic of the test case, suitable for use with t.Run().
// It takes a function f that processes the test data and returns an actual result along with an error, if any.
// After executing f, it verifies the actual result against the expected result or evaluates the error condition.
func (tc TestCase[T, D]) F(f func(D) (T, error)) func(t *testing.T) {
	return func(t *testing.T) {
		actual, err := f(tc.Data)

		if tc.Error != nil {
			tc.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.Expected, actual)
		}
	}
}

// ConfigTestData holds test data for loading and validating configuration from
// both YAML files and environment variables.
type ConfigTestData struct {
	// YAML file content to be tested.
	Yaml string
	// Environment variables to be used in the test.
	Env map[string]string
}

// ErrorAs returns a function that checks if the error is of a specific type T.
// This is useful for verifying that an error matches a particular interface or concrete type.
func ErrorAs[T error]() func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		var expected T
		require.ErrorAs(t, err, &expected)
	}
}

// ErrorContains returns a function that checks if the error message contains the expected substring.
// This is useful for validating that an error message includes specific information.
func ErrorContains(expected string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		require.ErrorContains(t, err, expected)
	}
}

// ErrorIs returns a function that checks if the error is equal to the expected error.
// This is useful for confirming that an error is exactly the one anticipated.
func ErrorIs(expected error) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		require.ErrorIs(t, err, expected)
	}
}

// WithYAMLFile creates a temporary YAML file with the provided content and executes a function with the file.
// It ensures the file is removed after the function execution, preventing resource leaks.
// This utility is helpful for tests that require file-based configuration.
func WithYAMLFile(t *testing.T, yaml string, f func(file *os.File)) {
	file, err := os.CreateTemp("", "*.yaml")
	require.NoError(t, err)

	defer func(name string) {
		_ = os.Remove(name) // #nosec G703 -- name is not user supplied, but from os.CreateTemp
	}(file.Name())

	_, err = file.WriteString(yaml)
	require.NoError(t, err)

	require.NoError(t, file.Close())

	f(file)
}
