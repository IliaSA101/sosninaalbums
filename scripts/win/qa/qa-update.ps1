# qa-update.ps1 - блок update (спека 118, design §3, items-map): Г1-Г5, Г9-Г16 (лента обновлений), Е1-Е5 (переносная -> установленная).
# Г6-Г8 идут в блоке install. Одна ВМ - одна цепочка: setup 0.25.0 /S -> config.json стенда -> Г1-Г5 -> Г9-Г16 -> Е1-Е5.
# Сбой пункта не останавливает задание; пункты, которым нужен его результат, получают "не проверено: нет <пункт>".
# -Sabotage: вместо feed лента feed-bad-byte; Г10 и Г13 тогда ПРОВАЛ (критерий приёмки самой проверки).
# Вызывается из run.ps1 (модули уже подключены). Windows PowerShell 5.1, файл в UTF-8 с BOM.
param([string]$Kit, [string]$OutDir, [string]$Dpi = '', [switch]$Sabotage, [string]$RunnerTemp = $env:TEMP)
$ErrorActionPreference = 'Continue'

$P = Get-QaPaths
$S = Get-QaStandPaths
$work = Join-Path $RunnerTemp 'update'
New-Item -ItemType Directory -Force -Path $work | Out-Null
$setup0 = Join-Path $Kit 'setup\SosninaAlbums-0.25.0-windows-x64-setup.exe'
$name1 = 'SosninaAlbums-0.25.1-windows-x64-setup'
$pub = Join-Path $Kit 'pub.txt'
$feedUrl = 'http://127.0.0.1:8765/'
$portableExe = Join-Path $Kit 'work\Sosnina Albums 0.25.0-qa\SosninaAlbums-0.25.0.exe'
$slowPy = Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'slowfeed.py'

# Состояние цепочки в хеш-таблице: тела пунктов идут в дочерних областях, обычные переменные из них не вернуть
$st = @{ Feed = $null; Pid = 0; SS = 0; Mark = [long]0; Ready = $false; Updated = $false; Prep = $false; PrepE = $false }
# На каждом шаге ожиданий: не показал ли Windows SmartScreen / проверку безопасности (Г16, Е3)
$watch = { if (Test-QaSmartScreen) { $st.SS++ } }

$allG = @('Г1', 'Г2', 'Г3', 'Г4', 'Г5', 'Г9', 'Г10', 'Г11', 'Г12', 'Г13', 'Г14', 'Г15', 'Г16')
$allE = @('Е1', 'Е2', 'Е3', 'Е4', 'Е5')

