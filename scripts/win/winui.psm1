# winui.psm1 - экран, курсор, ввод, UIA и OCR для scripts/win (спека 036, design §5.5).
# Всё, что трогает рабочий стол; чистая логика и журнал - в winlib.psm1. Только Windows PowerShell 5.1,
# файл в UTF-8 с BOM. Запускать на одноразовом раннере: меняется разрешение, обои и курсор.

Import-Module (Join-Path $PSScriptRoot 'winlib.psm1') -DisableNameChecking
Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

if (-not ('SaWin.Native' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
namespace SaWin {
    // DEVMODEW целиком: 220 байт (Unicode-строки по 32 символа); dmSize перед вызовом ставим Marshal.SizeOf, а Windows возвращает свой (188)
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct DEVMODE {
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)] public string dmDeviceName;
        public short dmSpecVersion; public short dmDriverVersion; public short dmSize; public short dmDriverExtra;
        public int dmFields;
        public int dmPositionX; public int dmPositionY; public int dmDisplayOrientation; public int dmDisplayFixedOutput;
        public short dmColor; public short dmDuplex; public short dmYResolution; public short dmTTOption; public short dmCollate;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)] public string dmFormName;
        public short dmLogPixels;
        public int dmBitsPerPel; public int dmPelsWidth; public int dmPelsHeight; public int dmDisplayFlags; public int dmDisplayFrequency;
        public int dmICMMethod; public int dmICMIntent; public int dmMediaType; public int dmDitherType;
        public int dmReserved1; public int dmReserved2; public int dmPanningWidth; public int dmPanningHeight;
    }
    [StructLayout(LayoutKind.Sequential)]
    public struct POINT { public int X; public int Y; }
    public static class Native {
        [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
        [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
        [DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT p);
        [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
        [DllImport("user32.dll")] public static extern void keybd_event(byte vk, byte scan, uint flags, UIntPtr extra);
        [DllImport("user32.dll", CharSet = CharSet.Unicode, EntryPoint = "EnumDisplaySettingsW")] public static extern bool EnumDisplaySettings(string dev, int mode, ref DEVMODE dm);
        [DllImport("user32.dll", CharSet = CharSet.Unicode, EntryPoint = "ChangeDisplaySettingsW")] public static extern int ChangeDisplaySettings(ref DEVMODE dm, int flags);
        [DllImport("user32.dll")] public static extern bool SetSysColors(int n, int[] elements, int[] rgb);
        [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern bool SystemParametersInfo(uint action, uint param, string data, uint flags);
        [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);

        // Режим экрана читаем и меняем внутри C#: из PowerShell вызов с [ref] на структуру возвращает False и нули
        static DEVMODE Fresh() {
            var dm = new DEVMODE();
            dm.dmSize = (short)Marshal.SizeOf(typeof(DEVMODE));
            return dm;
        }
        // Текущий режим основного экрана; при неудаче поля нулевые (dmPelsWidth == 0)
        public static DEVMODE Current() {
            var dm = Fresh();
            if (!EnumDisplaySettings(null, -1, ref dm)) return Fresh();
            return dm;
        }
        // Ставит разрешение w x h (freq > 0 - ещё и частоту). 0 - применено; иначе код ChangeDisplaySettings
        // (CDS_TEST отказал - ничего не менялось) или -1, если текущий режим не прочитан
        public static int SetMode(int w, int h, int freq) {
            var dm = Fresh();
            if (!EnumDisplaySettings(null, -1, ref dm)) return -1;
            dm.dmPelsWidth = w; dm.dmPelsHeight = h;
            dm.dmFields = 0x80000 | 0x100000;   // DM_PELSWIDTH | DM_PELSHEIGHT
            if (freq > 0) { dm.dmDisplayFrequency = freq; dm.dmFields |= 0x400000; }   // DM_DISPLAYFREQUENCY
            int r = ChangeDisplaySettings(ref dm, 2);   // CDS_TEST
            if (r != 0) return r;
            return ChangeDisplaySettings(ref dm, 0);
        }
    }
}
'@
}

$script:VK = @{ Enter = 0x0D; Esc = 0x1B; Tab = 0x09; Win = 0x5B; Alt = 0x12; Ctrl = 0x11; Shift = 0x10; D = 0x44; End = 0x23; L = 0x4C; V = 0x56 }

# Физические пиксели везде (снимки, UIA, SetCursorPos): без этого при масштабе 125 % координаты разъедутся
function Initialize-Dpi { [void][SaWin.Native]::SetProcessDPIAware() }

# ---------- экран ----------

function Get-ScreenSize {
    $b = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    return @($b.Width, $b.Height)
}

function Get-CurrentMode { return [SaWin.Native]::Current() }

# Первое из разрешений, которое принимает драйвер (CDS_TEST), - применить. Возвращает строку «ШxВ» или $null
function Set-ScreenResolution([string[]]$Candidates = @('1920x1080', '1680x1050', '1280x1024', '1280x720')) {
    foreach ($c in $Candidates) {
        $w, $h = $c -split 'x' | ForEach-Object { [int]$_ }
        if ([SaWin.Native]::SetMode($w, $h, 0) -eq 0) {
            Start-Sleep -Milliseconds 1500
            return $c
        }
    }
    return $null
}

# Однотонный стол: без картинки обоев и с ровным цветом, чтобы стрелки и рамки читались
function Set-SolidDesktop {
    try { [void][SaWin.Native]::SystemParametersInfo(0x14, 0, '', 3) } catch { }   # SPI_SETDESKWALLPAPER, пустой путь
    try { [void][SaWin.Native]::SetSysColors(1, @(1), @((96 + 112 * 256 + 128 * 65536))) } catch { }   # COLOR_BACKGROUND
}

function Save-Screen([string]$Path) {
    $dir = Split-Path -Parent $Path
    if ($dir -and -not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $b = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    $bmp = New-Object System.Drawing.Bitmap $b.Width, $b.Height
    try {
        $g = [System.Drawing.Graphics]::FromImage($bmp)
        try { $g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size) } finally { $g.Dispose() }
        $bmp.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)
    } finally { $bmp.Dispose() }
}

# Метка синхронизации: окно во весь экран поверх всех. OnShown вызывается, когда цвет уже на экране
# (так t в журнале не опережает кадр на время создания окна).
function Show-Flash([string]$Color = '#d81f26', [double]$Dur = 0.8, [scriptblock]$OnShown = $null) {
    $f = New-Object System.Windows.Forms.Form
    try {
        $f.FormBorderStyle = 'None'
        $f.StartPosition = 'Manual'
        $f.Bounds = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
        $f.TopMost = $true
        $f.ShowInTaskbar = $false
        $f.BackColor = [System.Drawing.ColorTranslator]::FromHtml($Color)
        $f.Show()
        [System.Windows.Forms.Application]::DoEvents()
        if ($OnShown) { & $OnShown }
        $sw = [Diagnostics.Stopwatch]::StartNew()
        while ($sw.Elapsed.TotalSeconds -lt $Dur) {
            [System.Windows.Forms.Application]::DoEvents()
            Start-Sleep -Milliseconds 15
        }
    } finally { $f.Close(); $f.Dispose() }
}

# ---------- курсор и ввод ----------

function Move-Cursor([int]$X, [int]$Y, [double]$Dur = 0.6, [int]$Steps = 30) {
    $p = New-Object SaWin.POINT
    [void][SaWin.Native]::GetCursorPos([ref]$p)
    $x0 = $p.X; $y0 = $p.Y
    for ($i = 1; $i -le $Steps; $i++) {
        $s = $i / $Steps
        $e = $s * $s * (3 - 2 * $s)   # плавный разгон и торможение
        [void][SaWin.Native]::SetCursorPos([int]($x0 + ($X - $x0) * $e), [int]($y0 + ($Y - $y0) * $e))
        Start-Sleep -Milliseconds ([int](1000 * $Dur / $Steps))
    }
    [void][SaWin.Native]::SetCursorPos($X, $Y)
}

function Invoke-MouseClick([string]$Mode = 'click') {
    $down = 0x2; $up = 0x4   # левая кнопка
    if ($Mode -eq 'right') { $down = 0x8; $up = 0x10 }
    $n = 1
    if ($Mode -eq 'dbl') { $n = 2 }
    for ($i = 0; $i -lt $n; $i++) {
        [SaWin.Native]::mouse_event($down, 0, 0, 0, [UIntPtr]::Zero)
        Start-Sleep -Milliseconds 60
        [SaWin.Native]::mouse_event($up, 0, 0, 0, [UIntPtr]::Zero)
        Start-Sleep -Milliseconds 90
    }
}

# Колесо: Notches > 0 - вверх, < 0 - вниз
function Invoke-Wheel([int]$Notches) {
    $delta = [BitConverter]::ToUInt32([BitConverter]::GetBytes([int](120 * $Notches)), 0)
    [SaWin.Native]::mouse_event(0x800, 0, 0, $delta, [UIntPtr]::Zero)
}

# Сочетание: нажать по порядку, отпустить в обратном. Имена - ключи $script:VK или один знак A-Z
function Send-Keys([string[]]$Keys) {
    $codes = @($Keys | ForEach-Object {
            if ($script:VK.ContainsKey($_)) { [byte]$script:VK[$_] } else { [byte][char]$_.ToUpperInvariant() }
        })
    foreach ($c in $codes) { [SaWin.Native]::keybd_event($c, 0, 0, [UIntPtr]::Zero); Start-Sleep -Milliseconds 40 }
    [array]::Reverse($codes)
    foreach ($c in $codes) { [SaWin.Native]::keybd_event($c, 0, 2, [UIntPtr]::Zero); Start-Sleep -Milliseconds 40 }
}

# ---------- UIA ----------

function Get-UiaWindows([string[]]$ProcessNames, [string[]]$Skip = @()) {
    $ids = $null
    if ($ProcessNames -and $ProcessNames.Count) {
        $ids = @(Get-Process -Name $ProcessNames -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
    }
    $skipIds = @()
    if ($Skip -and $Skip.Count) { $skipIds = @(Get-Process -Name $Skip -ErrorAction SilentlyContinue | ForEach-Object { $_.Id }) }
    $all = [System.Windows.Automation.AutomationElement]::RootElement.FindAll(
        [System.Windows.Automation.TreeScope]::Children, [System.Windows.Automation.Condition]::TrueCondition)
    foreach ($w in $all) {
        try { $owner = $w.Current.ProcessId } catch { continue }
        if ($null -ne $ids -and $ids -notcontains $owner) { continue }
        if ($skipIds -contains $owner) { continue }
        $w
    }
}

function ConvertTo-UiaHit($El) {
    try {
        $r = $El.Current.BoundingRectangle
        if ($r.IsEmpty -or [double]::IsInfinity($r.X) -or $r.Width -lt 2 -or $r.Height -lt 2) { return $null }
        if ($El.Current.IsOffscreen) { return $null }
        return [pscustomobject]@{
            Name = [string]$El.Current.Name
            Rect = @([int][math]::Round($r.X), [int][math]::Round($r.Y), [int][math]::Round($r.Width), [int][math]::Round($r.Height))
            Source = 'uia'
            Element = $El
        }
    } catch { return $null }
}

# Один проход: элемент с одним из имён (без учёта регистра) в окнах нужных процессов. Либо ControlType + регулярка по имени
# -All - все видимые совпадения списком (приоритет выбирает вызывающий), -Skip - процессы, которые не смотреть
function Find-UiaOnce([string[]]$Names = @(), [string[]]$Procs = @(), [string]$NameRegex = '', [string]$Type = '', [string[]]$Skip = @(), [switch]$All) {
    $hits = New-Object System.Collections.ArrayList
    $auto = [System.Windows.Automation.AutomationElement]
    $ic = [System.Windows.Automation.PropertyConditionFlags]::IgnoreCase
    $cond = $null
    if ($Names.Count -eq 1) {
        $cond = New-Object System.Windows.Automation.PropertyCondition($auto::NameProperty, $Names[0], $ic)
    } elseif ($Names.Count -gt 1) {
        $parts = [System.Windows.Automation.Condition[]]@($Names | ForEach-Object {
                New-Object System.Windows.Automation.PropertyCondition($auto::NameProperty, $_, $ic) })
        $cond = [System.Windows.Automation.OrCondition]::new($parts)
    } elseif ($Type) {
        $ct = [System.Windows.Automation.ControlType]::$Type
        $cond = New-Object System.Windows.Automation.PropertyCondition($auto::ControlTypeProperty, $ct)
    } else { return $null }
    foreach ($w in (Get-UiaWindows $Procs $Skip)) {
        if ($Names.Count -and ($Names -contains [string]$w.Current.Name)) {
            $hit = ConvertTo-UiaHit $w
            if ($hit) { if ($All) { [void]$hits.Add($hit) } else { return $hit } }
        }
        $found = $null
        try { $found = $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond) } catch { continue }
        foreach ($el in $found) {
            if ($NameRegex -and ([string]$el.Current.Name) -notmatch $NameRegex) { continue }
            $hit = ConvertTo-UiaHit $el
            if ($hit) { if ($All) { [void]$hits.Add($hit) } else { return $hit } }
        }
    }
    if ($All) { return ,@($hits) }
    return $null
}

# ---------- OCR (Windows.Media.Ocr, запасной путь для целей без UIA) ----------

function Initialize-Ocr {
    if ($script:OcrReady) { return }
    Add-Type -AssemblyName System.Runtime.WindowsRuntime
    $null = [Windows.Storage.StorageFile, Windows.Storage, ContentType = WindowsRuntime]
    $null = [Windows.Storage.FileAccessMode, Windows.Storage, ContentType = WindowsRuntime]
    $null = [Windows.Storage.Streams.IRandomAccessStream, Windows.Storage.Streams, ContentType = WindowsRuntime]
    $null = [Windows.Graphics.Imaging.BitmapDecoder, Windows.Graphics.Imaging, ContentType = WindowsRuntime]
    $null = [Windows.Graphics.Imaging.SoftwareBitmap, Windows.Graphics.Imaging, ContentType = WindowsRuntime]
    $null = [Windows.Media.Ocr.OcrEngine, Windows.Media.Ocr, ContentType = WindowsRuntime]
    $null = [Windows.Media.Ocr.OcrResult, Windows.Media.Ocr, ContentType = WindowsRuntime]
    $null = [Windows.Globalization.Language, Windows.Globalization, ContentType = WindowsRuntime]
    $script:AsTask = [System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
        $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1'
    } | Select-Object -First 1
    $script:OcrReady = $true
}

function Wait-Async($Op, [Type]$Result) {
    $task = $script:AsTask.MakeGenericMethod($Result).Invoke($null, @($Op))
    if (-not $task.Wait(30000)) { throw 'WinRT: операция не завершилась за 30 с' }
    return $task.Result
}

# Языки OCR, доступные в системе (для summary.md)
function Get-OcrLanguages {
    try {
        Initialize-Ocr
        return @([Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages | ForEach-Object { $_.LanguageTag })
    } catch { return @() }
}

# Слова снимка: объекты Text, X, Y, W, H. Движок: en-US, затем языки профиля
function Get-OcrWords([string]$Png, [string]$Lang = 'en-US') {
    Initialize-Ocr
    $engine = $null
    try { $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage([Windows.Globalization.Language]::new($Lang)) } catch { }
    if (-not $engine) { $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages() }
    if (-not $engine) { throw 'OCR: нет ни одного языка распознавания' }
    $file = Wait-Async ([Windows.Storage.StorageFile]::GetFileFromPathAsync($Png)) ([Windows.Storage.StorageFile])
    $stream = Wait-Async ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
    try {
        $dec = Wait-Async ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
        $bmp = Wait-Async ($dec.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])
        $res = Wait-Async ($engine.RecognizeAsync($bmp)) ([Windows.Media.Ocr.OcrResult])
    } finally { $stream.Dispose() }
    $out = New-Object System.Collections.ArrayList
    foreach ($line in $res.Lines) {
        foreach ($w in $line.Words) {
            $r = $w.BoundingRect
            [void]$out.Add([pscustomobject]@{ Text = $w.Text; X = $r.X; Y = $r.Y; W = $r.Width; H = $r.Height })
        }
    }
    return @($out)
}

# Первая из фраз, найденная на экране сейчас; результат как у UIA
function Find-OcrPhrase([string[]]$Phrases, [string]$TmpPng) {
    try {
        Save-Screen $TmpPng
        $words = Get-OcrWords $TmpPng
        foreach ($p in $Phrases) {
            $r = Find-PhraseRect $words $p
            if ($r) { return [pscustomobject]@{ Name = $p; Rect = $r; Source = 'ocr'; Element = $null } }
        }
    } catch { Write-Log "OCR: $($_.Exception.Message)" }
    return $null
}

# Ждёт цель: UIA каждые 0.7 с, OCR - раз в три прохода (снимок и распознавание ~1 с). $null - не дождались
function Wait-Target([string[]]$Names, [string[]]$Procs = @(), [double]$Timeout = 10, [string[]]$Ocr = @(),
    [string]$NameRegex = '', [string]$Type = '', [string]$Tmp = '', [string[]]$Skip = @()) {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    $i = 0
    do {
        $hit = Find-UiaOnce -Names $Names -Procs $Procs -NameRegex $NameRegex -Type $Type -Skip $Skip
        if ($hit) { return $hit }
        if ($Ocr.Count -and $Tmp -and ($i % 3) -eq 2) {
            $hit = Find-OcrPhrase $Ocr $Tmp
            if ($hit) { return $hit }
        }
        $i++
        Start-Sleep -Milliseconds 700
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $null
}
