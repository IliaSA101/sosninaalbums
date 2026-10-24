# qa-open.ps1 - блок open (спека 118, design §3, items-map): А1-А14 (первый запуск rel, связь, тур), Б1-Б9 (двойной щелчок по .salbum),
# Д1-Д7 (окно «Альбом из новой версии»). Одна ВМ - одна цепочка: rel из zip с меткой -> А -> Б -> setup 0.25.1 /S -> Д.
# Файл только общая часть и Б; А - qa-open-a.ps1, Д - qa-open-d.ps1 (подключаются точкой, переменные общие).
# Сбой пункта не останавливает задание; пункты, которым нужен его результат, получают "не проверено: нет <пункт>".
# Отличия от items-map: в А программа rel с проверкой обновлений выключенной (config.update.off: адрес GitHub и сеть портят А14);
# Б7 (старая 0.24 и «Всегда» в окне «Чем открыть») - ноутбук; в Д конфиг assoc=off, чтобы rel не перехватывала связь у установленной.
# Вызывается из run.ps1 (модули уже подключены). Windows PowerShell 5.1, файл в UTF-8 с BOM.
param([string]$Kit, [string]$OutDir, [string]$Dpi = '', [switch]$Sabotage, [string]$RunnerTemp = $env:TEMP)
$ErrorActionPreference = 'Continue'

$P = Get-QaPaths
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$work = Join-Path $RunnerTemp 'open'
New-Item -ItemType Directory -Force -Path $work | Out-Null
$setup1 = Join-Path $Kit 'setup\SosninaAlbums-0.25.1-windows-x64-setup.exe'
$relZip = Join-Path $Kit 'zip\SosninaAlbums-0.25.0-rel-windows-x64.zip'
$dl = Join-Path $env:USERPROFILE 'Downloads'
$relRoot = Join-Path $dl 'qa-rel'
$relName = 'Sosnina Albums 0.25.0-rel'  # папка внутри zip; реальный путь exe уточняет подготовка А
$albumsDir = Join-Path $work 'albums'
$photosDir = Join-Path $albumsDir 'photos'
$newer = @{
    N0251   = Join-Path $Kit 'albums\NewerTest-0.25.1.salbum'
    N026    = Join-Path $Kit 'albums\NewerTest-0.26.salbum'
    Unknown = Join-Path $Kit 'albums\NewerTest-unknown.salbum'
}

# Состояние цепочки в хеш-таблице: тела пунктов идут в дочерних областях, обычные переменные из них не вернуть
$st = @{
    RelExe = (Join-Path $relRoot "$relName\SosninaAlbums-0.25.0.exe"); Pid = 0
    PrepA = $false; A1 = $false; Tour = $false; PrepB = $false; PrepD = $false; Alpha = ''; Beta = ''; OpenWith = 0
}
$allA = 1..14 | ForEach-Object { 'А' + $_ }
$allB = 1..9 | ForEach-Object { 'Б' + $_ }
$allD = 1..7 | ForEach-Object { 'Д' + $_ }

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
    foreach ($ap in @(Get-AppProcs $st.RelExe) + @(Get-AppProcs $P.Exe)) { [void](Stop-QaApp -ProcId $ap.Id) }
}
# rel без стенда и ленты: настоящий профиль программы (APPDATA\SosninaAlbums), как у фотографа
function Start-Rel([string]$Arguments = '') {
    $proc = Start-QaApp -Exe $st.RelExe -Arguments $Arguments
    $st.Pid = $proc.Id
    return $proc
}
function Quote([string]$Path) { return '"' + $Path + '"' }
# Окно готово к щелчкам: в меню есть «Файл»
function Wait-Ready([int]$ProcId, [double]$Timeout = 40) { return (Wait-QaMenu $ProcId $Timeout) }
function Get-Cfg { return (Get-QaCfgFile $P.Config) }
function Get-LastLeaf {
    $c = Get-Cfg
    if ($c -and $c.lastProject) { return (Split-Path -Leaf ([string]$c.lastProject)) }
    return ''
}
function Test-Alive([int]$ProcId) { return [bool](Get-Process -Id $ProcId -ErrorAction SilentlyContinue) }
function Get-LogSince([long]$Mark) { return (Read-LogSince $P.Log $Mark) }
# Нажать кнопку по началу имени (Chromium берёт имя кнопки из содержимого: «Продолжить: Черновик»)
function Click-Prefix([int]$ProcId, [string]$Prefix, [double]$Timeout = 10, [switch]$Mouse) {
    $el = Find-AppEl -ProcId $ProcId -NameRegex ('^' + [regex]::Escape($Prefix) + '(:|$)') -Type 'Button' -Timeout $Timeout
    if (-not $el) { return $false }
    [void](Invoke-AppEl -El $el -Mouse:$Mouse)
    return $true
}
function Test-Button([int]$ProcId, [string]$Name, [double]$Timeout = 5) { return [bool](Find-AppEl -ProcId $ProcId -Name $Name -Type 'Button' -Timeout $Timeout) }
# Закрыть тур как в программе: «Пропустить» на карточке (если есть)
function Close-TourCard([int]$ProcId) { [void](Invoke-QaButton $ProcId 'Пропустить' 3) }
# Окно «Чем открыть» после щелчка: закрыть чужое (наше новое) окно по PID
function Close-OpenWith([int[]]$Before) {
    foreach ($a in @(Get-Process -Name 'OpenWith' -ErrorAction SilentlyContinue | Where-Object { $Before -notcontains $_.Id })) {
        $st.OpenWith++
        Stop-Process -Id $a.Id -Force -ErrorAction SilentlyContinue
    }
}
function Get-OpenWithIds { return @(Get-Process -Name 'OpenWith' -ErrorAction SilentlyContinue | ForEach-Object { $_.Id }) }