function Write-Utf8([string]$Path, [string]$Text) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
    [System.IO.File]::WriteAllText($Path, $Text, (New-Object System.Text.UTF8Encoding($false)))
}
# Итог пункта: Good - статус при успехе (ok или eyes, если нужен взгляд на снимок)
function Done([string]$Id, $Cond, [string]$Detail, [string]$Good = 'ok', [string[]]$Shots = @(), [string]$Video = '', [double]$Sec = 0) {
    $res = 'fail'
    if ([bool]$Cond) { $res = $Good }
    Add-QaResult -Id $Id -Status $res -Detail $Detail -Shots $Shots -Video $Video -Sec $Sec
}
function Skip-Blocked([string[]]$Ids, [string]$Because) { foreach ($i in $Ids) { Add-QaResult -Id $i -Status skip -Blocked $Because } }
function Stop-AllApps {
    foreach ($ap in @(Get-AppProcs $P.Exe) + @(Get-AppProcs $portableExe)) { [void](Stop-QaApp -ProcId $ap.Id) }
}
function Reset-Updates { Get-ChildItem -LiteralPath $S.Updates -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue }
function Get-UpdateFiles { return @(Get-ChildItem -LiteralPath $S.Updates -Force -ErrorAction SilentlyContinue) }
# Лента: остановить прежнюю, поднять нужную (feed подменяется при -Sabotage)
function Set-Feed([string]$Name) {
    Stop-QaFeed $st.Feed
    $st.Feed = Start-QaFeed $Kit (Get-QaFeedName $Name ([bool]$Sabotage))
    return [bool]$st.Feed
}
# Запуск установленной программы со стендом qa (переменные - как в start-installed.cmd)
function Start-Stand([string]$Arguments = '') {
    $proc = Start-QaApp -Exe $P.Exe -Stand qa -FeedUrl $feedUrl -KeyFile $pub -Arguments $Arguments
    $st.Pid = $proc.Id
    return $proc
}
# Запуск через .cmd набора (проверяется и набор ноутбука); процесс - по пути exe, потому что cmd уходит сразу
function Start-StandCmd([string]$CmdName, [string]$Exe) {
    $before = @(Get-AppProcs $Exe | ForEach-Object { $_.Id })
    [void](Invoke-QaCmdFile (Join-Path $Kit $CmdName))
    $proc = Wait-QaNewApp $Exe $before 40
    if ($proc) { $st.Pid = $proc.Id }
    return $proc
}
# Окно программы готово к щелчкам: в меню есть "Справка"
function Wait-AppReady([int]$ProcId, [double]$Timeout = 40) {
    if (Find-AppEl -ProcId $ProcId -NameRegex '^Справка' -Type 'MenuItem' -Timeout $Timeout) { return $true }
    return [bool](Find-AppEl -ProcId $ProcId -NameRegex '^Справка' -Type 'Button' -Timeout 3)
}
function Get-StandLog([long]$Mark) { return (Read-LogSince $S.Log $Mark) }
function Get-StandCfg {
    try { return ([System.IO.File]::ReadAllText($S.Config) | ConvertFrom-Json) } catch { return $null }
}
# "О программе" -> "Проверить обновления" -> регулярка строки состояния; имя строки или $null
function Check-Updates([int]$ProcId, [string]$Regex, [double]$Timeout = 25) {
    if (-not (Open-QaAbout $ProcId)) { return $null }
    Step-Qa 'нажимаю "Проверить обновления"'
    if (-not (Invoke-QaButton $ProcId 'Проверить обновления' 10)) { return $null }
    return (Wait-QaText -ProcId $ProcId -Regex $Regex -Timeout $Timeout -Watch $watch)
}

# ---------- Альбом для Г9: генератор набора ----------
$alpha = ''
$gen = Join-Path $Kit 'tools\qa-albums.exe'
if (Test-Path -LiteralPath $gen) {
    $o = (& $gen -out (Join-Path $work 'albums') 2>&1 | Out-String)
    $m = [regex]::Match($o, '(?m)^alpha=(.+?)\s*$')
    if ($m.Success -and (Test-Path -LiteralPath $m.Groups[1].Value)) { $alpha = $m.Groups[1].Value } else { Write-Log "qa-albums не дал alpha=: $o" }
}

# ---------- Подготовка: чистая машина, setup 0.25.0 /S, config.json стенда ----------
Invoke-QaItem 'подготовка' -Budget 300 {
    Step-Qa 'сброс: удаление программы, ключей реестра, данных стенда'
    Stop-AllApps
    [void](Invoke-QuietUninstall)
    foreach ($k in @($P.KExt, $P.KProg, $P.KApp)) { Remove-Item -LiteralPath "HKCU:\$k" -Recurse -Force -ErrorAction SilentlyContinue }
    foreach ($d in @($S.Roaming, $S.Local, $P.Roaming, $P.Local)) { Remove-Item -LiteralPath $d -Recurse -Force -ErrorAction SilentlyContinue }
    if (-not (Get-QaPython)) { Write-Log 'нет python в PATH: лента run-feed.cmd не запустится'; return }
    Step-Qa 'тихая установка 0.25.0'
    $code = Invoke-Setup $setup0 '/S'
    $ver = Get-Reg $P.KUninst 'DisplayVersion'
    Write-Log "setup 0.25.0 /S: код $code, DisplayVersion '$ver'"
    if ($code -ne 0 -or $ver -cne '0.25.0') { return }
    Write-Utf8 $S.Config (New-QaStandConfig)
    $st.Prep = $true
}
if (-not $st.Prep) {
    Skip-Blocked ($allG + $allE) 'подготовки (python, setup 0.25.0 /S)'
    return
}

