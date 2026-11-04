# winzip.psm1 - выпуск архивом (спека 039, design §4): выбор файлов выпуска, метка «из интернета», шаги Проводника
# («Загрузки», zip, «Извлечь все», окно «Сжатые ZIP-папки»). Чистые функции проверяет test_winlib.ps1; шаги с окнами
# видны только на раннере. Распаковка только средствами Проводника: Expand-Archive метку Zone.Identifier не переносит.
# Что умеет сценарий из shots.ps1 (Invoke-Act, снимки, запуск exe), модуль получает в хеш-таблице $C
# (скриптблоки, созданные в shots.ps1, выполняются в его области): Downloads, OcrTmp, Act, Idle, Missing, Procs, RunExe, AppShot.
# Windows PowerShell 5.1: файл в UTF-8 с BOM.

Import-Module (Join-Path $PSScriptRoot 'winlib.psm1') -DisableNameChecking
Import-Module (Join-Path $PSScriptRoot 'winui.psm1') -DisableNameChecking
Add-Type -AssemblyName System.IO.Compression.FileSystem

# Имена элементов окон Проводника: на раннере английские (zipfldr.dll.mui en-US), русские - для Windows фотографа
$script:ZN = @{
    ExtractAll = @('Извлечь все', 'Extract all')
    Extract = @('Извлечь', 'Extract')
    Cancel = @('Отмена', 'Cancel')
    Run = @('Запуск', 'Run')
    Dialog = @('Сжатые ZIP-папки', 'Compressed (zipped) Folders')
    Info = @('Подробнее', 'More info')
    Anyway = @('Выполнить в любом случае', 'Run anyway')
}

# Имя exe программы в zip и в окне Проводника: с версией (спека 111, SosninaAlbums-0.25.0.exe); прежнее без версии тоже узнаём
$script:ExeItemRx = '^SosninaAlbums(-\d+\.\d+\.\d+)?(\.exe)?$'

# ---------- чистые функции ----------

# Что качать со страницы выпуска: архив раньше «голого» exe (выпуск 0.10 ещё с exe). Пусто - ни того, ни другого
function Select-ReleaseAsset([string[]]$Names) {
    $list = @($Names | Where-Object { $_ })
    foreach ($k in @(@('zip', '-windows-x64\.zip\z'), @('exe', '-windows-x64\.exe\z'))) {
        foreach ($n in $list) {
            if ($n -cmatch ('^SosninaAlbums-\d+\.\d+\.\d+' + $k[1])) { return [pscustomobject]@{ Kind = $k[0]; Name = [string]$n } }
        }
    }
    return [pscustomobject]@{ Kind = ''; Name = '' }
}

# Инструкция для упаковки exe: pdf своего варианта (win), иначе docx, иначе общая. Пусто - ничего не подошло
function Select-GuideAsset([string[]]$Names) {
    $list = @($Names | Where-Object { $_ })
    foreach ($s in '-install-guide-windows-ru.pdf', '-install-guide-windows-ru.docx', '-install-guide-ru.pdf', '-install-guide-ru.docx') {
        foreach ($n in $list) {
            if ($n -cmatch ('^SosninaAlbums-\d+\.\d+\.\d+' + [regex]::Escape($s) + '\z')) { return [string]$n }
        }
    }
    return ''
}

# Раскладка по имени архива: «Извлечь все» кладёт файлы в папку с именем архива без .zip; внутри - папка выпуска
function Get-ZipLayout([string]$Name) {
    $leaf = Split-Path -Leaf $Name
    $ver = ''
    if ($leaf -cmatch '^SosninaAlbums-(\d+\.\d+\.\d+)-windows-x64\.zip\z') { $ver = $Matches[1] }
    $inner = ''
    if ($ver) { $inner = "Sosnina Albums $ver" }
    return [pscustomobject]@{ Ok = [bool]$ver; Leaf = $leaf; Folder = [IO.Path]::GetFileNameWithoutExtension($leaf); Version = $ver; Inner = $inner }
}

