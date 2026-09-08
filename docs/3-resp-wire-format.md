# Lesson 03: The RESP Wire Format & Centralized Serializers

> **Core Focus**: Completing the core RESP type system, distinguishing errors and nulls from data, and building a centralized wire serializer in Go.

---

## 1. Redis Internals & Protocol Deep-Dive

### The Core RESP Type Alphabet

In this lesson, we formalize the fundamental data types that comprise every single message a Redis server returns:

| Prefix | RESP Type | Wire Format | Example | Purpose & Semantic Meaning |
| :---: | :--- | :--- | :--- | :--- |
| `+` | **Simple String** | `+<text>\r\n` | `+OK\r\n` | Fast status replies. Never contains newlines (`\r` or `\n`). |
| `-` | **Simple Error** | `-<msg>\r\n` | `-ERR unknown command 'FOO'\r\n` | Failure responses. Starts with an uppercase category (e.g. `ERR`, `WRONGTYPE`). |
| `:` | **Integer** | `:<number>\r\n` | `:42\r\n` | 64-bit signed integers (counters from `INCR`, lengths from `LLEN`, boolean-like counts from `DEL`). |
| `$` | **Bulk String** | `$<len>\r\n<data>\r\n` | `$5\r\nhello\r\n` | Binary-safe strings with explicit byte length. |
| `$` | **Null Bulk String** | `$-1\r\n` | `$-1\r\n` | **Null value sentinel.** Signals "key does not exist" or "nil". |

---

### Why Errors Are a Distinct Type (Not Magic Strings)

If Redis returned errors as regular bulk strings:
```text
$26\r\nERR wrong number of args\r\n
```
Client libraries would have to inspect string contents to determine whether a query failed or if a key happened to literally contain `"ERR wrong number of args"` as its stored value.

By dedicating the `-` prefix exclusively to errors:
- A client parser checks the **first byte**: if byte is `-`, it immediately raises an exception or returns an error object without parsing the payload as data.
- Standard convention: the error message begins with an uppercase error category word (`ERR`, `WRONGTYPE`, `NOSUCHKEY`, etc.) followed by a human-readable explanation.

---

### Why Null Needs Its Own Encoding (`$-1\r\n`)

In database design, **null (absence of a value)** is fundamentally different from an **empty value**:

| Representation | Wire Format | Semantic Meaning | Real-world Scenario |
| :--- | :--- | :--- | :--- |
| **Empty String** | `$0\r\n\r\n` | Key exists, value is `""` | `SET empty ""` $\to$ `GET empty` returns `$0\r\n\r\n` |
| **Null Bulk String** | `$-1\r\n` | Key does **not** exist | `GET missing_key` returns `$-1\r\n` (`nil`) |

`$-1\r\n` is not a string of length -1 in any literal sense. It is a protocol **sentinel** indicating that no value exists. Client SDKs map `$-1\r\n` directly to language-level nulls (`nil` in Go, `None` in Python, `null` in JavaScript).

---

### Introspection: `COMMAND DOCS`
Real Redis clients (and testing harnesses like redis-cli or test runners) often send introspection commands on connect:
- `COMMAND DOCS` was introduced in Redis 7.0 to return machine-readable command documentation and schemas.
- For our minimal server, returning `+OK\r\n` (via `encodeSimpleString("OK")`) acknowledges client capability negotiation without crashing or throwing unrecognized command errors.

---

## 2. Go Concepts & Architecture: Centralized Serializers

Rather than inlining manual `fmt.Sprintf` formatting throughout various command handlers, we encapsulate wire framing behind a dedicated serializer layer.

### Benefits in Go Systems Programming:
1. **Single Point of Change**: If protocol rules evolve (e.g. migrating to RESP3 or streaming direct byte buffers), only the serializer functions change.
2. **Readability**: Handlers read declaratively:
   ```go
   return encodeSimpleString("OK")
   ```
   instead of:
   ```go
   return "+OK\r\n"
   ```
3. **Safety**: Handlers can never accidentally forget the trailing `\r\n` delimiter.

---

## 3. Implementation Reference

### Serializer Functions (`main.go`)

```go
func encodeSimpleString(s string) string {
	return fmt.Sprintf("+%s\r\n", s)
}

func encodeError(s string) string {
	return fmt.Sprintf("-%s\r\n", s)
}

func encodeInteger(i int) string {
	return fmt.Sprintf(":%d\r\n", i)
}

func encodeBulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func encodeNull() string {
	return "$-1\r\n"
}
```

### Dispatch Table & `COMMAND` Handler

```go
func cmdCommand(args []string) string {
	return encodeSimpleString("OK")
}

type CommandHandler func(args []string) string

var handlers = map[string]CommandHandler{
	"PING":    cmdPing,
	"ECHO":    cmdEcho,
	"COMMAND": cmdCommand,
}

func handleCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}

	cmd := strings.ToUpper(args[0])

	if handler, exists := handlers[cmd]; exists {
		return handler(args)
	}

	return fmt.Sprintf("-ERR unknown command '%s'\r\n", cmd)
}
```

---

## 4. Verification & Test Vectors

The test suite in `tests/03-resp-format/` validates:

| Test File | Input | Expected Output | Validated Behavior |
| :--- | :--- | :--- | :--- |
| `tests/03-resp-format/1.in` | `PING`<br>`ECHO test`<br>`COMMAND DOCS` | `+PONG\r\n`<br>`$4\r\ntest\r\n`<br>`+OK\r\n` | Multi-command stream testing Simple String, Bulk String, and Introspection |
| `tests/03-resp-format/2.in` | `FOOBAR` | `-ERR unknown command 'FOOBAR'\r\n` | Catch-all unknown command error handling |

Run tests locally:
```bash
./run_tests.sh 03
```
All tests passing (2 passed, 0 failed).