# ---------- Г1-Г2: битый байт установщика ----------
$st.Mark = Get-LogMark $S.Log
Invoke-QaItem 'Г1' -Budget 240 {
    Step-Qa 'лента feed-bad-byte'
    if (-not (Set-Feed 'feed-bad-byte')) { Add-QaResult -Id 'Г1' -Status fail -Detail 'лента feed-bad-byte не поднялась'; return }
    Step-Qa 'запуск start-installed.cmd'
    $app = Start-StandCmd 'start-installed.cmd' $P.Exe
    if (-not $app) { Add-QaResult -Id 'Г1' -Status fail -Detail 'start-installed.cmd не запустил программу'; return }
    $offer = Check-Updates $app.Id '^Вышла версия 0\.25\.1$'
    $before = Get-QaCount $app.Id 'ListItem'
    $notes = Find-AppEl -ProcId $app.Id -NameRegex '^Что нового' -Type 'Button' -Timeout 5
    if ($notes) { [void](Invoke-AppEl -El $notes) }
    Start-Sleep -Milliseconds 600
    $rows = (Get-QaCount $app.Id 'ListItem') - $before
    Step-Qa 'нажимаю "Обновить"'
    $clicked = Invoke-QaButton $app.Id 'Обновить' 10
    Step-Qa 'жду ошибку проверки'
    $err = Wait-QaText -ProcId $app.Id -Regex 'не прошло проверку' -Timeout 40
    $retry = [bool](Find-AppEl -ProcId $app.Id -Name 'Повторить' -Type 'Button' -Timeout 5)
    $page = [bool](Find-AppEl -ProcId $app.Id -Name 'Страница выпуска' -Type 'Button' -Timeout 3)
    $shot = Save-QaShot -Id 'Г1' -Slug 'error' -ProcId $app.Id
    $ok = $offer -and ($rows -ge 2) -and $clicked -and $err -and $retry -and $page
    Done 'Г1' $ok ("версия: '$offer'; строк 'Что нового': $rows; 'Обновить' нажата: $clicked; ошибка: '$err'; 'Повторить': $retry; 'Страница выпуска': $page; полосу хода раннер не ловит (локальная лента отдаёт файл за доли секунды)") 'eyes' @($shot)
}
Invoke-QaItem 'Г2' {
    $files = Get-UpdateFiles
    $bad = @($files | Where-Object { $_.Name -match '\.part$|setup' })
    $procs = @(Get-Process -Name 'SosninaAlbums-0.25.1*' -ErrorAction SilentlyContinue)
    $ver = Get-Reg $P.KUninst 'DisplayVersion'
    $log = Get-StandLog $st.Mark
    $logged = $log -match 'обновление: скачивание не удалось'
    Done 'Г2' (-not $bad.Count -and -not $procs.Count -and $ver -ceq '0.25.0' -and $logged) "лишних файлов в Updates: $($bad.Count), процессов установщика: $($procs.Count), DisplayVersion '$ver', 'скачивание не удалось' в журнале: $logged"
}

# ---------- Г3: чужой ключ ----------
Invoke-QaItem 'Г3' -Budget 240 {
    Stop-AllApps
    if (-not (Set-Feed 'feed-wrong-key')) { Add-QaResult -Id 'Г3' -Status fail -Detail 'лента feed-wrong-key не поднялась'; return }
    Reset-Updates
    $mark = Get-LogMark $S.Log
    $app = Start-Stand
    if (-not (Wait-AppReady $app.Id)) { Add-QaResult -Id 'Г3' -Status fail -Detail 'окно программы не появилось'; return }
    $err = Check-Updates $app.Id 'не прошло проверку'
    $noUpd = -not (Find-AppEl -ProcId $app.Id -Name 'Обновить' -Type 'Button' -Timeout 1.5)
    $shot = Save-QaShot -Id 'Г3' -Slug 'error' -ProcId $app.Id
    $logged = (Get-StandLog $mark) -match 'обновление: проверка не удалась'
    Done 'Г3' ($err -and $noUpd -and $logged) "ошибка: '$err'; кнопки 'Обновить' нет: $noUpd; 'проверка не удалась' в журнале: $logged" 'eyes' @($shot)
}

