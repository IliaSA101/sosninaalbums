# qa-open-a.ps1 - фаза А блока open (спека 118): первый запуск rel из «скачанного» zip, вопрос о связи, тур «Первые шаги» (А1-А14).
# Подключается точкой из qa-open.ps1: общие переменные ($P, $st, $work, $dl, $relRoot, $photosDir, $albumsDir) и помощники оттуда.
# rel живёт на настоящем профиле (APPDATA\SosninaAlbums), проверка обновлений выключена (Get-QaSeedConfig).
# Windows PowerShell 5.1, файл в UTF-8 с BOM.

# Шаг тура: текст счётчика «Шаг N из 7 · Название» (имя элемента) или $null
function Wait-Step([int]$ProcId, [int]$N, [double]$Timeout = 12) {
    return (Wait-QaText -ProcId $ProcId -Regex ('^Шаг ' + $N + ' из 7 · ') -Timeout $Timeout)
}
function Click-Next([int]$ProcId) { return (Invoke-QaButton $ProcId 'Дальше' 8) }
# Карточка тура целиком в окне программы: счётчик шага и кнопка внутри границ окна
function Test-CardInside([int]$ProcId) {
    $win = Get-QaWinRect -ProcId $ProcId
    $cnt = Find-AppEl -ProcId $ProcId -NameRegex '^Шаг \d+ из' -Timeout 3
    if (-not $win -or -not $cnt) { return $false }
    $btn = Find-AppEl -ProcId $ProcId -NameRegex '^(Дальше|Готово)$' -Type 'Button' -Timeout 2
    foreach ($el in @($cnt, $btn)) {
        if (-not $el) { continue }
        $r = Get-ElRect $el
        if ($r[0] -lt $win[0] - 2 -or $r[1] -lt $win[1] - 2 -or ($r[0] + $r[2]) -gt ($win[0] + $win[2] + 2) -or ($r[1] + $r[3]) -gt ($win[1] + $win[3] + 2)) { return $false }
    }
    return $true
}
# Карточки тура нет: ни «Начать», ни счётчика шага
function Test-NoTour([int]$ProcId, [double]$Timeout = 5) {
    if (Test-Button $ProcId 'Начать' $Timeout) { return $false }
    return -not (Find-AppEl -ProcId $ProcId -NameRegex '^Шаг \d+ из' -Timeout 1)
}
# Сброс профиля rel: данные программы, три ключа связи; config.json - сид без проверки обновлений
function Reset-RelProfile {
    foreach ($k in @($P.KExt, $P.KProg, $P.KApp)) { Remove-Item -LiteralPath "HKCU:\$k" -Recurse -Force -ErrorAction SilentlyContinue }
    foreach ($d in @($P.Roaming, $P.Local)) { Remove-Item -LiteralPath $d -Recurse -Force -ErrorAction SilentlyContinue }
    Write-Utf8 $P.Config (Get-QaSeedConfig)
}
# Перезапуск rel мягко: старый закрыт, новый с окном; процесс или $null
function Restart-Rel {
    Stop-AllApps
    $app = Start-Rel
    if (Wait-Ready $app.Id 60) { return $app }
    return $null
}
# «Справка ▸ Первые шаги» -> карточка с «Начать»
function Open-Tour([int]$ProcId) {
    $item = Invoke-QaMenu $ProcId 'Справка' '^Первые шаги'
    if (-not $item) { return $false }
    return (Test-Button $ProcId 'Начать' 15)
}

