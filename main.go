package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

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

func (a Arity) checkArity(cmd string, args []string) string {
	if a.min > len(args) {
		return encodeError(fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd))
	}

	if (a.max != -1) && (len(args) > a.max) {
		return encodeError(fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd))
	}

	return ""
}

var store = make(map[string]string)

var commands = map[string]Command{
	"PING":    {handler: cmdPing, arity: Arity{min: 0, max: 1}},
	"ECHO":    {handler: cmdEcho, arity: Arity{min: 1, max: 1}},
	"COMMAND": {handler: cmdCommand, arity: Arity{min: 0, max: -1}},
	"SET":     {handler: cmdSet, arity: Arity{min: 2, max: 2}},
	"GET":     {handler: cmdGet, arity: Arity{min: 1, max: 1}},
}

// ////////////////////////////////////////////////////////////
// //////////////Command Handlers/////////////////////////////////////
// ////////////////////////////////////////////////////////////
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

func cmdSet(args []string) string {
	key, value := args[1], args[2]
	store[key] = value

	return encodeSimpleString("OK")
}

func cmdGet(args []string) string {
	key := args[1]

	val, exists := store[key]
	if !exists {
		return encodeNull()
	}

	return encodeBulkString(val)
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

// ////////////////////////////////////////////////////////////
// ////////////////Entry Point////////////////////////////////
// ////////////////////////////////////////////////////////////
func main() {
	// bufio.Scanner reads incoming stream line-by-line from stdin
	reader := bufio.NewReader(os.Stdin)

	for {
		args, err := readCommand(reader)
		if err != nil {
			break // EOF / connection closed
		}
		if len(args) == 0 {
			continue
		}

		response := handleCommand(args)
		os.Stdout.WriteString(response)
	}
}

func readCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil // skip blank lines
	}

	// 1. If it's a RESP Array (starts with '*')
	if line[0] == '*' {
		count, err := strconv.Atoi(line[1:])
		if err != nil {
			return nil, err
		}

		args := make([]string, 0, count)
		for i := 0; i < count; i++ {
			// Read bulk string header (e.g. "$4\r\n")
			bulkHeader, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			bulkHeader = strings.TrimSpace(bulkHeader)
			if len(bulkHeader) == 0 || bulkHeader[0] != '$' {
				return nil, fmt.Errorf("expected '$', got %s", bulkHeader)
			}

			length, err := strconv.Atoi(bulkHeader[1:])
			if err != nil {
				return nil, err
			}

			// Read EXACTLY length bytes into a buffer
			buf := make([]byte, length)
			if _, err := io.ReadFull(reader, buf); err != nil {
				return nil, err
			}

			// Discard trailing \r\n (2 bytes)
			crlf := make([]byte, 2)
			if _, err := io.ReadFull(reader, crlf); err != nil {
				return nil, err
			}

			args = append(args, string(buf))
		}
		return args, nil
	}

	// 2. Fallback: Inline commands (for backwards compatibility with Lessons 01-04)
	return parseArgs(line), nil
}

// ////////////////////////////////////////////////////////////
// Helper functions
// ////////////////////////////////////////////////////////////
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
