# qa-probe.ps1 - проба П0 (спека 118, design §4): что раннер умеет на самом деле. Семь строк P0-* в results.jsonl.
# Отрицательный ответ - не сбой: он идёт как eyes (со снимком) или laptop, прогон остаётся зелёным; решение по ответам принимает оркестратор.
# Вызывается из run.ps1 (модули уже подключены). Windows PowerShell 5.1, файл в UTF-8 с BOM.
param([string]$Kit, [string]$OutDir, [string]$Dpi = '', [switch]$Sabotage, [string]$RunnerTemp = $env:TEMP)
$ErrorActionPreference = 'Continue'
if (-not ('SaQa.Fg' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
namespace SaQa {
    public static class Fg {
        [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
        [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
        // PID процесса, чьё окно сейчас на переднем плане
        public static int ForegroundPid() { uint p; GetWindowThreadProcessId(GetForegroundWindow(), out p); return (int)p; }
    }
}
'@
}

$work = Join-Path $RunnerTemp 'probe'
New-Item -ItemType Directory -Force -Path $work | Out-Null
$setup = Join-Path $Kit 'setup\SosninaAlbums-0.25.0-windows-x64-setup.exe'
$relZip = Join-Path $Kit 'zip\SosninaAlbums-0.25.0-rel-windows-x64.zip'
Expand-Archive -LiteralPath $relZip -DestinationPath (Join-Path $work 'rel') -Force
$relExe = Join-Path $work 'rel\Sosnina Albums 0.25.0-rel\SosninaAlbums-0.25.0.exe'
$P = Get-QaPaths
$tree = Join-Path (Join-Path $OutDir 'logs') 'uia-tree.txt'

# ---------- P0-uia, P0-front, P0-invoke, P0-keys: окно программы (rel, настоящий профиль) ----------

$app = $null
Invoke-QaItem 'P0-uia' {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    Step-Qa 'запуск rel-программы'
    $script:app = Start-QaApp -Exe $relExe
    Step-Qa 'жду первый узел Document (страница WebView2) в UIA'
    $docEl = Find-AppEl -ProcId $script:app.Id -Type 'Document' -Timeout 30
    $docSec = [math]::Round($sw.Elapsed.TotalSeconds, 1)
    Step-Qa 'ищу кнопку "Продолжить" в UIA'
    # имя кнопки в Chromium - всё её содержимое («Продолжить: Черновик»), поэтому по началу имени
    $btn = Find-AppEl -ProcId $script:app.Id -NameRegex '^Продолжить(:|$)' -Type 'Button' -Timeout 30
    $sec = [math]::Round($sw.Elapsed.TotalSeconds, 1)
    $front = [SaQa.Fg]::ForegroundPid()
    Step-Qa 'снимаю дерево UIA'
    $stat = Export-QaUiaTree -ProcId $script:app.Id -Path $tree -Depth 30
    $shot = Save-QaShot -Id 'P0-uia' -Slug 'window' -ProcId $script:app.Id
    $docText = 'узел Document не появился'
    if ($docEl) { $docText = "первый узел Document через $docSec с" }
    $detail = "узлов $($stat.Nodes), кнопок без имени $($stat.UnnamedButtons), $docText, дерево - logs/uia-tree.txt"
    if ($btn) { Add-QaResult -Id 'P0-uia' -Status ok -Detail "ДА: кнопка 'Продолжить' найдена за $sec с; $detail" -Shots $shot -Sec $sec }
    else {
        Add-QaResult -Id 'P0-uia' -Status eyes -Detail "НЕТ: 'Продолжить' не найдена за $sec с; $detail" -Shots $shot -Sec $sec
    }
    # P0-front: вышло ли окно программы на передний план (Б2 проверяет "окно вышло вперёд")
    if ($front -eq $script:app.Id) { Add-QaResult -Id 'P0-front' -Status ok -Detail 'ДА: программа на переднем плане сразу после запуска' }
    else { Add-QaResult -Id 'P0-front' -Status eyes -Detail "НЕТ: на переднем плане процесс $front, программа $($script:app.Id)" -Shots $shot }
}

Invoke-QaItem 'P0-invoke' {
    $no = Find-AppEl -ProcId $app.Id -Name 'Не сейчас' -Type 'Button' -Timeout 5
    if (-not $no) { Add-QaResult -Id 'P0-invoke' -Status eyes -Detail 'кнопка "Не сейчас" не найдена (вопроса о связи нет или WebView2 не виден)' -Shots (Save-QaShot -Id 'P0-invoke' -Slug 'nobutton' -ProcId $app.Id); return }
    $how = Invoke-AppEl -El $no
    $gone = $true
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        Start-Sleep -Milliseconds 400
        $again = Find-AppEl -ProcId $app.Id -Name 'Не сейчас' -Type 'Button' -Timeout 0.3
        $gone = -not $again
    } while (-not $gone -and $sw.Elapsed.TotalSeconds -lt 6)
    $shot = Save-QaShot -Id 'P0-invoke' -Slug 'after' -ProcId $app.Id
    if ($gone -and $how -eq 'invoke') { Add-QaResult -Id 'P0-invoke' -Status ok -Detail 'ДА: InvokePattern нажал кнопку, строка вопроса исчезла' -Shots $shot }
    else { Add-QaResult -Id 'P0-invoke' -Status eyes -Detail "способ: $how; строка исчезла: $gone (invoke не сработал - кнопки только мышью)" -Shots $shot }
}

Invoke-QaItem 'P0-keys' {
    Step-Qa 'жму "Продолжить"'
    $go = Find-AppEl -ProcId $app.Id -NameRegex '^Продолжить(:|$)' -Type 'Button' -Timeout 5
    if ($go) { [void](Invoke-AppEl -El $go) }
    Step-Qa 'жду карточку "Первые шаги"'
    $start = Find-AppEl -ProcId $app.Id -Name 'Начать' -Type 'Button' -Timeout 15
    if (-not $start) { Add-QaResult -Id 'P0-keys' -Status eyes -Detail 'карточка "Первые шаги" с кнопкой "Начать" не появилась' -Shots (Save-QaShot -Id 'P0-keys' -Slug 'nocard' -ProcId $app.Id); return }
    [void](Invoke-AppEl -El $start)
    Start-Sleep -Seconds 1
    $next = Find-AppEl -ProcId $app.Id -Name 'Дальше' -Type 'Button' -Timeout 8
    if (-not $next) { Add-QaResult -Id 'P0-keys' -Status eyes -Detail 'кнопка "Дальше" на шаге 1 не найдена' -Shots (Save-QaShot -Id 'P0-keys' -Slug 'nonext' -ProcId $app.Id); return }
    Step-Qa 'фокус на "Дальше" и клавиша {RIGHT}'
    $next.SetFocus()
    Start-Sleep -Milliseconds 300
    [System.Windows.Forms.SendKeys]::SendWait('{RIGHT}')
    $step2 = Find-AppEl -ProcId $app.Id -NameRegex 'Шаг 2 из \d+' -Timeout 6
    $shot = Save-QaShot -Id 'P0-keys' -Slug 'step2' -ProcId $app.Id
    if ($step2) { Add-QaResult -Id 'P0-keys' -Status ok -Detail 'ДА: SendKeys {RIGHT} при фокусе UIA перешёл на шаг 2' -Shots $shot }
    else { Add-QaResult -Id 'P0-keys' -Status eyes -Detail 'НЕТ: после {RIGHT} текст "Шаг 2 из 7" не появился' -Shots $shot }
}
if ($app) { Write-Log ('rel: ' + (Stop-QaApp -ProcId $app.Id)) }

# ---------- P0-choice: choice читает ввод из канала ----------

Invoke-QaItem 'P0-choice' {
    Step-Qa 'profile-restore.cmd со вводом из канала'
    $cmd = Join-Path $Kit 'profile-restore.cmd'
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $env:ComSpec
    $psi.Arguments = '/d /s /c "(echo Y& echo.)| "' + $cmd + '""'
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.CreateNoWindow = $true
    $pr = [System.Diagnostics.Process]::Start($psi)
    $out = $pr.StandardOutput.ReadToEnd()
    if (-not $pr.WaitForExit(30000)) { & taskkill.exe /PID $pr.Id /T /F | Out-Null }
    $flat = ($out -replace '\s+', ' ').Trim()
    if ($flat.Length -gt 200) { $flat = $flat.Substring(0, 200) }
    if ($pr.ExitCode -eq 0 -and $out -match 'Готово') { Add-QaResult -Id 'P0-choice' -Status ok -Detail "ДА: код 0, вывод: $flat" }
    else { Add-QaResult -Id 'P0-choice' -Status eyes -Detail "НЕТ: код $($pr.ExitCode), вывод: $flat" }
}

# ---------- P0-openwith: спросит ли Windows "Чем открыть" ----------

Invoke-QaItem 'P0-openwith' -Budget 300 {
    Step-Qa 'тихая установка для пробы "Чем открыть"'
    $pr = Start-Setup $setup '/S'
    if (-not $pr.WaitForExit(120000)) { & taskkill.exe /PID $pr.Id /T /F | Out-Null; throw 'тихая установка не закончилась за 120 с' }
    if ($pr.ExitCode -ne 0) { throw "тихая установка вернула код $($pr.ExitCode)" }
    $file = Join-Path $work 'Probe.salbum'
    Copy-Item -LiteralPath (Join-Path $Kit 'albums\NewerTest-unknown.salbum') -Destination $file -Force
    $before = @(Get-Process -Name 'OpenWith' -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
    Step-Qa 'открываю .salbum через оболочку'
    Start-Process -FilePath $file
    $sw = [Diagnostics.Stopwatch]::StartNew()
    $ask = $false; $started = @()
    do {
        Start-Sleep -Milliseconds 500
        $now = @(Get-Process -Name 'OpenWith' -ErrorAction SilentlyContinue | Where-Object { $before -notcontains $_.Id })
        $ask = $now.Count -gt 0
        $started = Get-AppProcs $P.Exe
    } while (-not $ask -and $started.Count -eq 0 -and $sw.Elapsed.TotalSeconds -lt 8)
    Start-Sleep -Seconds 2
    $shot = Save-QaShot -Id 'P0-openwith' -Slug 'after'
    $started = Get-AppProcs $P.Exe
    $detail = "окно 'Чем открыть': $(if ($ask) { 'ДА' } else { 'нет' }); установленная программа запущена: $(if ($started.Count) { 'ДА' } else { 'нет' })"
    Add-QaResult -Id 'P0-openwith' -Status ok -Detail $detail -Shots $shot
    foreach ($p in $started) { [void](Stop-QaApp -ProcId $p.Id -Hard) }
    foreach ($p in @(Get-Process -Name 'OpenWith' -ErrorAction SilentlyContinue | Where-Object { $before -notcontains $_.Id })) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
}
# Чистое состояние для следующих проб
[void](Invoke-QuietUninstall)

# ---------- P0-dpi: масштаб на лету и DPI окна мастера ----------

Invoke-QaItem 'P0-dpi' -Budget 300 {
    $modes = (Get-QaDisplayModes) -join ' '
    $st = Get-QaDpiState
    if (-not $st.Ok) { Add-QaResult -Id 'P0-dpi' -Status laptop -Detail "API масштаба недоступен: $($st.Why); режимы: $modes"; return }
    $orig = $st.Cur
    $set = Set-QaDpi 150
    if (-not $set.Ok) { Add-QaResult -Id 'P0-dpi' -Status laptop -Detail "150 % не поставился: $($set.Why); сейчас $($st.Cur) % (рек. $($st.Rec), макс. $($st.Max)); режимы: $modes"; return }
    $sys = Get-QaFreshSystemDpi
    $pr = Start-Setup $setup ''
    $win = $null
    try {
        [void](Wait-WizWindow -ProcNames @('SosninaAlbums-0.25.0-windows-x64-setup'))
        $el = (Get-QaWindows -ProcId $pr.Id | Select-Object -First 1)
        $dpi = 0
        if ($el) { $dpi = Get-QaWindowDpi $el }
        $shot = Save-QaShot -Id 'P0-dpi' -Slug 'wizard150' -ProcId $pr.Id
        $detail = "масштаб 150 %: GetDpiForWindow мастера $dpi, GetDpiForSystem нового процесса $sys; рек. $($st.Rec), макс. $($st.Max); режимы: $modes"
        if ($dpi -eq 144) { Add-QaResult -Id 'P0-dpi' -Status ok -Detail ('ДА: ' + $detail) -Shots $shot }
        else { Add-QaResult -Id 'P0-dpi' -Status laptop -Detail ('НЕТ: ' + $detail) -Shots $shot }
    } finally {
        [void](Stop-QaApp -ProcId $pr.Id -Hard)
        [void](Set-QaDpi $orig)
    }
}
