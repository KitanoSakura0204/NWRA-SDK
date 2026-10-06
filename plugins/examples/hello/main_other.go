//go:build !wasip1

// Command hello is a NoWork WebAssembly plugin; see main.go. On other platforms it only tells
// how to build it, so `go vet ./...` works on the host.
package main

import "fmt"

func main() {
	fmt.Println("build for WebAssembly: GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .")
}
