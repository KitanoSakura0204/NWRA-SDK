//go:build wasip1

package guest

import (
	"encoding/json"
	"unsafe"
)

// Host ABI v1, import module "lounge" (see SDK/plugins/ABI.md).

//go:wasmimport lounge input_len
func hostInputLen() int32

//go:wasmimport lounge input_read
func hostInputRead(ptr unsafe.Pointer, n uint32) int32

//go:wasmimport lounge output_write
func hostOutputWrite(ptr unsafe.Pointer, n uint32) int32

//go:wasmimport lounge log
func hostLog(level uint32, ptr unsafe.Pointer, n uint32) int32

//go:wasmimport lounge call
func hostCall(namePtr unsafe.Pointer, nameLen uint32, argsPtr unsafe.Pointer, argsLen uint32) int32

//go:wasmimport lounge result_read
func hostResultRead(ptr unsafe.Pointer, n uint32) int32

func ptr(b []byte) unsafe.Pointer {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Pointer(&b[0])
}

// InputBytes returns the raw JSON envelope of this invocation.
func InputBytes() []byte {
	n := hostInputLen()
	if n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	got := hostInputRead(ptr(buf), uint32(n))
	if got < 0 {
		return nil
	}
	return buf[:got]
}

// Input decodes the envelope.
func Input() (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(InputBytes(), &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// Output writes v as the handler's JSON answer (filters, GM actions, page calls). The total
// output of one call is limited to 256 KiB.
func Output(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if code := hostOutputWrite(ptr(b), uint32(len(b))); code != StatusOK {
		return &HostError{Code: code, Message: "output_write failed"}
	}
	return nil
}

// Log writes one line to the plugin log (capability "log"; 100 lines per call, 20 stored).
func Log(level uint32, msg string) {
	b := []byte(msg)
	hostLog(level, ptr(b), uint32(len(b)))
}

// Call runs a host function with JSON arguments and decodes its JSON result into out (nil to
// discard). A refused or failed call returns a *HostError.
func Call(fn string, args, out any) error {
	name := []byte(fn)
	var raw []byte
	if args != nil {
		var err error
		if raw, err = json.Marshal(args); err != nil {
			return err
		}
	}
	n := hostCall(ptr(name), uint32(len(name)), ptr(raw), uint32(len(raw)))
	if n < 0 {
		buf := make([]byte, 1024)
		got := hostResultRead(ptr(buf), uint32(len(buf)))
		msg := fn + " failed"
		if got > 0 {
			var e struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(buf[:got], &e) == nil && e.Error != "" {
				msg = e.Error
			}
		}
		return &HostError{Code: n, Message: msg}
	}
	if out == nil || n == 0 {
		return nil
	}
	buf := make([]byte, n)
	got := hostResultRead(ptr(buf), uint32(n))
	if got < 0 {
		return &HostError{Code: got, Message: "result_read failed"}
	}
	return json.Unmarshal(buf[:got], out)
}
