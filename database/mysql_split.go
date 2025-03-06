package database

import (
	"regexp"
	"strings"
)

var delimiterCommandRe = regexp.MustCompile(`(?im)\A\s*delimiter\s*(\S+)\s*$`)

// MysqlSplitStatements takes a string containing multiple SQL statements and splits them into individual statements
// with limited support for the DELIMITER keyword like implemented by the mysql command line client.
//
// The main purpose of this function is to allow importing a schema file containing stored functions from Go. Such files
// have to specify an alternative delimiter internally if the function has semicolons in its body, otherwise the mysql
// command line clients splits the CREATE FUNCTION statement somewhere in the middle. This delimiter handling is not
// supported by the MySQL server, so when trying to import such a schema file using a different method than the mysql
// command line client, the delimiter handling has to be reimplemented. This is what this function does.
//
// To avoid an overly complex implementation, this function has some limitations on its input:
//   - Specifying a delimiter using a quoted string is NOT supported.
//   - Statements are only split if the delimiter appears at the end of a line. This in done in order to avoid
//     accidentally splitting in the middle of string literals and comments.
//   - The function does not attempt to handle comments in any way, so there must not be a delimiter at the end of a line
//     within a comment.
//   - The delimiter command is only recognized at the beginning of the file or immediately following a delimiter at the
//     end of a previous line, there must not be a comment in between, empty lines are fine.
func MysqlSplitStatements(statements string) []string {
	delimiterRe := makeDelimiterRe(";")

	var result []string

	for len(statements) > 0 {
		if match := delimiterCommandRe.FindStringSubmatch(statements); match != nil {
			delimiterRe = makeDelimiterRe(match[1])
			statements = statements[len(match[0]):]
			continue
		}

		split := delimiterRe.Split(statements, 2)

		if statement := strings.TrimSpace(split[0]); len(statement) > 0 {
			result = append(result, statement)
		}

		if len(split) > 1 {
			statements = split[1]
		} else {
			statements = ""
		}

	}

	return result
}

func makeDelimiterRe(delimiter string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)` + regexp.QuoteMeta(delimiter) + `$`)
}
