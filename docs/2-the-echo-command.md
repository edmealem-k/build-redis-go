# Lesson 02: ECHO — Returning Data & Scaling Command Dispatch

> **Core Focus**: Parameter echoing via Bulk Strings, byte-length semantics, and scalable command dispatch architectures in Go.

---

## 1. Redis Internals & Protocol Mechanics

### Command Specification: `ECHO`
```text
ECHO message
```
- **Syntax**: `ECHO` accepts exactly one argument.
- **Complexity**: \(O(1)\).
- **Return Type**: **Bulk String** (`$<len>\r\n<message>\r\n`) containing the exact bytes sent by the client.
- **Purpose**: Diagnostics, client-server integrity verification, and ensuring arbitrary payloads round-trip without corruption.

### Byte Count vs. Character Count
A critical rule of RESP Bulk Strings:
> **The length prefix is strictly a byte count, NOT a character count.**

In standard ASCII (e.g. `"hello"`), character count equals byte count (5 characters = 5 bytes). However, for multi-byte UTF-8 or arbitrary binary payloads:
- In UTF-8, the emoji `"👋"` is **1 character / rune**, but **4 bytes** (`\xF0\x9F\x91\x8B`).
- Its RESP wire representation is:
  ```text
  $4\r\n👋\r\n
  ```
  *(Not `$1\r\n👋\r\n`, which would corrupt the client's framing parser).*

In Go, `len("👋")` evaluates to `4` because `len()` on a Go `string` returns the count of **underlying bytes**, not runes. This aligns naturally with RESP's byte-length wire protocol.

---

## 2. Command Dispatch Architecture: Scaling Handlers

With multiple commands now supported (`PING`, `ECHO`), how should command routing be structured in Go as the codebase expands?

### Approach A: The `switch` Statement
```go
switch cmd {
case "PING":
    ...
case "ECHO":
    ...
default:
    return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmd)
}
```
- **Characteristics**: Simple, fast, zero heap allocations.
- **Limitation**: As the course progresses to dozens of commands (`SET`, `GET`, `DEL`, `LPUSH`, `LRANGE`, etc.), a single switch statement in `handleCommand` becomes a monolith that mixes routing with business logic.

### Approach B: Table-Driven Dispatch (`map[string]CommandHandler`)
A common Go systems pattern for command routing is a dispatch map:
```go
type CommandHandler func(args []string) string

var commandHandlers = map[string]CommandHandler{
    "PING": handlePing,
    "ECHO": handleEcho,
}

func handleCommand(args []string) string {
    if len(args) == 0 {
        return ""
    }
    cmd := strings.ToUpper(args[0])
    if handler, exists := commandHandlers[cmd]; exists {
        return handler(args)
    }
    return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmd)
}
```
- **Characteristics**: Modular, clean separation of concerns, and easy to unit test individual command handlers in isolation.
- **Shared Helpers**: Factoring out serialization helpers like `encodeBulkString` ensures all command handlers share identical wire formatting without duplication.

---

## 3. Edge Cases & Protocol Gotchas

### 1. Missing Arguments (`len(args) < 2`)
What happens when a client sends a bare `ECHO` without arguments?
```text
Client: ECHO
Server: -ERR wrong number of arguments for 'echo' command\r\n
```
In Go, indexing `args[1]` directly when `len(args) == 1` causes a runtime panic (`index out of range [1] with length 1`), which crashes the server. Always guard argument boundaries defensively:
```go
if len(args) < 2 {
    return "-ERR wrong number of arguments for 'echo' command\r\n"
}
```

### 2. Multi-Word Arguments & Whitespace Ambiguity
If a client sends:
```text
ECHO hello world
```
Without quotes, an inline space tokenizer treats this as 3 tokens: `["ECHO", "hello", "world"]`.
- Is the payload `"hello"` or `"hello world"`?
- Our `parseArgs` supports quoted strings: `ECHO "hello world"` tokenizes cleanly into `["ECHO", "hello world"]`.
- This ambiguity in space-delimited text is why **real Redis clients never send plain space-delimited text in production**. Instead, clients transmit length-prefixed **RESP Arrays** (`*3\r\n$4\r\nECHO\r\n$5\r\nhello\r\n$5\r\nworld\r\n`), which eliminates delimiter ambiguity completely. We will implement RESP Array decoding in Lesson 05.

---

## 4. Implementation & Solution

### 1. Dedicated `cmdEcho` Handler
Guards against empty arguments to prevent slice out-of-bounds panics, joins any multi-word arguments with `strings.Join`, and returns the bulk string representation:

```go
func cmdEcho(args []string) string {
	// Guard against missing message argument
	if len(args) < 2 {
		return "-ERR wrong number of arguments for 'echo' command\r\n"
	}

	// Re-join tokens so unquoted multi-word messages are preserved
	msg := strings.Join(args[1:], " ")
	return encodeBulkString(msg)
}
```

### 2. Table-Driven Dispatch (`handlers` map)
Decouples command lookup from execution logic so new commands can be added by registering a single entry in the map:

```go
type CommandHandler func(args []string) string

var handlers = map[string]CommandHandler{
	"PING": cmdPing,
	"ECHO": cmdEcho,
}

func handleCommand(args []string) string {
	// Guard against empty command submissions
	if len(args) == 0 {
		return ""
	}

	// Redis command names are case-insensitive
	cmd := strings.ToUpper(args[0])

	if handler, exists := handlers[cmd]; exists {
		return handler(args)
	}

	return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmd)
}
```

### 3. Shared Bulk String Encoder
```go
// encodeBulkString formats raw string content into a RESP Bulk String:
func encodeBulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}
```

---

## 5. Verification & Test Vectors

The test suite in `tests/02-echo/` verifies:

| Test File | Input | Expected Output | Validated Behavior |
| :--- | :--- | :--- | :--- |
| `tests/02-echo/1.in` | `ECHO hey` | `$3\r\nhey\r\n` | Single-word argument echoing |
| `tests/02-echo/2.in` | `ECHO "hello world"` | `$11\r\nhello world\r\n` | Quoted string preserving internal spaces |
| `tests/02-echo/3.in` | `PING`<br>`ECHO foo`<br>`PING bar` | `+PONG\r\n`<br>`$3\r\nfoo\r\n`<br>`$3\r\nbar\r\n` | Multi-command stream interleaving `PING` and `ECHO` |

Run tests locally:
```bash
./run_tests.sh 02
```

