// Package guest is the Go side of the Lounge plugin host ABI (SDK/plugins/ABI.md) for
// WebAssembly plugins built with
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
//
// A handler is an exported function with the signature () -> int32:
//
//	//go:wasmexport on_login
//	func onLogin() int32 {
//		env, err := guest.Input()
//		if err != nil {
//			return 1
//		}
//		guest.Log(guest.LevelInfo, "hello "+env.User["nickname"].(string))
//		return 0
//	}
//
// Return 0 for success; anything else is reported as the handler's own error (audited, never
// counted towards auto-disable). Everything the plugin may do beyond computing goes through
// Call, and only with the capabilities the administrator granted.
//
// The functions are implemented for GOOS=wasip1 only; on other platforms the package builds
// (so tools and tests can import the types) but offers no host functions.
package guest

// Envelope is the JSON input of every handler.
type Envelope struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"` // event | filter | job | action | ui
	Point    string         `json:"point"`
	Plugin   PluginRef      `json:"plugin"`
	Settings map[string]any `json:"settings"`
	Server   map[string]any `json:"server,omitempty"`
	User     map[string]any `json:"user,omitempty"`
	Data     map[string]any `json:"data"`
	Time     int64          `json:"timestamp"`
}

// PluginRef identifies the running plugin.
type PluginRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// Host status codes (negative results of the host functions).
const (
	StatusOK       int32 = 0
	StatusDenied   int32 = -1 // capability not granted
	StatusInvalid  int32 = -2 // bad arguments
	StatusNotFound int32 = -3
	StatusLimit    int32 = -4 // size, quota or rate limit
	StatusUpstream int32 = -5 // the host function failed
	StatusUnknown  int32 = -6 // no such host function
)

// Log levels.
const (
	LevelDebug uint32 = iota
	LevelInfo
	LevelWarn
	LevelError
)

// HostError is a failed host call: the status code and the host's message.
type HostError struct {
	Code    int32
	Message string
}

func (e *HostError) Error() string { return e.Message }
