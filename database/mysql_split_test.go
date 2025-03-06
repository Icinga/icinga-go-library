package database

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMysqlSplitQueries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{{
		name:  "empty",
		input: "",
		want:  nil,
	}, {
		name:  "default_delimiter",
		input: "q1;\nq2;\nq3;\n",
		want:  []string{"q1", "q2", "q3"},
	}, {
		name:  "delimiter_at_eof",
		input: "q1;",
		want:  []string{"q1"},
	}, {
		name:  "delimiter_switch",
		input: "q1;\ndelimiter //\nq2//\ndelimiter ;\nq3;\n",
		want:  []string{"q1", "q2", "q3"},
	}, {
		name:  "delimiter_as_column_name",
		input: "SELECT 1 AS\ndelimiter WHERE\n1=1;\nSELECT 42 WHERE\n1=1",
		want:  []string{"SELECT 1 AS\ndelimiter WHERE\n1=1", "SELECT 42 WHERE\n1=1"},
	}, {
		name:  "delimiter_as_value",
		input: "SELECT ';';\ndelimiter //\nSELECT '//'//",
		want:  []string{"SELECT ';'", "SELECT '//'"},
	}, {
		name:  "delimiters_but_no_queries",
		input: "DELIMITER //\nDELIMITER ;",
		want:  nil,
	}, {
		name:  "extra_newlines",
		input: "\n\n\nSELECT 1;\n\n\nDELIMITER //\n\n\nSELECT 42//\n\n\nSELECT 23\n\n\n",
		want:  []string{"SELECT 1", "SELECT 42", "SELECT 23"},
	}, {
		name:  "ignore_empty_statements",
		input: "SELECT 1\n;\n;\nSELECT 2\n;\n;\nSELECT 3\n;\n;\n",
		want:  []string{"SELECT 1", "SELECT 2", "SELECT 3"},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, MysqlSplitStatements(tt.input), "MysqlSplitStatements(%v)", tt.input)
		})
	}
}
