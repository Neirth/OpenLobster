package wasm

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	stdfs "io/fs"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	extism "github.com/extism/go-sdk"
	"github.com/tetratelabs/wazero"
	wazeroapi "github.com/tetratelabs/wazero/api"
	wazerosys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:linkname wazeroOpenOSFile github.com/tetratelabs/wazero/internal/sysfs.OpenOSFile
func wazeroOpenOSFile(path string, flag wazerosys.Oflag, perm stdfs.FileMode) (wazerosys.File, wazerosys.Errno)

//go:linkname wazeroFdFdstatGetFn github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1.fdFdstatGetFn
func wazeroFdFdstatGetFn(ctx context.Context, mod wazeroapi.Module, params []uint64) wazerosys.Errno

//go:linkname wazeroFdCloseFn github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1.fdCloseFn
func wazeroFdCloseFn(ctx context.Context, mod wazeroapi.Module, params []uint64) wazerosys.Errno

const (
	abiAFUnspec = 0
	abiAFInet   = 1
	abiAFInet6  = 2
	abiAFUnix   = 3
)

const (
	abiSockAny = 0
	abiSockDgr = 1
	abiSockStr = 2
)

const (
	abiSOLSocket = 0
)

const (
	abiSOReuseAddr = 0
	abiSOError     = 2
	abiSOBroadcast = 4
)

const (
	abiAIPassive = 1
)

const (
	abiIPProtoTCP = 1
	abiIPProtoUDP = 2
)

const (
	wasiErrnoSuccess      = 0
	wasiErrnoAcces        = 2
	wasiErrnoAddrInUse    = 3
	wasiErrnoAddrNotAvail = 4
	wasiErrnoAFNoSupport  = 5
	wasiErrnoAgain        = 6
	wasiErrnoAlready      = 7
	wasiErrnoBadf         = 8
	wasiErrnoConnRefused  = 14
	wasiErrnoConnReset    = 15
	wasiErrnoDestAddrReq  = 17
	wasiErrnoHostUnreach  = 23
	wasiErrnoInProgress   = 26
	wasiErrnoIntr         = 27
	wasiErrnoInval        = 28
	wasiErrnoIO           = 29
	wasiErrnoIsConn       = 30
	wasiErrnoNetDown      = 37
	wasiErrnoNetUnreach   = 39
	wasiErrnoNoEnt        = 44
	wasiErrnoNoProtoOpt   = 50
	wasiErrnoNoSys        = 52
	wasiErrnoNotConn      = 53
	wasiErrnoProtoType    = 56
	wasiErrnoNotSock      = 57
	wasiErrnoNotSup       = 58
	wasiErrnoTimedOut     = 73
	wasiErrnoFault        = 21
)

const (
	wasiFiletypeSocketDgram  = 5
	wasiFiletypeSocketStream = 6
	wasiFDFlagNonblock       = 0x0004
)

var expectedWASIExports = map[string]struct{}{
	"args_get":                {},
	"args_sizes_get":          {},
	"clock_res_get":           {},
	"clock_time_get":          {},
	"environ_get":             {},
	"environ_sizes_get":       {},
	"fd_advise":               {},
	"fd_allocate":             {},
	"fd_close":                {},
	"fd_datasync":             {},
	"fd_fdstat_get":           {},
	"fd_fdstat_set_flags":     {},
	"fd_fdstat_set_rights":    {},
	"fd_filestat_get":         {},
	"fd_filestat_set_size":    {},
	"fd_filestat_set_times":   {},
	"fd_pread":                {},
	"fd_prestat_dir_name":     {},
	"fd_prestat_get":          {},
	"fd_pwrite":               {},
	"fd_read":                 {},
	"fd_readdir":              {},
	"fd_renumber":             {},
	"fd_seek":                 {},
	"fd_sync":                 {},
	"fd_tell":                 {},
	"fd_write":                {},
	"path_create_directory":   {},
	"path_filestat_get":       {},
	"path_filestat_set_times": {},
	"path_link":               {},
	"path_open":               {},
	"path_readlink":           {},
	"path_remove_directory":   {},
	"path_rename":             {},
	"path_symlink":            {},
	"path_unlink_file":        {},
	"poll_oneoff":             {},
	"proc_exit":               {},
	"proc_raise":              {},
	"random_get":              {},
	"sched_yield":             {},
	"sock_accept":             {},
	"sock_bind":               {},
	"sock_connect":            {},
	"sock_getaddrinfo":        {},
	"sock_getlocaladdr":       {},
	"sock_getpeeraddr":        {},
	"sock_getsockopt":         {},
	"sock_listen":             {},
	"sock_open":               {},
	"sock_recv":               {},
	"sock_recv_from":          {},
	"sock_send":               {},
	"sock_send_to":            {},
	"sock_setsockopt":         {},
	"sock_shutdown":           {},
}

