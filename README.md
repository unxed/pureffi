# pureffi — purego API built on goffi

`pureffi` is a transparent, drop-in replacement for the excellent [ebitengine/purego](https://github.com/ebitengine/purego) library, built entirely on top of [go-webgpu/goffi](https://github.com/go-webgpu/goffi). It provides 1:1 API compatibility with `purego` while eliminating linker conflicts when both ecosystems are used within the same Go project.

---

## The Motivation: Resolving Linker Conflicts in CGO-free Go

### The Conflict
Both `purego` (pioneered by the Ebitengine team) and `goffi` (engineered by the go-webgpu team) are outstanding achievements in the Go ecosystem. They proved that calling foreign C libraries is possible without a C compiler, using Go's internal runtime machinery.

To do this under `CGO_ENABLED=0`, both libraries include their own copy of `internal/fakecgo` to satisfy the Go runtime's requirements for dynamic symbol resolution.

However, because they both define identical runtime symbols (such as `_cgo_init`, `_cgo_thread_start`, etc.), **including both libraries in the dependency tree of a single project results in a fatal linker error**:
```bash
duplicate definition of symbol _cgo_init
```
This conflict made it difficult to build unified, CGO-free Go applications that wanted to leverage both Ebitengine packages (like `ebiten` or `oto`) and `go-webgpu` packages at the same time.

### The Solution: pureffi
`pureffi` solves this conflict by implementing the exact `purego` public API (`Dlopen`, `Dlsym`, `RegisterFunc`, `NewCallback`, `SyscallN`, and the `objc` / `cstrings` subpackages) using `goffi` as the underlying execution engine.

Because `pureffi` does not include its own duplicate assembler trampolines or `fakecgo` runtimes, **it resolves the symbol conflict entirely**. Your binary will compile successfully while using the highly optimized and audited FFI pathway provided by `goffi`.

---

## Features

- **1:1 API Compatibility**: Works as a drop-in replacement via Go's `replace` directive. You do not need to change a single import statement in your existing codebase.
- **Unified Dependency Graph**: Allows using `ebiten`-based projects and `go-webgpu` modules together without CGO or linker errors.
- **Apple Silicon C-Variadic ABI Fix**: Fixes a long-standing ABI mismatch on macOS/ARM64. Under the hood, `pureffi` uses runtime introspection (`dladdr`) to detect whether the target function is a true C-variadic function (like `sprintf`, which requires variadic arguments to be passed on the stack) or a standard dynamic dispatcher (like `objc_msgSend`, which expects them in registers), ensuring stable execution.
- **Optimized Fast-Path**: Leverages object pooling on dispatching hot-paths, reducing fast-path heap allocations down to a single allocation per call.
- **Fixed-size array arguments** support in RegisterFunc
- **Structs by Value**: Supports passing and returning structs by value.
- **CString and GoString functions** to match the Go C package
- **Robust Struct and Array Passing**: Inherits `goffi`'s mature, recursive struct-passing by value and by reference.
- **errno capturing**
- **structs.HostLayout** support

---

## Behind the Scenes

Initially, `pureffi` started as a straightforward, lightweight wrapper over `goffi`—primarily to resolve the symbol linker conflict without forcing developers to juggle custom build tags.

However, once the wrapper was working, the results looked promising enough that I decided to see if I could make it even better. So I spent some time optimizing dispatcher hot-paths and addressing a few community requests and architectural discussions from the `purego` issue tracker.

---

## Performance & Memory Benchmarks

Thanks to `goffi`'s highly optimized engine and targeted call-path optimizations, `pureffi` achieves lower latencies and fewer heap allocations on certain execution paths—especially the fast-path dispatcher.

Below is a comparison of benchmarks run on AMD64 Linux:

### `purego` (Original)
```
BenchmarkFastPath-4      2005327               574.4 ns/op           264 B/op          5 allocs/op
BenchmarkSlowPath-4      1861425               639.5 ns/op           296 B/op          6 allocs/op
BenchmarkSyscallN-4      5726388               199.6 ns/op           216 B/op          2 allocs/op
```

### `pureffi` (with `goffi` wrapper)
```
BenchmarkFastPath-4      4753861               255.5 ns/op           208 B/op          1 allocs/op
BenchmarkSlowPath-4      1718974               691.6 ns/op           304 B/op          7 allocs/op
BenchmarkSyscallN-4      4487470               259.6 ns/op           216 B/op          2 allocs/op
```

Notably, `pureffi` reduces **FastPath** heap allocations from **5 down to just 1 allocation per operation**, cutting average latency by over 50%.

---

## How to Use

To use `pureffi` in your project, simply add a `replace` directive to your `go.mod` file.

```go
module your_project

go 1.25.5

replace github.com/ebitengine/purego => github.com/unxed/pureffi v0.1.16
```

Once the replacement is configured, Go will automatically route all `import "github.com/ebitengine/purego"` statements to `pureffi`'s `goffi`-backed implementation. No further action is required.

Pick a `pureffi` version that matches the `goffi` version you build against: `ffi.CallFunction` gained a second return value in `goffi` v0.6.0, so `pureffi` v0.1.8 and older (which pin `goffi` v0.5.3) will not compile against a v0.6.x tree.

---

## Universal builds: one binary for glibc and musl

[goffi's Profile U](https://github.com/unxed/goffi/blob/main/docs/PROFILE_U.md) produces a single `CGO_ENABLED=0` binary that performs live FFI on both glibc and musl hosts — no C toolchain, no per-distro rebuild. `pureffi` works with it **unchanged**: it carries no `fakecgo` and emits no `//go:cgo_import_dynamic` directive of its own, so it adds no libc `SONAME` to the link and needs no build tags.

This is also why `pureffi` is the right way to satisfy a `purego`-API dependency here. The usual trick for running `goffi` next to upstream `purego` — `-tags nofakecgo` — is not available in universal mode, because that tag also removes the re-exec bridge that Profile U starts up with.

Profile U currently lives in the `unxed/goffi` fork, so a universal build needs two `replace` directives:

```go
module your_project

go 1.25.5

require (
	github.com/ebitengine/purego v0.9.0
	github.com/go-webgpu/goffi v0.6.2
)

replace github.com/ebitengine/purego => github.com/unxed/pureffi v0.1.16

replace github.com/go-webgpu/goffi => github.com/unxed/goffi <version-with-profile-u>
```

Then build, strip the ELF interpreter, and check the contract:

```bash
CGO_ENABLED=0 go build -tags goffi_universal -o app .
go run github.com/go-webgpu/goffi/cmd/goffi-strip-interp app
go run github.com/go-webgpu/goffi/cmd/goffi-audit app   # no PT_INTERP, no DT_NEEDED
```

The resulting file runs as-is on Debian/Ubuntu and on Alpine. `purego.Dlopen("libc.so.6")` keeps working on musl too, because musl's loader aliases the reserved libc-family SONAMEs (`libc.so.6`, `libm.so.6`, `libpthread.so.0`, …) to itself; when the real name is needed, `goffi` reports it via `ffi.HostLibC()`, `ffi.HostLoader()` and `ffi.LibcKind()`.

`cmd/universal-probe` is a ready-made smoke test for this mode: build it the same way and run the one artifact on both libcs.

---

## Testing

You can run the comprehensive test suite (which includes a "full-circle" roundtrip FFI test that calls Go callbacks back through C-ABI boundaries) using:

```bash
go test -v ./...
```

---

## Acknowledgments

`pureffi` is merely a bridge between two monumental projects. I would like to express our deepest gratitude to:

- **The Ebitengine team (Hajime Hoshi and contributors)**: For creating `purego`, proving that CGO-less FFI is possible, and establishing the groundwork for the modern pure-Go graphics ecosystem.
- **The go-webgpu team (kolkov and contributors)**: For engineering `goffi` with its highly optimized, audited ABI calling conventions, extensive structure alignment handling, and robust platform support.

This project exists solely to allow these two fantastic libraries to coexist and power the next generation of pure-Go high-performance software.

---

## License

BSD 3-claue. See the LICENSE file for details.