# Метка «скачано из интернета» (ZoneId=3): то же, что ставит браузер, - по ней SmartScreen и ругается
function Set-Zone([string]$Path, [string]$Url) {
    Set-Content -LiteralPath $Path -Stream Zone.Identifier -Value "[ZoneTransfer]`r`nZoneId=3`r`nHostUrl=$Url`r`nReferrerUrl=$Url"
}

# ZoneId файла или $null, если метки нет
function Get-ZoneId([string]$Path) {
    $lines = @()
    try { $lines = @(Get-Content -LiteralPath $Path -Stream Zone.Identifier -ErrorAction Stop) } catch { return $null }
    foreach ($l in $lines) { if (([string]$l) -match '^\s*ZoneId\s*=\s*(\d+)\s*$') { return [int]$Matches[1] } }
    return $null
}

# У всех файлов папки есть ZoneId=3? Ok - да (в пустой или несуществующей папке - нет); Missing - пути без метки
function Test-ZoneTree([string]$Root) {
    $missing = New-Object System.Collections.ArrayList
    $total = 0
    try {
        $full = (Resolve-Path -LiteralPath $Root -ErrorAction Stop).Path.TrimEnd('\')
        foreach ($f in @(Get-ChildItem -LiteralPath $full -Recurse -File -Force -ErrorAction SilentlyContinue)) {
            $total++
            if ((Get-ZoneId $f.FullName) -ne 3) { [void]$missing.Add($f.FullName.Substring($full.Length).TrimStart('\')) }
        }
    } catch { Write-Log "Test-ZoneTree: $($_.Exception.Message)" }
    return [pscustomobject]@{ Ok = ($total -gt 0 -and $missing.Count -eq 0); Total = $total; Missing = @($missing) }
}

# Имя exe программы в архиве (SosninaAlbums-X.Y.Z.exe или прежнее SosninaAlbums.exe); '' - архив не читается или exe нет
function Get-ZipExeName([string]$Zip) {
    $z = $null
    try {
        $z = [IO.Compression.ZipFile]::OpenRead($Zip)
        foreach ($e in $z.Entries) { if ($e.Name -cmatch '^SosninaAlbums(-\d+\.\d+\.\d+)?\.exe\z') { return [string]$e.Name } }
    } catch { Write-Log "zip: $($_.Exception.Message)" } finally { if ($z) { $z.Dispose() } }
    return ''
}

# Размер записи архива по имени файла; -1 - архив не читается или записи нет
function Get-ZipEntryLength([string]$Zip, [string]$Leaf) {
    $z = $null
    try {
        $z = [IO.Compression.ZipFile]::OpenRead($Zip)
        foreach ($e in $z.Entries) { if ($e.Name -ceq $Leaf) { return [long]$e.Length } }
    } catch { Write-Log "zip: $($_.Exception.Message)" } finally { if ($z) { $z.Dispose() } }
    return -1
}

# Подпись ролика не длиннее 80 знаков (контракт журнала)
function Format-Caption([string]$Text) {
    if ($Text.Length -gt 80) { return $Text.Substring(0, 80) }
    return $Text
}

# ---------- Проводник ----------

function Open-ExplorerAt([string]$Path) { Start-Process -FilePath explorer.exe -ArgumentList @("`"$Path`"") }

# Закрыть все окна Проводника (Shell.Application), оболочку и панель задач не трогая
function Close-ExplorerWindows {
    try {
        $sh = New-Object -ComObject Shell.Application
        foreach ($w in @($sh.Windows())) { try { $w.Quit() } catch { } }
    } catch { Write-Log "закрыть окна Проводника: $($_.Exception.Message)" }
    Start-Sleep -Milliseconds 800
}

# Элемент списка файлов в окне Проводника: ListItem, запасной вид - DataItem
function Find-ExplorerItem([string]$Regex, [double]$Timeout = 25) {
    $hit = Wait-Target -Names @() -Procs @('explorer') -Timeout $Timeout -NameRegex $Regex -Type 'ListItem'
    if (-not $hit) { $hit = Wait-Target -Names @() -Procs @('explorer') -Timeout 6 -NameRegex $Regex -Type 'DataItem' }
    return $hit
}

function Get-ZipItemRegex($Lay) { return ('^' + [regex]::Escape($Lay.Folder) + '(\.zip)?$') }

# Без записи в журнал: проба «в простое» и запасные ходы
function Invoke-HitClick($Hit, [string]$Mode = 'click') {
    Move-Cursor ([int]($Hit.Rect[0] + $Hit.Rect[2] / 2)) ([int]($Hit.Rect[1] + $Hit.Rect[3] / 2)) 0.3 10
    Invoke-MouseClick $Mode
}

# Кнопка внутри окна-диалога (Dlg - результат Wait-Target по имени окна). Окна нет - поиск по всему экрану,
# кроме -NoGlobal: «Извлечь все» есть и на панели команд Проводника, там нужная только в диалоге
function Find-DlgButton($Dlg, [string[]]$Names, [switch]$NoGlobal) {
    if ($Dlg -and $Dlg.Element) {
        $auto = [System.Windows.Automation.AutomationElement]
        $ic = [System.Windows.Automation.PropertyConditionFlags]::IgnoreCase
        foreach ($n in $Names) {
            $cond = New-Object System.Windows.Automation.PropertyCondition($auto::NameProperty, $n, $ic)
            $el = $null
            try { $el = $Dlg.Element.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $cond) } catch { }
            if ($el) { $h = ConvertTo-UiaHit $el; if ($h) { return $h } }
        }
        return $null
    }
    if ($NoGlobal) { return $null }
    return (Find-UiaOnce -Names $Names -Procs @() -Skip @('msedge'))
}

# ---------- распаковка ----------

function Find-ExtractedExe([string]$Root) {
    return (Get-ChildItem -LiteralPath $Root -Recurse -Filter 'SosninaAlbums*.exe' -File -ErrorAction SilentlyContinue | Select-Object -First 1)
}

# Ждёт, пока exe распакуется целиком (размер как в архиве); путь или $null
function Wait-Extracted([string]$Dest, [string]$Zip, [double]$Timeout = 90) {
    $want = Get-ZipEntryLength $Zip (Get-ZipExeName $Zip)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        $exe = Find-ExtractedExe $Dest
        if ($exe -and ($want -le 0 -or $exe.Length -eq $want)) {
            Start-Sleep -Seconds 3   # остальные файлы архива пишутся следом
            return $exe.FullName
        }
        Start-Sleep -Milliseconds 700
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $null
}

# Запасной путь: тот же движок Проводника через Shell.Application, метка переносится (не Expand-Archive)
function Expand-ZipShell([string]$Zip, [string]$Dest) {
    New-Item -ItemType Directory -Force -Path $Dest | Out-Null
    $sh = New-Object -ComObject Shell.Application
    $src = $sh.NameSpace($Zip)
    $dst = $sh.NameSpace($Dest)
    if ($null -eq $src -or $null -eq $dst) { return $false }
    $dst.CopyHere($src.Items(), 0x14)   # 4 - без окна хода, 16 - «да» на все вопросы
    return $true
}

# Глава 1: «Загрузки» -> двойной щелчок по архиву -> «Извлечь все» -> «Извлечь». Итог - $C.Unpacked (Exe, Dest, Inner)
function Invoke-ZipUnpack($C, [string]$Zip) {
    $lay = Get-ZipLayout $Zip
    $dest = Join-Path $C.Downloads $lay.Folder
    $C.Unpacked = $null
    Add-JChapter 'Распакуйте архив'
    & $C.Idle
    Add-JStep 1 3 'Откройте «Загрузки»'
    & $C.Idle
    Close-ExplorerWindows
    if (Test-Path -LiteralPath $dest) { Remove-Item -LiteralPath $dest -Recurse -Force -ErrorAction SilentlyContinue }
    Open-ExplorerAt $C.Downloads
    $item = Find-ExplorerItem (Get-ZipItemRegex $lay) 25
    if ($item) {
        & $C.Act $item (Format-Caption ('Дважды щёлкните по ' + $lay.Leaf)) 'dbl' 'win-08-zip-downloads' 1.5 $lay.Leaf
    } else {
        & $C.Missing 'win-08-zip-downloads' 'архив не найден в окне «Загрузки» (UIA)'
    }
    Add-JStep 2 3 'Извлеките всё'
    & $C.Idle
    $btn = $null
    if ($item) { $btn = Wait-Target -Names $script:ZN.ExtractAll -Procs @('explorer') -Timeout 20 -Ocr @('Extract all', 'Извлечь все') -Tmp $C.OcrTmp }
    if ($btn) {
        & $C.Act $btn (Get-Caption 'Извлечь все' $btn.Name) 'click' 'win-09-zip-extract-all' 1.5 'Извлечь все'
    } else {
        & $C.Missing 'win-09-zip-extract-all' 'кнопка «Извлечь все» не найдена (UIA и OCR)'
    }
    Add-JStep 3 3 'Нажмите «Извлечь»'
    & $C.Idle
    $ok = $null
    $via = 'Проводник («Извлечь все»)'
    if ($btn) {
        $dlg = Wait-Target -Names $script:ZN.Extract -Procs @() -Timeout 20 -Skip @('msedge')
        if ($dlg) {
            & $C.Act $dlg (Get-Caption 'Извлечь' $dlg.Name) 'click' 'win-10-extract-dialog' 1.5 'Извлечь'
        } else {
            & $C.Missing 'win-10-extract-dialog' 'кнопка «Извлечь» не найдена, нажимаю Enter'
            Send-Keys @('Enter')
        }
        $ok = Wait-Extracted $dest $Zip 90
    }
    if (-not $ok) {
        Write-Log 'мастер Проводника не распаковал архив - Shell.Application.CopyHere'
        Add-JNote 'запасной путь: распаковка через Shell.Application'
        $via = 'Shell.Application CopyHere (запасной путь)'
        if (Expand-ZipShell $Zip $dest) { $ok = Wait-Extracted $dest $Zip 90 }
    }
    if (-not $ok) { Add-SummaryRow 'Распаковка архива' 'нет' 'exe не появился в папке распаковки'; return }
    $null = Find-ExplorerItem '^Sosnina Albums' 8   # Проводник сам открывает папку («Показать извлечённые файлы»)
    & $C.Act $null 'Готово: открылась папка с программой' 'click' '' 1.5 ''
    $exeFile = Get-Item -LiteralPath $ok
    $C.Unpacked = [pscustomobject]@{ Exe = $exeFile.FullName; Dest = $dest; Inner = $exeFile.DirectoryName }
    Add-SummaryRow 'Распаковка архива' 'да' "$via; exe: $($exeFile.FullName)"
    $zid = Get-ZoneId $Zip
    Add-SummaryRow 'метка на архиве' $(if ($zid -eq 3) { 'да' } else { 'нет' }) "ZoneId=$zid"
    $eid = Get-ZoneId $exeFile.FullName
    Add-SummaryRow 'метка на извлечённом exe' $(if ($eid -eq 3) { 'да' } else { 'нет' }) "ZoneId=$eid"
    $zt = Test-ZoneTree $dest
    Add-SummaryRow 'метка на всех файлах распаковки' $(if ($zt.Ok) { 'да' } else { 'нет' }) ("файлов $($zt.Total); без метки: " + ($zt.Missing -join ', '))
    $rel = $exeFile.FullName.Substring($dest.Length).TrimStart('\')
    Add-SummaryRow 'двойная вложенность' $(if ($rel -match '\\') { 'да' } else { 'нет' }) $rel
}

# Глава 2: папка выпуска -> двойной щелчок по exe -> SmartScreen -> программа (RunExe и AppShot - из shots.ps1)
function Invoke-ZipFirstRun($C) {
    $u = $C.Unpacked
    Add-JChapter 'Первый запуск'
    & $C.Idle
    Add-JStep 1 3 'Откройте папку'
    & $C.Idle
    if ($u.Inner -ne $u.Dest) {
        $name = Split-Path -Leaf $u.Inner
        $rx = '^' + [regex]::Escape($name) + '$'
        $folder = Find-ExplorerItem $rx 8
        if (-not $folder) {
            Close-ExplorerWindows
            Open-ExplorerAt $u.Dest
            $folder = Find-ExplorerItem $rx 20
        }
        if ($folder) { & $C.Act $folder (Format-Caption ('Дважды щёлкните по папке ' + $name)) 'dbl' '' 1.5 $name }
        else { Add-JNote "папка $name не найдена в окне Проводника" }
    } else {
        Close-ExplorerWindows
        Open-ExplorerAt $u.Inner
    }
    $exeItem = Find-ExplorerItem $script:ExeItemRx 20
    $clicked = $false
    if ($exeItem) {
        & $C.Act $exeItem (Format-Caption ('Дважды щёлкните по ' + $exeItem.Name)) 'dbl' 'win-11-extracted' 0.5 $exeItem.Name
        $clicked = $true
    } else {
        & $C.Missing 'win-11-extracted' 'exe программы не найден в окне Проводника (UIA)'
    }
    & $C.RunExe $u.Exe 2 3 $clicked
    & $C.AppShot 3 3
}

# ---------- запуск прямо из архива ----------

# Что случилось после «Запуск» / «Извлечь все» в окне «Сжатые ZIP-папки»: программа (путь процесса), окно извлечения, ничего
function Watch-ZipOutcome($C) {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    $smart = $false
    while ($sw.Elapsed.TotalSeconds -lt 40) {
        $procs = @(& $C.Procs)
        if ($procs.Count -gt 0) {
            $path = ''
            try { $path = $procs[0].Path } catch { }
            $r = "программа запущена, путь процесса: $path"
            if ($smart) { $r += '; был SmartScreen' }
            return $r
        }
        if (-not $smart) {
            $info = Find-UiaOnce -Names $script:ZN.Info -Procs @() -Skip @('msedge')
            if ($info) {
                $smart = $true
                Invoke-HitClick $info 'click'
                $run = Wait-Target -Names $script:ZN.Anyway -Procs @() -Timeout 8 -Skip @('msedge')
                if ($run) { Invoke-HitClick $run 'click' }
            }
        }
        if (Find-UiaOnce -Names $script:ZN.Extract -Procs @() -Skip @('msedge')) { return 'открылось окно извлечения (программа не запущена)' }
        Start-Sleep -Milliseconds 700
    }
    return 'за 40 с ничего не открылось'
}

# Проба в простое: двойной щелчок по exe в открытом архиве -> нужная кнопка окна -> что открылось
function Invoke-ZipProbe($C, [string]$Label, [string[]]$Button) {
    $item = Find-ExplorerItem $script:ExeItemRx 12
    if (-not $item) { Add-SummaryRow $Label 'нет' 'exe в окне архива не найден'; return }
    Invoke-HitClick $item 'dbl'
    $dlg = Wait-Target -Names $script:ZN.Dialog -Procs @() -Timeout 20 -Skip @('msedge')
    $btn = Find-DlgButton $dlg $Button -NoGlobal:($Button -contains 'Extract all')
    if (-not $btn) { Add-SummaryRow $Label 'нет' 'окно «Сжатые ZIP-папки» или кнопка не найдены'; Send-Keys @('Esc'); return }
    Invoke-HitClick $btn 'click'
    $res = Watch-ZipOutcome $C
    Add-SummaryRow $Label 'да' $res
    @(& $C.Procs) | Stop-Process -Force -ErrorAction SilentlyContinue
    Send-Keys @('Esc')
    Start-Sleep -Seconds 1
}

# Глава 3: архив -> папка -> двойной щелчок по exe -> окно «Сжатые ZIP-папки» -> «Отмена»; затем проба «Запуск» и
# «Извлечь все» (в простое, не в ролик): путь процесса и что открылось - в summary.md (открытый вопрос спеки)
function Invoke-ZipFromArchive($C, [string]$Zip) {
    $lay = Get-ZipLayout $Zip
    Add-JChapter 'Если запустили прямо из архива'
    & $C.Idle
    Add-JStep 1 3 'Откройте архив'
    & $C.Idle
    Close-ExplorerWindows
    Open-ExplorerAt $C.Downloads
    $item = Find-ExplorerItem (Get-ZipItemRegex $lay) 25
    if (-not $item) {
        & $C.Missing 'win-12-run-from-zip' 'архив не найден в окне «Загрузки» (UIA)'
        Add-SummaryRow 'Запуск из архива' 'нет' 'архив не найден'
        return
    }
    & $C.Act $item (Format-Caption ('Дважды щёлкните по ' + $lay.Leaf)) 'dbl' '' 1.5 $lay.Leaf
    if ($lay.Inner) {
        $dir = Find-ExplorerItem ('^' + [regex]::Escape($lay.Inner) + '$') 15
        if ($dir) { & $C.Act $dir (Format-Caption ('Дважды щёлкните по папке ' + $lay.Inner)) 'dbl' '' 1.5 $lay.Inner }
    }
    Add-JStep 2 3 'Запустите программу из архива'
    & $C.Idle
    $exeItem = Find-ExplorerItem $script:ExeItemRx 15
    if (-not $exeItem) {
        & $C.Missing 'win-12-run-from-zip' 'exe программы не найден в открытом архиве (UIA)'
        Add-SummaryRow 'Запуск из архива' 'нет' 'exe в архиве не найден'
        return
    }
    & $C.Act $exeItem (Format-Caption ('Дважды щёлкните по ' + $exeItem.Name)) 'dbl' '' 0.5 $exeItem.Name
    Add-JStep 3 3 'Закройте окно'
    & $C.Idle
    $dlg = Wait-Target -Names $script:ZN.Dialog -Procs @() -Timeout 20 -Skip @('msedge')
    $cancel = Find-DlgButton $dlg $script:ZN.Cancel
    if (-not $cancel) {
        $why = 'окно «Сжатые ZIP-папки» не появилось'
        if (@(& $C.Procs).Count -gt 0) { $why += '; программа запустилась сразу' }
        & $C.Missing 'win-12-run-from-zip' $why
        Add-SummaryRow 'Окно «Сжатые ZIP-папки»' 'нет' $why
        @(& $C.Procs) | Stop-Process -Force -ErrorAction SilentlyContinue
        return
    }
    Add-SummaryRow 'Окно «Сжатые ZIP-папки»' 'да' "кнопка: $($cancel.Name)"
    & $C.Act $cancel ((Get-Caption 'Отмена' $cancel.Name) + ' и распакуйте архив') 'click' 'win-12-run-from-zip' 1.2 'Отмена'
    $t0 = Get-JTime
    try {
        Invoke-ZipProbe $C 'Запуск из архива: «Запуск»' $script:ZN.Run
        Invoke-ZipProbe $C 'Запуск из архива: «Извлечь все»' $script:ZN.ExtractAll
    } catch { Write-Log "проба запуска из архива: $($_.Exception.Message)" }
    Add-JSegment 'idle' $t0 (Get-JTime)
    & $C.Idle
    Close-ExplorerWindows
}
