# winqa-update.psm1 - помощники блока update (спека 118, design §3, §5): лента, стенд qa, меню «О программе», цепочка журнала.
# Чистые функции (Get-QaFeedName, Get-QaStandPaths, New-QaStandConfig, Test-QaUpdateChain, Test-QaOnlyData) проверяет test_winqa.ps1;
# остальное видно только на раннере. Windows PowerShell 5.1, файл в UTF-8 с BOM.

Import-Module (Join-Path $PSScriptRoot 'winqa.psm1') -DisableNameChecking

$script:FeedPort = 8765
# Причина отказа Open-QaAbout: своя переменная модуля ($script:Qa живёт в winqa.psm1 и здесь $null)
$script:AboutWhy = ''

# ---------- чистые функции ----------

# Какая лента нужна сценарию: -Sabotage подменяет настоящую feed на feed-bad-byte (критерий приёмки самой проверки: Г13 красный)
function Get-QaFeedName([string]$Name, [bool]$Sabotage = $false) {
    if ($Sabotage -and $Name -ceq 'feed') { return 'feed-bad-byte' }
    return $Name
}

# Папки стенда qa (SA_STAND=qa): настройки и журнал в Roaming, скачанные обновления в Local
function Get-QaStandPaths {
    $roaming = Join-Path $env:APPDATA 'SosninaAlbums-stand-qa'
    $local = Join-Path $env:LOCALAPPDATA 'SosninaAlbums-stand-qa'
    return @{ Roaming = $roaming; Local = $local; Config = (Join-Path $roaming 'config.json'); Log = (Join-Path $roaming 'log.txt'); Updates = (Join-Path $local 'Updates') }
}

function ConvertTo-JString([string]$s) {
    return '"' + ($s.Replace('\', '\\').Replace('"', '\"')) + '"'
}

# config.json стенда: без стартового окна, вопроса о связи и туров, без проверки при запуске (кнопка «Проверить» работает и при update.off)
function New-QaStandConfig([string]$LastProject = '') {
    $tours = @('first', 'folders', 'library', 'autobuild', 'timeline', 'layouts', 'photo', 'export') | ForEach-Object { '"' + $_ + '":"done"' }
    $last = ''
    if ($LastProject) { $last = '"lastProject":' + (ConvertTo-JString $LastProject) + ',' }
    return '{' + $last + '"level":"simple","hideStart":true,"assoc":"off","tours":{' + ($tours -join ',') + '},"update":{"off":true}}'
}

# Цепочка успешного обновления в журнале (Г14): четыре строки по порядку. Ok и список недостающих
function Test-QaUpdateChain([string]$LogText, [string]$Version) {
    $v = [regex]::Escape($Version)
    $steps = @(
        @{ Name = 'найдена версия'; Re = "обновление: найдена версия $v" },
        @{ Name = 'скачана и сверена'; Re = "обновление: версия $v скачана и сверена" },
        @{ Name = 'запущен установщик (install)'; Re = "обновление: запущен установщик версии $v \(install\)" },
        @{ Name = 'итог: обновлено'; Re = ('обновление: итог запуска установщика ' + $v + ' — обновлено "' + $v + '"') }
    )
    $missing = New-Object System.Collections.ArrayList
    $pos = 0
    foreach ($s in $steps) {
        $m = [regex]::Match($LogText.Substring($pos), $s.Re)
        if ($m.Success) { $pos += $m.Index + $m.Length } else { [void]$missing.Add($s.Name) }
    }
    return [pscustomobject]@{ Ok = ($missing.Count -eq 0); Missing = @($missing) }
}

# Имена потоков файла (Get-Item -Stream): метки скачанного (Zone.Identifier) нет, остался только основной поток
function Test-QaOnlyData([string[]]$StreamNames) {
    $extra = @($StreamNames | Where-Object { $_ -and $_ -ne ':$DATA' })
    return [pscustomobject]@{ Ok = ($extra.Count -eq 0); Extra = $extra }
}

# ---------- лента ----------

function Test-QaPort([int]$Port) {
    $c = New-Object System.Net.Sockets.TcpClient
    try {
        $ar = $c.BeginConnect('127.0.0.1', $Port, $null, $null)
        if ($ar.AsyncWaitHandle.WaitOne(300) -and $c.Connected) { return $true }
        return $false
    } catch { return $false } finally { $c.Close() }
}
function Get-QaPython {
    foreach ($n in 'python', 'py') { $c = Get-Command $n -ErrorAction SilentlyContinue; if ($c) { return $c.Source } }
    return ''
}

# Ждёт порт ленты до 15 с; $false - не поднялся или процесс вышел
function Wait-QaFeedPort($P) {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt 15) {
        if (Test-QaPort $script:FeedPort) { return $true }
        if ($P.HasExited) { return $false }
        Start-Sleep -Milliseconds 300
    }
    return $false
}

# Лента из набора: cmd /c run-feed.cmd <лента>; возвращает Process cmd (остановка - Stop-QaFeed) или $null, если порт не поднялся
function Start-QaFeed([string]$Kit, [string]$Name) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $env:ComSpec
    $psi.Arguments = '/d /s /c ""' + (Join-Path $Kit 'run-feed.cmd') + '" ' + $Name + '"'
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $psi.RedirectStandardInput = $true
    $p = [System.Diagnostics.Process]::Start($psi)
    if (Wait-QaFeedPort $p) { return $p }
    Stop-QaFeed $p
    return $null
}
# Медленная лента (python-сервер с ограничением скорости): нужна, чтобы успеть нажать "Отменить" (Г10)
function Start-QaSlowFeed([string]$Kit, [string]$Name, [string]$ServerPy, [int]$BytesPerSec = 1000000) {
    $py = Get-QaPython
    if (-not $py) { return $null }
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $py
    $psi.Arguments = '"' + $ServerPy + '" "' +(Join-Path $Kit $Name) + '" ' + $script:FeedPort + ' ' + $BytesPerSec
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $p = [System.Diagnostics.Process]::Start($psi)
    if (Wait-QaFeedPort $p) { return $p }
    Stop-QaFeed $p
    return $null
}
# Только свой PID вместе с python (дерево); порт освобождается не сразу - ждём до 10 с
function Stop-QaFeed($P) {
    if ($P) { try { & taskkill.exe /PID $P.Id /T /F 2>&1 | Out-Null } catch { } }
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ((Test-QaPort $script:FeedPort) -and $sw.Elapsed.TotalSeconds -lt 10) { Start-Sleep -Milliseconds 300 }
}

