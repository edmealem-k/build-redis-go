# Lesson 01: PING — Your First Redis Command

> **Core Focus**: RESP wire protocol fundamentals, Simple vs. Bulk strings, and Go stream I/O.

---

## 1. Redis Internals & Protocol Deep-Dive

Every Redis client communicates with the server using a text-based protocol called **RESP (REdis Serialization Protocol)**. While modern databases often use binary protocols (like Protobuf or custom binary framing), Redis chose RESP for three intentional design reasons:
1. **Human readable**: Easy to debug with tools like `telnet`, `nc`, or Wireshark without custom decoders.
2. **Fast to parse**: Prefix-based type indicators and length headers allow linear \(O(1)\) byte jumps without back-tracking.
3. **Binary-safe**: Payloads can contain arbitrary bytes (JPEG images, null bytes `\0`, compressed payloads, or raw binary data) without escaping.

### Command Specification: `PING`
```text
PING [message]
```
- **Syntax**: `PING` accepts either zero arguments or a single optional argument.
- **Complexity**: \(O(1)\).
- **Purpose**: Verifies connection liveness, measures round-trip time (RTT) latency, and confirms the server is not blocked.

### Return Values: Simple String vs. Bulk String

The Redis specification mandates two distinct RESP types depending on how `PING` is called:

```text
Client: PING
Server: +PONG\r\n              <-- RESP Simple String

Client: PING "hello world"
Server: $11\r\nhello world\r\n <-- RESP Bulk String
```

#### Why `+PONG\r\n` (Simple String)?
- **Prefix `+`**: Denotes a **Simple String**.
- Simple strings are designed for low-overhead status replies (e.g. `+OK\r\n`, `+PONG\r\n`).
- **Constraint**: Simple strings **cannot contain CR (`\r`) or LF (`\n`) characters**. Because they have no length prefix, the client reads until it hits the first `\r\n`. Any internal newline would corrupt the framing.

#### Why `$11\r\nhello world\r\n` (Bulk String)?
- **Prefix `$`**: Denotes a **Bulk String**.
- When an argument is supplied, Redis echoes it back as a bulk string rather than a simple string.
- **Wire Format**:
  ```text
  $<byte_length>\r\n<raw_bytes>\r\n
  ```
- **The Length Prefix**: The integer immediately following `$` specifies the exact number of bytes in the payload. The client uses this number to allocate a buffer and read exactly that many bytes before checking for the final trailing `\r\n`.
- **Binary Safety**: Because the reader knows the exact byte length in advance, the payload can safely contain null bytes (`\0`), internal newlines (`\r\n`), or non-UTF8 binary data.

#### Why CRLF (`\r\n`) Everywhere?
Every RESP message and line ends strictly with `\r\n` (ASCII 13 Carriage Return + ASCII 10 Line Feed). This standard:
- Was inherited from Redis's original C codebase and traditional Internet protocol standards (HTTP 1.1, SMTP, Telnet).
- Ensures that a simple newline (`\n`) typed into a raw socket doesn't accidentally terminate multi-part frames.
- **Common Gotcha**: Omitting `\r` (sending only `\n`) causes standard Redis client libraries (like `go-redis` or `redis-py`) to hang or report protocol desynchronization errors.

---

## 2. Go Concepts & Idioms Learned

### A. `bufio.Scanner` for Stream Processing
Instead of reading the entire input into memory with `io.ReadAll(os.Stdin)`, we use `bufio.NewScanner(os.Stdin)`.
- **Buffered I/O**: `bufio.Scanner` reads data in chunks (default 64KB internal buffer) and yields tokens line-by-line via `scanner.Scan()`.
- **Memory Efficiency**: Processes infinite or long-lived socket/stdin streams without exhausting memory.

```go
scanner := bufio.NewScanner(os.Stdin)
for scanner.Scan() {
    line := strings.TrimSpace(scanner.Text())
    // ...
}
```

### B. Efficient String Accumulation with `strings.Builder`
In Go, strings are **immutable byte sequences**. Concatenating strings using `s += string(ch)` inside a loop causes a new memory allocation on every iteration:
- Each `+` copies the existing string into a newly allocated heap block, resulting in \(O(N^2)\) memory churn.
- **`strings.Builder`**: Manages an internal slice `[]byte` that grows exponentially, minimizing reallocations. Calling `builder.String()` converts the buffer to a `string` with zero heap copying.

```go
var current strings.Builder
// current.WriteRune(ch) appends UTF-8 bytes to the internal buffer
current.WriteRune(ch)
```

### C. Runes vs. Bytes in String Traversal
When iterating a string in Go:
```go
for _, ch := range line { ... }
```
- The variable `ch` is a `rune` (alias for `int32`), representing a decoded Unicode code point, **not** a raw `byte` (`uint8`).
- Go source code is UTF-8 encoded. `range` on a string automatically decodes multi-byte UTF-8 sequences.
- Using `current.WriteRune(ch)` correctly encodes the Unicode code point back into UTF-8 bytes.

