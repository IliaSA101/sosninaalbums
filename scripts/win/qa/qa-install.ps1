# qa-install.ps1 - блок install (спека 118, design §3): В0-В11, Ж1-Ж4, Г6-Г8, profile-*, масштаб экрана.
# Окна мастера NSIS - Win32, WebView2 не нужен. Одна ВМ - одна цепочка состояний; сбой пункта не останавливает задание,
# а следующие пункты, которым нужен его результат, получают "не проверено: нет <пункт>".
# Отличие от design §3: Г6-Г7 идут до Ж (код 6 нужен при установленной 0.25.0, после Ж4 стоит 0.25.1).
# Вызывается из run.ps1 (модули уже подключены). Windows PowerShell 5.1, файл в UTF-8 с BOM.
param([string]$Kit, [string]$OutDir, [string]$Dpi = '125,150,175', [switch]$Sabotage, [string]$RunnerTemp = $env:TEMP)
$ErrorActionPreference = 'Continue'

$P = Get-QaPaths
$work = Join-Path $RunnerTemp 'install'
New-Item -ItemType Directory -Force -Path $work | Out-Null
$setup0 = Join-Path $Kit 'setup\SosninaAlbums-0.25.0-windows-x64-setup.exe'
$setup1 = Join-Path $Kit 'setup\SosninaAlbums-0.25.1-windows-x64-setup.exe'
$name0 = 'SosninaAlbums-0.25.0-windows-x64-setup'
$mark = '{"level":"simple"}'

function Write-Utf8([string]$Path, [string]$Text) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
    [System.IO.File]::WriteAllText($Path, $Text, (New-Object System.Text.UTF8Encoding($false)))
}
# .cmd набора со вводом из канала (pause и choice читают stdin): код и вывод
function Invoke-CmdFile([string]$Cmd, [string]$Stdin) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $env:ComSpec
    $psi.Arguments = '/d /s /c ""' + $Cmd + '""'
    $psi.UseShellExecute = $false
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardOutput = $true
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.CreateNoWindow = $true
    $pr = [System.Diagnostics.Process]::Start($psi)
    $pr.StandardInput.Write($Stdin)
    $pr.StandardInput.Close()
    $out = $pr.StandardOutput.ReadToEnd()
    if (-not $pr.WaitForExit(60000)) { & taskkill.exe /PID $pr.Id /T /F | Out-Null }
    return [pscustomobject]@{ Code = $pr.ExitCode; Out = $out }
}
# Пока условие не выполнено (до таймаута): удаление идёт в копии uninstall.exe из TEMP, и мастер закрывается позже родителя
function Wait-Until([scriptblock]$Cond, [double]$Timeout = 20) {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do { if (& $Cond) { return $true }; Start-Sleep -Milliseconds 400 } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return [bool](& $Cond)
}
# Запуск установленной программы из "Пуск" (ярлык, без .cmd): процессы программы или пусто
function Start-FromMenu([double]$Timeout = 30) {
    Start-Process -FilePath $P.Lnk
    [void](Wait-Until { (Get-AppProcs $P.Exe).Count -gt 0 } $Timeout)
    return @(Get-AppProcs $P.Exe)
}
function Stop-Installed {
    foreach ($p in @(Get-AppProcs $P.Exe)) { [void](Stop-QaApp -ProcId $p.Id) }
}
function Test-Installed { return ((Test-Path -LiteralPath $P.Exe) -and (Test-Path -LiteralPath $P.Uninst)) }
# Пункты, которым нужна установка, без неё - "не проверено", а не ложный провал
function Skip-Blocked([string[]]$Ids, [string]$Because) { foreach ($i in $Ids) { Add-QaResult -Id $i -Status skip -Blocked $Because } }

# ---------- Альбом для двойного щелчка: генератор набора или запасной файл ----------
$album = Join-Path $Kit 'albums\NewerTest-unknown.salbum'
$albumNote = 'запасной NewerTest-unknown (генератора нет)'
$gen = Join-Path $Kit 'tools\qa-albums.exe'
if (Test-Path -LiteralPath $gen) {
    $o = (& $gen -out (Join-Path $work 'albums') 2>&1 | Out-String)
    $m = [regex]::Match($o, '(?m)^alpha=(.+?)\s*$')
    if ($m.Success -and (Test-Path -LiteralPath $m.Groups[1].Value)) { $album = $m.Groups[1].Value; $albumNote = 'Alpha.salbum из qa-albums' }
    else { Write-Log "qa-albums не дал alpha=: $o" }
}