func instantiateExtendedWASI(ctx context.Context, compiled *extism.CompiledPlugin, env map[string]string) error {
	rt, err := compiledRuntime(compiled)
	if err != nil {
		return err
	}

	builder := rt.NewHostModuleBuilder(wasi_snapshot_preview1.ModuleName)
	wasi_snapshot_preview1.NewFunctionExporter().ExportFunctions(builder)
	bridge := &wasiSocketBridge{}
	bridge.export(builder)

	wasiCompiled, err := builder.Compile(ctx)
	if err != nil {
		return fmt.Errorf("compile wasi module: %w", err)
	}
	if err := validateWASIExportSet(wasiCompiled); err != nil {
		_ = wasiCompiled.Close(ctx)
		return err
	}

	wasiModuleCfg := wazero.NewModuleConfig().
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep().
		WithRandSource(rand.Reader)

	if len(env) > 0 {
		keys := make([]string, 0, len(env))
		for key, value := range env {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			wasiModuleCfg = wasiModuleCfg.WithEnv(key, env[key])
		}
	}

	if _, err := rt.InstantiateModule(ctx, wasiCompiled, wasiModuleCfg); err != nil {
		_ = wasiCompiled.Close(ctx)
		return fmt.Errorf("instantiate wasi module: %w", err)
	}
	if err := markCompiledHasWASI(compiled); err != nil {
		return err
	}
	return nil
}