### D. Direct Output Writing (`os.Stdout.WriteString`)
- `fmt.Print(response)` uses runtime reflection to inspect argument types before formatting.
- `os.Stdout.WriteString(response)` writes the underlying string bytes directly to file descriptor 1 via an optimized syscall, bypassing reflection overhead.

---

## 3. Wire Protocol & Data Flow Diagram

```text
                       Client Request (Inline Command)
                       "PING \"hello world\"\n"
                                  │
                                  ▼
                   ┌──────────────────────────────┐
                   │        bufio.Scanner         │  (Line-by-line reading)
                   └──────────────┬───────────────┘
                                  │ "PING \"hello world\""
                                  ▼
                   ┌──────────────────────────────┐
                   │          parseArgs           │  (Tokenizes arguments,
                   │                              │   preserves quoted strings)
                   └──────────────┬───────────────┘
                                  │ ["PING", "hello world"]
                                  ▼
                   ┌──────────────────────────────┐
                   │        handleCommand         │  (Case normalization: PING,
                   │                              │   Checks argument count)
                   └──────────────┬───────────────┘
                                  │
                  ┌───────────────┴───────────────┐
      len(args)==1 (Bare PING)        len(args)>1 (With Arg)
                  │                               │
                  ▼                               ▼
            Simple String                    Bulk String
            "+PONG\r\n"            "$11\r\nhello world\r\n"
                  │                               │
                  └───────────────┬───────────────┘
                                  │
                                  ▼
                   ┌──────────────────────────────┐
                   │    os.Stdout.WriteString     │  (Direct byte output)
                   └──────────────────────────────┘
```

---

## 4. Code Breakdown & Implementation Reference

### 1. Command Routing & Case Normalization
Redis command names are **case-insensitive**, but arguments are **case-sensitive**:

```go
func handleCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}

	// Redis command verbs are case-insensitive
	cmd := strings.ToUpper(args[0])

	switch cmd {
	case "PING":
		// Bare "PING" -> Simple String: "+PONG\r\n"
		if len(args) == 1 {
			return "+PONG\r\n"
		}

		// "PING <message>" -> Bulk String: "$<len>\r\n<message>\r\n"
		// If multiple arguments are passed, Redis echoes args[1]
		return encodeBulkString(args[1])

	default:
		// Standard Redis error format
		return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmd)
	}
}
```

### 2. Bulk String Serialization
Formats any raw string payload into standard RESP bulk string wire format:

```go
func encodeBulkString(s string) string {
	// Format: $<byte_length>\r\n<payload>\r\n
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}
```

### 3. Inline Command Tokenizer
Real clients can send inline commands separated by spaces, with strings grouped inside double quotes:

```go
func parseArgs(line string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false

	for _, ch := range line {
		switch {
		case ch == '"' && !inQuotes:
			inQuotes = true // Open quote: preserve subsequent spaces
		case ch == '"' && inQuotes:
			inQuotes = false // Close quote
		case ch == ' ' && !inQuotes:
			// Space outside quotes marks token boundary
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	// Append trailing token
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
```

### 4. Main Event Loop
Processes the incoming stream over standard I/O:

```go
func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		args := parseArgs(line)
		response := handleCommand(args)
		os.Stdout.WriteString(response)
	}
}
```

---

## 5. Redis Behavior & Edge Cases

| Scenario | Input | Expected Output | Rationale |
| :--- | :--- | :--- | :--- |
| **Bare PING** | `PING` | `+PONG\r\n` | Standard heartbeat / health check response. |
| **Mixed Casing** | `pInG` | `+PONG\r\n` | Command verb is case-insensitive. |
| **With Argument** | `PING "hello"` | `$5\r\nhello\r\n` | Echoes argument as a binary-safe Bulk String. |
| **Argument Casing** | `PING "HeLLo"` | `$5\r\nHeLLo\r\n` | Argument payload casing must be preserved. |
| **Empty Argument** | `PING ""` | `$0\r\n\r\n` | A zero-length bulk string has length 0 followed by CRLF. |
| **Unknown Command** | `FOOBAR` | `-ERR unknown command 'FOOBAR'\r\n` | RESP Simple Error prefixed with `-`. |

---

## 6. Architecture Cheat Sheet (Knowledge Base)

| Concept | Rule / Pattern | Go Implementation |
| :--- | :--- | :--- |
| **Simple String** | Starts with `+`, ends with `\r\n`, no internal newlines | `fmt.Sprintf("+%s\r\n", msg)` |
| **Bulk String** | Starts with `$`, followed by byte length, `\r\n`, payload, `\r\n` | `fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)` |
| **Simple Error** | Starts with `-`, followed by error code and message | `fmt.Sprintf("-ERR %s\r\n", msg)` |
| **CRLF Terminator** | Required across all RESP lines (ASCII 13 + ASCII 10) | Explicit `\r\n` literals |
| **Memory Allocation** | Avoid `+` in loops; accumulate in memory-backed buffers | `strings.Builder` |
| **Streaming I/O** | Read line-by-line without buffering full payload in memory | `bufio.NewScanner(os.Stdin)` |
