package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func encodeSimpleString(s string) string {
	return fmt.Sprintf("+%s\r\n", s)
}

func encodeError(s string) string {
	return fmt.Sprintf("-%s\r\n", s)
}

func encodeInteger(i int) string {
	return fmt.Sprintf(":%d\r\n", i)
}

// encodeBulkString formats raw string content into a RESP Bulk String:
func encodeBulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func encodeNull() string {
	return "$-1\r\n"
}

type (
	CommandHandler func(args []string) string
	Arity          struct {
		min int // minimum arguments (excluding command name)
		max int // maximum arguments (-1 means unlimited / variadic)
	}
	Command struct {
		handler CommandHandler
		arity   Arity
	}
)

var commands = map[string]Command{
	"PING":    {handler: cmdPing, arity: Arity{min: 0, max: 1}},
	"ECHO":    {handler: cmdEcho, arity: Arity{min: 1, max: 1}},
	"COMMAND": {handler: cmdCommand, arity: Arity{min: 0, max: -1}},
}

func (a Arity) checkArity(cmd string, args []string) string {
	if a.min > len(args) {
		return encodeError(fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd))
	}

	if (a.max != -1) && (len(args) > a.max) {
		return encodeError(fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd))
	}

	return ""
}

func cmdPing(args []string) string {
	if len(args) == 1 {
		return encodeSimpleString("PONG")
	}

	return encodeBulkString(args[1])
}

func cmdEcho(args []string) string {
	msg := strings.Join(args[1:], " ")
	return encodeBulkString(msg)
}

func cmdCommand(args []string) string {
	return encodeSimpleString("OK")
}

func handleCommand(args []string) string {
	// Guard against empty command submissions
	if len(args) == 0 {
		return ""
	}

	// Redis command names are case-insensitive
	cmd := strings.ToUpper(args[0])

	command, exists := commands[cmd]

	if !exists {
		return encodeError(fmt.Sprintf("ERR unknown command '%s'", cmd))
	}

	if errmsg := command.arity.checkArity(cmd, args[1:]); errmsg != "" {
		return errmsg
	}

	return command.handler(args)
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
// parseArgs("PING bar")
// In Go, strings are immutable, repeatedly doing `str += string(ch)`
// creates a brand new string in memory on every single letter.
// `strings.Builder` is Go's standard way to build a string piece-by-piece in memory.
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