# ---------- Г4-Г5: установщик "старее" ----------
$g4ok = $false
Start-QaVideo 'Г4'
Enter-QaItem -Id 'Г4' -Budget 300
try {
    Stop-AllApps
    if (-not (Set-Feed 'feed-old-setup')) { Add-QaResult -Id 'Г4' -Status fail -Detail 'лента feed-old-setup не поднялась' }
    else {
        Reset-Updates
        $mark = Get-LogMark $S.Log
        $app = Start-Stand
        $ready = $null
        if (Wait-AppReady $app.Id) {
            $null = Check-Updates $app.Id '^Вышла версия 0\.25\.1$'
            Step-Qa 'нажимаю "Обновить"'
            [void](Invoke-QaButton $app.Id 'Обновить' 10)
            Step-Qa 'жду готовность обновления'
            $ready = Wait-QaText -ProcId $app.Id -Regex '^Обновление 0\.25\.1 готово' -Timeout 60
        }
        if (-not $ready) { Add-QaResult -Id 'Г4' -Status fail -Detail 'обновление не дошло до готовности' }
        else {
            $before = @(Get-AppProcs $P.Exe | ForEach-Object { $_.Id })
            Step-Qa 'нажимаю "Перезапустить и обновить"'
            [void](Invoke-QaButton $app.Id 'Перезапустить и обновить' 10 -Mouse)
            Step-Qa 'жду выход старого процесса и запуск нового'
            $gone = Wait-QaCond { -not (Get-Process -Id $app.Id -ErrorAction SilentlyContinue) } 40
            $new = Wait-QaNewApp $P.Exe $before 60
            $toast = $null
            if ($new) { $toast = Wait-QaText -ProcId $new.Id -Regex '^Обновление до 0\.25\.1 не установилось' -Timeout 25 }
            $hang = Wait-QaCond { @(Get-Process -Name 'SosninaAlbums-0.25.1*' -ErrorAction SilentlyContinue).Count -eq 0 } 20
            $shot = ''
            if ($new) { $shot = Save-QaShot -Id 'Г4' -Slug 'toast' -ProcId $new.Id }
            $log = Get-StandLog $mark
            $starts = ([regex]::Matches($log, 'запуск, версия 0\.25\.0')).Count
            $resLine = $log -match 'итог запуска установщика 0\.25\.1 \S+ обновлено "", не установилось "0\.25\.1"'
            $okAll = $gone -and $new -and $toast -and $hang -and ($starts -ge 2) -and $resLine
            $g4ok = [bool]$okAll
            Add-QaResult -Id 'Г4' -Status $(if ($okAll) { 'eyes' } else { 'fail' }) -Detail "старый процесс вышел: $gone; новый процесс: $([bool]$new); тост: '$toast'; запусков 0.25.0 в журнале: $starts; итог 'не установилось': $resLine; процессов установщика не осталось: $hang" -Shots @($shot)
        }
    }
} catch {
    Add-QaResult -Id 'Г4' -Status fail -Detail ('исключение: ' + $_.Exception.Message)
}
Exit-QaItem
$vid4 = Stop-QaVideo
if ($vid4) { Write-Log "ролик Г4: $vid4" }

Invoke-QaItem 'Г5' -Budget 120 {
    $alive = @(Get-AppProcs $P.Exe)
    foreach ($ap in $alive) { [void](Stop-QaApp -ProcId $ap.Id) }
    Step-Qa 'мягкий перезапуск'
    $app = Start-Stand
    $up = Wait-AppReady $app.Id
    $toast = Wait-QaText -ProcId $app.Id -Regex 'не установилось' -Timeout 6
    $cfg = Get-StandCfg
    $pending = ''
    if ($cfg -and $cfg.update -and $cfg.update.pending) { $pending = [string]$cfg.update.pending }
    Done 'Г5' ($up -and -not $toast -and -not $pending) "окно: $up; тост 'не установилось' снова: '$toast'; update.pending: '$pending'"
}

