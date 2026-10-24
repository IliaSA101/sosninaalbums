# qa-open-d.ps1 - фаза Д блока open (спека 118): окно «Альбом из новой версии» и «Открыть в…» (Д1-Д7).
# Подключается точкой из qa-open.ps1: общие переменные ($P, $st, $newer, $setup1, $work) и помощники оттуда.
# Подготовка: setup 0.25.1 /S (установленная и связь на неё), rel 0.25.0 с assoc=off, чтобы не перехватывать связь; stand не нужен.
# Windows PowerShell 5.1, файл в UTF-8 с BOM.

$lead251 = Get-QaNewerLeadRegex '0.25.1' '0.25.0'
$next251 = Get-QaNewerNextRegex '0.25.1'

# Окно «Альбом из новой версии» открыто в процессе: заголовок, первая строка по регулярке; имя строки или $null
function Wait-NewerLead([int]$ProcId, [string]$LeadRegex, [double]$Timeout = 40) {
    return (Wait-QaText -ProcId $ProcId -Regex $LeadRegex -Timeout $Timeout)
}
# Есть ли кнопка «Открыть в …» (любая версия)
function Test-OpenIn([int]$ProcId, [double]$Timeout = 4) {
    return [bool](Find-AppEl -ProcId $ProcId -NameRegex '^Открыть в \d' -Type 'Button' -Timeout $Timeout)
}
# Запуск rel с альбомом в аргументе (так двойной щелчок передаёт файл); окно «из новой версии» поверх стартового
function Start-RelWith([string]$AlbumPath) {
    Stop-AllApps
    return (Start-Rel (Quote $AlbumPath))
}

Invoke-QaItem 'Д-подготовка' -Budget 360 {
    Stop-AllApps
    # Первое приветствие Edge ('Sign in to sync data') закрывает страницу выпуска на снимке Д2: гасим политикой (HKLM, иначе HKCU)
    foreach ($hive in @('HKLM:', 'HKCU:')) {
        try {
            $pol = "$hive\SOFTWARE\Policies\Microsoft\Edge"
            if (-not (Test-Path -LiteralPath $pol)) { New-Item -Path $pol -Force -ErrorAction Stop | Out-Null }
            Set-ItemProperty -LiteralPath $pol -Name 'HideFirstRunExperience' -Value 1 -Type DWord -ErrorAction Stop
            Write-Log "Д: политика Edge HideFirstRunExperience=1 ($hive)"
            break
        } catch { Write-Log "Д: политика Edge в $hive не выставлена: $($_.Exception.Message)" }
    }
    if (-not (Test-Path -LiteralPath $newer.N0251)) { Write-Log 'Д: нет NewerTest-*.salbum в наборе'; return }
    Step-Qa 'установка 0.25.1 /S'
    $code = Invoke-Setup $setup1 '/S'
    $ver = Get-Reg $P.KUninst 'DisplayVersion'
    $cmd = Get-Reg ($P.KProg + '\shell\open\command') ''
    Write-Log "Д: setup 0.25.1 /S: код $code, DisplayVersion '$ver', связь на установленную: $(Test-QaAssocCommand $cmd $P.Exe)"
    if ($code -ne 0 -or $ver -cne '0.25.1' -or -not (Test-Path -LiteralPath $P.Exe)) { return }
    # стартовое окно показываем: Д1 проверяет, что окно «из новой версии» поверх него
    Write-Utf8 $P.Config (New-QaOpenConfig $st.Alpha 'off' $true)
    $st.PrepD = $true
}
if (-not $st.PrepD) {
    Skip-Blocked $allD 'подготовки Д (setup 0.25.1 /S, NewerTest в наборе)'
    return
}
$markD = Get-LogMark $P.Log

Invoke-QaItem 'Д1' -Budget 240 {
    Step-Qa 'запуск rel с NewerTest-0.25.1.salbum'
    $app = Start-RelWith $newer.N0251
    $id = $app.Id
    $title = Wait-QaText -ProcId $id -Regex '^Альбом из новой версии$' -Timeout 60
    $lead = Wait-NewerLead $id $lead251 15
    $next = Wait-QaText -ProcId $id -Regex $next251 -Timeout 8
    $close = Find-AppEl -ProcId $id -Name 'Закрыть' -Type 'Button' -Timeout 5
    $down = Test-Button $id 'Скачать' 3
    $open = Test-Button $id 'Открыть в 0.25.1' 3
    $top = $close -and (Test-QaOnTop $close)
    $under = Test-Button $id 'Продолжить' 3
    $shot = Save-QaShot -Id 'Д1' -Slug 'dialog' -ProcId $id
    $st.D1 = [bool]($title -and $lead -and $close)
    Done 'Д1' ($title -and $lead -and $next -and $close -and $down -and $open -and $top) "заголовок: '$title'; строка 1: '$lead'; строка 2: '$next'; 'Закрыть': $([bool]$close); 'Скачать': $down; 'Открыть в 0.25.1': $open; кнопка не закрыта стартовым окном: $top; стартовое окно под ним: $under" 'eyes' @($shot)
}