# ---------- Чистое состояние ----------
[void](Invoke-QuietUninstall)
foreach ($k in @($P.KExt, $P.KProg, $P.KApp)) { Remove-Item -LiteralPath "HKCU:\$k" -Recurse -Force -ErrorAction SilentlyContinue }

# ---------- profile-save / profile-restore (набор ноутбука) ----------
Invoke-QaItem 'profile-save' {
    Write-Utf8 $P.Config $mark
    Write-Utf8 (Join-Path $P.Local 'thumbs\keep.txt') 'x'
    $r = Invoke-CmdFile (Join-Path $Kit 'profile-save.cmd') "`r`n"
    $saved = (Test-Path -LiteralPath ($P.Roaming + '.qa-save\config.json')) -and (Test-Path -LiteralPath ($P.Local + '.qa-save\thumbs\keep.txt'))
    $gone = -not (Test-Path -LiteralPath $P.Roaming) -and -not (Test-Path -LiteralPath $P.Local)
    if ($saved -and $gone) { Add-QaResult -Id 'profile-save' -Status ok -Detail "код $($r.Code): данные отложены в *.qa-save" }
    else { Add-QaResult -Id 'profile-save' -Status fail -Detail "отложено: $saved; исходные папки ушли: $gone; вывод: $($r.Out)" }
}
Invoke-QaItem 'profile-restore' {
    Write-Utf8 $P.Config '{"level":"made-during-check"}'
    $r = Invoke-CmdFile (Join-Path $Kit 'profile-restore.cmd') "Y`r`n`r`n"
    $back = (Test-Path -LiteralPath $P.Config) -and (([System.IO.File]::ReadAllText($P.Config)) -ceq $mark)
    $gone = -not (Test-Path -LiteralPath ($P.Roaming + '.qa-save'))
    if ($back -and $gone) { Add-QaResult -Id 'profile-restore' -Status ok -Detail "код $($r.Code): choice принял Y из канала, данные возвращены" }
    else { Add-QaResult -Id 'profile-restore' -Status fail -Detail "вернулось: $back; .qa-save ушла: $gone; код $($r.Code); вывод: $($r.Out)" }
}

# ---------- В0: тихая установка не возвращает снятую связь ----------
New-Item -Path "HKCU:\$($P.KApp)" -Force | Out-Null
New-ItemProperty -LiteralPath "HKCU:\$($P.KApp)" -Name 'AssocOff' -Value '1' -PropertyType String -Force | Out-Null
$sw = [Diagnostics.Stopwatch]::StartNew()
$code0 = Invoke-Setup $setup0 '/S'
Invoke-QaItem 'В0-1' {
    if ($code0 -eq 0) { Add-QaResult -Id 'В0-1' -Status ok -Detail ("setup /S: код 0 за {0:N1} с (отсутствие окон не наблюдаем)" -f $sw.Elapsed.TotalSeconds) -Sec $sw.Elapsed.TotalSeconds }
    else { Add-QaResult -Id 'В0-1' -Status fail -Detail "setup /S: код $code0" }
}
if ($code0 -ne 0) { Skip-Blocked @('В0-2', 'В0-3', 'В0-4') 'В0-1' } else {
    Invoke-QaItem 'В0-2' {
        $ok = (Test-Path -LiteralPath $P.Exe) -and (Test-Path -LiteralPath $P.Uninst) -and (Test-Path -LiteralPath $P.Lnk) -and -not (Test-Path -LiteralPath $P.DesktopLnk)
        $st = 'fail'; if ($ok) { $st = 'ok' }
        Add-QaResult -Id 'В0-2' -Status $st -Detail "exe и uninstall.exe, ярлык Пуска есть, на столе нет: $ok"
    }
    Invoke-QaItem 'В0-3' {
        $ok = ((Get-Reg $P.KExt '') -ne 'SosninaAlbums.Album') -and -not (Test-RegKey $P.KProg)
        $st = 'fail'; if ($ok) { $st = 'ok' }
        Add-QaResult -Id 'В0-3' -Status $st -Detail "связь не возвращена (.salbum = '$(Get-Reg $P.KExt '')'): $ok"
    }
    Invoke-QaItem 'В0-4' {
        $done = Invoke-QuietUninstall
        $ok = $done -and -not (Test-RegKey $P.KApp)
        $st = 'fail'; if ($ok) { $st = 'ok' }
        Add-QaResult -Id 'В0-4' -Status $st -Detail "удаление с ожиданием: $done; ключ HKCU\Software\SosninaAlbums с отметкой ушёл: $(-not (Test-RegKey $P.KApp))"
    }
}