# ---------- Г9-Г16: настоящее обновление 0.25.0 -> 0.25.1 ----------
$mark9 = Get-LogMark $S.Log
$g9 = $false
Invoke-QaItem 'Г9' -Budget 300 {
    Stop-AllApps
    Reset-Updates
    if (-not (Set-Feed 'feed')) { Add-QaResult -Id 'Г9' -Status fail -Detail 'лента feed не поднялась'; return }
    $args9 = ''
    if ($alpha) { $args9 = '"' + $alpha + '"' }
    Step-Qa 'запуск с Alpha.salbum в аргументе'
    $app = Start-Stand $args9
    $up = Wait-AppReady $app.Id 60
    $title = ''
    $w = @(Get-QaWindows -ProcId $app.Id) | Select-Object -First 1
    if ($w) { try { $title = [string]$w.Current.Name } catch { } }
    $offer = $null
    if ($up) { $offer = Check-Updates $app.Id '^Вышла версия 0\.25\.1$' }
    $before = Get-QaCount $app.Id 'ListItem'
    $notes = Find-AppEl -ProcId $app.Id -NameRegex '^Что нового' -Type 'Button' -Timeout 5
    if ($notes) { [void](Invoke-AppEl -El $notes) }
    Start-Sleep -Milliseconds 600
    $rows = (Get-QaCount $app.Id 'ListItem') - $before
    $shot = Save-QaShot -Id 'Г9' -Slug 'offer' -ProcId $app.Id
    $alb = 'без Alpha (генератора в наборе нет)'
    if ($alpha) { $alb = "заголовок окна '$title'" }
    $st.G9 = [bool]($up -and $offer -and $rows -ge 2)
    Done 'Г9' $st.G9 "окно: $up; $alb; версия: '$offer'; строк 'Что нового': $rows; правку в Alpha раннер не вносит, сохранность альбома - по Г13" 'eyes' @($shot)
}

Invoke-QaItem 'Г10' -Budget 300 {
    if (-not $st.G9) { Add-QaResult -Id 'Г10' -Status skip -Blocked 'Г9'; return }
    $id = $st.Pid
    Step-Qa 'медленная лента: успеть нажать "Отменить"'
    Stop-QaFeed $st.Feed; $st.Feed = $null
    $st.Feed = Start-QaSlowFeed $Kit (Get-QaFeedName 'feed' ([bool]$Sabotage)) $slowPy 1000000
    if (-not $st.Feed) { Add-QaResult -Id 'Г10' -Status fail -Detail 'медленная лента не поднялась'; return }
    [void](Invoke-QaButton $id 'Обновить' 10)
    $prog = Wait-QaText -ProcId $id -Regex '^Скачиваем обновление' -Timeout 20
    $cancelled = $false
    $back = $null
    if ($prog -and (Invoke-QaButton $id 'Отменить' 10)) {
        $cancelled = $true
        $back = Wait-QaText -ProcId $id -Regex '^Вышла версия 0\.25\.1$' -Timeout 20
    }
    $noPart = Wait-QaCond { @(Get-UpdateFiles | Where-Object { $_.Name -match '\.part$' }).Count -eq 0 } 15
    Step-Qa 'обычная лента: "Обновить" до конца'
    if (-not (Set-Feed 'feed')) { Add-QaResult -Id 'Г10' -Status fail -Detail 'лента feed не поднялась после отмены'; return }
    [void](Invoke-QaButton $id 'Обновить' 10)
    $ready = Wait-QaText -ProcId $id -Regex '^Обновление 0\.25\.1 готово' -Timeout 90 -Watch $watch
    $st.Ready = [bool]$ready
    Done 'Г10' ($prog -and $cancelled -and $back -and $noPart -and $ready) "полоса хода: '$prog'; 'Отменить' нажата: $cancelled; вернулось: '$back'; .part исчез: $noPart; затем скачано до конца: '$ready'"
}

Invoke-QaItem 'Г11' -Budget 120 {
    if (-not $st.Ready) { Add-QaResult -Id 'Г11' -Status skip -Blocked 'Г10'; return }
    $id = $st.Pid
    $line = Wait-QaText -ProcId $id -Regex '^Обновление 0\.25\.1 готово\. Программа закроется, установит его и откроется снова; альбом сохранится$' -Timeout 10
    $btn = [bool](Find-AppEl -ProcId $id -Name 'Перезапустить и обновить' -Type 'Button' -Timeout 5)
    $toast = [bool](Find-AppEl -ProcId $id -Name 'Перезапустить' -Type 'Button' -Timeout 2)
    $shot = Save-QaShot -Id 'Г11' -Slug 'ready' -ProcId $id
    Done 'Г11' ($line -and $btn) "строка: '$line'; кнопка 'Перезапустить и обновить': $btn; тост с кнопкой 'Перезапустить' виден: $toast (тост исчезает по таймеру, мог уйти)" 'eyes' @($shot)
}

