# winqa-open.psm1 - помощники блока open (спека 118, design §3, §5): конфиг и реестр связи .salbum, двойной щелчок по альбому
# в Проводнике, меню программы, окна Win32 (свернуть, передний план), системный выбор папки, тексты окна «Альбом из новой версии».
# Чистые функции (New-QaOpenConfig, Get-QaSeedConfig, Test-QaAssocCommand, Get-QaSpreadCount, Get-QaNewerLeadRegex, Get-QaNewerNextRegex,
# Test-QaTourLog) проверяет test_winqa.ps1; остальное видно только на раннере. Windows PowerShell 5.1, файл в UTF-8 с BOM.

Import-Module (Join-Path $PSScriptRoot 'winqa-update.psm1') -DisableNameChecking
Add-Type -AssemblyName WindowsBase

if (-not ('SaQa.Win' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
namespace SaQa {
    public static class Win {
        [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
        [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
        [DllImport("user32.dll")] static extern bool ShowWindow(IntPtr h, int cmd);
        [DllImport("user32.dll")] static extern bool IsIconic(IntPtr h);
        // PID процесса, чьё окно сейчас на переднем плане
        public static int ForegroundPid() { uint p; GetWindowThreadProcessId(GetForegroundWindow(), out p); return (int)p; }
        // SW_MINIMIZE = 6
        public static bool Minimize(IntPtr h) { return ShowWindow(h, 6); }
        public static bool Iconic(IntPtr h) { return IsIconic(h); }
    }
}
'@
}

# ---------- чистые функции ----------

# Начальный config.json фазы А: только выключена проверка обновлений (rel ходила бы на настоящий GitHub: адрес в журнале и сеть
# на раннере портят А14). Остальное пусто - первый запуск настоящий: стартовое окно, вопрос о связи, тур
function Get-QaSeedConfig { return '{"update":{"off":true}}' }

# config.json фаз Б и Д: полный уровень, без вопроса о связи (Assoc: on - связь обновляется сама, off - не трогает), туры закрыты.
# LastProject - альбом, на котором откроется программа; StartDialog - показывать ли стартовое окно (Д1 проверяет окно поверх него)
function New-QaOpenConfig {
    param([string]$LastProject = '', [string]$Assoc = 'off', [bool]$StartDialog = $false)
    $tours = @('first', 'folders', 'library', 'autobuild', 'timeline', 'layouts', 'photo', 'export') | ForEach-Object { '"' + $_ + '":"done"' }
    $last = ''
    if ($LastProject) { $last = '"lastProject":' + (ConvertTo-JString $LastProject) + ',' }
    $hide = ''
    if (-not $StartDialog) { $hide = '"hideStart":true,' }
    return '{' + $last + '"level":"full",' + $hide + '"assoc":"' + $Assoc + '","tours":{' + ($tours -join ',') + '},"update":{"off":true}}'
}

# Команда открытия связанного .salbum ведёт на данный exe: "<exe>" "%1" (регистр пути не важен)
function Test-QaAssocCommand([string]$Command, [string]$Exe) {
    if (-not $Command -or -not $Exe) { return $false }
    return [bool]($Command.Trim() -imatch ('^"?' + [regex]::Escape($Exe) + '"?\s+"?%1"?$'))
}

# Число разворотов в .salbum (JSON с массивом spreads); -1 - файл не читается
function Get-QaSpreadCount([string]$AlbumPath) {
    try {
        $j = [System.IO.File]::ReadAllText($AlbumPath) | ConvertFrom-Json
        if ($null -eq $j.spreads) { return 0 }
        return @($j.spreads).Count
    } catch { return -1 }
}

# Тексты окна «Альбом из новой версии» (frontend/src/shell/ask/newerask.ts): первая строка. Need - версия из файла, Mine - эта программа;
# Need пусто - версия в файле неизвестна. Имя альбома в «ёлочках» любое (с .salbum или без)
function Get-QaNewerLeadRegex([string]$Need, [string]$Mine = '') {
    if (-not $Need) { return '^«[^»]+» сохранён более новой версией программы\.$' }
    $m = ''
    if ($Mine) { $m = ', а эта программа — ' + [regex]::Escape($Mine) }
    return '^«[^»]+» сохранён в Sosnina Albums ' + [regex]::Escape($Need) + $m + '\. Открыть его может версия ' + [regex]::Escape($Need) + ' или новее\.$'
}
# Версии в тексте окна «Альбом из новой версии» (internal/app/newer.go shownVers): major.minor;
# если они совпали и полные номера есть - полные. Возвращает объект Need/Mine
function Get-QaShownVers([string]$Need, [string]$Mine) {
    $s = @{}
    $f = @{}
    foreach ($k in 'Need', 'Mine') {
        $v = if ($k -eq 'Need') { $Need } else { $Mine }
        $s[$k] = ''
        $f[$k] = ''
        $m = [regex]::Match($v, '^(\d+)\.(\d+)')
        if ($m.Success) { $s[$k] = $m.Groups[1].Value + '.' + $m.Groups[2].Value }
        $m3 = [regex]::Match($v, '^(\d+)\.(\d+)\.(\d+)')
        if ($m3.Success) { $f[$k] = $m3.Value }
    }
    if ($s.Need -and $s.Need -ceq $s.Mine -and $f.Need -and $f.Mine) {
        return [pscustomobject]@{ Need = $f.Need; Mine = $f.Mine }
    }
    return [pscustomobject]@{ Need = $s.Need; Mine = $s.Mine }
}
# Вторая строка: версия, которая уже есть на компьютере (Have), или призыв скачать
function Get-QaNewerNextRegex([string]$Have) {
    if (-not $Have) { return '^Скачайте новую версию — альбомы и настройки сохранятся\.$' }
    $h = [regex]::Escape($Have)
    return '^Версия ' + $h + ' уже есть на компьютере\. Эта программа закроется, альбом откроется в ' + $h + '\.$'
}

# «тур закрыт: …, шаг N» в журнале (А13): номер шага или 0
function Test-QaTourLog([string]$LogText) {
    $m = [regex]::Match($LogText, 'тур закрыт: [^\r\n]*шаг (\d+)')
    if ($m.Success) { return [int]$m.Groups[1].Value }
    return 0
}

# ---------- окна Win32 ----------

function Get-QaMainHandle([int]$ProcId) {
    $p = Get-Process -Id $ProcId -ErrorAction SilentlyContinue
    if (-not $p) { return [IntPtr]::Zero }
    $p.Refresh()
    return $p.MainWindowHandle
}
function Test-QaIconic([int]$ProcId) { return [SaQa.Win]::Iconic((Get-QaMainHandle $ProcId)) }
function Set-QaMinimized([int]$ProcId) { return [SaQa.Win]::Minimize((Get-QaMainHandle $ProcId)) }
function Get-QaForegroundPid { return [SaQa.Win]::ForegroundPid() }

# Заголовок окна программы (первого окна процесса)
function Get-QaWinTitle([int]$ProcId) {
    $w = @(Get-QaWindows -ProcId $ProcId) | Select-Object -First 1
    if ($w) { try { return [string]$w.Current.Name } catch { } }
    return ''
}

# Элемент виден в точке своего центра, а не закрыт другим окном: UIA-элемент под курсором - он сам или его потомок/предок
function Test-QaOnTop($El) {
    try {
        $r = $El.Current.BoundingRectangle
        $pt = New-Object System.Windows.Point(($r.X + $r.Width / 2), ($r.Y + $r.Height / 2))
        $hit = [System.Windows.Automation.AutomationElement]::FromPoint($pt)
        $walker = [System.Windows.Automation.TreeWalker]::ControlViewWalker
        $name = [string]$El.Current.Name
        $cur = $hit
        for ($k = 0; $k -lt 4 -and $cur; $k++) {
            if ([string]$cur.Current.Name -ceq $name) { return $true }
            $cur = $walker.GetParent($cur)
        }
    } catch { }
    return $false
}

# ---------- меню программы ----------

# Пункт меню или кнопка строки меню по регулярке имени (имя может нести значок и сочетание клавиш)
function Find-QaMenuEl([int]$ProcId, [string]$Regex, [double]$Timeout = 8) {
    $el = Find-AppEl -ProcId $ProcId -NameRegex $Regex -Type 'MenuItem' -Timeout $Timeout
    if (-not $el) { $el = Find-AppEl -ProcId $ProcId -NameRegex $Regex -Type 'Button' -Timeout 2 }
    # пункт-переключатель (menuitemcheckbox) Chromium отдаёт в UIA как CheckBox
    if (-not $el) { $el = Find-AppEl -ProcId $ProcId -NameRegex $Regex -Type 'CheckBox' -Timeout 2 }
    return $el
}
# «Файл ▸ пункт»: раскрыть верхний пункт, щёлкнуть мышью по нужному; сам пункт или $null
function Invoke-QaMenu([int]$ProcId, [string]$Top, [string]$ItemRegex) {
    $topEl = Find-QaMenuEl $ProcId ('^' + $Top) 20
    if (-not $topEl) { return $null }
    [void](Invoke-AppEl -El $topEl)
    $item = Find-QaMenuEl $ProcId $ItemRegex 8
    if (-not $item) { return $null }
    [void](Invoke-AppEl -El $item -Mouse)
    return $item
}
# Окно программы готово к щелчкам: в меню есть «Файл»
function Wait-QaMenu([int]$ProcId, [double]$Timeout = 40) {
    return [bool](Find-QaMenuEl $ProcId '^Файл' $Timeout)
}

# ---------- Проводник, SmartScreen ----------

# Двойной щелчок по альбому в Проводнике (окна Проводника каждый раз заново: окно программы после прошлого щелчка закрывает папку)
function Invoke-QaAlbumDblClick([string]$AlbumPath) {
    Close-ExplorerWindows
    Open-ExplorerAt (Split-Path -Parent $AlbumPath)
    $leaf = [IO.Path]::GetFileNameWithoutExtension($AlbumPath)
    $item = Find-ExplorerItem ('^' + [regex]::Escape($leaf) + '(\.salbum)?$') 25
    if (-not $item) { return $false }
    Invoke-HitClick $item 'dbl'
    return $true
}

# Запуск exe с меткой скачанного двойным щелчком в Проводнике (как В2): SmartScreen или окно проверки безопасности Проводника.
# Результат: Detail, Shots, Kind (smartscreen | aes | none). При none exe запускается без Проводника (CreateProcess: ShellExecuteEx
# показал бы окно проверки синхронно и повесил сценарий)
function Invoke-QaZoneLaunch {
    param([Parameter(Mandatory)][string]$ExePath, [Parameter(Mandatory)][string]$Id)
    $res = [pscustomobject]@{ Detail = ''; Shots = @(); Kind = 'none' }
    Close-ExplorerWindows
    Open-ExplorerAt (Split-Path -Parent $ExePath)
    $leaf = [IO.Path]::GetFileNameWithoutExtension($ExePath)
    Step-Qa 'ищу exe в Проводнике'
    $item = Find-ExplorerItem ('^' + [regex]::Escape($leaf) + '(\.exe)?$') 25
    $how = 'двойной щелчок в Проводнике'
    Step-Qa 'двойной щелчок по exe'
    if ($item) { Invoke-HitClick $item 'dbl' } else { $how = 'Проводник не нашёл файл' }
    Step-Qa 'жду SmartScreen или окно проверки безопасности'
    $info = $null; $aes = $null
    $wait = [Diagnostics.Stopwatch]::StartNew()
    while ($item -and -not $info -and -not $aes -and $wait.Elapsed.TotalSeconds -lt 25) {
        $info = Wait-Target -Names @('Подробнее', 'More info') -Timeout 1 -Skip @('msedge')
        if (-not $info) { $aes = Find-QaAesWindow }
    }
    if ($info) {
        $res.Kind = 'smartscreen'
        $res.Shots += Save-QaShot -Id $Id -Slug 'smartscreen'
        Step-Qa 'щелчок по "Подробнее"'
        Invoke-HitClick $info 'click'
        $run = Wait-Target -Names @('Выполнить в любом случае', 'Run anyway') -Timeout 10
        $res.Shots += Save-QaShot -Id $Id -Slug 'more'
        Step-Qa 'щелчок по "Выполнить в любом случае"'
        if ($run) { Invoke-HitClick $run 'click' }
        $res.Detail = "окно SmartScreen ($($info.Name)), $how; 'Выполнить в любом случае' найдена: $([bool]$run)"
    } elseif ($aes) {
        $res.Kind = 'aes'
        $res.Shots += Save-QaShot -Id $Id -Slug 'aes'
        $runBtn = Find-QaWindowButton $aes @('Run', 'Запустить')
        Step-Qa 'щелчок по "Запустить" в окне проверки безопасности'
        if ($runBtn) { Invoke-HitClick $runBtn 'click' }
        $res.Detail = "окно проверки безопасности Проводника (не SmartScreen), $how; кнопка 'Запустить' найдена: $([bool]$runBtn)"
    } else {
        $res.Shots += Save-QaShot -Id $Id -Slug 'nosmartscreen'
        $res.Detail = "SmartScreen не показан за 25 с ($how); файл с меткой ZoneId=3"
    }
    return $res
}

# ---------- системный выбор папки, браузер ----------

# В открытом системном окне выбора папки (окно принадлежит процессу программы) вписать путь и нажать Enter
function Set-QaFolderDialog([int]$ProcId, [string]$Folder) {
    $ed = Find-AppEl -ProcId $ProcId -NameRegex '^(Folder|Папка)' -Type 'Edit' -Timeout 20
    if (-not $ed) { return $false }
    $done = $false
    try {
        $vp = $null
        if ($ed.TryGetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern, [ref]$vp)) { $vp.SetValue($Folder); $done = $true }
    } catch { Write-Log "выбор папки: $($_.Exception.Message)" }
    if (-not $done) { return $false }
    Start-Sleep -Milliseconds 500
    Send-Keys @('Enter')
    # Enter по пути существующей папки только переходит в неё: окно закрывает кнопка «Выбор папки»
    Start-Sleep -Milliseconds 1500
    if (Find-AppEl -ProcId $ProcId -NameRegex '^(Folder|Папка)' -Type 'Edit' -Timeout 1) {
        $sel = Find-AppEl -ProcId $ProcId -NameRegex '^(Select Folder|Выбор папки)$' -Type 'Button' -Timeout 5
        if ($sel) { [void](Invoke-AppEl -El $sel) }
        $t0 = Get-Date
        while (((Get-Date) - $t0).TotalSeconds -lt 10) {
            if (-not (Find-AppEl -ProcId $ProcId -NameRegex '^(Folder|Папка)' -Type 'Edit' -Timeout 1)) { return $true }
        }
        return $false
    }
    return $true
}

# Адрес в адресной строке браузера (Edge): значение поля или пусто
function Get-QaBrowserUrl([int]$ProcId) {
    $ed = Find-AppEl -ProcId $ProcId -NameRegex '(?i)address|адрес' -Type 'Edit' -Timeout 6
    if (-not $ed) { return '' }
    try {
        $vp = $null
        if ($ed.TryGetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern, [ref]$vp)) { return [string]$vp.Current.Value }
    } catch { }
    return ''
}
# Процессы по имени, которых не было в списке до шага: их запустили мы (программа открыла браузер)
function Get-QaNewProcs([string]$Name, [int[]]$Before) {
    return @(Get-Process -Name $Name -ErrorAction SilentlyContinue | Where-Object { $Before -notcontains $_.Id })
}

# Сколько элементов типа в окнах процесса, у которых имя подходит под регулярку (кнопки шагов тура «1. Папка со съёмкой»)
function Get-QaCountRe([int]$ProcId, [string]$Type, [string]$Regex) {
    $auto = [System.Windows.Automation.AutomationElement]
    $cond = New-Object System.Windows.Automation.PropertyCondition($auto::ControlTypeProperty, [System.Windows.Automation.ControlType]::$Type)
    $n = 0
    foreach ($w in (Get-QaWindows -ProcId $ProcId)) {
        $found = $null
        try { $found = $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond) } catch { continue }
        foreach ($el in $found) { try { if ([string]$el.Current.Name -match $Regex) { $n++ } } catch { } }
    }
    return $n
}

# config.json программы как объект или $null (файл пишет программа: читаем с общим доступом)
function Get-QaCfgFile([string]$CfgPath) {
    try {
        $fs = New-Object System.IO.FileStream($CfgPath, 'Open', 'Read', 'ReadWrite')
        try {
            $sr = New-Object System.IO.StreamReader($fs, [System.Text.Encoding]::UTF8)
            return ($sr.ReadToEnd() | ConvertFrom-Json)
        } finally { $fs.Dispose() }
    } catch { return $null }
}