# ---------- Альбомы: генератор набора ----------
$gen = Join-Path $Kit 'tools\qa-albums.exe'
if (Test-Path -LiteralPath $gen) {
    $o = (& $gen -out $albumsDir 2>&1 | Out-String)
    $ma = [regex]::Match($o, '(?m)^alpha=(.+?)\s*$')
    $mb = [regex]::Match($o, '(?m)^beta=(.+?)\s*$')
    if ($ma.Success -and $mb.Success -and (Test-Path -LiteralPath $ma.Groups[1].Value) -and (Test-Path -LiteralPath $mb.Groups[1].Value)) {
        $st.Alpha = $ma.Groups[1].Value
        $st.Beta = $mb.Groups[1].Value
    } else { Write-Log "qa-albums не дал alpha= и beta=: $o" }
}

# ---------- Фаза А ----------
. (Join-Path $here 'qa-open-a.ps1')
if (-not $st.PrepA) { return }

# ---------- Фаза Б ----------
Invoke-QaItem 'Б-подготовка' -Budget 240 {
    Stop-AllApps
    if (-not $st.Alpha) { Write-Log 'нет Alpha/Beta (генератора нет в наборе)'; return }
    # lastProject = Beta: «открывается на этом альбоме, а не на последнем» видно по Alpha
    Write-Utf8 $P.Config (New-QaOpenConfig $st.Beta 'on')
    Step-Qa 'запуск rel: связь должна ехать на неё'
    $app = Start-Rel
    $up = Wait-Ready $app.Id 60
    $reg = Wait-QaCond { Test-QaAssocCommand (Get-Reg ($P.KProg + '\shell\open\command') '') $st.RelExe } 15
    Write-Log "Б: окно rel: $up; связь на rel: $reg"
    Stop-AllApps
    $st.PrepB = [bool]($up -and $reg)
}
if (-not $st.PrepB) {
    Skip-Blocked ($allB + $allD) 'подготовки Б (альбомы, связь на rel)'
    Stop-AllApps
    return
}