Invoke-QaItem 'Г12' {
    if (-not $st.Ready) { Add-QaResult -Id 'Г12' -Status skip -Blocked 'Г10'; return }
    $files = @(Get-ChildItem -LiteralPath $S.Updates -Filter '*setup.exe' -ErrorAction SilentlyContinue)
    $names = @()
    foreach ($f in $files) { $names += @(Get-Item -LiteralPath $f.FullName -Stream * | ForEach-Object { $_.Stream }) }
    $r = Test-QaOnlyData $names
    Done 'Г12' ($files.Count -gt 0 -and $r.Ok) "установщиков в Updates: $($files.Count); потоки: $($names -join ', '); лишних: $($r.Extra -join ', ')"
}

# Г13: самое важное - "Перезапустить и обновить" и новый процесс 0.25.1 с окном. Без готовности - ПРОВАЛ, а не "не проверено"
Start-QaVideo 'Г13'
Enter-QaItem -Id 'Г13' -Budget 300
try {
    if (-not $st.Ready) { Add-QaResult -Id 'Г13' -Status fail -Detail 'обновление не дошло до готовности (Г10): перезапускать нечего; при -Sabotage так и должно быть' }
    else {
        $id = $st.Pid
        $before = @(Get-AppProcs $P.Exe | ForEach-Object { $_.Id })
        $mark13 = Get-LogMark $S.Log
        $sw = [Diagnostics.Stopwatch]::StartNew()
        Step-Qa 'нажимаю "Перезапустить и обновить"'
        $clicked = Invoke-QaButton $id 'Перезапустить и обновить' 10 -Mouse
        Step-Qa 'жду новый процесс 0.25.1 с окном'
        $new = Wait-QaNewApp $P.Exe $before 90 $watch
        $sec = $sw.Elapsed.TotalSeconds
        $oldGone = -not (Get-Process -Id $id -ErrorAction SilentlyContinue)
        $ver = ''
        [void](Wait-QaCond { (Get-Reg $P.KUninst 'DisplayVersion') -ceq '0.25.1' } 15)
        $ver = Get-Reg $P.KUninst 'DisplayVersion'
        $toast = $null
        $title = ''
        $shot = ''
        if ($new) {
            $st.Pid = $new.Id
            $toast = Wait-QaText -ProcId $new.Id -Regex '^Программа обновлена до 0\.25\.1$' -Timeout 25 -Watch $watch
            $w = @(Get-QaWindows -ProcId $new.Id) | Select-Object -First 1
            if ($w) { try { $title = [string]$w.Current.Name } catch { } }
            $shot = Save-QaShot -Id 'Г13' -Slug 'toast' -ProcId $new.Id
        }
        $okAll = $clicked -and $new -and $oldGone -and ($sec -le 60) -and ($ver -ceq '0.25.1') -and $toast
        $st.Updated = [bool]$okAll
        $alb = 'без Alpha'
        if ($alpha) { $alb = "заголовок окна '$title' (Alpha - по снимку)" }
        Add-QaResult -Id 'Г13' -Status $(if ($okAll) { 'eyes' } else { 'fail' }) -Detail ("{0:N1} с от нажатия до окна 0.25.1 (предел 60); старый вышел: $oldGone; DisplayVersion '$ver'; тост: '$toast'; $alb; правка в Alpha - глазами по снимку" -f $sec) -Shots @($shot) -Sec $sec
    }
} catch {
    Add-QaResult -Id 'Г13' -Status fail -Detail ('исключение: ' + $_.Exception.Message)
}
Exit-QaItem
$vid13 = Stop-QaVideo
if ($vid13) { Write-Log "ролик Г13: $vid13" }

