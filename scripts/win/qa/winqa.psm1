# winqa.psm1 - помощники проверки Windows-exe в Actions (спека 118, design §5): итоги results.jsonl, запуск и закрытие
# программы по PID, UIA, снимки окна и ролики, журнал программы, реестр, проверка набора.
# Windows PowerShell 5.1, файл в UTF-8 с BOM. Чистые функции проверяет test_winqa.ps1; окна видны только на раннере.
# Отличие от design §5: параметр называется -ProcId, а не -Pid: $PID в PowerShell - неизменяемая переменная.

$winDir = Split-Path -Parent $PSScriptRoot
Import-Module (Join-Path $winDir 'winlib.psm1') -DisableNameChecking
Import-Module (Join-Path $winDir 'winui.psm1') -DisableNameChecking
. (Join-Path $winDir 'record.ps1')

$script:Utf8 = New-Object System.Text.UTF8Encoding($false)
$script:Qa = @{ Out = ''; Job = ''; Runner = ''; Video = $false; Rec = $null; RecFile = ''; Item = ''; Step = ''; Seq = 0; Deadline = 0.0; Status = @{}; FindErr = '' }

# ---------- DPI процесса ----------

# Снимок и UIA-координаты считаются в пикселях экрана, поэтому сам процесс должен быть DPI-aware (PerMonitorV2, -4):
# иначе GetWindowRect/CopyFromScreen на масштабе 125-175 % вырезают не то окно. Выставляется один раз за процесс
# (повторный вызов и вызов после SetProcessDPIAware молча отказывает - это нормально). Возвращает $true, если режим включён
function Enable-QaDpiAware {
    if (-not ('SaQa.DpiAware' -as [type])) {
        Add-Type -TypeDefinition @'
namespace SaQa {
    public static class DpiAware {
        [System.Runtime.InteropServices.DllImport("user32.dll")] static extern bool SetProcessDpiAwarenessContext(System.IntPtr ctx);
        [System.Runtime.InteropServices.DllImport("user32.dll")] static extern System.IntPtr GetThreadDpiAwarenessContext();
        [System.Runtime.InteropServices.DllImport("user32.dll")] static extern int GetAwarenessFromDpiAwarenessContext(System.IntPtr ctx);
        public static bool Enable() {
            try {
                SetProcessDpiAwarenessContext(new System.IntPtr(-4));
                return GetAwarenessFromDpiAwarenessContext(GetThreadDpiAwarenessContext()) >= 1;
            } catch { return false; }
        }
    }
}
'@
    }
    return [SaQa.DpiAware]::Enable()
}

# ---------- Win32-окна и кнопки (когда UIA окна не видит; мастер NSIS, передний план программы) ----------

