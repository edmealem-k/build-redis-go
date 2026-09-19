# Lesson 05: RESP Array Parsing — The Real Wire Protocol

> **Core Focus**: Decoding client requests framed as RESP Arrays (`*<count>\r\n`), binary-safe streaming with `io.ReadFull`, dual-mode parsing, and basic in-memory `SET`/`GET` key-value storage.

---

## 1. Redis Internals & Protocol Deep-Dive

### Why Real Redis Clients Never Use Space-Delimited Commands

Until now, our server relied on inline space-delimited text commands (`PING`, `ECHO "hello"`). While handy for manual `telnet` testing, real Redis clients (`redis-cli`, `redis-py`, `ioredis`, `go-redis`) **never** transmit commands as space-separated text.

Space-splitting fails in production systems because:
1. **Values containing whitespace**: A value like `"John Doe"` or `"SELECT * FROM users"` is ambiguous without complex escaping rules.
2. **Binary payloads**: Image files, compressed gzip buffers, protobufs, or encrypted blobs can contain arbitrary null bytes (`\0`) or newlines (`\r\n`), which would prematurely truncate naive line scanners.

Real Redis completely avoids guessing where tokens begin and end by prefixing every piece of data with its exact byte count.

---

### The RESP Array Format (`*<count>\r\n`)

Every client request is transmitted as a **RESP Array of Bulk Strings**:

```text
*<num_arguments>\r\n
$<len1>\r\n<arg1>\r\n
$<len2>\r\n<arg2>\r\n
...
$<lenN>\r\n<argN>\r\n
```

#### Example: `SET foo bar`
```text
*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n
```

1. `*3\r\n`: An array containing 3 elements.
2. `$3\r\nSET\r\n`: Element 1 is a 3-byte bulk string `"SET"`.
3. `$3\r\nfoo\r\n`: Element 2 is a 3-byte bulk string `"foo"`.
4. `$3\r\nbar\r\n`: Element 3 is a 3-byte bulk string `"bar"`.

---

### The Binary-Safe Parsing Rule

When parsing incoming commands:
1. **Headers are lines**: Read the array element count (`*<N>\r\n`) and each bulk string length (`$<len>\r\n`) as lines, because headers are guaranteed to be ASCII digits terminated by `\r\n`.
2. **Payloads are exact byte reads**: Read **exactly `<len>` bytes** from the stream (using `io.ReadFull`). **Never scan the payload for spaces or newlines.**
3. **Trailing delimiters**: Unconditionally consume the trailing `\r\n` (2 bytes) following each payload.

---

## 2. Go Concepts & Idioms Learned

### A. `bufio.Reader` and `io.ReadFull`
While `bufio.Scanner` is convenient for line-by-line reading, it cannot read fixed byte slices mid-stream.
`bufio.Reader` paired with `io.ReadFull` provides the exact primitives required:

```go
// Allocate buffer of exact byte length
buf := make([]byte, length)

// Read exactly length bytes, blocking until all bytes are received or error
if _, err := io.ReadFull(reader, buf); err != nil {
    return nil, err
}

// Discard trailing \r\n
crlf := make([]byte, 2)
io.ReadFull(reader, crlf)
```

### B. Modern Go 1.22+ Integer Range
Instead of the classic C-style `for i := 0; i < count; i++`, Go 1.22+ allows iterating directly over an integer:
```go
for range count {
    // executes count times
}
```

### C. Graceful EOF Handling on Non-Newline Terminated Streams
`reader.ReadString('\n')` can return `io.EOF` alongside the bytes read if the client disconnects or if a test file lacks a trailing newline. Checking:
```go
line, err := reader.ReadString('\n')
if err != nil && len(line) == 0 {
    return nil, err
}
```
ensures that trailing commands ending directly at EOF are processed instead of dropped.

### D. Dual-Mode Protocol Support (Array + Inline Fallback)
By inspecting the first character:
- If `line[0] == '*'`: Parse as binary-safe **RESP Array**.
- Otherwise: Fall back to inline text tokenizer (`parseArgs(line)`).

This allows the server to simultaneously speak modern RESP2/RESP3 and support raw interactive `telnet` connections.

---

## 3. Implementation Reference (`main.go`)

### 1. The Wire Parser (`readCommand`)

```go
func readCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil // skip blank lines
	}

	// 1. RESP Array Format (starts with '*')
	if line[0] == '*' {
		count, err := strconv.Atoi(line[1:])
		if err != nil {
			return nil, err
		}

		args := make([]string, 0, count)
		for range count {
			// Read bulk string length header (e.g. "$4\r\n")
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

	// 2. Fallback: Inline commands (backwards compatible with telnet & early lessons)
	return parseArgs(line), nil
}
```

### 2. In-Memory Key-Value Handlers (`SET` and `GET`)

```go
var store = make(map[string]string)

func cmdSet(args []string) string {
	store[args[1]] = args[2]
	return encodeSimpleString("OK")
}

func cmdGet(args []string) string {
	val, exists := store[args[1]]
	if !exists {
		return encodeNull()
	}
	return encodeBulkString(val)
}
```

Registered with arity bounds in `commands`:
```go
"SET": {handler: cmdSet, arity: Arity{min: 2, max: 2}},
"GET": {handler: cmdGet, arity: Arity{min: 1, max: 1}},
```

---

## 4. Verification & Test Vectors

The test suite in `tests/05-resp-array-parsing/` validates:

| Test File | Input | Expected Output | Validated Behavior | Result |
| :--- | :--- | :--- | :--- | :---: |
| `tests/05-resp-array-parsing/1.in` | `*1\r\n$4\r\nPING\r\n` | `+PONG\r\n` | Single-element RESP array | **PASS** |
| `tests/05-resp-array-parsing/2.in` | `*2\r\n$4\r\nECHO\r\n$11\r\nhello world\r\n` | `$11\r\nhello world\r\n` | Multi-word payload preserving spaces | **PASS** |
| `tests/05-resp-array-parsing/3.in` | `*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n`<br>`*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n` | `+OK\r\n`<br>`$3\r\nbar\r\n` | Sequential stateful commands over same stream | **PASS** |

Run tests locally:
```bash
./run_tests.sh 05
```
All tests passing (3 passed, 0 failed).

