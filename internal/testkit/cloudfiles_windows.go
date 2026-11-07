//go:build windows

package testkit

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Эмуляция поставщика облачных файлов (Cloud Files API, cldapi.dll) для проб «фото OneDrive только в сети»
// (160-M1): папка регистрируется корнем синхронизации, в ней создаются заглушки без данных (dehydrated), а при
// чтении фильтр cldflt зовёт колбэк FETCH_DATA, и тот отдаёт байты через CfExecute. OneDrive строит заглушки
// тем же API, так что файл ведёт себя как у него: точка повторной обработки с тегом IO_REPARSE_TAG_CLOUD_*.
// Без cgo, через syscall.NewLazyDLL; раскладка структур — по cfapi.h (x64/arm64, естественное выравнивание).

var (
	cldapi                   = syscall.NewLazyDLL("cldapi.dll")
	procCfRegisterSyncRoot   = cldapi.NewProc("CfRegisterSyncRoot")
	procCfUnregisterSyncRoot = cldapi.NewProc("CfUnregisterSyncRoot")
	procCfConnectSyncRoot    = cldapi.NewProc("CfConnectSyncRoot")
	procCfDisconnectSyncRoot = cldapi.NewProc("CfDisconnectSyncRoot")
	procCfCreatePlaceholders = cldapi.NewProc("CfCreatePlaceholders")
	procCfExecute            = cldapi.NewProc("CfExecute")
	procGetLongPathName      = syscall.NewLazyDLL("kernel32.dll").NewProc("GetLongPathNameW")
)

const (
	cfCallbackFetchData uint32 = 0           // CF_CALLBACK_TYPE_FETCH_DATA
	cfCallbackNone      uint32 = 0xFFFFFFFF  // CF_CALLBACK_TYPE_NONE: конец таблицы
	cfOpTransferData    uint32 = 0           // CF_OPERATION_TYPE_TRANSFER_DATA
	cfHydrationFull     uint16 = 2           // CF_HYDRATION_POLICY_FULL
	cfPopulationFull    uint16 = 3           // CF_POPULATION_POLICY_ALWAYS_FULL: заглушки создаём сами заранее
	cfPlaceholderInSync uint32 = 2           // CF_PLACEHOLDER_CREATE_FLAG_MARK_IN_SYNC
	statusUnsuccessful  int32  = -0x3FFFFFFF // 0xC0000001
)

type cfSyncRegistration struct {
	StructSize             uint32
	ProviderName           *uint16
	ProviderVersion        *uint16
	SyncRootIdentity       unsafe.Pointer
	SyncRootIdentityLength uint32
	FileIdentity           unsafe.Pointer
	FileIdentityLength     uint32
	ProviderID             [16]byte
}

type cfSyncPolicies struct {
	StructSize            uint32
	HydrationPrimary      uint16
	HydrationModifier     uint16
	PopulationPrimary     uint16
	PopulationModifier    uint16
	InSync                uint32
	HardLink              uint32
	PlaceholderManagement uint32
}

type cfCallbackReg struct {
	Type uint32
	Fn   uintptr
}

// cfCallbackInfo — CF_CALLBACK_INFO (поля до RequestKey).
type cfCallbackInfo struct {
	StructSize             uint32
	ConnectionKey          int64
	CallbackContext        unsafe.Pointer
	VolumeGUIDName         *uint16
	VolumeDosName          *uint16
	VolumeSerialNumber     uint32
	SyncRootFileID         int64
	SyncRootIdentity       unsafe.Pointer
	SyncRootIdentityLength uint32
	FileID                 int64
	FileSize               int64
	FileIdentity           unsafe.Pointer
	FileIdentityLength     uint32
	NormalizedPath         *uint16
	TransferKey            int64
	PriorityHint           uint8
	CorrelationVector      unsafe.Pointer
	ProcessInfo            unsafe.Pointer
	RequestKey             int64
}

// cfFetchParams — CF_CALLBACK_PARAMETERS.FetchData (объединение начинается со смещения 8).
type cfFetchParams struct {
	ParamSize          uint32
	_                  uint32
	Flags              uint32
	RequiredFileOffset int64
	RequiredLength     int64
}

type cfOperationInfo struct {
	StructSize        uint32
	Type              uint32
	ConnectionKey     int64
	TransferKey       int64
	CorrelationVector unsafe.Pointer
	SyncStatus        unsafe.Pointer
	RequestKey        int64
}

// cfTransferParams — CF_OPERATION_PARAMETERS.TransferData; хвост — запас под большие члены объединения.
type cfTransferParams struct {
	ParamSize        uint32
	_                uint32
	Flags            uint32
	CompletionStatus int32
	Buffer           unsafe.Pointer
	Offset           int64
	Length           int64
	_                [64]byte
}

// cfTransferParamSize — CF_SIZE_OF_OP_PARAM(TransferData): смещение объединения (8) и размер члена (32).
const cfTransferParamSize = 8 + 32

