# pureffi — Unified Drop-In Replacement for purego built on goffi

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
This conflict made it extremely difficult to build unified, CGO-free Go applications that wanted to leverage both Ebitengine packages (like `ebiten` or `oto`) and `go-webgpu` packages at the same time.

### The Solution: pureffi
`pureffi` solves this conflict by implementing the exact `purego` public API (`Dlopen`, `Dlsym`, `RegisterFunc`, `NewCallback`, `SyscallN`, and the `objc` / `cstrings` subpackages) using `goffi` as the underlying execution engine.

Because `pureffi` does not include its own duplicate assembler trampolines or `fakecgo` runtimes, **it resolves the symbol conflict entirely**. Your binary will compile successfully while using the highly optimized and audited FFI pathway provided by `goffi`.

---

## Features

- **1:1 API Compatibility**: Works as a drop-in replacement via Go's `replace` directive. You do not need to change a single import statement in your existing codebase.
- **Unified Dependency Graph**: Allows using `ebiten`-based projects and `go-webgpu` modules together without CGO or linker errors.
- **Apple Silicon C-Variadic ABI Fix**: Fixes a long-standing ABI mismatch on macOS/ARM64. Under the hood, `pureffi` uses runtime introspection (`dladdr`) to detect whether the target function is a true C-variadic function (like `sprintf`, which requires variadic arguments to be passed on the stack) or a standard dynamic dispatcher (like `objc_msgSend`, which expects them in registers), ensuring stable execution.
- **Robust Struct and Array Passing**: Inherits `goffi`'s mature, recursive struct-passing by value and by reference.

---

## How to Use

To use `pureffi` in your project, simply add a `replace` directive to your `go.mod` file.

### Local Replacement
If you have cloned this repository locally:
```go
module your_project

go 1.25.5

replace github.com/ebitengine/purego => ../pureffi
```

### Remote Replacement
If you are pointing to a published fork:
```go
module your_project

go 1.25.5

replace github.com/ebitengine/purego => github.com/unxed/pureffi v0.1.0
```

Once the replacement is configured, Go will automatically route all `import "github.com/ebitengine/purego"` statements to `pureffi`'s `goffi`-backed implementation. No further action is required.

---

## Testing

You can run the comprehensive test suite (which includes a "full-circle" roundtrip FFI test that calls Go callbacks back through C-ABI boundaries) using:

```bash
go test -v ./...
```

---

## Acknowledgments

`pureffi` is merely a bridge between two monumental projects. We would like to express our deepest gratitude to:

- **The Ebitengine team (Hajime Hoshi and contributors)**: For creating `purego`, proving that CGO-less FFI is possible, and establishing the groundwork for the modern pure-Go graphics ecosystem.
- **The go-webgpu team (kolkov and contributors)**: For engineering `goffi` with its highly optimized, audited ABI calling conventions, extensive structure alignment handling, and robust platform support.

This project exists solely to allow these two fantastic libraries to coexist and power the next generation of pure-Go high-performance software.

---

## License

BSD 3-claue. See the LICENSE file for details.