# ---------- Подготовка: чистая машина, rel из zip с меткой скачанного ----------
Invoke-QaItem 'А-подготовка' -Budget 420 {
    Step-Qa 'сброс: удаление программы, ключей реестра, данных'
    Stop-AllApps
    [void](Invoke-QuietUninstall)
    Reset-RelProfile
    New-Item -ItemType Directory -Force -Path $dl | Out-Null
    Remove-Item -LiteralPath $relRoot -Recurse -Force -ErrorAction SilentlyContinue
    $zipCopy = Join-Path $dl (Split-Path -Leaf $relZip)
    Copy-Item -LiteralPath $relZip -Destination $zipCopy -Force
    Set-Zone $zipCopy 'https://example.invalid/'
    Step-Qa 'распаковка zip движком Проводника (метка переходит на exe)'
    $exe = $null
    if (Expand-ZipShell $zipCopy $relRoot) { $exe = Wait-Extracted $relRoot $zipCopy 120 }
    $how = 'через оболочку Проводника'
    if (-not $exe) {
        $how = 'Expand-Archive (оболочка не распаковала; метки на exe нет)'
        Expand-Archive -LiteralPath $relZip -DestinationPath $relRoot -Force
        $f = Find-ExtractedExe $relRoot
        if ($f) { $exe = $f.FullName }
    }
    if (-not $exe) { Write-Log 'А: rel не распаковалась'; return }
    $st.RelExe = $exe
    $zone = Get-ZoneId $exe
    Write-Log "А: rel $how, ZoneId на exe: $zone"
    $st.PrepA = $true
}
if (-not $st.PrepA) {
    Skip-Blocked ($allA + $allB + $allD) 'подготовки А (чистая машина, rel из zip)'
    return
}
$markA = Get-LogMark $P.Log

# ---------- А1: SmartScreen на exe с меткой ----------
Invoke-QaItem 'А1' -Budget 300 {
    $z = Invoke-QaZoneLaunch $st.RelExe 'А1'
    Step-Qa 'жду окно rel'
    $app = Wait-QaNewApp $st.RelExe @() 45
    $viaExplorer = [bool]$app
    if (-not $app) { $app = Start-Rel } else { $st.Pid = $app.Id }
    Close-ExplorerWindows
    $st.A1 = [bool](Wait-Ready $st.Pid 60)
    Done 'А1' ($viaExplorer -and $st.A1) "$($z.Detail); программа запущена двойным щелчком: $viaExplorer; меню в окне: $($st.A1)" 'eyes' $z.Shots
}