type cfPlaceholderCreateInfo struct {
	RelativeFileName   *uint16
	CreationTime       int64
	LastAccessTime     int64
	LastWriteTime      int64
	ChangeTime         int64
	FileAttributes     uint32
	FileSize           int64
	FileIdentity       unsafe.Pointer
	FileIdentityLength uint32
	Flags              uint32
	Result             int32
	CreateUsn          int64
}

// CloudRoot — корень синхронизации с поставщиком-заглушкой: файлы, созданные Stub, скачиваются из памяти.
type CloudRoot struct {
	Dir string // корень синхронизации (длинное имя, без 8.3)

	id    uintptr
	key   int64
	table [2]cfCallbackReg

	mu      sync.Mutex
	files   map[string][]byte
	fetches int
	errs    []string
}

var (
	cloudMu    sync.Mutex
	cloudNext  uintptr
	cloudRoots = map[uintptr]*CloudRoot{}
	cloudCbOne sync.Once
	cloudCbPtr uintptr
)

// cloudCallback — единственный колбэк процесса (syscall.NewCallback ограничен по числу): корень по контексту.
func cloudCallback() uintptr {
	cloudCbOne.Do(func() { cloudCbPtr = syscall.NewCallback(cloudFetch) })
	return cloudCbPtr
}

func cloudFetch(info *cfCallbackInfo, prm *cfFetchParams) uintptr {
	cloudMu.Lock()
	r := cloudRoots[uintptr(info.CallbackContext)]
	cloudMu.Unlock()
	if r != nil {
		r.fetch(info, prm)
	}
	return 0
}

// fetch отдаёт запрошенный диапазон файла; любая неудача — в r.errs и ответ с ошибкой (чтение не зависает).
func (r *CloudRoot) fetch(info *cfCallbackInfo, prm *cfFetchParams) {
	var name string
	if info.FileIdentity != nil && info.FileIdentityLength > 0 {
		name = string(unsafe.Slice((*byte)(info.FileIdentity), info.FileIdentityLength))
	}
	r.mu.Lock()
	r.fetches++
	data, ok := r.files[name]
	r.mu.Unlock()
	off, n := prm.RequiredFileOffset, prm.RequiredLength
	if n > int64(len(data))-off {
		n = int64(len(data)) - off // хвост файла: длина не кратна 4 КБ — допустимо только в конце
	}
	tp := cfTransferParams{ParamSize: cfTransferParamSize, Offset: off, Length: n}
	var dummy [1]byte
	switch {
	case !ok || off < 0 || n <= 0:
		r.fail(fmt.Sprintf("FETCH_DATA: файл %q найден=%v, диапазон %d+%d", name, ok, off, prm.RequiredLength))
		tp.CompletionStatus, tp.Buffer, tp.Length = statusUnsuccessful, unsafe.Pointer(&dummy[0]), prm.RequiredLength
	default:
		tp.Buffer = unsafe.Pointer(&data[off])
	}
	op := cfOperationInfo{StructSize: uint32(unsafe.Sizeof(cfOperationInfo{})), Type: cfOpTransferData,
		ConnectionKey: info.ConnectionKey, TransferKey: info.TransferKey}
	hr, _, _ := procCfExecute.Call(uintptr(unsafe.Pointer(&op)), uintptr(unsafe.Pointer(&tp))) //nolint:gosec // G103: аргументы WinAPI
	if int32(hr) < 0 {
		r.fail(fmt.Sprintf("CfExecute(TRANSFER_DATA): HRESULT %#08x", uint32(hr)))
	}
}

func (r *CloudRoot) fail(msg string) {
	r.mu.Lock()
	r.errs = append(r.errs, msg)
	r.mu.Unlock()
}

// Fetches — сколько раз фильтр просил данные у поставщика (гидратация случилась).
func (r *CloudRoot) Fetches() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fetches
}

// Errors — сбои поставщика в колбэке (пусто — всё отдано).
func (r *CloudRoot) Errors() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.errs...)
}

func hresult(r uintptr) string { return fmt.Sprintf("HRESULT %#08x", uint32(r)) }

