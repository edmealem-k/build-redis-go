# Lesson 06: SET & GET — Key-Value Storage

> **Core Focus**: Core in-memory associative storage (`SET`/`GET`), keyspace hash maps in Go, single-threaded atomicity, and distinguishing null bulk strings from empty strings.

---

## 1. Redis Internals & Systems Deep-Dive

### The Heart of Redis: The Keyspace Hash Map
This is the milestone where our project transitions from a protocol parser into a database. Everything prior was wire protocol plumbing. At its core, Redis is an ultra-fast, in-memory associative key-value store.

In Redis internals:
- The keyspace is fundamentally a hash table mapping string keys to string values.
- Advanced features added in later chapters (TTL expirations, LRU cache eviction, persistence snapshots) are layered directly around this core dictionary without modifying its fundamental structure.

---

### Command Specifications

#### 1. `SET key value`
- **Syntax**: `SET key value`
- **Complexity**: \(O(1)\)
- **Behavior**: Associates `value` with `key`. If `key` already holds a value, it is overwritten, regardless of type.
- **Return Type**: Simple String `+OK\r\n`.

#### 2. `GET key`
- **Syntax**: `GET key`
- **Complexity**: \(O(1)\)
- **Behavior**: Retrieves the value associated with `key`.
- **Return Type**:
  - If key exists: **Bulk String** containing the value (`$<len>\r\n<data>\r\n`).
  - If key does not exist: **Null Bulk String** (`$-1\r\n`).

---

### Null (`$-1\r\n`) vs. Empty String (`$0\r\n\r\n`)
A critical database invariant:
> **A missing key is NOT an error and NOT an empty string.**

| Scenario | Command | Wire Output | Client SDK Representation |
| :--- | :--- | :--- | :--- |
| **Missing Key** | `GET nonexistent` | `$-1\r\n` | `nil` (Go), `None` (Python), `null` (JS) |
| **Empty String** | `SET key ""` $\to$ `GET key` | `$0\r\n\r\n` | `""` (empty string) |

If a server mistakenly returned an empty string for a missing key, client applications would not be able to distinguish between *"the user has no bio"* and *"the user set their bio to an empty string"*.

---

### Why Single-Threaded "Atomicity" Matters
Redis executes commands sequentially on a single thread. This architectural choice means:
- Every `SET` and `GET` is **trivially atomic**: no other client command can interleave mid-operation.
- We get thread-safety for free within our command loop because commands are processed one-by-one from the stream without background concurrency.
- Keys and values are stored as raw strings: Redis does not coerce `"42"` into an integer at write time; type coercion is deferred until arithmetic commands (like `INCR`) are invoked.

---

## 2. Go Concepts & Idioms Learned

### A. The "Comma-Ok" Idiom on Go Maps
In Go, reading a missing key from a map returns the zero value (`""` for strings). To distinguish between a key holding an empty string `""` and a missing key, Go provides the **comma-ok idiom**:

```go
val, exists := store[key]
if !exists {
    return encodeNull() // returns "$-1\r\n"
}
return encodeBulkString(val)
```

### B. Global Keyspace Map
Using a package-level map represents the server's in-memory storage:
```go
var store = make(map[string]string)
```
- Map insertions (`store[key] = value`) provide \(O(1)\) average-time lookups and writes backed by Go's runtime hash table.

---

## 3. Implementation Reference (`main.go`)

### 1. In-Memory Store & Handlers

```go
var store = make(map[string]string)

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
```

### 2. Command Table Registration with Arity Bounds

```go
var commands = map[string]Command{
	"PING":    {handler: cmdPing, arity: Arity{min: 0, max: 1}},
	"ECHO":    {handler: cmdEcho, arity: Arity{min: 1, max: 1}},
	"COMMAND": {handler: cmdCommand, arity: Arity{min: 0, max: -1}},
	"SET":     {handler: cmdSet, arity: Arity{min: 2, max: 2}},
	"GET":     {handler: cmdGet, arity: Arity{min: 1, max: 1}},
}
```

---

## 4. Verification & Test Vectors

The test suite in `tests/06-set-get/` validates:

| Test File | Input | Expected Output | Validated Behavior | Result |
| :--- | :--- | :--- | :--- | :---: |
| `tests/06-set-get/1.in` | `SET name Alice`<br>`GET name` | `+OK\r\n`<br>`$5\r\nAlice\r\n` | Basic string storage and retrieval | **PASS** |
| `tests/06-set-get/2.in` | `GET nonexistent` | `$-1\r\n` | Missing key returns Null Bulk String sentinel | **PASS** |
| `tests/06-set-get/3.in` | `SET key1 hello`<br>`SET key2 world`<br>`GET key1`<br>`GET key2` | `+OK\r\n`<br>`+OK\r\n`<br>`$5\r\nhello\r\n`<br>`$5\r\nworld\r\n` | Multiple independent keys in associative map | **PASS** |

Run tests locally:
```bash
./run_tests.sh 06
```
All tests passing (3 passed, 0 failed).