# .cmd набора со вводом из канала (pause читает stdin); код выхода, -1 - не уложился в срок
function Invoke-QaCmdFile([string]$Cmd, [string]$Stdin = "`r`n", [int]$TimeoutSec = 60) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $env:ComSpec
    $psi.Arguments = '/d /s /c ""' + $Cmd + '""'
    $psi.UseShellExecute = $false
    $psi.RedirectStandardInput = $true
    $psi.CreateNoWindow = $true
    $pr = [System.Diagnostics.Process]::Start($psi)
    $pr.StandardInput.Write($Stdin)
    $pr.StandardInput.Close()
    if (-not $pr.WaitForExit($TimeoutSec * 1000)) { & taskkill.exe /PID $pr.Id /T /F | Out-Null; return -1 }
    return $pr.ExitCode
}

# ---------- программа: ожидание, окна, тексты ----------

# Пока условие не выполнено (до таймаута); Watch вызывается на каждом шаге опроса (сторож SmartScreen)
function Wait-QaCond([scriptblock]$Cond, [double]$Timeout = 20, [scriptblock]$Watch = $null) {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        if ($Watch) { & $Watch }
        if (& $Cond) { return $true }
        Start-Sleep -Milliseconds 400
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return [bool](& $Cond)
}

# Windows SmartScreen или окно проверки безопасности Проводника: $true, если сейчас на экране
function Test-QaSmartScreen {
    if (Find-QaAesWindow) { return $true }
    foreach ($w in (Get-UiaWindows)) {
        $n = ''
        try { $n = [string]$w.Current.Name } catch { continue }
        if ($n -match 'protected your PC|SmartScreen|защитил ваш компьютер|Windows защитил') { return $true }
    }
    return $false
}