Invoke-QaItem 'Б1' -Budget 240 {
    $mark = Get-LogMark $P.Log
    $ow = Get-OpenWithIds
    Step-Qa 'двойной щелчок по Alpha.salbum, программа закрыта'
    $clicked = Invoke-QaAlbumDblClick $st.Alpha
    Step-Qa 'жду окно rel'
    $app = Wait-QaNewApp $st.RelExe @() 45
    $same = $false
    $title = ''
    if ($app) {
        $st.Pid = $app.Id
        $same = Wait-QaCond { (Get-LastLeaf) -ceq 'Alpha.salbum' } 20
        $title = Get-QaWinTitle $app.Id
    }
    Start-Sleep -Seconds 2
    $procs = @(Get-AppProcs $st.RelExe).Count
    $logged = Wait-QaCond { (Get-LogSince $mark) -match 'альбом из командной строки: открыт' } 10
    $asked = @(Get-Process -Name 'OpenWith' -ErrorAction SilentlyContinue | Where-Object { $ow -notcontains $_.Id }).Count
    $shot = Save-QaShot -Id 'Б1' -Slug 'open' -ProcId $st.Pid
    Close-OpenWith $ow
    Close-ExplorerWindows
    # повтор без Проводника: cmd /c start (так же открывает ShellExecute)
    $viaCmd = $false
    if ($app) {
        Stop-AllApps
        Write-Utf8 $P.Config (New-QaOpenConfig $st.Beta 'on')
        Step-Qa 'повтор: cmd /c start'
        $before = @()
        [void](Start-Process -FilePath $env:ComSpec -ArgumentList @('/d', '/c', 'start', '""', (Quote $st.Alpha)) -WindowStyle Hidden)
        $again = Wait-QaNewApp $st.RelExe $before 45
        if ($again) {
            $st.Pid = $again.Id
            $viaCmd = Wait-QaCond { (Get-LastLeaf) -ceq 'Alpha.salbum' } 20
        }
    }
    $ok = $clicked -and $app -and $same -and ($procs -eq 1) -and $logged -and $viaCmd
    Done 'Б1' $ok "щелчок в Проводнике: $clicked; окно rel: $([bool]$app); заголовок '$title'; lastProject = Alpha: $same; процессов: $procs; строка журнала 'открыт': $logged; 'Чем открыть' спросило: $($asked -gt 0); повтор через cmd start открыл Alpha: $viaCmd" 'eyes' @($shot)
}

# Альбом открыт в живой rel: если нет (Б1 не вышел), открываем его аргументом, а не считаем Б2 непроверенным
function Ensure-Open([string]$AlbumPath) {
    $leaf = Split-Path -Leaf $AlbumPath
    if ($st.Pid -gt 0 -and (Test-Alive $st.Pid) -and ((Get-LastLeaf) -ceq $leaf)) { return $true }
    Stop-AllApps
    $app = Start-Rel (Quote $AlbumPath)
    return (Wait-Ready $app.Id 60)
}

Invoke-QaItem 'Б2' -Budget 240 {
    if (-not (Ensure-Open $st.Alpha)) { Add-QaResult -Id 'Б2' -Status fail -Detail 'Alpha не открылась'; return }
    $id = $st.Pid
    $hash0 = (Get-FileHash -LiteralPath $st.Alpha -Algorithm SHA256).Hash
    $sp0 = Get-QaSpreadCount $st.Alpha
    $mark = Get-LogMark $P.Log
    Step-Qa 'двойной щелчок по Beta.salbum, Alpha открыта'
    $clicked = Invoke-QaAlbumDblClick $st.Beta
    $moved = Wait-QaCond { (Get-LastLeaf) -ceq 'Beta.salbum' } 20
    Start-Sleep -Seconds 2
    $procs = @(Get-AppProcs $st.RelExe | ForEach-Object { $_.Id })
    $front = Get-QaForegroundPid
    $shot = Save-QaShot -Id 'Б2' -Slug 'front' -ProcId $id
    $hash1 = (Get-FileHash -LiteralPath $st.Alpha -Algorithm SHA256).Hash
    $sp1 = Get-QaSpreadCount $st.Alpha
    $intact = ($hash0 -ceq $hash1) -or (($sp0 -gt 0) -and ($sp0 -eq $sp1))
    $same = ($procs.Count -eq 1) -and ($procs[0] -eq $id)
    $ok = $clicked -and $moved -and $same -and $intact
    Done 'Б2' $ok "щелчок: $clicked; lastProject = Beta: $moved; тот же процесс (PID $id), всего процессов $($procs.Count): $same; Alpha цела (хеш совпал: $($hash0 -ceq $hash1), разворотов $sp0 -> $sp1): $intact; окно вышло вперёд: $($front -eq $id) (на переднем плане PID $front); строка журнала: $((Get-LogSince $mark) -match 'альбом из командной строки: уже открыт|открыт')" 'eyes' @($shot)
}