# ---------- В2-В8: мастер из "скачанного" файла ----------
$dl = Join-Path $env:USERPROFILE 'Downloads'
New-Item -ItemType Directory -Force -Path $dl | Out-Null
$dlSetup = Join-Path $dl "$name0.exe"
Copy-Item -LiteralPath $setup0 -Destination $dlSetup -Force
Set-Zone $dlSetup 'https://example.invalid/'
$consentBefore = @(Get-Process -Name 'consent' -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })

Invoke-QaItem 'В2' {
    Step-Qa 'закрываю окна Проводника'
    Close-ExplorerWindows
    Step-Qa 'открываю Загрузки в Проводнике'
    Open-ExplorerAt $dl
    Step-Qa 'ищу файл установщика в Проводнике'
    $item = Find-ExplorerItem ('^' + [regex]::Escape($name0) + '(\.exe)?$') 25
    $how = 'двойной щелчок в Проводнике'
    Step-Qa 'двойной щелчок по установщику'
    if ($item) { Invoke-HitClick $item 'dbl' } else { $how = 'Проводник не нашёл файл, запуск через Start-Process'; Start-Process -FilePath $dlSetup }
    # после щелчка бывает SmartScreen (Подробнее) или окно проверки безопасности Проводника AES (Run): ждём любое из двух
    Step-Qa 'жду SmartScreen или окно проверки безопасности'
    $info = $null; $aes = $null
    $wait = [Diagnostics.Stopwatch]::StartNew()
    do {
        $info = Wait-Target -Names @('Подробнее', 'More info') -Timeout 1 -Skip @('msedge')
        if (-not $info) { $aes = Find-QaAesWindow }
    } while (-not $info -and -not $aes -and $wait.Elapsed.TotalSeconds -lt 25)
    $shots = @()
    if ($info) {
        $shots += Save-QaShot -Id 'В2' -Slug 'smartscreen'
        Step-Qa 'щелчок по "Подробнее"'
        Invoke-HitClick $info 'click'
        Step-Qa 'жду "Выполнить в любом случае"'
        $run = Wait-Target -Names @('Выполнить в любом случае', 'Run anyway') -Timeout 10
        $shots += Save-QaShot -Id 'В2' -Slug 'more'
        Step-Qa 'щелчок по "Выполнить в любом случае"'
        if ($run) { Invoke-HitClick $run 'click' }
        $detail = "окно было SmartScreen ($($info.Name)), $how; 'Выполнить в любом случае' найдена: $([bool]$run)"
    } elseif ($aes) {
        $shots += Save-QaShot -Id 'В2' -Slug 'aes'
        $runBtn = Find-QaWindowButton $aes @('Run', 'Запустить')
        Step-Qa 'щелчок по "Запустить" в окне проверки безопасности'
        if ($runBtn) { Invoke-HitClick $runBtn 'click' }
        $detail = "окно было проверкой безопасности Проводника (AES: Security Warning), не SmartScreen; $how; кнопка 'Запустить' найдена: $([bool]$runBtn)"
    } else {
        $shots += Save-QaShot -Id 'В2' -Slug 'nosmartscreen'
        $detail = "SmartScreen не показан за 25 с ($how); файл с меткой ZoneId=3"
        # CreateProcess вместо Start-Process: ShellExecuteEx для файла с меткой зоны показывает AES синхронно и вешает сценарий
        if (-not (Get-Process -Name $name0 -ErrorAction SilentlyContinue)) { Step-Qa 'запускаю мастер без Проводника'; [void](Start-Setup $dlSetup ''); $detail += '; мастер запущен без Проводника' }
    }
    Add-QaResult -Id 'В2' -Status eyes -Detail $detail -Shots $shots
    Close-ExplorerWindows
}

