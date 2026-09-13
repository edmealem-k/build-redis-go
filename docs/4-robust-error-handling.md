# Lesson 04: Robust Error Handling & Arity Validation

> **Core Focus**: Separating concerns (Tokenize $\to$ Validate $\to$ Execute), centralized command arity checking, boundary case-normalization, and standardized Redis error framing.

---

## 1. Redis Internals & Protocol Deep-Dive

### Separation of Concerns
In a production database server, request processing is divided into three distinct stages:
1. **Tokenize**: Parse raw client input lines into discrete tokens (`args []string`).
2. **Validate**: Validate command existence, casing, and **arity** (argument counts).
3. **Execute**: Run the command handler with guaranteed invariants (e.g. arguments are valid and present).

By decoupling validation from execution, command handlers remain focused purely on their core business logic without repeating defensive boilerplate.

---

### Command Arity & Standardized Error Format

Every Redis command defines an **arity** — the minimum and maximum number of arguments it accepts:

| Command | Min Arguments | Max Arguments | Valid Examples | Invalid Example |
| :--- | :---: | :---: | :--- | :--- |
| `PING` | `0` | `1` | `PING`, `PING "hello"` | `PING a b` (Too many args) |
| `ECHO` | `1` | `1` | `ECHO "hello"` | `ECHO` (Missing arg) |
| `COMMAND` | `0` | Variadic (`-1`) | `COMMAND`, `COMMAND DOCS` | — |

When a client violates arity, Redis returns a standardized error:
```text
-ERR wrong number of arguments for '<cmd>' command\r\n
```

> **Important Testing Detail**: The course test suite matches this exact string format. Notice that `<cmd>` is formatted with single quotes and matches the canonical uppercase command name (`'ECHO'`).

---

### Boundary Normalization (Case-Insensitivity)

Redis commands are case-insensitive (`set`, `SET`, `Set` are identical).
- **Best Practice**: Normalize the command verb to uppercase **once at the boundary** in `handleCommand`.
- **Anti-Pattern**: Calling `strings.ToUpper()` inside individual command handlers. Handlers should always receive already-normalized, canonical command names.

---

### Empty Line Handling

Clients and interactive terminal sessions often transmit blank lines or trailing `\r\n` characters.
- **Rule**: Blank lines must be skipped silently as a no-op.
- **Consequence of Bugs**: Treating blank lines as unknown commands (`-ERR unknown command ''`) shifts the server response stream out of alignment, breaking subsequent client pipeline replies.

---

## 2. Go Concepts & Idioms Learned

### A. Encapsulating Commands with Structs & Type Aliases
Instead of maintaining separate maps for handlers and arity rules, we define a `CommandHandler` function signature and bundle execution logic and metadata together in a `Command` struct:

```go
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
```

### B. Method Receivers for Self-Contained Validation
Attaching `checkArity` as a method on `Arity` (`func (a Arity) checkArity(...)`) encapsulates validation rules directly within the type:

```go
func (a Arity) checkArity(cmd string, args []string) string {
	if a.min > len(args) {
		return encodeError(fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd))
	}

	if (a.max != -1) && (len(args) > a.max) {
		return encodeError(fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd))
	}

	return ""
}
```

### C. Consistent RESP Prefix Handling in `encodeError`
By having `encodeError` manage the `-` prefix:
```go
func encodeError(s string) string {
	return fmt.Sprintf("-%s\r\n", s)
}
```
All wire-protocol type prefixes (`+`, `-`, `:`, `$`) remain centralized inside the encoder layer rather than requiring callers to hardcode `-` prefixes manually.

### D. Zero-Allocation Slicing & Inline Error Guards
Passing `args[1:]` passes a slice view without allocating new memory on the heap. Using Go's inline `if` statement with initialization cleanly tests and returns validation errors:
```go
if errmsg := command.arity.checkArity(cmd, args[1:]); errmsg != "" {
	return errmsg
}
```

---

## 3. Implementation Reference (`main.go`)

### The Dispatch Pipeline

```go
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

	// Validate arity before executing
	if errmsg := command.arity.checkArity(cmd, args[1:]); errmsg != "" {
		return errmsg
	}

	return command.handler(args)
}
```

---

## 4. Verification & Test Vectors

The test suite in `tests/04-error-handling/` validates:

| Test File | Input | Expected Output | Validated Behavior | Result |
| :--- | :--- | :--- | :--- | :---: |
| `tests/04-error-handling/1.in` | `ECHO` | `-ERR wrong number of arguments for 'ECHO' command\r\n` | Missing argument rejected with arity error | **PASS** |
| `tests/04-error-handling/2.in` | `ping`<br>`Ping`<br>`PING` | `+PONG\r\n`<br>`+PONG\r\n`<br>`+PONG\r\n` | Case-insensitive boundary normalization | **PASS** |
| `tests/04-error-handling/3.in` | `BADCMD arg1 arg2`<br>`PING` | `-ERR unknown command 'BADCMD'\r\n`<br>`+PONG\r\n` | Unknown command rejected without breaking stream | **PASS** |

Run tests locally:
```bash
./run_tests.sh 04
```
All tests passing (3 passed, 0 failed).

