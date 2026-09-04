package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func handleCommand(args []string) string {
	// Guard against empty command submissions
	if len(args) == 0 {
		return ""
	}

	// Redis command names are case-insensitive
	cmd := strings.ToUpper(args[0])

	switch cmd {
	case "PING":
		// Handle bare "PING" -> Simple String "+PONG\r\n"
		if len(args) == 1 {
			return "+PONG\r\n"
		}

		// Handle "PING <message>" -> Bulk String "$<len>\r\n<message>\r\n"
		// If additional arguments are provided, Redis echoes the first argument back
		return encodeBulkString(args[1])
	default:
		// unknown command error format following RESP specs
		return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmd)
	}
}

// encodeBulkString formats raw string content into a RESP Bulk String:
func encodeBulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func main() {
	// bufio.Scanner reads incoming stream line-by-line from stdin
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Ignore blank lines
		if line == "" {
			continue
		}

		// Parse shell-style arguments (preserving quoted strings)
		args := parseArgs(line)

		// Execute command and write exact byte sequence to stdout
		response := handleCommand(args)
		os.Stdout.WriteString(response)
	}
}

// parseArgs tokenizes an inline command string while
// keeping quoted values together.
func parseArgs(line string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	for _, ch := range line {
		switch {
		case ch == '"' && !inQuotes:
			// Start capturing inside quotes
			inQuotes = true
		case ch == '"' && inQuotes:
			// Finished quoted segment
			inQuotes = false
		case ch == ' ' && !inQuotes:
			// Space outside quotes marks the end of an argument token
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	// Flush any remaining accumulated token
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
