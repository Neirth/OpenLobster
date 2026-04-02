package ipc

const (
	// EnvHostRPCReadFD is the file descriptor number (in the helper process)
	// used to read RPC responses from the host callback server.
	EnvHostRPCReadFD = "OPENLOBSTER_HOST_RPC_READ_FD"
	// EnvHostRPCWriteFD is the file descriptor number (in the helper process)
	// used to write RPC requests to the host callback server.
	EnvHostRPCWriteFD = "OPENLOBSTER_HOST_RPC_WRITE_FD"
)

// EmptyArgs is used for RPC methods that do not need input values.
type EmptyArgs struct{}

// EmptyReply is used for RPC methods that do not need output values.
type EmptyReply struct{}

// PluginInfo is fetched by the host after helper startup to prime metadata.
type PluginInfo struct {
	ID          string
	Name        string
	Version     string
	Description string
	Type        string
	Schema      []byte
}

// CallArgs is the host->helper invocation payload.
type CallArgs struct {
	Function string
	Input    []byte
}

// CallReply is the helper->host invocation result.
// Error is serialized as a plain string to keep transport errors separate.
type CallReply struct {
	Output []byte
	Error  string
}

// EmitMessageArgs is used by helper->host callback RPC for messaging events.
type EmitMessageArgs struct {
	Payload []byte
}