Invoke-QaItem 'Д2' -Budget 300 {
    if (-not $st.D1) { Add-QaResult -Id 'Д2' -Status skip -Blocked 'Д1'; return }
    $id = $st.Pid
    $titleBefore = Get-QaWinTitle $id
    $count0 = @(Get-AppProcs $st.RelExe).Count + @(Get-AppProcs $P.Exe).Count
    Step-Qa 'нажимаю "Закрыть"'
    $clicked = Invoke-QaButton $id 'Закрыть' 8
    $gone = Wait-QaCond { -not (Test-Button $id 'Скачать' 1) } 10
    $count1 = @(Get-AppProcs $st.RelExe).Count + @(Get-AppProcs $P.Exe).Count
    $titleAfter = Get-QaWinTitle $id
    $quiet = ($count0 -eq $count1) -and ($titleBefore -ceq $titleAfter)
    # снова то же окно: второй запуск с тем же файлом передаёт его первому процессу
    Step-Qa 'второй запуск с тем же файлом'
    [void](Start-QaApp -Exe $st.RelExe -Arguments (Quote $newer.N0251))
    $again = Wait-NewerLead $id $lead251 30
    $edge0 = @(Get-Process -Name 'msedge' -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
    Step-Qa 'нажимаю "Скачать"'
    $down = $again -and (Invoke-QaButton $id 'Скачать' 8 -Mouse)
    $edge = $null
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($down -and -not $edge -and $sw.Elapsed.TotalSeconds -lt 30) {
        $edge = Get-QaNewProcs 'msedge' $edge0 | Where-Object { $_.MainWindowHandle -ne [IntPtr]::Zero } | Select-Object -First 1
        if (-not $edge) { Start-Sleep -Milliseconds 500 }
    }
    $url = ''
    if ($edge) { Start-Sleep -Seconds 3; $url = Get-QaBrowserUrl $edge.Id }
    $shot = Save-QaShot -Id 'Д2' -Slug 'edge'
    foreach ($e in (Get-QaNewProcs 'msedge' $edge0)) { try { & taskkill.exe /PID $e.Id /T /F 2>&1 | Out-Null } catch { } }
    $urlOk = (-not $url) -or ($url -match 'github\.com/IliaSA101/sosninaalbums')
    Done 'Д2' ($clicked -and $gone -and $quiet -and $edge -and $urlOk) "'Закрыть' нажата: $clicked; окно ушло: $gone; процессов $count0 -> $count1, заголовок прежний: $quiet; после 'Скачать' новое окно браузера: $([bool]$edge); адрес: '$url' (пусто - адресную строку UIA не отдала, смотреть снимок)" 'eyes' @($shot)
}

# Д3: ролик на отрезке пункта, как у Г13
Start-QaVideo 'Д3'
Enter-QaItem -Id 'Д3' -Budget 300
try {
    $id = $st.Pid
    if (-not $st.D1 -or -not (Test-Alive $id)) { Add-QaResult -Id 'Д3' -Status skip -Blocked 'Д1' }
    else {
        if (-not (Test-Button $id 'Открыть в 0.25.1' 2)) {
            [void](Start-QaApp -Exe $st.RelExe -Arguments (Quote $newer.N0251))
            [void](Wait-NewerLead $id $lead251 30)
        }
        $before = @(Get-AppProcs $P.Exe | ForEach-Object { $_.Id })
        $sw = [Diagnostics.Stopwatch]::StartNew()
        Step-Qa 'нажимаю "Открыть в 0.25.1"'
        $clicked = Invoke-QaButton $id 'Открыть в 0.25.1' 10 -Mouse
        Step-Qa 'жду выход rel и запуск установленной'
        $relGone = Wait-QaCond { -not (Test-Alive $id) } 40
        $inst = Wait-QaNewApp $P.Exe $before 60
        $lead = $null
        $sec = 0.0
        $noOpen = $false
        $shot = ''
        if ($inst) {
            $st.Pid = $inst.Id
            $lead = Wait-QaText -ProcId $inst.Id -Regex 'сохранён более новой версией программы' -Timeout 40
            $sec = $sw.Elapsed.TotalSeconds
            $noOpen = -not (Test-OpenIn $inst.Id 4)
            $shot = Save-QaShot -Id 'Д3' -Slug 'open' -ProcId $inst.Id
        }
        $okAll = $clicked -and $relGone -and $inst -and $lead -and $noOpen
        Add-QaResult -Id 'Д3' -Status $(if ($okAll) { 'eyes' } else { 'fail' }) -Detail ("{0:N1} с от нажатия до окна в установленной; rel вышла: $relGone; установленная запущена: $([bool]$inst) (путь из %LOCALAPPDATA%\Programs); строка окна: '$lead'; кнопки 'Открыть в…' нет: $noOpen" -f $sec) -Shots @($shot) -Sec $sec
    }
} catch {
    Add-QaResult -Id 'Д3' -Status fail -Detail ('исключение: ' + $_.Exception.Message)
}
Exit-QaItem
$vidD3 = Stop-QaVideo
if ($vidD3) { Write-Log "ролик Д3: $vidD3" }

Invoke-QaItem 'Д4' -Budget 180 {
    Step-Qa 'запуск rel с NewerTest-0.26.salbum'
    $app = Start-RelWith $newer.N026
    $id = $app.Id
    $v4 = Get-QaShownVers '0.26' '0.25.0'
    $lead = Wait-NewerLead $id (Get-QaNewerLeadRegex $v4.Need $v4.Mine) 60
    $next = Wait-QaText -ProcId $id -Regex (Get-QaNewerNextRegex '') -Timeout 8
    $close = Test-Button $id 'Закрыть' 4
    $down = Test-Button $id 'Скачать' 3
    $noOpen = -not (Test-OpenIn $id 4)
    $shot = Save-QaShot -Id 'Д4' -Slug 'v026' -ProcId $id
    Done 'Д4' ($lead -and $next -and $close -and $down -and $noOpen) "строка 1: '$lead'; строка 2: '$next'; 'Закрыть': $close; 'Скачать': $down; 'Открыть в…' нет: $noOpen" 'eyes' @($shot)
}

Invoke-QaItem 'Д5' -Budget 180 {
    Step-Qa 'запуск rel с NewerTest-unknown.salbum'
    $app = Start-RelWith $newer.Unknown
    $id = $app.Id
    $lead = Wait-NewerLead $id (Get-QaNewerLeadRegex '') 60
    $close = Test-Button $id 'Закрыть' 4
    $down = Test-Button $id 'Скачать' 3
    $open = Test-OpenIn $id 4
    $shot = Save-QaShot -Id 'Д5' -Slug 'unknown' -ProcId $id
    Done 'Д5' ($lead -and $close -and $down -and $open) "строка 1: '$lead'; 'Закрыть': $close; 'Скачать': $down; 'Открыть в…' есть: $open" 'eyes' @($shot)
}

Invoke-QaItem 'Д6' -Budget 300 {
    Stop-AllApps
    # связь должна вести на установленную: rel с assoc=off её не трогает, но проверяем и при расхождении ставим заново
    $cmd = Get-Reg ($P.KProg + '\shell\open\command') ''
    $onInst = Test-QaAssocCommand $cmd $P.Exe
    $fixed = ''
    if (-not $onInst) {
        Step-Qa 'связь не на установленной: setup 0.25.1 /S заново'
        $code = Invoke-Setup $setup1 '/S'
        $onInst = Test-QaAssocCommand (Get-Reg ($P.KProg + '\shell\open\command') '') $P.Exe
        $fixed = "; связь стояла не на установленной, переустановка (код $code) вернула её: $onInst"
    }
    $before = @()
    $sw = [Diagnostics.Stopwatch]::StartNew()
    Step-Qa 'двойной щелчок по NewerTest-0.26.salbum, все программы закрыты'
    $clicked = Invoke-QaAlbumDblClick $newer.N026
    $inst = Wait-QaNewApp $P.Exe $before 60
    $lead = $null
    $sec = 0.0
    $shot = ''
    if ($inst) {
        $st.Pid = $inst.Id
        $v6 = Get-QaShownVers '0.26' '0.25.1'
        $lead = Wait-QaText -ProcId $inst.Id -Regex (Get-QaNewerLeadRegex $v6.Need $v6.Mine) -Timeout 40
        $sec = $sw.Elapsed.TotalSeconds
        $shot = Save-QaShot -Id 'Д6' -Slug 'cold' -ProcId $inst.Id
    }
    Close-ExplorerWindows
    Done 'Д6' ($onInst -and $clicked -and $inst -and $lead) ("связь на установленную: $onInst$fixed; щелчок: $clicked; установленная запущена: $([bool]$inst); окно «из новой версии» с версией 0.26: '$lead' за {0:N1} с" -f $sec) 'eyes' @($shot) '' $sec
}

Invoke-QaItem 'Д7' -Budget 60 {
    $log = Get-LogSince $markD
    $bad = $log -match 'запуск новой версии:'
    Done 'Д7' (-not $bad) "строка ошибки 'запуск новой версии:' есть: $bad"
}
Stop-AllApps