Invoke-QaItem 'Б3' -Budget 240 {
    if (-not (Ensure-Open $st.Beta)) { Add-QaResult -Id 'Б3' -Status fail -Detail 'Beta не открылась'; return }
    $id = $st.Pid
    $mark = Get-LogMark $P.Log
    $about = Open-QaAbout $id
    Step-Qa 'двойной щелчок по Alpha при открытом окне'
    $clicked = $about -and (Invoke-QaAlbumDblClick $st.Alpha)
    $toast = Wait-QaText -ProcId $id -Regex '^Альбом «Alpha» откроется, когда закроете окно$' -Timeout 12
    $shot = Save-QaShot -Id 'Б3' -Slug 'toast' -ProcId $id
    $still = (Get-LastLeaf) -cne 'Alpha.salbum'
    Step-Qa 'закрываю окно "О программе"'
    $btn = Find-AppEl -ProcId $id -Name 'Проверить обновления' -Type 'Button' -Timeout 3
    if ($btn) { try { $btn.SetFocus() } catch { } }
    Send-Keys @('Esc')
    $opened = Wait-QaCond { (Get-LastLeaf) -ceq 'Alpha.salbum' } 20
    $queued = (Get-LogSince $mark) -match 'альбом из командной строки: отложен до закрытия окна'
    Done 'Б3' ($about -and $clicked -and $toast -and $opened) "'О программе' открыто: $about; тост: '$toast'; до закрытия окна Alpha не открылась: $still; после закрытия lastProject = Alpha: $opened; строка журнала 'отложен': $queued" 'eyes' @($shot)
}

Invoke-QaItem 'Б4' -Budget 180 {
    if (-not (Ensure-Open $st.Alpha)) { Add-QaResult -Id 'Б4' -Status fail -Detail 'Alpha не открылась'; return }
    $id = $st.Pid
    Step-Qa 'сворачиваю окно'
    [void](Set-QaMinimized $id)
    $min = Wait-QaCond { Test-QaIconic $id } 6
    if (-not $min) { Add-QaResult -Id 'Б4' -Status fail -Detail 'окно не свернулось (ShowWindow SW_MINIMIZE)'; return }
    Step-Qa 'двойной щелчок по Beta.salbum, окно свёрнуто'
    $clicked = Invoke-QaAlbumDblClick $st.Beta
    $shown = Wait-QaCond { -not (Test-QaIconic $id) } 20
    $moved = Wait-QaCond { (Get-LastLeaf) -ceq 'Beta.salbum' } 20
    Done 'Б4' ($clicked -and $shown -and $moved) "свёрнуто: $min; щелчок: $clicked; окно развернулось: $shown; lastProject = Beta: $moved"
}

Invoke-QaItem 'Б5' -Budget 240 {
    if (-not (Ensure-Open $st.Beta)) { Add-QaResult -Id 'Б5' -Status fail -Detail 'Beta не открылась'; return }
    $id = $st.Pid
    $n0 = Get-QaSpreadCount $st.Beta
    $mark = Get-LogMark $P.Log
    Step-Qa 'правка: "+ Новый разворот"'
    $plus = Find-AppEl -ProcId $id -Name 'Новый разворот' -Type 'Button' -Timeout 15
    if (-not $plus) { Add-QaResult -Id 'Б5' -Status fail -Detail "кнопка «Новый разворот» («+» в ленте) не найдена по имени: проверьте aria-label у кнопки в AddColumn.svelte"; return }
    [void](Invoke-AppEl -El $plus -Mouse)
    Start-Sleep -Seconds 2
    Step-Qa 'двойной щелчок по тому же альбому'
    $clicked = Invoke-QaAlbumDblClick $st.Beta
    $already = Wait-QaCond { (Get-LogSince $mark) -match 'альбом из командной строки: уже открыт' } 10
    $alive = Test-Alive $id
    $how = Stop-QaApp -ProcId $id
    $n1 = Get-QaSpreadCount $st.Beta
    Done 'Б5' ($clicked -and $alive -and ($n0 -ge 0) -and ($n1 -eq $n0 + 1)) "щелчок: $clicked; процесс жив: $alive; строка 'уже открыт': $already; программа $how; разворотов в Beta: $n0 -> $n1 (ждали +1)"
}

