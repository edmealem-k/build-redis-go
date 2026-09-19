# Build Redis from Scratch (Go)

[![shipthatcode — Build Redis from Scratch](https://api.shipthatcode.com/cert/b37ddee5d1c20462e849d348d299c087.svg)](https://shipthatcode.com/courses/build-redis)

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![Tests](https://img.shields.io/badge/tests-passing-brightgreen?style=flat)
![License](https://img.shields.io/badge/license-MIT-blue.svg)
[![wakatime](https://wakatime.com/badge/github/edmealem-k/build-redis-go.svg)](https://wakatime.com/badge/github/edmealem-k/build-redis-go)

A ground-up implementation of a fast, in-memory **Redis** server written in Go. Built lesson-by-lesson through test-driven development following the [ShipThatCode](https://shipthatcode.com/courses/build-redis) _Build Redis from Scratch_ systems course.

---

## 📌 Overview

This project explores the internal mechanics of Redis by implementing its core subsystems from scratch:

- **RESP (REdis Serialization Protocol)** parser and serializer
- **In-memory key-value store** with string operations and counters
- **Key expiration** mechanics (TTL, passive expiration, active eviction)
- **Complex data structures**: Doubly-linked Lists, Hash Maps, Sets, and Skip List-backed Sorted Sets
- **Transactions**: Atomic command queuing (`MULTI`, `EXEC`) and optimistic locking (`WATCH`)
- **Pub/Sub**: Channel-based messaging system
- **Persistence**: Snapshotting (RDB) and append-only logs (AOF)
- **Memory management**: Least Recently Used (LRU) cache eviction policies
- **Lua scripting engine**: Server-side script execution (`EVAL`)
- **Replication**: Leader-follower master/replica state synchronization
- **Event streams**: Append-only log streaming data structure (`XADD`, `XREAD`, `XRANGE`)

---

## 🗺️ Curriculum Roadmap & Progress

<details open>
<summary><b>Progress Tracker (29 Lessons)</b></summary>

### 1. Protocol & Basics

- [x] **01. Ping** — Initial connection handling and basic `PING` / `PONG` response
- [x] **02. Echo** — Parameter handling and string echoing with `ECHO`
- [x] **03. RESP Format** — Bulk strings, simple strings, integers, and arrays
- [x] **04. Error Handling** — Redis-compliant error responses (`-ERR ...`)
- [x] **05. RESP Array Parsing** — Decoding framed RESP protocol client commands

### 2. Key-Value Storage & Expiration

- [ ] **06. Set & Get** — Fundamental string key-value storage (`SET`, `GET`)
- [ ] **07. Multiple Keys** — Multi-key retrieval and batch assignment (`MGET`, `MSET`)
- [ ] **08. Set NX / XX** — Conditional writes (set if not exists / set if exists)
- [ ] **09. Incr & Decr** — Atomic integer increments and decrements
- [ ] **10. Expire & TTL** — Time-To-Live key expiration management (`EXPIRE`, `TTL`)
- [ ] **11. Set EX** — Atomic set with expiration time in seconds
- [ ] **12. Passive Expiry** — Lazy expiration on access

### 3. Rich Data Structures

- [ ] **13. LPUSH & RPUSH** — Double-ended list insertion
- [ ] **14. LPOP, RPOP & LLEN** — List popping and length inspection
- [ ] **15. LRANGE** — Slicing and retrieving ranges from list structures
- [ ] **16. HSET & HGET** — Hash map field setting and retrieval
- [ ] **17. HDEL & HGETALL** — Hash field deletion and full dictionary inspection
- [ ] **18. SADD & SMEMBERS** — Unique set membership and inspection
- [ ] **19. ZADD, ZSCORE & ZRANGE** — Priority/sorted sets with float scores

### 4. Database Operations & Transactions

- [ ] **20. Generic Keys** — `DEL`, `EXISTS`, `KEYS`, and `TYPE` introspection
- [ ] **21. Transactions (MULTI & EXEC)** — Atomic command queuing and batch execution
- [ ] **26. Optimistic Locking (WATCH)** — Check-and-set concurrency control

### 5. Advanced Systems & Persistence

- [ ] **22. Pub/Sub** — Channel publishing and real-time subscriber fanout
- [ ] **23. RDB Persistence** — Point-in-time binary snapshot creation and restoration
- [ ] **24. AOF Logging** — Write-ahead append-only log replay and durability
- [ ] **25. LRU Eviction** — Memory bounds enforcement with least-recently-used eviction
- [ ] **27. Lua Scripting (EVAL)** — Atomic server-side script evaluation
- [ ] **28. Replication** — Master-replica state replication
- [ ] **29. Streams** — Append-only stream data type

</details>

---

## 🛠️ Project Structure

```text
.
├── main.go               # Active Redis server implementation
├── run_tests.sh          # Local test runner script
├── tests/                # Test cases for all 29 lessons (input/output assertions)
│   ├── 01-ping/
│   ├── 02-echo/
│   └── ...
├── .shipthatcode.json    # Course configuration & verification hashes
└── README.md             # Project documentation & live progress badge
```

---

## 🚀 Getting Started

### Prerequisites

- [Go 1.21+](https://go.dev/dl/) installed (`go version`)
- Git

### Running Tests Locally

Run the test suite against the current lesson:

```bash
# Test a specific lesson (e.g. lesson 01)
./run_tests.sh 01

# Test lesson 06
./run_tests.sh 06

# Run all test suites
./run_tests.sh
```

### Checking Official Progress

1. Run local tests: `./run_tests.sh <lesson>`
2. Commit and push:

   ```bash
   git add -A
   git commit -m "Complete lesson <XX>"
   git push origin main
   ```

3. Visit the [Course Page](https://shipthatcode.com/courses/build-redis) and click **Check my solution**.

---

## 📜 License

This project is open source and available under the [MIT License](LICENSE).