# Имя первого элемента окна программы по регулярке (текст строки, тост) или $null; Watch - на каждом шаге
function Wait-QaText {
    param([Parameter(Mandatory)][int]$ProcId, [Parameter(Mandatory)][string]$Regex, [double]$Timeout = 15, [scriptblock]$Watch = $null)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        if ($Watch) { & $Watch }
        $el = Find-AppEl -ProcId $ProcId -NameRegex $Regex -Timeout 1
        if ($el) { try { return [string]$el.Current.Name } catch { return $Regex } }
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $null
}

# Сколько элементов типа в окнах процесса (строки "Что нового" - ListItem)
function Get-QaCount([int]$ProcId, [string]$Type) {
    $auto = [System.Windows.Automation.AutomationElement]
    $cond = New-Object System.Windows.Automation.PropertyCondition($auto::ControlTypeProperty, [System.Windows.Automation.ControlType]::$Type)
    $n = 0
    foreach ($w in (Get-QaWindows -ProcId $ProcId)) { try { $n += $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond).Count } catch { } }
    return $n
}

# Нажать кнопку по точному имени: $true, если нашли
function Invoke-QaButton([int]$ProcId, [string]$Name, [double]$Timeout = 10, [switch]$Mouse) {
    $el = Find-AppEl -ProcId $ProcId -Name $Name -Type 'Button' -Timeout $Timeout
    if (-not $el) { return $false }
    [void](Invoke-AppEl -El $el -Mouse:$Mouse)
    return $true
}

# Причина последнего отказа Open-QaAbout (состояние модуля не видно из сценария напрямую)
function Get-QaAboutWhy { return [string]$script:AboutWhy }

# Окно программы на передний план по PID: запущенная мастером программа не активна (активна консоль раннера), и меню не открывается
function Set-QaAppForeground([int]$ProcId) {
    $ok = $false
    foreach ($top in [SaQa.Wiz]::TopWindows(@($ProcId))) { if ([SaQa.Wiz]::ForegroundNoKey($top)) { $ok = $true } }
    Start-Sleep -Milliseconds 300
    return $ok
}

# «Справка ▸ О программе…»: пункты меню - role=menuitem (имя - по началу, в имени могут быть значок и сочетание).
# Причина отказа - в $script:AboutWhy (пусто, если открылось)
function Open-QaAbout([int]$ProcId) {
    $script:AboutWhy = ''
    if (Find-AppEl -ProcId $ProcId -Name 'Проверить обновления' -Type 'Button' -Timeout 0.5) { return $true }
    Step-Qa 'вывожу окно программы вперёд'
    [void](Set-QaAppForeground $ProcId)
    Step-Qa "ищу 'Справка'"
    $help = Find-AppEl -ProcId $ProcId -NameRegex '^Справка' -Type 'MenuItem' -Timeout 30
    if (-not $help) { $help = Find-AppEl -ProcId $ProcId -NameRegex '^Справка' -Type 'Button' -Timeout 5 }
    if (-not $help) { $script:AboutWhy = "нет 'Справка'"; return $false }
    Step-Qa "нажимаю 'Справка'"
    [void](Invoke-AppEl -El $help)
    Step-Qa "ищу 'О программе'"
    $about = Find-AppEl -ProcId $ProcId -NameRegex '^О программе' -Type 'MenuItem' -Timeout 8
    if (-not $about) { $about = Find-AppEl -ProcId $ProcId -NameRegex '^О программе' -Type 'Button' -Timeout 3 }
    if (-not $about) { $script:AboutWhy = "нет пункта 'О программе'"; return $false }
    Step-Qa "нажимаю 'О программе'"
    [void](Invoke-AppEl -El $about -Mouse)
    Step-Qa 'жду окно О программе'
    if (Find-AppEl -ProcId $ProcId -Name 'Проверить обновления' -Type 'Button' -Timeout 10) { return $true }
    $script:AboutWhy = 'окно не открылось'
    return $false
}

# Процесс программы с окном, которого не было среди $Before; $null за срок - нет
function Wait-QaNewApp([string]$Exe, [int[]]$Before, [double]$Timeout = 60, [scriptblock]$Watch = $null) {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        if ($Watch) { & $Watch }
        foreach ($p in (Get-AppProcs $Exe)) {
            if ($Before -contains $p.Id) { continue }
            if (Get-QaWinRect -ProcId $p.Id) { return $p }
        }
        Start-Sleep -Milliseconds 300
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $null
}