Invoke-QaItem 'В3' {
    $lvl = Get-ManifestLevel $setup0
    $consent = @(Get-Process -Name 'consent' -ErrorAction SilentlyContinue | Where-Object { $consentBefore -notcontains $_.Id }).Count
    if ($lvl -eq 'asInvoker' -and $consent -eq 0) { Add-QaResult -Id 'В3' -Status ok -Detail 'манифест asInvoker, consent.exe не появлялся (UAC на раннере выключен: доказательство - манифест)' }
    else { Add-QaResult -Id 'В3' -Status fail -Detail "манифест: '$lvl'; consent.exe: $consent" }
}

$wiz = $null
Start-QaVideo 'В4'
Enter-QaItem -Id 'В4' -Budget 300
Step-Qa 'мастер установки: страницы по порядку'
try { $wiz = Invoke-QaWizard -ProcNames @($name0) -Id 'В4' } catch { Write-Log "В4: $($_.Exception.Message)" }
Exit-QaItem
$vid = Stop-QaVideo
Invoke-QaItem 'В4' {
    $w = $wiz
    $shots = @($w.Pages | ForEach-Object { $_.Shot })
    if (-not $w -or -not $w.Ok) { Add-QaResult -Id 'В4' -Status fail -Detail ("мастер не дошёл до конца: {0}; {1}" -f $w.Why, (Get-WizSummary $w)) -Shots $shots -Video $vid; return }
    $first = $w.Pages[0].Text
    $ru = $first -match 'Будет установлена программа'
    $nodir = $first -notmatch 'Выберите папку|Папка назначения'
    $st = 'eyes'; if (-not ($ru -and $nodir)) { $st = 'fail' }
    Add-QaResult -Id 'В4' -Status $st -Detail ("страниц {0}; по-русски: {1}; без выбора папки: {2}; {3}" -f $w.Pages.Count, $ru, $nodir, (Get-WizSummary $w)) -Shots $shots -Video $vid
}