Invoke-QaItem 'Б6' -Budget 240 {
    Stop-AllApps
    Start-Sleep -Seconds 2
    $new = Join-Path $work 'moved'
    New-Item -ItemType Directory -Force -Path $new | Out-Null
    $oldDir = Split-Path -Parent $st.RelExe
    $moved = $false
    try { Move-Item -LiteralPath $oldDir -Destination $new -Force -ErrorAction Stop; $moved = $true } catch { Write-Log "Б6: перенос: $($_.Exception.Message)" }
    if (-not $moved) { Add-QaResult -Id 'Б6' -Status fail -Detail 'папку rel перенести не удалось'; return }
    $st.RelExe = Join-Path (Join-Path $new (Split-Path -Leaf $oldDir)) (Split-Path -Leaf $st.RelExe)
    Write-Utf8 $P.Config (New-QaOpenConfig $st.Beta 'on')
    Step-Qa 'запуск rel из нового места'
    $app = Start-Rel
    $up = Wait-Ready $app.Id 60
    $reg = Wait-QaCond { Test-QaAssocCommand (Get-Reg ($P.KProg + '\shell\open\command') '') $st.RelExe } 15
    Stop-AllApps
    Step-Qa 'двойной щелчок по Alpha: открывает программу из нового места'
    $clicked = Invoke-QaAlbumDblClick $st.Alpha
    $opened = Wait-QaNewApp $st.RelExe @() 45
    if ($opened) { $st.Pid = $opened.Id }
    Done 'Б6' ($up -and $reg -and $clicked -and $opened) "перенос в '$new': $moved; окно из нового места: $up; команда связи на новый путь: $reg; щелчок открыл программу оттуда: $([bool]$opened)"
    Close-ExplorerWindows
}

Add-QaResult -Id 'Б7' -Status laptop -Detail 'нужна старая 0.24 и выбор "Всегда" в окне "Чем открыть" (Windows защищает выбор хешем UserChoice: из скрипта не подставить, UIA окна хрупок): проверяется на ноутбуке'

Invoke-QaItem 'Б8' -Budget 300 {
    Stop-AllApps
    $app = Start-Rel
    if (-not (Wait-Ready $app.Id 60)) { Add-QaResult -Id 'Б8' -Status fail -Detail 'окно rel не появилось'; return }
    $id = $app.Id
    Step-Qa 'снимаю галочку: Файл - Открывать .salbum этой программой'
    $item = Invoke-QaMenu $id 'Файл' 'Открывать \.salbum этой программой'
    $toast = $null
    if ($item) { $toast = Wait-QaText -ProcId $id -Regex '^Альбомы \.salbum больше не открываются этой программой$' -Timeout 10 }
    $regOff = Wait-QaCond { ((Get-Reg $P.KExt '') -cne 'SosninaAlbums.Album') -and ((Get-Reg $P.KApp 'AssocOff') -eq '1') } 10
    Open-ExplorerAt (Split-Path -Parent $st.Alpha)
    Start-Sleep -Seconds 3
    $shot = Save-QaShot -Id 'Б8' -Slug 'icon'
    Close-ExplorerWindows
    Step-Qa 'перезапуск: связь не должна вернуться'
    [void](Stop-QaApp -ProcId $id)
    $again = Start-Rel
    $up = Wait-Ready $again.Id 60
    $noAsk = -not (Test-Button $again.Id 'Не спрашивать' 5)
    $stay = ((Get-Reg $P.KExt '') -cne 'SosninaAlbums.Album') -and ((Get-Reg $P.KApp 'AssocOff') -eq '1')
    Done 'Б8' ($item -and $toast -and $regOff -and $up -and $noAsk -and $stay) "пункт меню: $([bool]$item); тост: '$toast'; ключа связи нет и AssocOff=1: $regOff; после перезапуска связь не вернулась: $stay; вопроса нет: $noAsk; значок - по снимку" 'eyes' @($shot)
}

Invoke-QaItem 'Б9' -Budget 240 {
    if (-not $st.Pid -or -not (Test-Alive $st.Pid)) { Stop-AllApps; $a = Start-Rel; [void](Wait-Ready $a.Id 60) }
    $id = $st.Pid
    Step-Qa 'ставлю галочку обратно'
    $item = Invoke-QaMenu $id 'Файл' 'Открывать \.salbum этой программой'
    $toast = $null
    if ($item) { $toast = Wait-QaText -ProcId $id -Regex '^Альбомы \.salbum открываются этой программой$' -Timeout 10 }
    $regOn = Wait-QaCond { ((Get-Reg $P.KExt '') -ceq 'SosninaAlbums.Album') -and (Test-QaAssocCommand (Get-Reg ($P.KProg + '\shell\open\command') '') $st.RelExe) } 10
    $offGone = $null -eq (Get-Reg $P.KApp 'AssocOff')
    Done 'Б9' ($item -and $toast -and $regOn -and $offGone) "пункт меню: $([bool]$item); тост: '$toast'; связь вернулась: $regOn; AssocOff ушла: $offGone (связь оставлена включённой: Д6 нужен установщик, который её пишет)"
    Stop-AllApps
}

# ---------- Фаза Д ----------
. (Join-Path $here 'qa-open-d.ps1')

Stop-AllApps