if (-not ('SaQa.Wiz' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
namespace SaQa {
    public static class Wiz {
        delegate bool EnumProc(IntPtr h, IntPtr l);
        [DllImport("user32.dll")] static extern bool EnumWindows(EnumProc p, IntPtr l);
        [DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr parent, EnumProc p, IntPtr l);
        [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
        [DllImport("user32.dll")] static extern bool IsWindowVisible(IntPtr h);
        [DllImport("user32.dll")] static extern bool IsWindowEnabled(IntPtr h);
        [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetClassNameW(IntPtr h, StringBuilder sb, int max);
        [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetWindowTextW(IntPtr h, StringBuilder sb, int max);
        [DllImport("user32.dll")] static extern bool PostMessage(IntPtr h, uint msg, IntPtr w, IntPtr l);
        [DllImport("user32.dll")] static extern bool SetForegroundWindow(IntPtr h);
        [DllImport("user32.dll")] static extern bool ShowWindow(IntPtr h, int cmd);
        [DllImport("user32.dll")] static extern bool IsIconic(IntPtr h);
        [DllImport("user32.dll")] static extern void keybd_event(byte vk, byte scan, uint flags, UIntPtr extra);
        // Окно на передний план: свернутое разворачиваем; одиночное нажатие Alt снимает запрет Windows на SetForegroundWindow
        public static bool Foreground(long h) {
            var p = new IntPtr(h);
            if (IsIconic(p)) ShowWindow(p, 9);
            keybd_event(0x12, 0, 0, UIntPtr.Zero); keybd_event(0x12, 0, 2, UIntPtr.Zero);
            return SetForegroundWindow(p);
        }
        [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
        [DllImport("user32.dll")] static extern bool AttachThreadInput(uint from, uint to, bool attach);
        [DllImport("kernel32.dll")] static extern uint GetCurrentThreadId();
        [DllImport("user32.dll")] static extern bool BringWindowToTop(IntPtr h);
        [DllImport("user32.dll")] static extern bool GetWindowRect(IntPtr h, out RECT r);
        [DllImport("user32.dll")] static extern bool SetCursorPos(int x, int y);
        [DllImport("user32.dll")] static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
        [StructLayout(LayoutKind.Sequential)] struct RECT { public int L, T, R, B; }
        // Окно программы на передний план без клавиш: одиночный Alt вводит окно без нативного меню в модальный цикл меню (WebView2 замирает).
        // Присоединяем свой поток ко входу потока переднего окна; если не вышло - щелчок мышью по пустой части заголовка
        public static bool ForegroundNoKey(long h) {
            var p = new IntPtr(h);
            if (IsIconic(p)) ShowWindow(p, 9);
            var fg = GetForegroundWindow();
            if (fg == p) return true;
            uint pid; uint ft = fg == IntPtr.Zero ? 0 : GetWindowThreadProcessId(fg, out pid);
            uint me = GetCurrentThreadId();
            bool att = ft != 0 && ft != me && AttachThreadInput(me, ft, true);
            BringWindowToTop(p); SetForegroundWindow(p);
            if (att) AttachThreadInput(me, ft, false);
            if (GetForegroundWindow() == p) return true;
            RECT r;
            if (GetWindowRect(p, out r)) {
                SetCursorPos((r.L + r.R) / 2, r.T + 10);
                mouse_event(0x0002, 0, 0, 0, UIntPtr.Zero); mouse_event(0x0004, 0, 0, 0, UIntPtr.Zero);
            }
            return GetForegroundWindow() == p;
        }
        // Видимые окна верхнего уровня процессов
        public static long[] TopWindows(int[] pids) {
            var set = new HashSet<int>(pids);
            var res = new List<long>();
            EnumWindows((h, l) => { uint pid; GetWindowThreadProcessId(h, out pid); if (set.Contains((int)pid) && IsWindowVisible(h)) res.Add(h.ToInt64()); return true; }, IntPtr.Zero);
            return res.ToArray();
        }
        // Включённые кнопки (класс Button) окна и всех его потомков: "дескриптор|текст"
        public static string[] Buttons(long top) {
            var res = new List<string>();
            EnumChildWindows(new IntPtr(top), (h, l) => {
                var cls = new StringBuilder(64); GetClassNameW(h, cls, 64);
                if (cls.ToString() == "Button" && IsWindowEnabled(h)) { var t = new StringBuilder(256); GetWindowTextW(h, t, 256); res.Add(h.ToInt64() + "|" + t); }
                return true;
            }, IntPtr.Zero);
            return res.ToArray();
        }
        // BM_CLICK через PostMessage: SendMessage блокируется, пока обработчик кнопки не вернётся (модальное окно)
        public static bool Click(long h) { return PostMessage(new IntPtr(h), 0x00F5, IntPtr.Zero, IntPtr.Zero); }
    }
}
'@
}

# ---------- итоги ----------

function Initialize-Qa {
    param([Parameter(Mandatory)][string]$OutDir, [Parameter(Mandatory)][string]$Job, [string]$Runner = '', [switch]$Video)
    foreach ($d in 'png', 'video', 'logs') { New-Item -ItemType Directory -Force -Path (Join-Path $OutDir $d) | Out-Null }
    $script:Qa = @{ Out = $OutDir; Job = $Job; Runner = $Runner; Video = [bool]$Video; Rec = $null; RecFile = ''; Item = ''; Step = ''; Seq = 0; Deadline = 0.0; Status = @{}; FindErr = '' }
    Set-LogPath (Join-Path $OutDir "logs\$Job.txt")
    Set-SummaryPath (Join-Path $OutDir 'summary.md')
}

# А->A, Б->B, В->V, Г->G, Д->D, Е->E, Ж->Zh: имена файлов в артефакте только ASCII (коды, чтобы не зависеть от кодировки файла)
function ConvertTo-AsciiId([string]$Id) {
    $map = @{ 0x410 = 'A'; 0x411 = 'B'; 0x412 = 'V'; 0x413 = 'G'; 0x414 = 'D'; 0x415 = 'E'; 0x416 = 'Zh' }
    $sb = New-Object System.Text.StringBuilder
    foreach ($ch in $Id.ToCharArray()) {
        $code = [int]$ch
        if ($map.ContainsKey($code)) { [void]$sb.Append($map[$code]) } else { [void]$sb.Append($ch) }
    }
    return $sb.ToString()
}

# Строка results.jsonl (UTF-8 без BOM). -Blocked <id>: пункту не хватило результата другого - итог "не проверено", не ложный провал
function Add-QaResult {
    param(
        [Parameter(Mandatory)][string]$Id,
        [Parameter(Mandatory)][ValidateSet('ok', 'fail', 'eyes', 'laptop', 'skip')][string]$Status,
        [string]$Detail = '', [string[]]$Shots = @(), [string]$Video = '', [double]$Sec = 0, [string]$Blocked = ''
    )
    if ($Blocked) {
        $Status = 'skip'  # param-reassign-ok: нормализация собственного параметра
        if (-not $Detail) { $Detail = "нет $Blocked" }  # param-reassign-ok
    }
    $vid = $null
    if ($Video) { $vid = $Video }
    $row = [ordered]@{
        id = $Id; status = $Status; detail = $Detail
        shots = @($Shots | Where-Object { $_ })
        video = $vid; sec = [math]::Round($Sec, 1); job = $script:Qa.Job; runner = $script:Qa.Runner
    }
    $path = Join-Path $script:Qa.Out 'results.jsonl'
    [System.IO.File]::AppendAllText($path, ((ConvertTo-JLine $row) + "`n"), $script:Utf8)
    $script:Qa.Status[$Id] = $Status
    # в журнал тоже: по logs/<блок>.txt видно, чем кончился каждый пункт, без разбора results.jsonl
    Write-Log ("[{0}] {1} {2}" -f $Status, $Id, $Detail)
}

# ---------- пульс для сторожа (watchdog.ps1) ----------

function Get-QaUnixNow { return [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds() / 1000.0 }

# logs/current.json: что идёт сейчас, на каком шаге и до какого срока. Сторож читает его раз в 2 с
function Write-QaPulse {
    $q = $script:Qa
    if (-not $q.Out) { return }
    $row = [ordered]@{ id = $q.Item; step = $q.Step; seq = $q.Seq; deadline = $q.Deadline; now = (Get-QaUnixNow) }
    $path = Join-Path (Join-Path $q.Out 'logs') 'current.json'
    try { [System.IO.File]::WriteAllText($path, (ConvertTo-JLine $row), $script:Utf8) } catch { }
}

# Начало отрезка пункта: срок в секундах, строка журнала, пульс
function Enter-QaItem {
    param([Parameter(Mandatory)][string]$Id, [int]$Budget = 180)
    $q = $script:Qa
    $q.Item = $Id; $q.Step = 'начало'; $q.Seq++
    $q.Deadline = (Get-QaUnixNow) + $Budget
    $q.Started = [Diagnostics.Stopwatch]::StartNew()
    Write-Log ([string][char]0x2192 + ' ' + $Id + " (срок $Budget с)")
    Write-QaPulse
}
function Exit-QaItem {
    $q = $script:Qa
    if (-not $q.Item) { return }
    $sec = 0.0
    if ($q.Started) { $sec = $q.Started.Elapsed.TotalSeconds }
    $res = 'без итога'
    if ($q.Status.ContainsKey($q.Item)) { $res = $q.Status[$q.Item] }
    Write-Log ([string][char]0x2190 + ' ' + $q.Item + ': ' + $res + ' за ' + [math]::Round($sec, 1) + ' с')
    $q.Item = ''; $q.Step = ''; $q.Seq++; $q.Deadline = 0.0
    Write-QaPulse
}
# Что собираюсь делать: вызывать перед каждым блокирующим действием (щелчок, запуск, ожидание окна)
function Step-Qa([Parameter(Mandatory)][string]$What) {
    $q = $script:Qa
    $q.Step = $What; $q.Seq++
    Write-Log ('  . ' + $What)
    Write-QaPulse
}

# Решение сторожа по пульсу (чистая функция, проверяет test_winqa.ps1). Состояние $State: HangAt, HangId, HangSeq, Block.
# Возвращает Action = none | hang | abort и Reason. hang - срок вышел (снимок, провал, отмена окон), abort - ещё $StallSec без движения
function Get-QaWatchAction {
    param($Pulse, [double]$Now, [hashtable]$State, [double]$BlockStart, [int]$BlockSec = 1800, [int]$StallSec = 60)
    $id = ''; $seq = -1; $deadline = 0.0; $step = ''
    if ($Pulse) { $id = [string]$Pulse.id; $seq = [long]$Pulse.seq; $deadline = [double]$Pulse.deadline; $step = [string]$Pulse.step }
    if ($State.HangAt -gt 0) {
        if ($State.Block -or ($State.HangId -eq $id -and $State.HangSeq -eq $seq)) {
            if (($Now - $State.HangAt) -ge $StallSec) { return [pscustomobject]@{ Action = 'abort'; Reason = ''; Id = $State.HangId; Step = $State.HangStep } }
            return [pscustomobject]@{ Action = 'none'; Reason = ''; Id = $id; Step = $step }
        }
        $State.HangAt = 0   # пункт сдвинулся с места: не завис
    }
    if (($Now - $BlockStart) -ge $BlockSec) {
        return [pscustomobject]@{ Action = 'hang'; Reason = 'block'; Id = $id; Step = $step }
    }
    if ($id -and $deadline -gt 0 -and $Now -gt $deadline) {
        return [pscustomobject]@{ Action = 'hang'; Reason = 'item'; Id = $id; Step = $step }
    }
    return [pscustomobject]@{ Action = 'none'; Reason = ''; Id = $id; Step = $step }
}

# Окно проверки безопасности Проводника (AES: "Open File - Security Warning") - то, что вешало сценарий. $null - нет
function Find-QaAesWindow {
    foreach ($w in (Get-UiaWindows)) {
        $n = ''
        try { $n = [string]$w.Current.Name } catch { continue }
        if ($n -match 'Security Warning|предупреждение системы безопасности') { return $w }
    }
    return $null
}
# Кнопка в окне по любому из имён: результат как у UIA-поиска (Name, Rect, Element) или $null
function Find-QaWindowButton($Win, [string[]]$Names) {
    $auto = [System.Windows.Automation.AutomationElement]
    $ic = [System.Windows.Automation.PropertyConditionFlags]::IgnoreCase
    $parts = [System.Windows.Automation.Condition[]]@($Names | ForEach-Object { New-Object System.Windows.Automation.PropertyCondition($auto::NameProperty, $_, $ic) })
    $cond = $parts[0]
    if ($parts.Count -gt 1) { $cond = [System.Windows.Automation.OrCondition]::new($parts) }
    try { foreach ($el in $Win.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond)) { $hit = ConvertTo-UiaHit $el; if ($hit) { return $hit } } } catch { }
    return $null
}

# Тело пункта; исключение - ПРОВАЛ этого пункта, задание идёт дальше
function Invoke-QaItem {
    param([Parameter(Mandatory)][string]$Id, [Parameter(Mandatory)][scriptblock]$Body, [int]$Budget = 180)
    Enter-QaItem -Id $Id -Budget $Budget
    try { & $Body } catch {
        Add-QaResult -Id $Id -Status fail -Detail ('исключение: ' + $_.Exception.Message)
        Write-Log ("{0}: {1}" -f $Id, $_.ScriptStackTrace)
    } finally { Exit-QaItem }
}

# ---------- журнал программы ----------

function Get-LogMark([string]$Path) {
    if (Test-Path -LiteralPath $Path) { return [long](Get-Item -LiteralPath $Path).Length }
    return [long]0
}

# log.txt от отметки (длина в байтах до шага); файл открыт программой на запись - читаем с общим доступом.
# Отметка может попасть внутрь многобайтного знака: начальные байты-продолжения отбрасываем
function Read-LogSince {
    param([string]$Path, [long]$Mark = 0)
    if (-not (Test-Path -LiteralPath $Path)) { return '' }
    $fs = New-Object System.IO.FileStream($Path, 'Open', 'Read', 'ReadWrite')
    try {
        if ($Mark -gt $fs.Length) { $Mark = 0 }  # param-reassign-ok: журнал усечён - читаем с начала
        [void]$fs.Seek($Mark, 'Begin')
        $ms = New-Object System.IO.MemoryStream
        $fs.CopyTo($ms)
        $bytes = $ms.ToArray()
    } finally { $fs.Dispose() }
    $i = 0
    if ($Mark -gt 0) { while ($i -lt $bytes.Length -and (($bytes[$i] -band 0xC0) -eq 0x80)) { $i++ } }
    $text = $script:Utf8.GetString($bytes, $i, $bytes.Length - $i)
    return $text.TrimStart([char]0xFEFF)
}

# Приватность и паника: нет "ПАНИКА", адресов http(s), путей C:\Users\ и имени пользователя
function Test-LogClean {
    param([string]$Text, [string]$UserName = $env:USERNAME)
    $why = New-Object System.Collections.ArrayList
    if ($Text -cmatch 'ПАНИКА') { [void]$why.Add('ПАНИКА') }
    if ($Text -match 'https?://') { [void]$why.Add('адрес http(s)') }
    if ($Text -match '(?i)\b[A-Z]:\\Users\\') { [void]$why.Add('путь с \Users\') }
    if ($UserName -and $UserName.Length -ge 3 -and $Text.IndexOf($UserName, [StringComparison]::OrdinalIgnoreCase) -ge 0) { [void]$why.Add('имя пользователя') }
    return [pscustomobject]@{ Ok = ($why.Count -eq 0); Why = ($why -join '; ') }
}

# ---------- реестр, набор ----------

function Get-Reg([string]$Key, [string]$Name = '') {
    $k = Get-Item -LiteralPath "HKCU:\$Key" -ErrorAction SilentlyContinue
    if (-not $k) { return $null }
    return $k.GetValue($Name)
}
function Test-RegKey([string]$Key) { return [bool](Test-Path -LiteralPath "HKCU:\$Key") }

# Манифест установщика пишется в ресурсы открытым XML: ищем строку в байтах
function Get-ManifestLevel([string]$Path) {
    $text = [System.Text.Encoding]::GetEncoding('iso-8859-1').GetString([System.IO.File]::ReadAllBytes($Path))
    $m = [regex]::Match($text, 'requestedExecutionLevel\s+level="([A-Za-z]+)"')
    if ($m.Success) { return $m.Groups[1].Value }
    return ''
}

# Хеши набора из kit-sha256.txt ("хеш  путь", как sha256sum): Ok и список расхождений
function Test-QaKit([string]$Kit) {
    $list = Join-Path $Kit 'kit-sha256.txt'
    if (-not (Test-Path -LiteralPath $list)) { return [pscustomobject]@{ Ok = $false; Why = 'нет kit-sha256.txt' } }
    $bad = New-Object System.Collections.ArrayList
    $n = 0
    foreach ($line in [System.IO.File]::ReadAllLines($list, $script:Utf8)) {
        $m = [regex]::Match($line, '^([0-9a-fA-F]{64}) [ \*](.+)$')
        if (-not $m.Success) { continue }
        $n++
        $f = Join-Path $Kit ($m.Groups[2].Value -replace '/', '\')
        if (-not (Test-Path -LiteralPath $f)) { [void]$bad.Add("нет $($m.Groups[2].Value)"); continue }
        if ((Get-FileHash -LiteralPath $f -Algorithm SHA256).Hash -ne $m.Groups[1].Value.ToUpperInvariant()) { [void]$bad.Add("хеш $($m.Groups[2].Value)") }
    }
    if ($n -eq 0) { [void]$bad.Add('список пуст') }
    return [pscustomobject]@{ Ok = ($bad.Count -eq 0); Why = ($bad -join '; ') }
}

# ---------- пути установленной программы, установщик ----------

function Get-QaPaths {
    $inst = Join-Path $env:LOCALAPPDATA 'Programs\Sosnina Albums'
    $roaming = Join-Path $env:APPDATA 'SosninaAlbums'
    $local = Join-Path $env:LOCALAPPDATA 'SosninaAlbums'
    return @{
        Inst = $inst; Exe = (Join-Path $inst 'SosninaAlbums.exe'); Uninst = (Join-Path $inst 'uninstall.exe')
        Roaming = $roaming; Local = $local; Config = (Join-Path $roaming 'config.json'); Log = (Join-Path $roaming 'log.txt')
        Lnk = (Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Sosnina Albums.lnk')
        DesktopLnk = (Join-Path ([Environment]::GetFolderPath('Desktop')) 'Sosnina Albums.lnk')
        KUninst = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\SosninaAlbums'
        KApp = 'Software\SosninaAlbums'; KProg = 'Software\Classes\SosninaAlbums.Album'; KExt = 'Software\Classes\.salbum'
    }
}

# Запуск установщика без ожидания потомков (Start-Process -Wait ждёт и программу, запущенную /relaunch)
function Start-Setup([string]$Exe, [string]$Arguments = '') {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $Exe
    $psi.Arguments = $Arguments
    $psi.UseShellExecute = $false
    return [System.Diagnostics.Process]::Start($psi)
}
# Код выхода; -1 - не уложился в срок (процесс снят)
function Invoke-Setup([string]$Exe, [string]$Arguments, [int]$TimeoutSec = 180) {
    $p = Start-Setup $Exe $Arguments
    if (-not $p.WaitForExit($TimeoutSec * 1000)) {
        try { & taskkill.exe /PID $p.Id /T /F | Out-Null } catch { }
        return -1
    }
    return $p.ExitCode
}
# Тихое удаление как из "Параметров": uninstall.exe без _?= копирует себя в TEMP и сразу выходит, поэтому ждём исчезновения ключа
function Invoke-QuietUninstall([int]$TimeoutSec = 90) {
    $p = Get-QaPaths
    if (-not (Test-Path -LiteralPath $p.Uninst)) { return $true }
    [void](Start-Setup $p.Uninst '/S')
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt $TimeoutSec) {
        if (-not (Test-RegKey $p.KUninst) -and -not (Test-Path -LiteralPath $p.Exe)) { Start-Sleep -Seconds 2; return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

# ---------- программа: запуск и закрытие по PID ----------

# Без оболочки; переменные стенда - как в start-installed.cmd набора. Возвращает Process
function Start-QaApp {
    param([Parameter(Mandatory)][string]$Exe, [string]$Stand = '', [string]$FeedUrl = '', [string]$KeyFile = '', [string]$Arguments = '')
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $Exe
    $psi.Arguments = $Arguments
    $psi.UseShellExecute = $false
    $psi.WorkingDirectory = Split-Path -Parent $Exe
    if ($Stand) { $psi.EnvironmentVariables['SA_STAND'] = $Stand }
    if ($FeedUrl) { $psi.EnvironmentVariables['SA_UPDATE_URL'] = $FeedUrl }
    if ($KeyFile) { $psi.EnvironmentVariables['SA_UPDATE_KEY'] = ([System.IO.File]::ReadAllText($KeyFile)).Trim() }
    return [System.Diagnostics.Process]::Start($psi)
}

# Процессы программы с данным путём exe
function Get-AppProcs([string]$Exe) {
    return @(Get-Process -Name ([IO.Path]::GetFileNameWithoutExtension($Exe)) -ErrorAction SilentlyContinue | Where-Object {
            try { $_.Path -ieq $Exe } catch { $false }
        })
}

# Мягко (программа успевает записать альбом), затем taskkill по PID вместе с WebView2. Только свои PID, по имени - никогда.
# Возвращает "закрыта мягко" / "закрыта силой" / "уже закрыта"
function Stop-QaApp {
    param([Parameter(Mandatory)][int]$ProcId, [switch]$Hard)
    $p = Get-Process -Id $ProcId -ErrorAction SilentlyContinue
    if (-not $p) { return 'уже закрыта' }
    if (-not $Hard) {
        try { [void]$p.CloseMainWindow() } catch { }
        if ($p.WaitForExit(15000)) { return 'закрыта мягко' }
    }
    & taskkill.exe /PID $ProcId /T /F | Out-Null
    [void]$p.WaitForExit(10000)
    return 'закрыта силой'
}

# ---------- UIA ----------

function Get-QaWindows {
    param([int]$ProcId = 0, [string[]]$ProcNames = @())
    if ($ProcId -gt 0) {
        $auto = [System.Windows.Automation.AutomationElement]
        $cond = New-Object System.Windows.Automation.PropertyCondition($auto::ProcessIdProperty, $ProcId)
        return @($auto::RootElement.FindAll([System.Windows.Automation.TreeScope]::Children, $cond))
    }
    if ($ProcNames.Count) { return @(Get-UiaWindows $ProcNames) }
    return @()
}

# Элемент окна процесса по имени (точному или регулярке) и типу; дерево Chromium строится лениво, поэтому повтор каждые 0,3 с
function Find-AppEl {
    param([int]$ProcId = 0, [string[]]$ProcNames = @(), [string]$Name = '', [string]$NameRegex = '', [string]$Type = '', [double]$Timeout = 10)
    $auto = [System.Windows.Automation.AutomationElement]
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        foreach ($w in (Get-QaWindows -ProcId $ProcId -ProcNames $ProcNames)) {
            $cond = [System.Windows.Automation.Condition]::TrueCondition
            if ($Name) { $cond = New-Object System.Windows.Automation.PropertyCondition($auto::NameProperty, $Name) }
            elseif ($NameRegex -and $Type) {
                # регулярка по имени идёт после FindAll, поэтому сужаем выборку типом: иначе перебираем всё дерево Chromium
                $ct = [System.Windows.Automation.ControlType]::$Type
                $cond = New-Object System.Windows.Automation.PropertyCondition($auto::ControlTypeProperty, $ct)
            }
            $found = $null
            try { $found = $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond) } catch { $script:Qa.FindErr = $_.Exception.Message; continue }
            foreach ($el in $found) {
                try {
                    if ($NameRegex -and ([string]$el.Current.Name) -notmatch $NameRegex) { continue }
                    if ($Type -and $el.Current.ControlType.ProgrammaticName -ne "ControlType.$Type") { continue }
                    if ($el.Current.IsOffscreen) { continue }
                    return $el
                } catch { continue }
            }
        }
        Start-Sleep -Milliseconds 300
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $null
}

# Кнопка с именем $Name внутри карточки, где лежит элемент-якорь (по началу имени): от якоря вверх по родителям (до $Up уровней)
# ищем Button; так «Закрыть» карточки тура не путается с крестиком тоста, стоящим в дереве раньше
function Find-QaButtonNear {
    param([int]$ProcId, [Parameter(Mandatory)][string]$AnchorRegex, [Parameter(Mandatory)][string]$Name, [int]$Up = 3, [double]$Timeout = 10)
    $auto = [System.Windows.Automation.AutomationElement]
    $walker = [System.Windows.Automation.TreeWalker]::ControlViewWalker
    $cond = New-Object System.Windows.Automation.PropertyCondition($auto::NameProperty, $Name)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        $anchor = Find-AppEl -ProcId $ProcId -NameRegex $AnchorRegex -Timeout 1
        $node = $anchor
        for ($i = 0; $anchor -and $i -lt $Up; $i++) {
            try { $node = $walker.GetParent($node) } catch { break }
            if (-not $node) { break }
            try {
                foreach ($el in $node.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond)) {
                    if ($el.Current.ControlType.ProgrammaticName -eq 'ControlType.Button' -and -not $el.Current.IsOffscreen) { return $el }
                }
            } catch { $script:Qa.FindErr = $_.Exception.Message }
        }
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $null
}

function Get-ElRect($El) {
    $r = $El.Current.BoundingRectangle
    return @([int][math]::Round($r.X), [int][math]::Round($r.Y), [int][math]::Round($r.Width), [int][math]::Round($r.Height))
}

# Нажать: InvokePattern, иначе TogglePattern, иначе щелчок мышью по центру. -Mouse - сразу мышью: Invoke на кнопке, после
# которой открывается модальное окно, может не вернуться, пока окно не закрыто
function Invoke-AppEl {
    param([Parameter(Mandatory)]$El, [switch]$Mouse)
    if (-not $Mouse) {
        try {
            $p = $null
            if ($El.TryGetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern, [ref]$p)) { $p.Invoke(); return 'invoke' }
            if ($El.TryGetCurrentPattern([System.Windows.Automation.TogglePattern]::Pattern, [ref]$p)) { $p.Toggle(); return 'toggle' }
        } catch { Write-Log "UIA pattern: $($_.Exception.Message)" }
    }
    $r = Get-ElRect $El
    Move-Cursor ([int]($r[0] + $r[2] / 2)) ([int]($r[1] + $r[3] / 2)) 0.2 8
    Invoke-MouseClick 'click'
    return 'mouse'
}

# Имена всех Text-элементов окон процесса (для детали пункта: что написано на странице)
function Get-QaTexts {
    param([int]$ProcId = 0, [string[]]$ProcNames = @())
    $auto = [System.Windows.Automation.AutomationElement]
    $cond = New-Object System.Windows.Automation.PropertyCondition($auto::ControlTypeProperty, [System.Windows.Automation.ControlType]::Text)
    $names = New-Object System.Collections.ArrayList
    foreach ($w in (Get-QaWindows -ProcId $ProcId -ProcNames $ProcNames)) {
        try { [void]$names.Add([string]$w.Current.Name) } catch { }
        try { foreach ($el in $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond)) { [void]$names.Add([string]$el.Current.Name) } } catch { $script:Qa.FindErr = $_.Exception.Message }
    }
    return @($names | Where-Object { $_ })
}

# Диагностика, когда UIA не видит кнопок окна: последнее исключение FindAll, отзывчивость процесса, состояние окон, дерево UIA в logs/
function Write-QaUiaDiag {
    param([string[]]$ProcNames = @(), [int]$ProcId = 0, [string]$Tag = 'diag')
    Write-Log "UIA-диагностика ($Tag): последнее исключение FindAll: '$($script:Qa.FindErr)'"
    $ids = New-Object System.Collections.ArrayList
    if ($ProcId -gt 0) { [void]$ids.Add($ProcId) }
    foreach ($pn in $ProcNames) { foreach ($pp in @(Get-Process -Name $pn -ErrorAction SilentlyContinue)) { [void]$ids.Add($pp.Id) } }
    foreach ($pid1 in $ids) {
        $resp = '?'
        try { $resp = (Get-Process -Id $pid1 -ErrorAction Stop).Responding } catch { $resp = 'нет процесса' }
        Write-Log "UIA-диагностика ($Tag): PID $pid1 Responding=$resp"
        foreach ($w in (Get-QaWindows -ProcId $pid1)) {
            try { Write-Log ("  окно '{0}': IsEnabled={1} IsOffscreen={2} rect={3}" -f $w.Current.Name, $w.Current.IsEnabled, $w.Current.IsOffscreen, ((Get-ElRect $w) -join ',')) } catch { Write-Log "  окно: $($_.Exception.Message)" }
        }
        try {
            $tree = Join-Path $script:Qa.Out ("logs\uia-{0}-{1}.txt" -f (ConvertTo-AsciiId $Tag), $pid1)
            $stat = Export-QaUiaTree -ProcId $pid1 -Path $tree
            Write-Log "  дерево UIA: узлов $($stat.Nodes), кнопок без имени $($stat.UnnamedButtons) -> $tree"
        } catch { Write-Log "  дерево UIA не записано: $($_.Exception.Message)" }
    }
}

# Дерево UIA окна программы в файл: тип, имя, AutomationId; глубина по умолчанию 8. Возвращает число узлов и число кнопок без имени
function Export-QaUiaTree {
    param([Parameter(Mandatory)][int]$ProcId, [Parameter(Mandatory)][string]$Path, [int]$Depth = 8, [int]$Max = 4000)
    $walker = [System.Windows.Automation.TreeWalker]::ControlViewWalker
    $lines = New-Object System.Collections.ArrayList
    $stat = @{ Nodes = 0; Unnamed = 0 }
    $walk = {
        param($el, $d)
        if ($stat.Nodes -ge $Max) { return }
        $stat.Nodes++
        $type = ''; $name = ''; $aid = ''
        try { $type = $el.Current.ControlType.ProgrammaticName -replace '^ControlType\.', ''; $name = [string]$el.Current.Name; $aid = [string]$el.Current.AutomationId } catch { }
        if ($type -eq 'Button' -and -not $name) { $stat.Unnamed++ }
        [void]$lines.Add(('{0}{1} "{2}" [{3}]' -f ('  ' * $d), $type, $name, $aid))
        if ($d -ge $Depth) { return }
        $c = $null
        try { $c = $walker.GetFirstChild($el) } catch { }
        while ($c) {
            & $walk $c ($d + 1)
            try { $c = $walker.GetNextSibling($c) } catch { $c = $null }
        }
    }
    foreach ($w in (Get-QaWindows -ProcId $ProcId)) { & $walk $w 0 }
    [System.IO.File]::WriteAllLines($Path, [string[]]$lines, $script:Utf8)
    return [pscustomobject]@{ Nodes = $stat.Nodes; UnnamedButtons = $stat.Unnamed }
}

# ---------- снимки и ролики ----------

# Объединение видимых окон процесса (в том числе окон-сообщений) в пределах экрана: @(x,y,w,h) или $null
function Get-QaWinRect {
    param([int]$ProcId = 0, [string[]]$ProcNames = @())
    $x0 = [int]::MaxValue; $y0 = [int]::MaxValue; $x1 = [int]::MinValue; $y1 = [int]::MinValue
    foreach ($w in (Get-QaWindows -ProcId $ProcId -ProcNames $ProcNames)) {
        try {
            if ($w.Current.IsOffscreen) { continue }
            $r = Get-ElRect $w
            if ($r[2] -lt 20 -or $r[3] -lt 20) { continue }
            $x0 = [math]::Min($x0, $r[0]); $y0 = [math]::Min($y0, $r[1])
            $x1 = [math]::Max($x1, $r[0] + $r[2]); $y1 = [math]::Max($y1, $r[1] + $r[3])
        } catch { }
    }
    if ($x1 -le $x0) { return $null }
    $s = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    $x0 = [math]::Max($x0, $s.X); $y0 = [math]::Max($y0, $s.Y)
    $x1 = [math]::Min($x1, $s.X + $s.Width); $y1 = [math]::Min($y1, $s.Y + $s.Height)
    if ($x1 -le $x0 -or $y1 -le $y0) { return $null }
    return @($x0, $y0, ($x1 - $x0), ($y1 - $y0))
}

# Вырез @(x,y,w,h) внутри картинки W x H: сдвиг и усечение, минимум 1 px
function Get-QaClampedCrop([int[]]$Crop, [int]$W, [int]$H) {
    $cx = [math]::Max(0, [math]::Min($Crop[0], $W - 1)); $cy = [math]::Max(0, [math]::Min($Crop[1], $H - 1))
    $cw = [math]::Max(1, [math]::Min($Crop[2], $W - $cx)); $ch = [math]::Max(1, [math]::Min($Crop[3], $H - $cy))
    return @($cx, $cy, $cw, $ch)
}

# Ждёт видимое главное окно одного из процессов (UIA-окно не пустое); $true - появилось. Снимок до этого пустой
function Wait-QaAppWindow {
    param([int[]]$ProcIds, [double]$Timeout = 40)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        foreach ($wid in $ProcIds) {
            if (Get-QaWinRect -ProcId $wid) { Start-Sleep -Seconds 2; return $true }   # 2 с: окно успевает отрисоваться
        }
        Start-Sleep -Milliseconds 400
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $false
}

# Закрыть процессы <Name>, которых не было в списке Before (только свои, по PID)
function Stop-QaNewProcs {
    param([Parameter(Mandatory)][string]$Name, [int[]]$Before = @())
    foreach ($np in @(Get-Process -Name $Name -ErrorAction SilentlyContinue | Where-Object { $Before -notcontains $_.Id })) {
        try { & taskkill.exe /PID $np.Id /T /F | Out-Null } catch { }
    }
}

# Снимок окна процесса (нет окна - весь экран) в png/<ASCII-id>-<slug>.png. -Crop x,y,w,h внутри снимка и -Zoom: вырез с увеличением.
# Возвращает относительный путь "png/…" или '' при неудаче
function Save-QaShot {
    param([Parameter(Mandatory)][string]$Id, [Parameter(Mandatory)][string]$Slug, [int]$ProcId = 0, [string[]]$ProcNames = @(), [int[]]$Crop = @(), [double]$Zoom = 1)
    [void](Enable-QaDpiAware)
    $safe = ($Slug -replace '[^A-Za-z0-9_-]', '_')
    $rel = 'png/{0}-{1}.png' -f (ConvertTo-AsciiId $Id), $safe
    $path = Join-Path $script:Qa.Out ($rel -replace '/', '\')
    try {
        $rect = $null
        if ($ProcId -gt 0 -or $ProcNames.Count) { $rect = Get-QaWinRect -ProcId $ProcId -ProcNames $ProcNames }
        if (-not $rect) {
            $s = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
            $rect = @($s.X, $s.Y, $s.Width, $s.Height)
        }
        $bmp = New-Object System.Drawing.Bitmap $rect[2], $rect[3]
        try {
            $g = [System.Drawing.Graphics]::FromImage($bmp)
            try { $g.CopyFromScreen($rect[0], $rect[1], 0, 0, (New-Object System.Drawing.Size $rect[2], $rect[3])) } finally { $g.Dispose() }
            $out = $bmp
            if ($Crop.Count -eq 4) {
                # вырез в границах снимка: на нестандартном масштабе окно может быть меньше расчётного
                $cr = Get-QaClampedCrop $Crop $bmp.Width $bmp.Height
                $dw = [int]($cr[2] * $Zoom); $dh = [int]($cr[3] * $Zoom)
                $out = New-Object System.Drawing.Bitmap $dw, $dh
                $g2 = [System.Drawing.Graphics]::FromImage($out)
                try {
                    $g2.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
                    $g2.DrawImage($bmp, (New-Object System.Drawing.Rectangle 0, 0, $dw, $dh), (New-Object System.Drawing.Rectangle $cr[0], $cr[1], $cr[2], $cr[3]), [System.Drawing.GraphicsUnit]::Pixel)
                } finally { $g2.Dispose() }
            }
            $out.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
            if ($out -ne $bmp) { $out.Dispose() }
        } finally { $bmp.Dispose() }
        return $rel
    } catch {
        Write-Log "снимок $Id/${Slug}: $($_.Exception.Message)"
        return ''
    }
}

# Ролик на отрезке пункта, 5 кадров/с; без -Video у Initialize-Qa ничего не делает
function Start-QaVideo([string]$Id) {
    if (-not $script:Qa.Video) { return }
    $file = Join-Path $script:Qa.Out ('video\' + (ConvertTo-AsciiId $Id) + '.mp4')
    foreach ($m in 'gdigrab', 'frames') {
        try { $script:Qa.Rec = Start-Rec $file $m 5; $script:Qa.RecFile = $file; return } catch { Write-Log "запись ($m): $($_.Exception.Message)" }
    }
}
# Путь "video/<id>.mp4" или ''
function Stop-QaVideo {
    if (-not $script:Qa.Rec) { return '' }
    $rec = $script:Qa.Rec; $file = $script:Qa.RecFile
    $script:Qa.Rec = $null
    $ok = $false
    try { $ok = Stop-Rec $rec } catch { Write-Log "остановка записи: $($_.Exception.Message)" }
    if ($ok) { return ('video/' + [IO.Path]::GetFileName($file)) }
    return ''
}
