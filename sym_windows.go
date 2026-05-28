//go:build windows

package purego

// getSymbolName is a stub for Windows. We don't have objc_msgSend on Windows anyway,
// so the heuristic defaults to treating all `...any` functions as true C-variadics.
func getSymbolName(addr uintptr) string {
	return ""
}