# ---------- А2-А3: вопрос о связи, «Да» ----------
Invoke-QaItem 'А2' -Budget 180 {
    $id = $st.Pid
    $go = Find-AppEl -ProcId $id -NameRegex '^Продолжить(:|$)' -Type 'Button' -Timeout 40
    $ask = Wait-QaText -ProcId $id -Regex '^Открывать альбомы \.salbum двойным щелчком\?$' -Timeout 10
    $yes = Test-Button $id 'Да' 5
    $later = Test-Button $id 'Не сейчас' 3
    $never = Test-Button $id 'Не спрашивать' 3
    $shot = Save-QaShot -Id 'А2' -Slug 'start' -ProcId $id
    Done 'А2' ($go -and $ask -and $yes -and $later -and $never) "'Продолжить': $([bool]$go); вопрос: '$ask'; 'Да': $yes; 'Не сейчас': $later; 'Не спрашивать': $never" 'eyes' @($shot)
}
Invoke-QaItem 'А3' -Budget 240 {
    $id = $st.Pid
    Step-Qa 'нажимаю "Да"'
    $clicked = Invoke-QaButton $id 'Да' 10
    $toast = Wait-QaText -ProcId $id -Regex '^Альбомы \.salbum открываются этой программой$' -Timeout 12
    $shotToast = Save-QaShot -Id 'А3' -Slug 'toast' -ProcId $id
    $ext = Get-Reg $P.KExt ''
    $cmd = Get-Reg ($P.KProg + '\shell\open\command') ''
    $cmdOk = Test-QaAssocCommand $cmd $st.RelExe
    $on = Wait-QaCond { $c = Get-Cfg; [bool]($c -and $c.assoc -ceq 'on') } 10
    Open-ExplorerAt $albumsDir
    Start-Sleep -Seconds 3
    $shotIcon = Save-QaShot -Id 'А3' -Slug 'icon'
    Close-ExplorerWindows
    Done 'А3' ($clicked -and $toast -and $ext -ceq 'SosninaAlbums.Album' -and $cmdOk -and $on) "'Да' нажата: $clicked; тост: '$toast'; .salbum = '$ext'; команда на этот exe с `"%1`": $cmdOk; config.assoc = on: $on; значок .salbum - по снимку" 'eyes' @($shotToast, $shotIcon)
}

# ---------- А4-А6: «Продолжить», карточка «Первые шаги», «Пропустить», перезапуск ----------
Invoke-QaItem 'А4' -Budget 180 {
    $id = $st.Pid
    Step-Qa 'нажимаю "Продолжить"'
    $clicked = Click-Prefix $id 'Продолжить' 10
    $begin = Test-Button $id 'Начать' 25
    $skip = Test-Button $id 'Пропустить' 3
    $video = Test-Button $id 'Видео о программе ↗' 3
    $steps = Get-QaCountRe $id 'Button' '^\d+\. '
    $shot = Save-QaShot -Id 'А4' -Slug 'card' -ProcId $id
    $st.Tour = [bool]$begin
    Done 'А4' ($clicked -and $begin -and $skip -and $video -and $steps -eq 7) "'Продолжить' нажата: $clicked; 'Начать': $begin; 'Пропустить': $skip; 'Видео о программе': $video; шагов в списке: $steps (ждали 7); страница видео до выпуска может отдавать 404 - не провал" 'eyes' @($shot)
}
Invoke-QaItem 'А5' -Budget 240 {
    if (-not $st.Tour) { Add-QaResult -Id 'А5' -Status skip -Blocked 'А4'; return }
    $id = $st.Pid
    Step-Qa 'нажимаю "Пропустить"'
    $clicked = Invoke-QaButton $id 'Пропустить' 8
    $toast = Wait-QaText -ProcId $id -Regex '^Первые шаги — в «Справка ▸ Первые шаги»$' -Timeout 10
    $shot = Save-QaShot -Id 'А5' -Slug 'toast' -ProcId $id
    $done = Wait-QaCond { $c = Get-Cfg; [bool]($c -and $c.tours -and $c.tours.first -ceq 'done') } 10
    Step-Qa 'перезапуск: карточки быть не должно'
    $app = Restart-Rel
    $st.Restart1 = $app
    $up = [bool]$app
    $none = $up -and (Test-NoTour $app.Id 6)
    $askAgain = $up -and (Test-Button $app.Id 'Не спрашивать' 4)
    $st.NoAsk = $up -and -not $askAgain
    $manual = $false
    if ($up) {
        [void](Click-Prefix $app.Id 'Продолжить' 10)
        $manual = Open-Tour $app.Id
        Close-TourCard $app.Id
    }
    Done 'А5' ($clicked -and $toast -and $done -and $none -and $manual) "'Пропустить' нажата: $clicked; тост: '$toast'; config.tours.first = done: $done; после перезапуска карточки нет: $none; «Справка ▸ Первые шаги» открывает её: $manual" 'eyes' @($shot)
}
Invoke-QaItem 'А6' -Budget 60 {
    if ($null -eq $st.NoAsk) { Add-QaResult -Id 'А6' -Status skip -Blocked 'А5'; return }
    Done 'А6' $st.NoAsk "после перезапуска кнопки 'Не спрашивать' (вопроса о связи) нет за 4 с: $($st.NoAsk)"
}

# ---------- А7: «Не спрашивать» ----------
Invoke-QaItem 'А7' -Budget 300 {
    Step-Qa 'сброс профиля и ключей, новый запуск'
    Stop-AllApps
    Reset-RelProfile
    $app = Start-Rel
    $id = $app.Id
    $ask = Test-Button $id 'Не спрашивать' 60
    $clicked = $ask -and (Invoke-QaButton $id 'Не спрашивать' 8)
    $toast = Wait-QaText -ProcId $id -Regex '^Включить можно в меню «Файл ▸ Открывать \.salbum этой программой»$' -Timeout 12
    $shot = Save-QaShot -Id 'А7' -Slug 'toast' -ProcId $id
    $noKeys = -not (Test-RegKey $P.KExt) -and -not (Test-RegKey $P.KProg)
    $off = Wait-QaCond { $c = Get-Cfg; [bool]($c -and $c.assoc -ceq 'off') } 10
    Step-Qa 'перезапуск: вопроса быть не должно'
    $again = Restart-Rel
    $noAsk = $again -and -not (Test-Button $again.Id 'Не спрашивать' 5)
    Done 'А7' ($clicked -and $toast -and $noKeys -and $off -and $noAsk) "вопрос показан: $ask; 'Не спрашивать' нажата: $clicked; тост: '$toast'; ключей .salbum нет: $noKeys; config.assoc = off: $off; после перезапуска вопроса нет: $([bool]$noAsk)" 'eyes' @($shot)
}

# ---------- А8-А9: тур, шаг 1 ----------
Invoke-QaItem 'А8' -Budget 240 {
    $id = $st.Pid
    $app = Get-Process -Id $id -ErrorAction SilentlyContinue
    if (-not $app) { Add-QaResult -Id 'А8' -Status skip -Blocked 'А7'; return }
    [void](Click-Prefix $id 'Продолжить' 20)
    $begin = Test-Button $id 'Начать' 25
    if (-not $begin) { Add-QaResult -Id 'А8' -Status fail -Detail "карточка с 'Начать' после 'Продолжить' не появилась"; return }
    Step-Qa 'нажимаю "Начать"'
    [void](Invoke-QaButton $id 'Начать' 8)
    $step1 = Wait-Step $id 1
    $head = Wait-QaText -ProcId $id -Regex '^Нажмите «\+ Папка с фото»' -Timeout 6
    $win = Get-QaWinRect -ProcId $id
    $same = $false
    if ($win) {
        Step-Qa 'щелчок в угол окна вне выреза'
        Move-Cursor ($win[0] + $win[2] - 40) ($win[1] + $win[3] - 40) 0.3 10
        Invoke-MouseClick 'click'
        Start-Sleep -Seconds 1
        $same = [bool](Wait-Step $id 1 3)
    }
    $shot = Save-QaShot -Id 'А8' -Slug 'cutout' -ProcId $id
    $st.Step1 = [bool]$step1
    Done 'А8' ($step1 -and $head -and $same) "счётчик: '$step1'; текст шага: '$head'; после щелчка мимо выреза шаг тот же: $same; вырез вокруг '+ Папка с фото' - по снимку" 'eyes' @($shot)
}
Invoke-QaItem 'А9' -Budget 240 {
    if (-not $st.Step1) { Add-QaResult -Id 'А9' -Status skip -Blocked 'А8'; return }
    $id = $st.Pid
    Step-Qa 'щелчок по "+ Папка с фото"'
    $clicked = Invoke-QaButton $id '+ Папка с фото' 10 -Mouse
    Step-Qa 'системный выбор папки: вписываю путь'
    $dlg = $clicked -and (Set-QaFolderDialog $id $photosDir)
    $text = Wait-QaText -ProcId $id -Regex '^Фото уже в ленте' -Timeout 30
    $shot = Save-QaShot -Id 'А9' -Slug 'step1' -ProcId $id
    $st.Library = [bool]$text
    Done 'А9' ($clicked -and $dlg -and $text) "кнопка нажата: $clicked; выбор папки (поле Folder, Enter): $dlg; текст шага: '$text'" 'eyes' @($shot)
}

# ---------- А10: шаги 2-7, клавиши, Esc ----------
Invoke-QaItem 'А10' -Budget 420 {
    if (-not $st.Library) { Add-QaResult -Id 'А10' -Status skip -Blocked 'А9'; return }
    $id = $st.Pid
    $shots = @()
    $fails = New-Object System.Collections.ArrayList
    $inside = $true
    Step-Qa 'шаг 2: Дальше'
    if (-not (Click-Next $id) -or -not (Wait-Step $id 2)) { [void]$fails.Add('шаг 2') }
    if (-not (Test-CardInside $id)) { $inside = $false }
    $shots += Save-QaShot -Id 'А10' -Slug 'step2' -ProcId $id
    # клавиши: фокус на кнопке карточки (тур слушает ←/→/Enter, пока фокус внутри карточки)
    $nextBtn = Find-AppEl -ProcId $id -Name 'Дальше' -Type 'Button' -Timeout 5
    if ($nextBtn) { try { $nextBtn.SetFocus() } catch { } }
    Step-Qa 'клавиши: вправо, влево, Enter'
    Send-Keys @('Right')
    if (-not (Wait-Step $id 3 6)) { [void]$fails.Add('клавиша вправо') }
    Send-Keys @('Left')
    if (-not (Wait-Step $id 2 6)) { [void]$fails.Add('клавиша влево') }
    Send-Keys @('Enter')
    if (-not (Wait-Step $id 3 6)) { [void]$fails.Add('клавиша Enter') }
    $shots += Save-QaShot -Id 'А10' -Slug 'step3' -ProcId $id
    for ($n = 4; $n -le 7; $n++) {
        Step-Qa "шаг ${n}: Дальше"
        if (-not (Click-Next $id) -or -not (Wait-Step $id $n)) { [void]$fails.Add("шаг $n"); continue }
        if (-not (Test-CardInside $id)) { $inside = $false }
        if ($n -eq 5 -and -not (Wait-QaText -ProcId $id -Regex '^Когда появится разворот' -Timeout 5)) { [void]$fails.Add('текст шага 5') }
        if ($n -eq 6 -and -not (Wait-QaText -ProcId $id -Regex '^Щёлкните по фото' -Timeout 5)) { [void]$fails.Add('текст шага 6') }
        $shots += Save-QaShot -Id 'А10' -Slug "step$n" -ProcId $id
    }
    Step-Qa 'Esc закрывает тур'
    Send-Keys @('Esc')
    $closed = Wait-QaCond { -not (Find-AppEl -ProcId $id -NameRegex '^Шаг \d+ из' -Timeout 0.5) } 8
    if (-not $closed) { [void]$fails.Add('Esc') }
    Done 'А10' ($fails.Count -eq 0 -and $inside) "сбои: $($fails -join ', '); карточка целиком в окне на шагах: $inside; подсветка под карточкой - по снимкам" 'eyes' $shots
}

# ---------- А11: автосборка в туре ----------
Invoke-QaItem 'А11' -Budget 420 {
    $id = $st.Pid
    if (-not $st.Library) { Add-QaResult -Id 'А11' -Status skip -Blocked 'А9'; return }
    if (-not (Open-Tour $id)) { Add-QaResult -Id 'А11' -Status fail -Detail 'тур не открылся из меню'; return }
    [void](Invoke-QaButton $id 'Начать' 8)
    $step = Wait-Step $id 1
    for ($n = 2; $n -le 3 -and $step; $n++) { if (Click-Next $id) { $step = Wait-Step $id $n } else { $step = $null } }
    if (-not $step) { Add-QaResult -Id 'А11' -Status fail -Detail 'до шага 3 не дошли'; return }
    Step-Qa 'шаг 3: "Автосборка"'
    $auto = Invoke-QaButton $id 'Автосборка' 10 -Mouse
    $hidden = $auto -and (Wait-QaCond { -not (Test-Button $id 'Дальше' 1) } 10)
    $build = $null
    if ($auto) { $build = Find-AppEl -ProcId $id -NameRegex '^(Собрать|Пересобрать)$' -Type 'Button' -Timeout 15 }
    $shotDlg = Save-QaShot -Id 'А11' -Slug 'dialog' -ProcId $id
    if ($build) { Step-Qa 'нажимаю "Собрать"'; [void](Invoke-AppEl -El $build -Mouse) }
    $back = $build -and (Wait-QaCond { Test-Button $id 'Дальше' 1 } 60)
    $ring = $false
    $text5 = $null
    if ($back) {
        [void](Click-Next $id)
        [void](Wait-Step $id 4)
        [void](Click-Next $id)
        $ring = [bool](Wait-Step $id 5)
        $text5 = Wait-QaText -ProcId $id -Regex '^Кнопка ⊞ над разворотом' -Timeout 8
    }
    $shot5 = Save-QaShot -Id 'А11' -Slug 'step5' -ProcId $id
    $st.Step5 = [bool]$ring
    Done 'А11' ($auto -and $hidden -and $build -and $back -and $text5) "'Автосборка' нажата: $auto; карточки в UIA нет, пока открыто окно: $hidden; 'Собрать': $([bool]$build); карточка вернулась: $([bool]$back); шаг 5: '$text5'; шаг 6 после щелчка по фото и кольцо у ⊞ - глазами по снимкам (щелчок по фото на холсте раннер не делает)" 'eyes' @($shotDlg, $shot5)
}

# ---------- А12: «Готово», «Закрыть», перезапуск ----------
Invoke-QaItem 'А12' -Budget 300 {
    $id = $st.Pid
    if (-not $st.Step5) { Add-QaResult -Id 'А12' -Status skip -Blocked 'А11'; return }
    $ok = $true
    for ($n = 6; $n -le 7; $n++) {
        if (-not (Click-Next $id) -or -not (Wait-Step $id $n)) { $ok = $false; break }
    }
    Step-Qa 'нажимаю "Готово"'
    $fin = $ok -and (Invoke-QaButton $id 'Готово' 8)
    # «Закрыть» ищем внутри карточки тура (рядом с «Видео о программе»): первая «Закрыть» в дереве - крестик тоста «Собрано разворотов»
    $final = $fin -and [bool](Find-QaButtonNear -ProcId $id -AnchorRegex '^Видео о программе' -Name 'Закрыть' -Timeout 10)
    $shot = Save-QaShot -Id 'А12' -Slug 'final' -ProcId $id
    if ($final) {
        $closeBtn = Find-QaButtonNear -ProcId $id -AnchorRegex '^Видео о программе' -Name 'Закрыть' -Timeout 8
        if ($closeBtn) { [void](Invoke-AppEl -El $closeBtn) }
    }
    $gone = $final -and (Wait-QaCond { -not (Find-AppEl -ProcId $id -NameRegex '^Видео о программе' -Timeout 0.5) } 8)
    Step-Qa 'перезапуск: ни тура, ни тоста'
    $app = Restart-Rel
    $none = $app -and (Test-NoTour $app.Id 6)
    $noToast = $app -and -not (Wait-QaText -ProcId $app.Id -Regex '^Первые шаги — в «Справка' -Timeout 3)
    Done 'А12' ($ok -and $fin -and $final -and $gone -and $none -and $noToast) "до шага 7: $ok; 'Готово': $fin; финальная карточка с 'Закрыть': $final; закрылась: $gone; после перезапуска тура нет: $([bool]$none), тоста нет: $([bool]$noToast)" 'eyes' @($shot)
}

# ---------- А13: закрытие окна посреди тура ----------
Invoke-QaItem 'А13' -Budget 420 {
    Stop-AllApps
    Write-Utf8 $P.Config (Get-QaSeedConfig)
    $mark = Get-LogMark $P.Log
    Step-Qa 'запуск без config.json'
    $app = Start-Rel
    $id = $app.Id
    $asked = Test-Button $id 'Да' 60
    if ($asked) { [void](Invoke-QaButton $id 'Да' 8) }
    [void](Click-Prefix $id 'Продолжить' 20)
    $begin = Test-Button $id 'Начать' 25
    $step3 = $null
    if ($begin) {
        [void](Invoke-QaButton $id 'Начать' 8)
        $step3 = Wait-Step $id 1
        for ($n = 2; $n -le 3 -and $step3; $n++) { if (Click-Next $id) { $step3 = Wait-Step $id $n } else { $step3 = $null } }
    }
    Step-Qa 'закрываю окно крестиком (CloseMainWindow) на шаге 3'
    $how = Stop-QaApp -ProcId $id
    $line = Test-QaTourLog (Get-LogSince $mark)
    # закрытие окна посреди тура строку не пишет и tours.first не меняет (пишут только Готово/Закрыть/Пропустить/Esc)
    $cfgPending = $false
    $c = Get-Cfg
    if ($c -and $c.tours) { $cfgPending = ($c.tours.first -ceq 'pending') }
    $app2 = Start-Rel
    $up = Wait-Ready $app2.Id 60
    # открыто стартовое окно (tours.first = pending): тур стартует только после него
    if ($up) { [void](Click-Prefix $app2.Id 'Продолжить' 20) }
    $welcome = $up -and (Test-Button $app2.Id 'Начать' 25)
    Done 'А13' ($asked -and $step3 -and $line -eq 0 -and $cfgPending -and $welcome) "вопрос о связи вернулся: $asked (ответ 'Да'); шаг 3 достигнут: $([bool]$step3); программа $how; строки 'тур закрыт' нет (шаг: $line); config.tours.first = pending: $cfgPending; после запуска снова приветствие: $([bool]$welcome)"
    Stop-AllApps
}

# ---------- А14: журнал ----------
Invoke-QaItem 'А14' -Budget 60 {
    $log = Get-LogSince $markA
    $clean = Test-LogClean $log
    $assocFail = $log -match 'связь \.salbum:'
    Done 'А14' ($clean.Ok -and -not $assocFail) "журнал фазы А ($($log.Length) знаков): чисто: $($clean.Ok) $($clean.Why); отказ 'связь .salbum:': $assocFail"
}