if (-not (Test-Installed)) { Skip-Blocked @('В5', 'В6', 'В7', 'В8', 'В9', 'В10', 'В11', 'Г6', 'Г7', 'Ж1', 'Ж2', 'Ж3', 'Ж4') 'В4' } else {
    Invoke-QaItem 'В5' {
        $pub = [string](Get-Reg $P.KUninst 'Publisher')
        $okPub = $pub -like '*Илья Соснин*'
        $ok = (Test-Path -LiteralPath $P.Lnk) -and (Test-Path -LiteralPath $P.DesktopLnk) -and ((Get-Reg $P.KUninst 'DisplayName') -ceq 'Sosnina Albums') -and ((Get-Reg $P.KUninst 'DisplayVersion') -ceq '0.25.0') -and $okPub
        $st = 'fail'; if ($ok) { $st = 'ok' }
        Add-QaResult -Id 'В5' -Status $st -Detail "ярлыки в Пуске и на столе, DisplayName, версия 0.25.0, издатель '$pub': $ok"
    }
    Invoke-QaItem 'В6' {
        $lm = Get-LogMark $P.Log
        Step-Qa 'запуск программы из Пуска'
        $procs = Start-FromMenu 30
        Step-Qa 'жду строку запуска в log.txt'
        $seen = Wait-Until { (Read-LogSince $P.Log $lm) -match 'запуск, версия 0\.25\.0' } 30
        $shot = ''
        if ($procs.Count) {
            Step-Qa 'жду главное окно программы'
            [void](Wait-QaAppWindow -ProcIds @($procs | ForEach-Object { $_.Id }) -Timeout 40)
            $shot = Save-QaShot -Id 'В6' -Slug 'start' -ProcId $procs[0].Id
        }
        $st = 'fail'; if ($procs.Count -and $seen) { $st = 'eyes' }
        Add-QaResult -Id 'В6' -Status $st -Detail "запуск из Пуска (ярлык): процесс $($procs.Count -gt 0), в log.txt 'запуск, версия 0.25.0': $seen; окно 'О программе' не открывалось, версия по журналу" -Shots $shot
        Stop-Installed
    }
    Invoke-QaItem 'В7' {
        Close-ExplorerWindows
        Open-ExplorerAt (Split-Path -Parent $album)
        $leaf = [IO.Path]::GetFileNameWithoutExtension($album)
        $item = Find-ExplorerItem ('^' + [regex]::Escape($leaf) + '(\.salbum)?$') 25
        if (-not $item) { Add-QaResult -Id 'В7' -Status fail -Detail "Проводник не показал $leaf"; Close-ExplorerWindows; return }
        $ow = @(Get-Process -Name 'OpenWith' -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
        $shotIcon = Save-QaShot -Id 'В7' -Slug 'icon'
        Step-Qa 'двойной щелчок по альбому'
        Invoke-HitClick $item 'dbl'
        Step-Qa 'жду запуск установленной программы'
        $up = Wait-Until { (Get-AppProcs $P.Exe).Count -gt 0 } 25
        $asked = @(Get-Process -Name 'OpenWith' -ErrorAction SilentlyContinue | Where-Object { $ow -notcontains $_.Id })
        # Снимок окна программы с открытым альбомом, а не Проводника: ждём главное окно установленной программы
        $appIds = @(Get-AppProcs $P.Exe | ForEach-Object { $_.Id })
        if ($appIds.Count) { Step-Qa 'жду окно программы с альбомом'; [void](Wait-QaAppWindow -ProcIds $appIds -Timeout 40) }
        $shot = ''
        if ($appIds.Count) { $shot = Save-QaShot -Id 'В7' -Slug 'after' -ProcId $appIds[0] } else { $shot = Save-QaShot -Id 'В7' -Slug 'after' }
        $st = 'fail'; if ($up) { $st = 'eyes' }
        Add-QaResult -Id 'В7' -Status $st -Detail "($albumNote) установленная программа открылась: $up; 'Чем открыть' спросило: $($asked.Count -gt 0); значок .salbum - по снимку" -Shots @($shotIcon, $shot)
        foreach ($a in $asked) { Stop-Process -Id $a.Id -Force -ErrorAction SilentlyContinue }
        Stop-Installed
        Close-ExplorerWindows
    }
    Invoke-QaItem 'В8' {
        $ok = ((Get-Reg $P.KProg 'AppVersion') -ceq '0.25.0') -and ((Get-Reg $P.KApp 'InstallDir') -ceq $P.Inst) -and ((Get-Reg $P.KApp 'Version') -ceq '0.25.0')
        $st = 'fail'; if ($ok) { $st = 'ok' }
        Add-QaResult -Id 'В8' -Status $st -Detail "AppVersion, InstallDir, Version: $ok"
    }

    # ---------- В9-В11: повторная установка поверх, программа открыта ----------
    $lm11 = Get-LogMark $P.Log
    Write-Utf8 $P.Config $mark
    $procs = Start-FromMenu 30
    Start-QaVideo 'В9'
    # Мастер сам не появится: Start-FromMenu запускает программу, а не установщик
    $pr9 = $null
    try { $pr9 = Start-Setup $setup0 '' } catch { Write-Log "В9: установщик не запустился: $($_.Exception.Message)" }
    $wiz2 = $null
    $retryShot = ''
    Enter-QaItem -Id 'В9' -Budget 300
    Step-Qa 'мастер поверх установленной программы'
    try {
        $wiz2 = Invoke-QaWizard -ProcNames @($name0) -Id 'В9' -OnRetry {
            param($retryBtn)
            $script:retryShot = Save-QaShot -Id 'В10' -Slug 'retry' -ProcNames @($name0)
            Stop-Installed
        }
    } catch { Write-Log "В9: $($_.Exception.Message)" }
    Exit-QaItem
    $vid2 = Stop-QaVideo
    # Провал: мастер ещё ждёт "Повтор" - закрываем по PID, иначе висит до конца блока
    if ($pr9 -and -not $pr9.HasExited) { try { & taskkill.exe /PID $pr9.Id /T /F | Out-Null } catch { } }
    Invoke-QaItem 'В9' {
        $w = $wiz2
        $shots = @($w.Pages | ForEach-Object { $_.Shot })
        $said = $w -and $w.Pages.Count -gt 0 -and ($w.Pages[0].Text -match 'уже установлена')
        $st = 'eyes'; if (-not $said) { $st = 'fail' }
        Add-QaResult -Id 'В9' -Status $st -Detail ("первая страница: '{0}'; 'уже установлена': {1}" -f $(if ($w.Pages.Count) { $w.Pages[0].Text } else { '' }), $said) -Shots @($shots[0]) -Video $vid2
    }
    Invoke-QaItem 'В10' {
        $w = $wiz2
        $okText = $w.RetryText -match 'Закройте'
        if ($w.Retried -and $okText -and $w.Ok) { Add-QaResult -Id 'В10' -Status eyes -Detail "окно 'Повтор': '$($w.RetryText)'; программа закрыта, Повтор нажат, установка закончилась" -Shots @($retryShot, @($w.Pages)[-1].Shot) -Video $vid2 }
        else { Add-QaResult -Id 'В10' -Status fail -Detail "Повтор показан: $($w.Retried); текст: '$($w.RetryText)'; мастер закончил: $($w.Ok); $($w.Why)" -Shots @($retryShot) -Video $vid2 }
    }
    Invoke-QaItem 'В11' {
        $procs = Start-FromMenu 30
        $seen = Wait-Until { (Read-LogSince $P.Log $lm11) -match 'запуск, версия 0\.25\.0' } 30
        $cfg = $null
        try { $cfg = [System.IO.File]::ReadAllText($P.Config) | ConvertFrom-Json } catch { }
        $kept = $cfg -and $cfg.level -eq 'simple'
        $st = 'fail'; if ($procs.Count -and $seen -and $kept) { $st = 'ok' }
        Add-QaResult -Id 'В11' -Status $st -Detail "программа запущена: $($procs.Count -gt 0), 'запуск' в журнале: $seen, настройки (level=simple) на месте: $kept; список 'Последние альбомы' глазами не смотрим"
        Stop-Installed
    }

    # ---------- Г6-Г7: сбой замены exe (код 6) ----------
    Invoke-QaItem 'Г6' -Budget 300 {
        # Занятый .old установщик больше не считает сбоем (уходит в .old-<тики>, 0.31-143-32); код 6 даёт exe,
        # который нельзя переименовать (открыт без FileShare.Delete): замка Wails нет, ждёт 5 с и 5 с повторяет
        $t = [Diagnostics.Stopwatch]::StartNew()
        Step-Qa 'тихая установка 0.25.1 при exe, который нельзя переименовать'
        $lockExe = [System.IO.File]::Open($P.Exe, 'Open', 'Read', 'Read')
        try { $code = Invoke-Setup $setup1 '/S' } finally { $lockExe.Dispose() }
        $sec = $t.Elapsed.TotalSeconds
        $verOk = (Get-Reg $P.KUninst 'DisplayVersion') -ceq '0.25.0'
        $noNew = -not (Test-Path -LiteralPath ($P.Exe + '.new'))
        $lm = Get-LogMark $P.Log
        $procs = Start-FromMenu 30
        $ran = Wait-Until { (Read-LogSince $P.Log $lm) -match 'запуск, версия 0\.25\.0' } 30
        Stop-Installed
        $ok = ($code -eq 6) -and $verOk -and $noNew -and (Test-Path -LiteralPath $P.Lnk) -and $ran -and ($sec -ge 3) -and ($sec -le 40)
        $st = 'fail'; if ($ok) { $st = 'ok' }
        Add-QaResult -Id 'Г6' -Status $st -Detail ("код $code (ждём 6), {0:N1} с (около 5), версия 0.25.0: $verOk, нет .exe.new: $noNew, из Пуска запускается 0.25.0: $ran" -f $sec) -Sec $sec
    }
    Invoke-QaItem 'Г7' {
        # После сбоя замены рядом с exe не должно остаться ни .old*, ни .new (маска: .old-<тики> имя не знает)
        $left = @(Get-ChildItem -LiteralPath (Split-Path $P.Exe) -Force -ErrorAction SilentlyContinue | Where-Object { $_.Name -like 'SosninaAlbums.exe.old*' -or $_.Name -like 'SosninaAlbums.exe.new*' })
        $st = 'fail'; if ($left.Count -eq 0) { $st = 'ok' }
        $left | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
        Add-QaResult -Id 'Г7' -Status $st -Detail ('остатков .old*/.new нет: {0}; {1}' -f ($left.Count -eq 0), (($left | ForEach-Object { $_.Name }) -join ', '))
    }

    # ---------- Ж1-Ж4: удаление через мастер ----------
    Write-Utf8 (Join-Path $P.Local 'Updates\old.part') 'x'
    Write-Utf8 (Join-Path $P.Local 'thumbs\keep.txt') 'x'
    Start-QaVideo 'Ж1'
    $wiz3 = $null
    $unBefore = @()
    $btn = Get-WizButtons
    Enter-QaItem -Id 'Ж1' -Budget 300
    Step-Qa 'мастер удаления'
    try {
        # без _?=: uninstall.exe копирует себя в TEMP (NSIS 3: ~nsuXXX.tmp\Un.exe), окна принадлежат копии - процессу Un
        $unBefore = @(Get-Process -Name 'Un' -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
        [void](Start-Setup $P.Uninst '')
        $wiz3 = Invoke-QaWizard -ProcNames @('Un') -Id 'Ж1' -First $btn.Remove -Last $btn.Close
    } catch { Write-Log "Ж1: $($_.Exception.Message)" }
    # Мастер не дошёл до конца: закрываем только новые процессы Un (по PID)
    if (-not ($wiz3 -and $wiz3.Ok)) { Stop-QaNewProcs -Name 'Un' -Before $unBefore }
    Exit-QaItem
    $vid3 = Stop-QaVideo
    Invoke-QaItem 'Ж1' {
        $w = $wiz3
        $shots = @($w.Pages | ForEach-Object { $_.Shot })
        $st = 'eyes'; if (-not $w.Ok) { $st = 'fail' }
        Add-QaResult -Id 'Ж1' -Status $st -Detail ("мастер удаления дошёл до конца: {0}; {1} {2}" -f $w.Ok, $w.Why, (Get-WizSummary $w)) -Shots $shots -Video $vid3
    }
    Invoke-QaItem 'Ж2' {
        [void](Wait-Until { -not (Test-Path -LiteralPath $P.Exe) -and -not (Test-RegKey $P.KUninst) } 30)
        $left = New-Object System.Collections.ArrayList
        foreach ($f in @($P.Exe, $P.Uninst, $P.Lnk, $P.DesktopLnk, (Join-Path $P.Local 'Updates'))) { if (Test-Path -LiteralPath $f) { [void]$left.Add($f) } }
        foreach ($k in @($P.KProg, $P.KApp, $P.KUninst)) { if (Test-RegKey $k) { [void]$left.Add("HKCU\$k") } }
        if ($null -ne (Get-Reg $P.KExt '')) { [void]$left.Add('значение .salbum') }
        # ключ .salbum целиком, если он был только наш (installer-check проверяет то же отдельной строкой)
        if (Test-RegKey $P.KExt) { [void]$left.Add("HKCU\$($P.KExt) (пустой ключ остался)") }
        $st = 'fail'; if ($left.Count -eq 0) { $st = 'ok' }
        Add-QaResult -Id 'Ж2' -Status $st -Detail ("осталось лишнего: {0}" -f $(if ($left.Count) { $left -join '; ' } else { 'ничего' }))
    }
    Invoke-QaItem 'Ж3' {
        $ok = (Test-Path -LiteralPath $P.Config) -and (Test-Path -LiteralPath (Join-Path $P.Local 'thumbs\keep.txt'))
        $st = 'fail'; if ($ok) { $st = 'ok' }
        Add-QaResult -Id 'Ж3' -Status $st -Detail "config.json и миниатюры на месте: $ok"
    }
    Invoke-QaItem 'Ж4' -Budget 300 {
        Step-Qa 'тихая установка 0.25.1'
        $code = Invoke-Setup $setup1 '/S'
        $lm = Get-LogMark $P.Log
        Step-Qa 'запуск 0.25.1 из Пуска'
        $procs = Start-FromMenu 30
        $ran = Wait-Until { (Read-LogSince $P.Log $lm) -match 'запуск, версия 0\.25\.1' } 30
        $cfg = $null
        try { $cfg = [System.IO.File]::ReadAllText($P.Config) | ConvertFrom-Json } catch { }
        $kept = $cfg -and $cfg.level -eq 'simple'
        Stop-Installed
        $st = 'fail'; if ($code -eq 0 -and $ran -and $kept) { $st = 'ok' }
        Add-QaResult -Id 'Ж4' -Status $st -Detail "setup 0.25.1 /S: код $code; запуск 0.25.1: $ran; старые настройки (level=simple) видны: $kept"
    }
    [void](Invoke-QuietUninstall)
}

# ---------- Г8: installer-check.ps1 из набора ----------
Invoke-QaItem 'Г8' -Budget 960 {
    $log = Join-Path (Join-Path $OutDir 'logs') 'installer-check.txt'
    $sum = Join-Path (Join-Path $OutDir 'logs') 'installer-check-summary.md'
    $ic = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', ('"' + (Join-Path $Kit 'installer-check.ps1') + '"'), '-Setup', ('"' + $setup0 + '"'), '-Next', ('"' + $setup1 + '"'), '-Version', '0.25.0', '-Summary', ('"' + $sum + '"'), '-Force')
    $t = [Diagnostics.Stopwatch]::StartNew()
    Step-Qa 'installer-check.ps1 из набора'
    $pr = Start-Process -FilePath 'powershell.exe' -ArgumentList $ic -NoNewWindow -PassThru -RedirectStandardOutput $log
    # Без обращения к Handle до завершения Start-Process -PassThru оставляет ExitCode пустым (известная особенность .NET)
    $null = $pr.Handle
    if (-not $pr.WaitForExit(900000)) { & taskkill.exe /PID $pr.Id /T /F | Out-Null; Add-QaResult -Id 'Г8' -Status fail -Detail 'installer-check не закончился за 15 мин'; return }
    $fails = @(Get-Content -LiteralPath $log -Encoding UTF8 -ErrorAction SilentlyContinue | Where-Object { $_ -match '^FAIL' })
    $st = 'fail'; if ($pr.ExitCode -eq 0) { $st = 'ok' }
    Add-QaResult -Id 'Г8' -Status $st -Detail ("код {0}; строк FAIL: {1}; {2}" -f $pr.ExitCode, $fails.Count, ($fails -join ' | ')) -Sec $t.Elapsed.TotalSeconds
}

# ---------- Масштаб экрана: мастер на 125-200 % (последним: меняет экран) ----------
if ($Dpi -and $Dpi -ne 'none') {
    $state = Get-QaDpiState
    $orig = 100
    if ($state.Ok) { $orig = $state.Cur }
    foreach ($pct in @($Dpi.Split(',') | ForEach-Object { [int]$_.Trim() })) {
        $id = "DPI-$pct"
        Invoke-QaItem $id -Budget 360 {
            if (-not $state.Ok) { Add-QaResult -Id $id -Status laptop -Detail "масштаб на лету недоступен: $($state.Why)"; return }
            $set = Set-QaDpi $pct
            if (-not $set.Ok) { Add-QaResult -Id $id -Status laptop -Detail "$pct % не поставился: $($set.Why) (рек. $($state.Rec), макс. $($state.Max))"; return }
            $sys = Get-QaFreshSystemDpi
            $pr = Start-Setup $setup0 ''
            [void](Wait-WizWindow -ProcNames @($name0))
            $win = Get-QaWindows -ProcId $pr.Id | Select-Object -First 1
            $wdpi = 0
            if ($win) { $wdpi = Get-QaWindowDpi $win }
            $want = [int][math]::Round(96 * $pct / 100)
            $k = $pct / 100.0
            # левая картинка мастера: 164x314 при 100 %, вырезаем с запасом и увеличиваем вдвое, чтобы видеть размытие
            $logo = Save-QaShot -Id $id -Slug 'logo' -ProcId $pr.Id -Crop @(0, 30, [int](180 * $k), [int](340 * $k)) -Zoom 2
            $w = Invoke-QaWizard -ProcNames @($name0) -Id $id
            $shots = @($logo) + @($w.Pages | ForEach-Object { $_.Shot })
            [void](Invoke-QuietUninstall)
            $detail = "GetDpiForWindow мастера $wdpi (ждём $want), GetDpiForSystem нового процесса $sys, мастер дошёл до конца: $($w.Ok)"
            if ($wdpi -eq $want) { Add-QaResult -Id $id -Status eyes -Detail $detail -Shots $shots }
            else { Add-QaResult -Id $id -Status laptop -Detail ("масштаб не дошёл до окна: " + $detail) -Shots $shots }
        }
    }
    if ($state.Ok) { [void](Set-QaDpi $orig) }
}