func validateWASIExportSet(compiled wazero.CompiledModule) error {
	if compiled.Name() != wasi_snapshot_preview1.ModuleName {
		return fmt.Errorf("unexpected wasi module name: %s", compiled.Name())
	}

	exports := compiled.ExportedFunctions()
	missing := make([]string, 0)
	unexpected := make([]string, 0)

	for name := range expectedWASIExports {
		if _, ok := exports[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range exports {
		if _, ok := expectedWASIExports[name]; !ok {
			unexpected = append(unexpected, name)
		}
	}

	if len(missing) == 0 && len(unexpected) == 0 {
		return nil
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	return fmt.Errorf("wasi export set mismatch: missing=%v unexpected=%v", missing, unexpected)
}

func appendPluginCloser(plugin *extism.Plugin, closer func(context.Context) error) error {
	if plugin == nil {
		return errors.New("nil plugin")
	}
	pluginVal := reflect.ValueOf(plugin)
	if pluginVal.Kind() != reflect.Pointer || pluginVal.IsNil() {
		return errors.New("plugin is not a pointer")
	}
	closeField := pluginVal.Elem().FieldByName("close")
	if !closeField.IsValid() {
		return errors.New("plugin close field not found")
	}

	closers := *(*[]func(context.Context) error)(unsafe.Pointer(closeField.UnsafeAddr()))
	closers = append(closers, closer)
	*(*[]func(context.Context) error)(unsafe.Pointer(closeField.UnsafeAddr())) = closers
	return nil
}

func compiledRuntime(compiled *extism.CompiledPlugin) (wazero.Runtime, error) {
	compiledVal := reflect.ValueOf(compiled)
	if compiledVal.Kind() != reflect.Pointer || compiledVal.IsNil() {
		return nil, errors.New("compiled plugin is not a pointer")
	}
	runtimeField := compiledVal.Elem().FieldByName("runtime")
	if !runtimeField.IsValid() {
		return nil, errors.New("compiled runtime field not found")
	}
	return *(*wazero.Runtime)(unsafe.Pointer(runtimeField.UnsafeAddr())), nil
}

func markCompiledHasWASI(compiled *extism.CompiledPlugin) error {
	compiledVal := reflect.ValueOf(compiled)
	if compiledVal.Kind() != reflect.Pointer || compiledVal.IsNil() {
		return errors.New("compiled plugin is not a pointer")
	}
	hasWASIField := compiledVal.Elem().FieldByName("hasWasi")
	if !hasWASIField.IsValid() {
		return errors.New("compiled hasWasi field not found")
	}
	*(*bool)(unsafe.Pointer(hasWASIField.UnsafeAddr())) = true
	return nil
}

type wasiSocketBridge struct{}

type iovec struct {
	ptr uint32
	len uint32
}

type addressBuffer struct {
	buf    uint32
	bufLen uint32
}

type moduleSocketState struct {
	mu       sync.RWMutex
	fileType map[int32]uint8
}

var socketStateByModule sync.Map // map[uintptr]*moduleSocketState

func moduleState(mod wazeroapi.Module, create bool) *moduleSocketState {
	modulePtr := modulePointer(mod)
	if modulePtr == 0 {
		return nil
	}

	if v, ok := socketStateByModule.Load(modulePtr); ok {
		return v.(*moduleSocketState)
	}
	if !create {
		return nil
	}

	state := &moduleSocketState{fileType: make(map[int32]uint8)}
	actual, _ := socketStateByModule.LoadOrStore(modulePtr, state)
	return actual.(*moduleSocketState)
}

func modulePointer(mod wazeroapi.Module) uintptr {
	moduleVal := reflect.ValueOf(mod)
	if !moduleVal.IsValid() || moduleVal.Kind() != reflect.Pointer || moduleVal.IsNil() {
		return 0
	}
	return moduleVal.Pointer()
}

func trackSocketFD(mod wazeroapi.Module, guestFD int32, fileType uint8) {
	state := moduleState(mod, true)
	if state == nil {
		return
	}
	state.mu.Lock()
	state.fileType[guestFD] = fileType
	state.mu.Unlock()
}

func socketFDFileType(mod wazeroapi.Module, guestFD int32) (uint8, bool) {
	state := moduleState(mod, false)
	if state == nil {
		return 0, false
	}
	state.mu.RLock()
	fileType, ok := state.fileType[guestFD]
	state.mu.RUnlock()
	return fileType, ok
}

func untrackSocketFD(mod wazeroapi.Module, guestFD int32) {
	modulePtr := modulePointer(mod)
	if modulePtr == 0 {
		return
	}

	state := moduleState(mod, false)
	if state == nil {
		return
	}

	state.mu.Lock()
	delete(state.fileType, guestFD)
	empty := len(state.fileType) == 0
	state.mu.Unlock()

	if empty {
		socketStateByModule.Delete(modulePtr)
	}
}

func socketFileTypeForSockType(sockType int32) uint8 {
	if sockType == abiSockDgr {
		return wasiFiletypeSocketDgram
	}
	return wasiFiletypeSocketStream
}

func (b *wasiSocketBridge) export(builder wazero.HostModuleBuilder) {
	export := func(name string, params []wazeroapi.ValueType, callback wazeroapi.GoModuleFunc) {
		builder.NewFunctionBuilder().WithGoModuleFunction(callback, params, []wazeroapi.ValueType{wazeroapi.ValueTypeI32}).Export(name)
	}

	// Override random_get to provide entropy directly from crypto/rand.
	// The default implementation depends on module-level sys context wiring,
	// while this bridge keeps behavior stable for embedded Extism runtimes.
	export("random_get", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.randomGet)
	export("fd_close", []wazeroapi.ValueType{wazeroapi.ValueTypeI32}, b.fdClose)
	export("fd_fdstat_get", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.fdFdstatGet)

	export("sock_open", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockOpen)
	export("sock_bind", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockBind)
	export("sock_listen", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockListen)
	export("sock_connect", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockConnect)
	export("sock_getsockopt", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockGetSockOpt)
	export("sock_setsockopt", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockSetSockOpt)
	export("sock_getlocaladdr", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockGetLocalAddr)
	export("sock_getpeeraddr", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockGetPeerAddr)
	export("sock_recv_from", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockRecvFrom)
	export("sock_send_to", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockSendTo)
	export("sock_getaddrinfo", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockGetAddrInfo)
	export("sock_shutdown", []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI32}, b.sockShutdown)
}

func (b *wasiSocketBridge) randomGet(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	bufPtr := uint32(stack[0])
	bufLen := uint32(stack[1])
	if bufLen == 0 {
		stack[0] = wasiErrnoSuccess
		return
	}

	bytes := make([]byte, bufLen)
	if _, err := rand.Read(bytes); err != nil {
		stack[0] = wasiErrnoIO
		return
	}
	if !mod.Memory().Write(bufPtr, bytes) {
		stack[0] = wasiErrnoFault
		return
	}

	stack[0] = wasiErrnoSuccess
}

func (b *wasiSocketBridge) fdClose(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
	errno := uint32(wazeroFdCloseFn(ctx, mod, stack))
	if errno == wasiErrnoSuccess || errno == wasiErrnoBadf {
		untrackSocketFD(mod, int32(stack[0]))
	}
	stack[0] = uint64(errno)
}

func (b *wasiSocketBridge) fdFdstatGet(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
	guestFD := int32(stack[0])
	if fileType, ok := socketFDFileType(mod, guestFD); ok {
		resultFDStat := uint32(stack[1])
		if !writeSocketFDStat(mod.Memory(), resultFDStat, fileType) {
			stack[0] = wasiErrnoFault
			return
		}
		stack[0] = wasiErrnoSuccess
		return
	}

	stack[0] = uint64(wazeroFdFdstatGetFn(ctx, mod, stack))
}

func writeSocketFDStat(mem wazeroapi.Memory, resultFDStat uint32, fileType uint8) bool {
	buf, ok := mem.Read(resultFDStat, 24)
	if !ok {
		return false
	}
	for i := range buf {
		buf[i] = 0
	}
	buf[0] = fileType
	binary.LittleEndian.PutUint16(buf[2:4], wasiFDFlagNonblock)
	return true
}

func (b *wasiSocketBridge) sockOpen(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	guestFDPtr := uint32(stack[2])

	domain, ok := abiToSysDomain(int32(stack[0]))
	if !ok {
		stack[0] = wasiErrnoAFNoSupport
		return
	}
	typ, ok := abiToSysType(int32(stack[1]))
	if !ok {
		stack[0] = wasiErrnoProtoType
		return
	}

	hostFD, err := syscall.Socket(domain, typ, 0)
	if err != nil {
		stack[0] = uint64(wasiErrnoFromError(err))
		return
	}

	file, errno := openSocketAsWASIFile(hostFD)
	if errno != wasiErrnoSuccess {
		_ = syscall.Close(hostFD)
		stack[0] = uint64(errno)
		return
	}

	guestFD, regErr := registerFileDescriptor(mod, file)
	_ = syscall.Close(hostFD)
	if regErr != wasiErrnoSuccess {
		_ = file.Close()
		stack[0] = uint64(regErr)
		return
	}

	trackSocketFD(mod, guestFD, socketFileTypeForSockType(int32(stack[1])))

	if !mod.Memory().WriteUint32Le(guestFDPtr, uint32(guestFD)) {
		stack[0] = wasiErrnoFault
		return
	}
	stack[0] = wasiErrnoSuccess
}

func (b *wasiSocketBridge) sockBind(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	sa, errno := decodeSockaddr(mod.Memory(), uint32(stack[1]), uint32(stack[2]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	stack[0] = uint64(wasiErrnoFromError(syscall.Bind(hostFD, sa)))
}

func (b *wasiSocketBridge) sockListen(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}
	stack[0] = uint64(wasiErrnoFromError(syscall.Listen(hostFD, int(stack[1]))))
}

func (b *wasiSocketBridge) sockConnect(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	sa, errno := decodeSockaddr(mod.Memory(), uint32(stack[1]), uint32(stack[2]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	stack[0] = uint64(wasiErrnoFromError(syscall.Connect(hostFD, sa)))
}

func (b *wasiSocketBridge) sockGetSockOpt(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	level, name, ok := mapSocketOption(uint32(stack[1]), uint32(stack[2]))
	if !ok {
		stack[0] = wasiErrnoNoProtoOpt
		return
	}

	valuePtr := uint32(stack[3])
	valueLen := uint32(stack[4])
	if valueLen < 4 {
		stack[0] = wasiErrnoInval
		return
	}

	value, err := syscall.GetsockoptInt(hostFD, level, name)
	if err != nil {
		stack[0] = uint64(wasiErrnoFromError(err))
		return
	}
	if !mod.Memory().WriteUint32Le(valuePtr, uint32(int32(value))) {
		stack[0] = wasiErrnoFault
		return
	}

	stack[0] = wasiErrnoSuccess
}

func (b *wasiSocketBridge) sockSetSockOpt(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	level, name, ok := mapSocketOption(uint32(stack[1]), uint32(stack[2]))
	if !ok {
		stack[0] = wasiErrnoNoProtoOpt
		return
	}

	valuePtr := uint32(stack[3])
	valueLen := uint32(stack[4])
	if valueLen < 4 {
		stack[0] = wasiErrnoInval
		return
	}

	value, ok := mod.Memory().ReadUint32Le(valuePtr)
	if !ok {
		stack[0] = wasiErrnoFault
		return
	}

	stack[0] = uint64(wasiErrnoFromError(syscall.SetsockoptInt(hostFD, level, name, int(int32(value)))))
}

func (b *wasiSocketBridge) sockGetLocalAddr(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	sa, err := syscall.Getsockname(hostFD)
	if err != nil {
		stack[0] = uint64(wasiErrnoFromError(err))
		return
	}
	stack[0] = uint64(writeSockaddr(mod.Memory(), uint32(stack[1]), uint32(stack[2]), sa))
}

func (b *wasiSocketBridge) sockGetPeerAddr(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	sa, err := syscall.Getpeername(hostFD)
	if err != nil {
		stack[0] = uint64(wasiErrnoFromError(err))
		return
	}
	stack[0] = uint64(writeSockaddr(mod.Memory(), uint32(stack[1]), uint32(stack[2]), sa))
}

func (b *wasiSocketBridge) sockRecvFrom(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	mem := mod.Memory()
	iovs, errno := readIOVecs(mem, uint32(stack[1]), uint32(stack[2]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	total := totalIOVecLen(iovs)
	buf := make([]byte, total)
	n, from, err := syscall.Recvfrom(hostFD, buf, int(stack[4]))
	if err != nil {
		stack[0] = uint64(wasiErrnoFromError(err))
		return
	}

	if ok := writeIOVecs(mem, iovs, buf[:n]); !ok {
		stack[0] = wasiErrnoFault
		return
	}

	portPtr := uint32(stack[5])
	if portPtr != 0 {
		if from == nil {
			if !mem.WriteUint32Le(portPtr, 0) {
				stack[0] = wasiErrnoFault
				return
			}
		} else if errno := writeSockaddr(mem, uint32(stack[3]), portPtr, from); errno != wasiErrnoSuccess {
			stack[0] = uint64(errno)
			return
		}
	}

	if !mem.WriteUint32Le(uint32(stack[6]), uint32(n)) {
		stack[0] = wasiErrnoFault
		return
	}
	if !mem.WriteUint32Le(uint32(stack[7]), 0) {
		stack[0] = wasiErrnoFault
		return
	}

	stack[0] = wasiErrnoSuccess
}

func (b *wasiSocketBridge) sockSendTo(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	mem := mod.Memory()
	iovs, errno := readIOVecs(mem, uint32(stack[1]), uint32(stack[2]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}
	payload, ok := gatherIOVecs(mem, iovs)
	if !ok {
		stack[0] = wasiErrnoFault
		return
	}

	var n int
	var err error
	if uint32(stack[3]) == 0 {
		n, err = syscall.Write(hostFD, payload)
	} else {
		sa, decErr := decodeSockaddr(mem, uint32(stack[3]), uint32(stack[4]))
		if decErr != wasiErrnoSuccess {
			stack[0] = uint64(decErr)
			return
		}
		err = syscall.Sendto(hostFD, payload, int(stack[5]), sa)
		if err == nil {
			n = len(payload)
		}
	}
	if err != nil {
		stack[0] = uint64(wasiErrnoFromError(err))
		return
	}

	if !mem.WriteUint32Le(uint32(stack[6]), uint32(n)) {
		stack[0] = wasiErrnoFault
		return
	}
	stack[0] = wasiErrnoSuccess
}

func (b *wasiSocketBridge) sockGetAddrInfo(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	mem := mod.Memory()

	node, ok := readGuestString(mem, uint32(stack[0]), uint32(stack[1]))
	if !ok {
		stack[0] = wasiErrnoFault
		return
	}
	service, ok := readGuestString(mem, uint32(stack[2]), uint32(stack[3]))
	if !ok {
		stack[0] = wasiErrnoFault
		return
	}

	flags, family, sockType, errno := readAddrInfoHints(mem, uint32(stack[4]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	port, err := resolveServicePort(service, sockType)
	if err != nil {
		stack[0] = uint64(wasiErrnoFromError(err))
		return
	}

	ips, errno := lookupAddrInfoIPs(node, family, flags)
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}
	if len(ips) == 0 {
		if !mem.WriteUint32Le(uint32(stack[7]), 0) {
			stack[0] = wasiErrnoFault
			return
		}
		stack[0] = wasiErrnoNoEnt
		return
	}

	resPtrPtr := uint32(stack[5])
	entryPtr, ok := mem.ReadUint32Le(resPtrPtr)
	if !ok || entryPtr == 0 {
		stack[0] = wasiErrnoFault
		return
	}

	maxRes := uint32(stack[6])
	if maxRes == 0 {
		if !mem.WriteUint32Le(uint32(stack[7]), 0) {
			stack[0] = wasiErrnoFault
			return
		}
		stack[0] = wasiErrnoSuccess
		return
	}

	written := uint32(0)
	current := entryPtr
	for i := 0; i < len(ips) && written < maxRes && current != 0; i++ {
		familyCode, sockData := marshalAddrInfoData(ips[i], port)
		if familyCode == 0 {
			continue
		}

		aiAddrPtr, ok := mem.ReadUint32Le(current + 12)
		if !ok || aiAddrPtr == 0 {
			stack[0] = wasiErrnoFault
			return
		}

		nextPtr, ok := mem.ReadUint32Le(current + 28)
		if !ok {
			stack[0] = wasiErrnoFault
			return
		}

		saDataLen, ok := mem.ReadUint32Le(aiAddrPtr + 4)
		if !ok {
			stack[0] = wasiErrnoFault
			return
		}
		saDataPtr, ok := mem.ReadUint32Le(aiAddrPtr + 8)
		if !ok || saDataPtr == 0 {
			stack[0] = wasiErrnoFault
			return
		}
		if saDataLen < uint32(len(sockData)) {
			stack[0] = wasiErrnoInval
			return
		}

		if !mem.WriteUint32Le(aiAddrPtr, uint32(familyCode)) {
			stack[0] = wasiErrnoFault
			return
		}
		if !mem.Write(saDataPtr, sockData) {
			stack[0] = wasiErrnoFault
			return
		}
		if !writeUint8(mem, current+2, familyCode) {
			stack[0] = wasiErrnoFault
			return
		}

		resolvedSockType := sockType
		if resolvedSockType == abiSockAny {
			resolvedSockType = abiSockStr
		}
		if !writeUint8(mem, current+3, resolvedSockType) {
			stack[0] = wasiErrnoFault
			return
		}

		protocol := uint32(0)
		switch resolvedSockType {
		case abiSockStr:
			protocol = abiIPProtoTCP
		case abiSockDgr:
			protocol = abiIPProtoUDP
		}
		if !mem.WriteUint32Le(current+4, protocol) {
			stack[0] = wasiErrnoFault
			return
		}

		written++
		current = nextPtr
	}

	if !mem.WriteUint32Le(uint32(stack[7]), written) {
		stack[0] = wasiErrnoFault
		return
	}
	if written == 0 {
		stack[0] = wasiErrnoNoEnt
		return
	}
	stack[0] = wasiErrnoSuccess
}

func (b *wasiSocketBridge) sockShutdown(_ context.Context, mod wazeroapi.Module, stack []uint64) {
	hostFD, errno := hostFDFromGuest(mod, int32(stack[0]))
	if errno != wasiErrnoSuccess {
		stack[0] = uint64(errno)
		return
	}

	var how int
	switch int32(stack[1]) {
	case 1:
		how = syscall.SHUT_RD
	case 2:
		how = syscall.SHUT_WR
	case 3:
		how = syscall.SHUT_RDWR
	default:
		stack[0] = wasiErrnoInval
		return
	}
	stack[0] = uint64(wasiErrnoFromError(syscall.Shutdown(hostFD, how)))
}

func abiToSysDomain(af int32) (int, bool) {
	switch af {
	case abiAFInet:
		return syscall.AF_INET, true
	case abiAFInet6:
		return syscall.AF_INET6, true
	case abiAFUnix:
		return syscall.AF_UNIX, true
	default:
		return 0, false
	}
}

func abiToSysType(sockType int32) (int, bool) {
	switch sockType {
	case abiSockStr:
		return syscall.SOCK_STREAM, true
	case abiSockDgr:
		return syscall.SOCK_DGRAM, true
	case abiSockAny:
		return syscall.SOCK_STREAM, true
	default:
		return 0, false
	}
}

func mapSocketOption(level, name uint32) (int, int, bool) {
	if level != abiSOLSocket {
		return 0, 0, false
	}
	switch name {
	case abiSOReuseAddr:
		return syscall.SOL_SOCKET, syscall.SO_REUSEADDR, true
	case abiSOError:
		return syscall.SOL_SOCKET, syscall.SO_ERROR, true
	case abiSOBroadcast:
		return syscall.SOL_SOCKET, syscall.SO_BROADCAST, true
	default:
		return 0, 0, false
	}
}

func openSocketAsWASIFile(hostFD int) (wazerosys.File, uint32) {
	paths := []string{
		fmt.Sprintf("/dev/fd/%d", hostFD),
		fmt.Sprintf("/proc/self/fd/%d", hostFD),
	}
	var lastErr uint32 = wasiErrnoNoEnt
	for _, p := range paths {
		f, errno := wazeroOpenOSFile(p, wazerosys.O_RDWR, 0)
		if errno == 0 {
			return f, wasiErrnoSuccess
		}
		lastErr = wasiErrnoFromExperimental(errno)
	}
	return nil, lastErr
}

func registerFileDescriptor(mod wazeroapi.Module, file wazerosys.File) (int32, uint32) {
	fsCtx, errno := moduleFSContext(mod)
	if errno != wasiErrnoSuccess {
		return -1, errno
	}

	openMethod := fsCtx.MethodByName("OpenFile")
	if !openMethod.IsValid() {
		return -1, wasiErrnoNoSys
	}

	fsImpl := &singleUseFS{file: file}
	results := openMethod.Call([]reflect.Value{
		reflect.ValueOf(fsImpl),
		reflect.ValueOf("socket-" + strconv.FormatInt(int64(os.Getpid()), 10)),
		reflect.ValueOf(wazerosys.O_RDWR),
		reflect.ValueOf(stdfs.FileMode(0o600)),
	})
	if len(results) != 2 {
		return -1, wasiErrnoIO
	}

	errnoExp := valueToExperimentalErrno(results[1])
	if errnoExp != 0 {
		return -1, wasiErrnoFromExperimental(errnoExp)
	}
	return int32(results[0].Int()), wasiErrnoSuccess
}

func moduleFSContext(mod wazeroapi.Module) (reflect.Value, uint32) {
	modVal := reflect.ValueOf(mod)
	if modVal.Kind() != reflect.Pointer || modVal.IsNil() {
		return reflect.Value{}, wasiErrnoInval
	}
	modElem := modVal.Elem()
	sysField := modElem.FieldByName("Sys")
	if !sysField.IsValid() || sysField.IsNil() {
		return reflect.Value{}, wasiErrnoBadf
	}
	fsMethod := sysField.MethodByName("FS")
	if !fsMethod.IsValid() {
		return reflect.Value{}, wasiErrnoNoSys
	}
	out := fsMethod.Call(nil)
	if len(out) != 1 || !out[0].IsValid() || out[0].IsNil() {
		return reflect.Value{}, wasiErrnoBadf
	}
	return out[0], wasiErrnoSuccess
}

func hostFDFromGuest(mod wazeroapi.Module, guestFD int32) (int, uint32) {
	fsCtx, errno := moduleFSContext(mod)
	if errno != wasiErrnoSuccess {
		return -1, errno
	}
	lookup := fsCtx.MethodByName("LookupFile")
	if !lookup.IsValid() {
		return -1, wasiErrnoNoSys
	}
	results := lookup.Call([]reflect.Value{reflect.ValueOf(guestFD)})
	if len(results) != 2 || !results[1].Bool() {
		return -1, wasiErrnoBadf
	}

	fd, ok := extractFileDescriptor(results[0])
	if !ok {
		return -1, wasiErrnoNotSock
	}
	return fd, wasiErrnoSuccess
}

func extractFileDescriptor(entry reflect.Value) (int, bool) {
	if !entry.IsValid() {
		return 0, false
	}

	switch entry.Kind() {
	case reflect.Interface:
		if entry.IsNil() {
			return 0, false
		}
		return extractFileDescriptor(entry.Elem())

	case reflect.Pointer:
		if entry.IsNil() {
			return 0, false
		}
		return extractFileDescriptor(entry.Elem())

	case reflect.Struct:
		if fdField := entry.FieldByName("fd"); fdField.IsValid() && fdField.Kind() == reflect.Uintptr && fdField.CanAddr() {
			return int(*(*uintptr)(unsafe.Pointer(fdField.UnsafeAddr()))), true
		}

		for _, wrapperField := range []string{"File", "file", "f"} {
			child := entry.FieldByName(wrapperField)
			if !child.IsValid() {
				continue
			}
			if fd, ok := extractFileDescriptor(child); ok {
				return fd, true
			}
		}
	}

	return 0, false
}

func decodeSockaddr(mem wazeroapi.Memory, addressBufferPtr uint32, port uint32) (syscall.Sockaddr, uint32) {
	addrBuf, ok := readAddressBuffer(mem, addressBufferPtr)
	if !ok {
		return nil, wasiErrnoFault
	}
	raw, ok := mem.Read(addrBuf.buf, addrBuf.bufLen)
	if !ok {
		return nil, wasiErrnoFault
	}

	if len(raw) == netIP4Len {
		sa := &syscall.SockaddrInet4{Port: int(port)}
		copy(sa.Addr[:], raw)
		return sa, wasiErrnoSuccess
	}
	if len(raw) == netIP6Len {
		sa := &syscall.SockaddrInet6{Port: int(port)}
		copy(sa.Addr[:], raw)
		return sa, wasiErrnoSuccess
	}
	if len(raw) < 2 {
		return nil, wasiErrnoInval
	}

	family := int32(binary.LittleEndian.Uint16(raw[:2]))
	switch family {
	case abiAFInet:
		if len(raw) < 6 {
			return nil, wasiErrnoInval
		}
		sa := &syscall.SockaddrInet4{Port: int(port)}
		copy(sa.Addr[:], raw[2:6])
		return sa, wasiErrnoSuccess
	case abiAFInet6:
		if len(raw) < 18 {
			return nil, wasiErrnoInval
		}
		sa := &syscall.SockaddrInet6{Port: int(port)}
		copy(sa.Addr[:], raw[2:18])
		return sa, wasiErrnoSuccess
	case abiAFUnix:
		name := trimCString(raw[2:])
		return &syscall.SockaddrUnix{Name: name}, wasiErrnoSuccess
	default:
		return nil, wasiErrnoAFNoSupport
	}
}

func writeSockaddr(mem wazeroapi.Memory, addressBufferPtr, portPtr uint32, sa syscall.Sockaddr) uint32 {
	addrBuf, ok := readAddressBuffer(mem, addressBufferPtr)
	if !ok {
		return wasiErrnoFault
	}

	var raw []byte
	var port uint32

	switch a := sa.(type) {
	case *syscall.SockaddrInet4:
		raw = make([]byte, 6)
		binary.LittleEndian.PutUint16(raw[:2], uint16(abiAFInet))
		copy(raw[2:], a.Addr[:])
		port = uint32(a.Port)
	case *syscall.SockaddrInet6:
		raw = make([]byte, 18)
		binary.LittleEndian.PutUint16(raw[:2], uint16(abiAFInet6))
		copy(raw[2:], a.Addr[:])
		port = uint32(a.Port)
	case *syscall.SockaddrUnix:
		raw = make([]byte, 2+len(a.Name)+1)
		binary.LittleEndian.PutUint16(raw[:2], uint16(abiAFUnix))
		copy(raw[2:], a.Name)
	default:
		return wasiErrnoNotSup
	}

	if uint32(len(raw)) > addrBuf.bufLen {
		return wasiErrnoInval
	}
	if !mem.Write(addrBuf.buf, raw) {
		return wasiErrnoFault
	}
	if !mem.WriteUint32Le(addressBufferPtr+4, uint32(len(raw))) {
		return wasiErrnoFault
	}
	if portPtr != 0 && !mem.WriteUint32Le(portPtr, port) {
		return wasiErrnoFault
	}
	return wasiErrnoSuccess
}

func readAddressBuffer(mem wazeroapi.Memory, ptr uint32) (addressBuffer, bool) {
	buf, ok := mem.ReadUint32Le(ptr)
	if !ok {
		return addressBuffer{}, false
	}
	bufLen, ok := mem.ReadUint32Le(ptr + 4)
	if !ok {
		return addressBuffer{}, false
	}
	return addressBuffer{buf: buf, bufLen: bufLen}, true
}

func readIOVecs(mem wazeroapi.Memory, ptr uint32, count uint32) ([]iovec, uint32) {
	iovs := make([]iovec, 0, count)
	for i := uint32(0); i < count; i++ {
		offset := ptr + (i * 8)
		bufPtr, ok := mem.ReadUint32Le(offset)
		if !ok {
			return nil, wasiErrnoFault
		}
		bufLen, ok := mem.ReadUint32Le(offset + 4)
		if !ok {
			return nil, wasiErrnoFault
		}
		iovs = append(iovs, iovec{ptr: bufPtr, len: bufLen})
	}
	return iovs, wasiErrnoSuccess
}

func totalIOVecLen(iovs []iovec) int {
	total := 0
	for _, io := range iovs {
		total += int(io.len)
	}
	return total
}

func gatherIOVecs(mem wazeroapi.Memory, iovs []iovec) ([]byte, bool) {
	total := totalIOVecLen(iovs)
	payload := make([]byte, 0, total)
	for _, io := range iovs {
		chunk, ok := mem.Read(io.ptr, io.len)
		if !ok {
			return nil, false
		}
		payload = append(payload, chunk...)
	}
	return payload, true
}

func writeIOVecs(mem wazeroapi.Memory, iovs []iovec, data []byte) bool {
	remaining := data
	for _, io := range iovs {
		if len(remaining) == 0 {
			return true
		}
		n := int(io.len)
		if n > len(remaining) {
			n = len(remaining)
		}
		if !mem.Write(io.ptr, remaining[:n]) {
			return false
		}
		remaining = remaining[n:]
	}
	return len(remaining) == 0
}

func trimCString(b []byte) string {
	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func readGuestString(mem wazeroapi.Memory, ptr, length uint32) (string, bool) {
	if ptr == 0 || length == 0 {
		return "", true
	}
	raw, ok := mem.Read(ptr, length)
	if !ok {
		return "", false
	}
	return trimCString(raw), true
}

func readAddrInfoHints(mem wazeroapi.Memory, hintsPtr uint32) (uint16, uint8, uint8, uint32) {
	if hintsPtr == 0 {
		return 0, abiAFUnspec, abiSockAny, wasiErrnoSuccess
	}
	raw, ok := mem.Read(hintsPtr, 8)
	if !ok || len(raw) < 8 {
		return 0, 0, 0, wasiErrnoFault
	}
	flags := binary.LittleEndian.Uint16(raw[:2])
	family := raw[2]
	sockType := raw[3]
	return flags, family, sockType, wasiErrnoSuccess
}

func resolveServicePort(service string, sockType uint8) (int, error) {
	if service == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(service); err == nil {
		return n, nil
	}
	network := "tcp"
	if sockType == abiSockDgr {
		network = "udp"
	}
	return net.DefaultResolver.LookupPort(context.Background(), network, service)
}

func lookupAddrInfoIPs(node string, family uint8, flags uint16) ([]net.IP, uint32) {
	if node == "" {
		if flags&abiAIPassive != 0 {
			switch family {
			case abiAFInet:
				return []net.IP{net.IPv4zero}, wasiErrnoSuccess
			case abiAFInet6:
				return []net.IP{net.IPv6zero}, wasiErrnoSuccess
			default:
				return []net.IP{net.IPv4zero, net.IPv6zero}, wasiErrnoSuccess
			}
		}
		switch family {
		case abiAFInet:
			return []net.IP{net.IPv4(127, 0, 0, 1)}, wasiErrnoSuccess
		case abiAFInet6:
			return []net.IP{net.IPv6loopback}, wasiErrnoSuccess
		default:
			return []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}, wasiErrnoSuccess
		}
	}

	if ip := net.ParseIP(node); ip != nil {
		return filterIPsByFamily([]net.IP{ip}, family), wasiErrnoSuccess
	}

	rawIPs, err := net.DefaultResolver.LookupIP(context.Background(), "ip", node)
	if err != nil {
		return nil, wasiErrnoFromError(err)
	}
	return filterIPsByFamily(rawIPs, family), wasiErrnoSuccess
}

func filterIPsByFamily(ips []net.IP, family uint8) []net.IP {
	result := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil {
			if family == abiAFInet6 {
				continue
			}
			result = append(result, append(net.IP(nil), ip4...))
			continue
		}
		ip16 := ip.To16()
		if ip16 == nil {
			continue
		}
		if family == abiAFInet {
			continue
		}
		result = append(result, append(net.IP(nil), ip16...))
	}
	return result
}

func marshalAddrInfoData(ip net.IP, port int) (uint8, []byte) {
	if ip4 := ip.To4(); ip4 != nil {
		data := make([]byte, 2+netIP4Len)
		binary.BigEndian.PutUint16(data[:2], uint16(port))
		copy(data[2:], ip4)
		return abiAFInet, data
	}
	ip16 := ip.To16()
	if ip16 == nil {
		return 0, nil
	}
	data := make([]byte, 2+netIP6Len)
	binary.BigEndian.PutUint16(data[:2], uint16(port))
	copy(data[2:], ip16)
	return abiAFInet6, data
}

func writeUint8(mem wazeroapi.Memory, ptr uint32, value uint8) bool {
	return mem.Write(ptr, []byte{value})
}

func valueToExperimentalErrno(v reflect.Value) wazerosys.Errno {
	if !v.IsValid() {
		return wazerosys.EIO
	}
	switch v.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return wazerosys.Errno(v.Uint())
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return wazerosys.Errno(v.Int())
	default:
		return wazerosys.EIO
	}
}

func wasiErrnoFromExperimental(errno wazerosys.Errno) uint32 {
	switch errno {
	case 0:
		return wasiErrnoSuccess
	case wazerosys.EACCES:
		return wasiErrnoAcces
	case wazerosys.EAGAIN:
		return wasiErrnoAgain
	case wazerosys.EBADF:
		return wasiErrnoBadf
	case wazerosys.EEXIST:
		return wasiErrnoAddrInUse
	case wazerosys.EFAULT:
		return wasiErrnoFault
	case wazerosys.EINTR:
		return wasiErrnoIntr
	case wazerosys.EINVAL:
		return wasiErrnoInval
	case wazerosys.EIO:
		return wasiErrnoIO
	case wazerosys.ENOENT:
		return wasiErrnoNoEnt
	case wazerosys.ENOSYS:
		return wasiErrnoNoSys
	case wazerosys.ENOTSOCK:
		return wasiErrnoNotSock
	case wazerosys.ENOTSUP:
		return wasiErrnoNotSup
	default:
		return wasiErrnoIO
	}
}

func wasiErrnoFromError(err error) uint32 {
	if err == nil {
		return wasiErrnoSuccess
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return wasiErrnoFromSyscall(errno)
	}
	return wasiErrnoIO
}

func wasiErrnoFromSyscall(errno syscall.Errno) uint32 {
	switch errno {
	case 0:
		return wasiErrnoSuccess
	case syscall.EACCES:
		return wasiErrnoAcces
	case syscall.EADDRINUSE:
		return wasiErrnoAddrInUse
	case syscall.EADDRNOTAVAIL:
		return wasiErrnoAddrNotAvail
	case syscall.EAFNOSUPPORT:
		return wasiErrnoAFNoSupport
	case syscall.EALREADY:
		return wasiErrnoAlready
	case syscall.EBADF:
		return wasiErrnoBadf
	case syscall.ECONNREFUSED:
		return wasiErrnoConnRefused
	case syscall.ECONNRESET:
		return wasiErrnoConnReset
	case syscall.EDESTADDRREQ:
		return wasiErrnoDestAddrReq
	case syscall.EHOSTUNREACH:
		return wasiErrnoHostUnreach
	case syscall.EINPROGRESS:
		return wasiErrnoInProgress
	case syscall.EINTR:
		return wasiErrnoIntr
	case syscall.EINVAL:
		return wasiErrnoInval
	case syscall.EISCONN:
		return wasiErrnoIsConn
	case syscall.ENETDOWN:
		return wasiErrnoNetDown
	case syscall.ENETUNREACH:
		return wasiErrnoNetUnreach
	case syscall.ENOENT:
		return wasiErrnoNoEnt
	case syscall.ENOPROTOOPT:
		return wasiErrnoNoProtoOpt
	case syscall.ENOTCONN:
		return wasiErrnoNotConn
	case syscall.ENOTSOCK:
		return wasiErrnoNotSock
	case syscall.ENOTSUP:
		return wasiErrnoNotSup
	case syscall.EPROTOTYPE:
		return wasiErrnoProtoType
	case syscall.ETIMEDOUT:
		return wasiErrnoTimedOut
	case syscall.EAGAIN:
		return wasiErrnoAgain
	default:
		return wasiErrnoIO
	}
}

type singleUseFS struct {
	wazerosys.UnimplementedFS
	file wazerosys.File
}

func (s *singleUseFS) OpenFile(string, wazerosys.Oflag, stdfs.FileMode) (wazerosys.File, wazerosys.Errno) {
	if s.file == nil {
		return nil, wazerosys.EBADF
	}
	f := s.file
	s.file = nil
	return f, 0
}

const (
	netIP4Len = 4
	netIP6Len = 16
)
