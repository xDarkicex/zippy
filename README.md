# Zippy

[![Go Reference](https://pkg.go.dev/badge/github.com/xDarkicex/zippy.svg)](https://pkg.go.dev/github.com/xDarkicex/zippy)
[![Go Report Card](https://goreportcard.com/badge/github.com/xDarkicex/zippy)](https://goreportcard.com/report/github.com/xDarkicex/zippy)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.23-blue)](https://go.dev/dl/)

**Zippy** is a high-performance, zero-copy I/O library for Go.  
It delivers **7–12× higher throughput than the standard library** by automatically selecting the fastest available kernel-level primitives (`splice`, `writev`) with transparent fallbacks.

---

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [API Overview](#api-overview)
- [Kernel & Streaming APIs](#kernel--streaming-apis)
- [Use Cases](#use-cases)
- [Benchmarks](#benchmarks)
- [Performance Characteristics](#performance-characteristics)
- [Thread Safety](#thread-safety)
- [Platform Support](#platform-support)
- [Security](#security)
- [Contributing](#contributing)
- [FAQ](#faq)
- [Roadmap](#roadmap)
- [License](#license)

---

## Overview

Zippy provides zero-copy buffering and data transfer with **automatic optimization selection**.  
When possible, it uses kernel-level mechanisms such as Linux `splice(2)` and `writev(2)`. When not, it gracefully falls back to standard `io.Reader` / `io.Writer` semantics.

The result: extremely high throughput, predictable memory usage, and minimal allocations—without forcing you to write platform-specific code.

---

## Features

- 🚀 **7–12× faster than `bytes.Buffer`** for real I/O workloads
- 🔄 **Zero-copy write path** using pooled 4MB buckets
- ⚡ **Automatic kernel optimizations**
  - `splice` on Linux
  - `writev` on all platforms
- 💾 **Zero allocations on hot write paths**
- 🛡️ **Built-in DoS protection** via configurable size limits
- 🔧 **Drop-in replacement for `io.Copy`**
- 📊 **Byte-level I/O** (`ReadByte` / `WriteByte`) for protocol parsing
- 🌊 **Streaming APIs** with bounded memory usage
- 🔒 **Thread-safe pools** with per-buffer goroutine ownership

---

## Installation

```bash
go get github.com/xDarkicex/zippy
````

### Requirements

* Go **1.23+**
* Linux kernel **2.6.17+** for `splice` (optional; automatic fallback elsewhere)

---

## Quick Start

### Basic Buffer Usage

```go
buf := zippy.New()
defer buf.Release()

buf.WriteString("Hello, ")
buf.Write([]byte("World!"))

fmt.Println(buf.String()) // "Hello, World!"
```

### High-Performance File Copy

```go
func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = zippy.KernelCopy(out, in)
	return err
}
```

### Streaming with Progress Tracking

```go
progress := func(read, written int64) error {
	fmt.Printf("Transferred %.2f MB\n", float64(written)/(1024*1024))
	return nil
}

_, err := zippy.StreamCopy(dst, src, 4*1024*1024, progress)
```

---

## API Overview

### Buffer Creation

```go
buf := zippy.New()          // Default 100MB limit
buf := zippy.NewUnlimited() // No size limit (trusted input only)
```

### Writing

```go
buf.Write(p)
buf.WriteString(s)
buf.WriteByte(b)
buf.ReadFrom(r)
buf.ReadN(r, n)
```

### Reading

```go
buf.Read(p)
buf.ReadByte()
buf.Bytes()
buf.String()
```

### Output

```go
buf.WriteTo(w) // writev when possible
```

### Management

```go
buf.Reset()
buf.Release()
buf.Truncate(n)
buf.Grow(n)
buf.SetMaxSize(limit)
```

---

## Kernel & Streaming APIs

### KernelCopy

Drop-in replacement for `io.Copy`:

```go
n, err := zippy.KernelCopy(dst, src)
```

### SpliceConn (Linux)

```go
n, err := zippy.SpliceConn(dstUnix, srcUnix)
// Returns ENOTSUP on non-Linux platforms
```

### ProxyLoop

```go
n, err := zippy.ProxyLoop(dst, src)
```

### StreamCopy

```go
n, err := zippy.StreamCopy(dst, src, 8*1024*1024, progressFn)
```

### StreamCopyWithTimeout

```go
n, err := zippy.StreamCopyWithTimeout(
	dst,
	src,
	4*1024*1024,
	10*time.Second,
	nil,
)
```

---

## Use Cases

### High-Performance HTTP Proxies

```go
buf := zippy.New()
defer buf.Release()

buf.ReadFrom(resp.Body)
buf.WriteTo(w)
```

### Protocol Parsing

```go
buf := zippy.New()
defer buf.Release()

buf.ReadN(conn, 12)
header := buf.Bytes()
msgLen := binary.BigEndian.Uint32(header[8:12])

buf.Reset()
buf.ReadN(conn, int64(msgLen))
```

### Memory-Bounded Streaming

```go
// Stream a 100GB file using only 8MB of memory
err := zippy.StreamCopy(dst, src, 8*1024*1024, nil)
```

---

## Benchmarks

Benchmarks run on **Apple M2**, Go **1.23**, macOS (darwin/arm64).

### `zippy.Buffer` vs `bytes.Buffer`

```
Write:
zippy.Buffer     2.88 ns/op   355 GB/s   0 B/op   0 allocs/op
bytes.Buffer    22.08 ns/op    46 GB/s   0 B/op   0 allocs/op
→ 7.7× faster

ReadFrom (4MB):
zippy.Buffer     18.5 µs   56.7 GB/s    120 B/op    3 allocs/op
bytes.Buffer    213 µs     4.9 GB/s   4.2 MB/op   14 allocs/op
→ 11.5× faster, ~35,000× less memory
```

### Streaming

```
StreamCopy (10MB)   192 µs   54 GB/s   548 B/op   12 allocs/op
KernelCopy (4MB)   71.6 µs   58 GB/s    96 B/op    3 allocs/op
```

---

## Performance Characteristics

| Mode     | Throughput | Platform | Use Case         |
| -------- | ---------- | -------- | ---------------- |
| Splice   | 40–60 GB/s | Linux    | Socket-to-socket |
| Writev   | 15–25 GB/s | All      | Files, network   |
| Buffered | 8–12 GB/s  | All      | Generic I/O      |

---

## Thread Safety

* **Buffer**: **NOT** thread-safe (one goroutine per buffer).
* **Pools**: Thread-safe (`New()` / `Release()` are safe concurrently).

---

## Platform Support

| Feature    | Linux | macOS | Windows |
| ---------- | ----- | ----- | ------- |
| Buffer API | ✅     | ✅     | ✅       |
| KernelCopy | ✅     | ✅     | ✅       |
| SpliceConn | ✅     | ❌     | ❌       |
| ProxyLoop  | ✅     | ✅     | ✅       |

Linux automatically enables true zero-copy via `splice(2)`.

---

## Security

### DoS Protection

Buffers default to a **100MB size limit**:

```go
buf.SetMaxSize(10 * 1024 * 1024)
```

Exceeding the limit returns `zippy.ErrBufferFull`.

### Memory Safety

* Use-after-release protection
* Bounds-checked slices
* Panic recovery in read paths
* No unsafe memory tricks in hot paths

---

## Contributing

1. `go test ./...`
2. `go test -bench=. -benchmem`
3. `go fmt ./...`
4. `golangci-lint run`

PRs welcome. Keep it fast. Keep it clean.

---

## FAQ

**When should I use Zippy?**

Use it for:

* Proxies and gateways
* Large file I/O
* Protocol parsers
* Streaming with strict memory bounds

Avoid it for:

* Tiny buffers (<1KB)
* Casual string building

**Does it work on Windows?**
Yes. Linux-only optimizations are disabled automatically.

**How do I get max performance?**

1. Reuse buffers (`Reset`)
2. Pre-size with `Grow`
3. Prefer `WriteTo` over `Bytes`
4. Let `KernelCopy` pick the path

---

## Roadmap

* [ ] Windows IOCP optimizations
* [ ] Pluggable allocators
* [ ] `io/fs` helpers
* [ ] QUIC / HTTP/3 zero-copy examples

---

## License

MIT — see [LICENSE](LICENSE).

---

If this saves you CPU cycles and syscalls,
**star the repo ⭐**. It helps.