Invoke-QaItem 'Г14' -Budget 120 {
    if (-not $st.Updated) { Add-QaResult -Id 'Г14' -Status skip -Blocked 'Г13'; return }
    $chain = Test-QaUpdateChain (Get-StandLog $mark9) '0.25.1'
    $ver = Get-Reg $P.KUninst 'DisplayVersion'
    $line = Check-Updates $st.Pid '^Установлена последняя версия$'
    Done 'Г14' ($chain.Ok -and $ver -ceq '0.25.1' -and $line) "цепочка журнала: $($chain.Ok) (нет: $($chain.Missing -join ', ')); DisplayVersion '$ver'; после 'Проверить': '$line'; 'О программе' не открылась из-за: '$(Get-QaAboutWhy)'"
}

Invoke-QaItem 'Г15' -Budget 120 {
    if (-not $st.Updated) { Add-QaResult -Id 'Г15' -Status skip -Blocked 'Г13'; return }
    Stop-AllApps
    $app = Start-Stand
    $up = Wait-AppReady $app.Id
    $empty = Wait-QaCond { (Get-UpdateFiles).Count -eq 0 } 20
    $toast = Wait-QaText -ProcId $app.Id -Regex '^Программа обновлена' -Timeout 5
    Done 'Г15' ($up -and $empty -and -not $toast) "окно: $up; Updates пуста: $empty ($((Get-UpdateFiles).Count) файлов); тост 'Программа обновлена' снова: '$toast'"
}

Invoke-QaItem 'Г16' {
    $now = Test-QaSmartScreen
    Done 'Г16' (-not $now -and $st.SS -eq 0) "опросов, где видно SmartScreen или проверку безопасности: $($st.SS) (опрос на каждом шаге ожиданий Г10-Г15); сейчас окно есть: $now"
}