// NewCloudRoot регистрирует и подключает корень синхронизации во временной папке теста. Нет cldapi.dll,
// не зарегистрировался (фильтра cldflt нет, сервер без него, том не NTFS) — t.Skip с кодом HRESULT:
// пропуск виден в журнале CI. Отключение и снятие регистрации — в Cleanup, до удаления папки.
func NewCloudRoot(t testing.TB) *CloudRoot {
	t.Helper()
	if err := cldapi.Load(); err != nil {
		t.Skipf("Cloud Files API недоступен: %v", err)
	}
	dir := longPath(filepath.Join(TempDir(t), "sync"))
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("корень синхронизации: %v", err)
	}
	dir = longPath(dir)
	r := &CloudRoot{Dir: dir, files: map[string][]byte{}}
	cloudMu.Lock()
	cloudNext++
	r.id = cloudNext
	cloudRoots[r.id] = r
	cloudMu.Unlock()

	dirP := wstr(dir)
	name, ver := wstr("SosninaAlbums test"), wstr("1")
	ident := []byte("sa-test-root")
	var why []string
	registered := false
	// Размеры структур отличаются между версиями SDK/Windows: пробуем с новыми полями и без них.
	for _, sz := range [][2]uint32{{72, 24}, {56, 24}, {72, 20}, {56, 20}} {
		reg := cfSyncRegistration{StructSize: sz[0], ProviderName: name, ProviderVersion: ver,
			SyncRootIdentity: unsafe.Pointer(&ident[0]), SyncRootIdentityLength: uint32(len(ident)),
			ProviderID: [16]byte{0x53, 0x41, 0x2d, 0x74, 0x65, 0x73, 0x74, 1}}
		pol := cfSyncPolicies{StructSize: sz[1], HydrationPrimary: cfHydrationFull, PopulationPrimary: cfPopulationFull}
		hr, _, _ := procCfRegisterSyncRoot.Call(uintptr(unsafe.Pointer(dirP)), uintptr(unsafe.Pointer(&reg)), //nolint:gosec // G103: аргументы WinAPI
			uintptr(unsafe.Pointer(&pol)), 0) //nolint:gosec // G103: аргументы WinAPI
		if int32(hr) >= 0 {
			registered = true
			break
		}
		why = append(why, fmt.Sprintf("размеры %d/%d: %s", sz[0], sz[1], hresult(hr)))
	}
	if !registered {
		r.forget()
		t.Skipf("CfRegisterSyncRoot(%s) не удался (фильтр cldflt недоступен или том не NTFS): %v", dir, why)
	}
	t.Cleanup(func() {
		_, _, _ = procCfDisconnectSyncRoot.Call(uintptr(r.key))
		_, _, _ = procCfUnregisterSyncRoot.Call(uintptr(unsafe.Pointer(dirP))) //nolint:gosec // G103: аргумент WinAPI
		r.forget()
	})

	r.table = [2]cfCallbackReg{{Type: cfCallbackFetchData, Fn: cloudCallback()}, {Type: cfCallbackNone}}
	hr, _, _ := procCfConnectSyncRoot.Call(uintptr(unsafe.Pointer(dirP)), uintptr(unsafe.Pointer(&r.table[0])), //nolint:gosec // G103: аргументы WinAPI
		r.id, 0, uintptr(unsafe.Pointer(&r.key))) //nolint:gosec // G103: аргумент WinAPI
	if int32(hr) < 0 {
		t.Fatalf("CfConnectSyncRoot: %s", hresult(hr))
	}
	return r
}

func (r *CloudRoot) forget() {
	cloudMu.Lock()
	delete(cloudRoots, r.id)
	cloudMu.Unlock()
}

// Stub создаёт в корне заглушку name (без данных на диске); при первом чтении данные берутся из data.
func (r *CloudRoot) Stub(t testing.TB, name string, data []byte) string {
	t.Helper()
	r.mu.Lock()
	r.files[name] = data
	r.mu.Unlock()
	ident := []byte(name)
	ft := syscall.NsecToFiletime(time.Now().UnixNano())
	now := int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
	info := cfPlaceholderCreateInfo{RelativeFileName: wstr(name), CreationTime: now, LastAccessTime: now,
		LastWriteTime: now, ChangeTime: now, FileAttributes: syscall.FILE_ATTRIBUTE_NORMAL, FileSize: int64(len(data)),
		FileIdentity: unsafe.Pointer(&ident[0]), FileIdentityLength: uint32(len(ident)), Flags: cfPlaceholderInSync}
	var done uint32
	hr, _, _ := procCfCreatePlaceholders.Call(uintptr(unsafe.Pointer(wstr(r.Dir))), uintptr(unsafe.Pointer(&info)), 1, 0, //nolint:gosec // G103: аргументы WinAPI
		uintptr(unsafe.Pointer(&done))) //nolint:gosec // G103: аргумент WinAPI
	if int32(hr) < 0 || info.Result < 0 || done != 1 {
		t.Fatalf("CfCreatePlaceholders(%s): %s, результат %#08x, создано %d", name, hresult(hr), uint32(info.Result), done)
	}
	return filepath.Join(r.Dir, name)
}

func wstr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic(err) // имена теста без нулевого байта
	}
	return p
}

// longPath — путь без 8.3 (RUNNER~1): корень синхронизации с коротким именем фильтр сверяет по длинному.
func longPath(p string) string {
	src := wstr(p)
	n, _, _ := procGetLongPathName.Call(uintptr(unsafe.Pointer(src)), 0, 0) //nolint:gosec // G103: аргумент WinAPI
	if n == 0 {
		return p
	}
	buf := make([]uint16, n)
	m, _, _ := procGetLongPathName.Call(uintptr(unsafe.Pointer(src)), uintptr(unsafe.Pointer(&buf[0])), n) //nolint:gosec // G103: аргументы WinAPI
	if m == 0 || m >= n {
		return p
	}
	return syscall.UTF16ToString(buf[:m])
}