# ---------- Е: переносная -> установленная ----------
Invoke-QaItem 'Е-подготовка' -Budget 300 {
    Step-Qa 'удаление установленной, стенд чистый, переносная из zip'
    Stop-AllApps
    [void](Invoke-QuietUninstall)
    $gone = -not (Test-Path -LiteralPath $P.Exe)
    [void](Invoke-QaCmdFile (Join-Path $Kit 'stand-clean.cmd'))
    $zip = Join-Path $Kit 'zip\SosninaAlbums-0.25.0-qa-windows-x64.zip'
    Remove-Item -LiteralPath (Join-Path $Kit 'work') -Recurse -Force -ErrorAction SilentlyContinue
    Expand-Archive -LiteralPath $zip -DestinationPath (Join-Path $Kit 'work') -Force
    $haveExe = Test-Path -LiteralPath $portableExe
    $feed = Set-Feed 'feed'
    Write-Utf8 $S.Config (New-QaStandConfig)
    Write-Log "Е: установленная удалена: $gone; переносная распакована: $haveExe; лента: $feed"
    $st.PrepE = [bool]($gone -and $haveExe -and $feed)
}
if (-not $st.PrepE) { Skip-Blocked $allE 'подготовки блока Е (удаление, zip, лента)' } else {
    Invoke-QaItem 'Е1' -Budget 180 {
        $app = Start-StandCmd 'start-0.25.0.cmd' $portableExe
        if (-not $app) { Add-QaResult -Id 'Е1' -Status fail -Detail 'start-0.25.0.cmd не запустил переносную копию'; return }
        $st.PortPid = $app.Id
        $up = Wait-AppReady $app.Id 60
        $opened = $up -and (Open-QaAbout $app.Id)
        $hint = Wait-QaText -ProcId $app.Id -Regex '^Поставьте установщиком' -Timeout 10
        $shot = Save-QaShot -Id 'Е1' -Slug 'hint' -ProcId $app.Id
        Done 'Е1' ($opened -and $hint) "окно: $up; 'О программе': $opened; подсказка: '$hint'" 'eyes' @($shot)
    }
    Invoke-QaItem 'Е2' -Budget 240 {
        if (-not $st.PortPid) { Add-QaResult -Id 'Е2' -Status skip -Blocked 'Е1'; return }
        $id = $st.PortPid
        $offer = Check-Updates $id '^Вышла версия 0\.25\.1$'
        Step-Qa 'нажимаю "Обновить"'
        [void](Invoke-QaButton $id 'Обновить' 10)
        $line = Wait-QaText -ProcId $id -Regex '^Обновление 0\.25\.1 готово\. Программа закроется и запустит установщик$' -Timeout 90
        $btn = [bool](Find-AppEl -ProcId $id -Name 'Перезапустить и установить' -Type 'Button' -Timeout 5)
        $st.ReadyE = [bool]($line -and $btn)
        Done 'Е2' $st.ReadyE "версия: '$offer'; строка: '$line'; кнопка 'Перезапустить и установить': $btn"
    }
    Invoke-QaItem 'Е3' -Budget 360 {
        if (-not $st.ReadyE) { Add-QaResult -Id 'Е3' -Status skip -Blocked 'Е2'; return }
        $id = $st.PortPid
        $ss0 = $st.SS
        $setupBefore = @(Get-Process -Name $name1 -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
        Start-QaVideo 'Е3'
        Step-Qa 'нажимаю "Перезапустить и установить"'
        [void](Invoke-QaButton $id 'Перезапустить и установить' 10 -Mouse)
        $gone = Wait-QaCond { -not (Get-Process -Id $id -ErrorAction SilentlyContinue) } 40 $watch
        Step-Qa 'мастер установки (не тихий)'
        $w = Invoke-QaWizard -ProcNames @($name1) -Id 'Е3'
        $st.WizOk = [bool]$w.Ok
        # Не дошёл до конца: закрыть мастер по PID, иначе он держит Е4 и следующий блок
        if (-not $w.Ok) { Stop-QaNewProcs -Name $name1 -Before $setupBefore }
        $vid3 = Stop-QaVideo
        & $watch
        $shots = @($w.Pages | ForEach-Object { $_.Shot })
        $noSs = ($st.SS -eq $ss0)
        $first = ''
        if ($w.Pages.Count) { $first = $w.Pages[0].Text }
        Done 'Е3' ($gone -and $w.Ok -and $noSs) ("программа вышла: $gone; мастер с окнами дошёл до конца: $($w.Ok) $($w.Why); SmartScreen не показан: $noSs; первая страница: '$first'; нажато: $(@($w.Via) -join ', ')") 'eyes' $shots $vid3
    }
    Invoke-QaItem 'Е4' -Budget 180 {
        if (-not $st.ReadyE) { Add-QaResult -Id 'Е4' -Status skip -Blocked 'Е2'; return }
        if (-not $st.WizOk) { Add-QaResult -Id 'Е4' -Status skip -Blocked 'Е3' -Detail 'нет Е3: мастер установки не дошёл до конца'; return }
        $verOk = Wait-QaCond { (Get-Reg $P.KUninst 'DisplayVersion') -ceq '0.25.1' } 20
        # Мастер Е3 с финишным флажком мог сам запустить установленную программу (ожидаемо при обновлении): второй экземпляр
        # SingleInstance сразу передал бы первому, и новый процесс не нашёлся бы - работаем с уже запущенной
        $app = @(Get-AppProcs $P.Exe) | Select-Object -First 1
        $how = 'запущена мастером после установки'
        if ($app) { $st.Pid = $app.Id } else { $how = 'start-installed.cmd'; $app = Start-StandCmd 'start-installed.cmd' $P.Exe }
        $about = $false
        $hint = $null
        if ($app) {
            $about = (Wait-AppReady $app.Id 60) -and (Open-QaAbout $app.Id)
            if (-not $about) {
                [void](Save-QaShot -Id 'Е4' -Slug 'about' -ProcId $app.Id)
                Write-QaUiaDiag -ProcId $app.Id -Tag 'E4'
            }
            $hint = Wait-QaText -ProcId $app.Id -Regex '^Поставьте установщиком' -Timeout 4
        }
        $kept = Test-Path -LiteralPath $portableExe
        Done 'Е4' ($verOk -and $app -and $about -and -not $hint -and $kept) "DisplayVersion 0.25.1: $verOk; программа запущена: $([bool]$app) ($how); 'О программе': $about ($(Get-QaAboutWhy)); подсказка про установщик: '$hint'; переносная копия цела: $kept"
    }
    Invoke-QaItem 'Е5' {
        $log = Get-StandLog 0
        $ok = $log -match 'обновление: запущен установщик версии 0\.25\.1 \(setup\)'
        Done 'Е5' $ok "'запущен установщик версии 0.25.1 (setup)' в журнале стенда: $ok"
    }
}

Stop-AllApps
Stop-QaFeed $st.Feed
