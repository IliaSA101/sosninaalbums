# test_winqa.ps1 - проверки чистых функций winqa без Pester (спека 118, design §9). В workflow идёт до сценариев, как test_winlib.ps1.
# Запуск: powershell -NoProfile -ExecutionPolicy Bypass -File scripts/win/qa/test_winqa.ps1  (Windows PowerShell 5.1).
# Код возврата: 0 - всё прошло, 1 - есть провалы. Рабочего стола не трогает. Файл в UTF-8 с BOM.
$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here 'winqa.psm1') -Force -DisableNameChecking
Import-Module (Join-Path $here 'winqa-dpi.psm1') -Force -DisableNameChecking
Import-Module (Join-Path $here 'winqa-update.psm1') -Force -DisableNameChecking
Import-Module (Join-Path $here 'winqa-open.psm1') -Force -DisableNameChecking
Import-Module (Join-Path (Split-Path -Parent $here) 'winui.psm1') -DisableNameChecking

$script:Fail = 0
$script:Pass = 0
function Check([string]$Name, $Cond, [string]$Detail = '') {
    if ($Cond -is [bool] -and $Cond) { $script:Pass++; Write-Host "ok    $Name" }
    elseif ($Cond -is [bool]) { $script:Fail++; Write-Host "FAIL  $Name  $Detail" }
    else { $script:Fail++; Write-Host "FAIL  $Name  условие не bool ($($Cond.GetType().Name))" }
}
function Same([string]$Name, $Got, $Want) { Check $Name ("$Got" -ceq "$Want") "получено '$Got', ожидалось '$Want'" }
function Section([string]$Title) { Write-Host "-- $Title" }

$tmp = Join-Path ([IO.Path]::GetTempPath()) ('sa-winqa-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$utf8 = New-Object System.Text.UTF8Encoding($false)

try {
    Section 'ConvertTo-AsciiId'
    Same 'Г13' (ConvertTo-AsciiId 'Г13') 'G13'
    Same 'В0-4' (ConvertTo-AsciiId 'В0-4') 'V0-4'
    Same 'Ж2' (ConvertTo-AsciiId 'Ж2') 'Zh2'
    Same 'А Б Д Е' (ConvertTo-AsciiId 'АБДЕ') 'ABDE'
    Same 'латиница не меняется' (ConvertTo-AsciiId 'P0-uia') 'P0-uia'

    Section 'Select-WizButtonText (Win32-нажатие мастера, чистая часть)'
    Import-Module (Join-Path $here 'winqa-wizard.psm1') -Force -DisableNameChecking
    $texts = @('', '&Назад', '&Далее >', 'Отмена')
    Same 'Далее: индекс' (Select-WizButtonText $texts (Get-WizButtons).Next).Index 2
    Same 'Далее: текст без &' (Select-WizButtonText $texts (Get-WizButtons).Next).Text 'Далее >'
    Same 'Установить с &' (Select-WizButtonText @('Отмена', '&Установить') (Get-WizButtons).Next).Text 'Установить'
    Same 'Повтор' (Select-WizButtonText @('&Повтор', 'Отмена') (Get-WizButtons).Retry).Index 0
    Same 'Готово для Close' (Select-WizButtonText @('&Готово') (Get-WizButtons).Close).Text 'Готово'
    Check 'нет подходящей - $null' ($null -eq (Select-WizButtonText @('Отмена') (Get-WizButtons).Remove))
    Check 'пустой список - $null' ($null -eq (Select-WizButtonText @() (Get-WizButtons).Next))

    Section 'Get-QaClampedCrop'
    Same 'внутри' ((Get-QaClampedCrop @(0, 30, 100, 100) 500 400) -join ',') '0,30,100,100'
    Same 'усечение по краю' ((Get-QaClampedCrop @(0, 30, 600, 500) 500 400) -join ',') '0,30,500,370'
    Same 'сдвиг за границу' ((Get-QaClampedCrop @(900, 900, 10, 10) 500 400) -join ',') '499,399,1,1'

    Section 'Test-LogClean'
    Check 'кириллица - ок' (Test-LogClean 'запуск, версия 0.25.0' 'runneradmin').Ok
    Check 'ПАНИКА - провал' (-not (Test-LogClean 'ПАНИКА: x' 'runneradmin').Ok)
    Check 'https - провал' (-not (Test-LogClean 'открыл https://example.invalid/x' 'runneradmin').Ok)
    Check 'http - провал' (-not (Test-LogClean 'лента http://127.0.0.1:8765/' 'runneradmin').Ok)
    Check 'C:\Users\ - провал' (-not (Test-LogClean 'файл C:\Users\x\a.salbum' 'runneradmin').Ok)
    Check 'имя пользователя - провал' (-not (Test-LogClean 'папка runneradmin' 'runneradmin').Ok)
    Check 'короткое имя пользователя не ловится' (Test-LogClean 'ab cd' 'ab').Ok

    Section 'Read-LogSince'
    $log = Join-Path $tmp 'log.txt'
    $text = "строка один`nстрока два`n"
    [System.IO.File]::WriteAllText($log, $text, $utf8)
    $mark = $utf8.GetByteCount("строка один`n")
    Same 'от отметки' (Read-LogSince $log $mark) "строка два`n"
    Same 'отметка 0 - весь файл' (Read-LogSince $log 0) $text
    # отметка в середине многобайтного знака: хвост символа отбрасывается, остальное читается
    Same 'отметка внутри знака' (Read-LogSince $log 1) (($utf8.GetString($utf8.GetBytes($text), 2, $utf8.GetByteCount($text) - 2)))
    Same 'нет файла - пусто' (Read-LogSince (Join-Path $tmp 'нет.txt') 0) ''
    Same 'отметка больше файла - весь файл' (Read-LogSince $log 99999) $text
    Same 'Get-LogMark - длина в байтах' (Get-LogMark $log) $utf8.GetByteCount($text)
    # файл открыт на запись, как у работающей программы
    $fs = New-Object System.IO.FileStream($log, 'Append', 'Write', 'ReadWrite')
    try { Same 'чтение при открытой записи' (Read-LogSince $log $mark) "строка два`n" } finally { $fs.Dispose() }

    Section 'Add-QaResult'
    $out = Join-Path $tmp 'out'
    Initialize-Qa -OutDir $out -Job 'test' -Runner 'windows-2025'
    Add-QaResult -Id 'Г13' -Status eyes -Detail "кавычка `" и обратная \ и перевод`nстроки" -Shots @('png/G13-a.png', '') -Sec 11.84
    Add-QaResult -Id 'В4' -Status skip -Blocked 'В0-1'
    Add-QaResult -Id 'А1' -Status ok -Video 'video/A1.mp4'
    $rows = [System.IO.File]::ReadAllLines((Join-Path $out 'results.jsonl'), $utf8)
    Same 'три строки' $rows.Count 3
    Same 'экранирование, пустой снимок отброшен, сек до десятых' $rows[0] '{"id":"Г13","status":"eyes","detail":"кавычка \" и обратная \\ и перевод\nстроки","shots":["png/G13-a.png"],"video":null,"sec":11.8,"job":"test","runner":"windows-2025"}'
    Check 'blocked - статус skip и причина' ($rows[1] -like '*"status":"skip","detail":"нет В0-1"*')
    Check 'видео' ($rows[2] -like '*"video":"video/A1.mp4"*')
    Check 'без BOM' ([System.IO.File]::ReadAllBytes((Join-Path $out 'results.jsonl'))[0] -eq 0x7B)
    $threw = $false
    try { Add-QaResult -Id 'X' -Status 'bad' } catch { $threw = $true }
    Check 'неизвестный статус отвергается' $threw

    Section 'Invoke-QaItem, Step-Qa, пульс'
    Invoke-QaItem 'T-pulse' -Budget 50 {
        Step-Qa 'шаг один'
        $pf = Join-Path $out 'logs\current.json'
        $pj = Get-Content -LiteralPath $pf -Raw -Encoding UTF8 | ConvertFrom-Json
        Same 'пульс: пункт' $pj.id 'T-pulse'
        Same 'пульс: шаг' $pj.step 'шаг один'
        Check 'пульс: срок впереди' (([double]$pj.deadline - [double]$pj.now) -gt 40 -and ([double]$pj.deadline - [double]$pj.now) -le 50)
        Add-QaResult -Id 'T-pulse' -Status ok
    }
    $pj2 = Get-Content -LiteralPath (Join-Path $out 'logs\current.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    Same 'после пункта пульс пуст' $pj2.id ''
    $threw = $false
    $bad = 'double'   # нарочно неверный режим
    try { Invoke-MouseClick $bad } catch { $threw = $true }
    Check 'Invoke-MouseClick: неизвестный режим - исключение' $threw

    Section 'Get-QaWatchAction'
    $st = @{ HangAt = 0.0; HangId = ''; HangSeq = -1; HangStep = ''; Block = $false }
    $pu = [pscustomobject]@{ id = 'В2'; step = 'щелчок'; seq = 3; deadline = 1000.0 }
    Same 'в срок - ничего' (Get-QaWatchAction $pu 900 $st 0).Action 'none'
    $a = Get-QaWatchAction $pu 1001 $st 0
    Same 'срок вышел - hang' $a.Action 'hang'
    Same 'hang: пункт' $a.Id 'В2'
    $st.HangAt = 1001.0; $st.HangId = 'В2'; $st.HangSeq = 3; $st.HangStep = 'щелчок'
    Same 'до минуты - ждём' (Get-QaWatchAction $pu 1030 $st 0).Action 'none'
    Same 'минута без движения - abort' (Get-QaWatchAction $pu 1062 $st 0).Action 'abort'
    $pu2 = [pscustomobject]@{ id = 'В2'; step = 'другой'; seq = 4; deadline = 1000.0 }
    $a2 = Get-QaWatchAction $pu2 1062 $st 0
    Check 'пульс сдвинулся - не abort' ($a2.Action -ne 'abort')
    $st = @{ HangAt = 0.0; HangId = ''; HangSeq = -1; HangStep = ''; Block = $false }
    Same 'срок блока - hang' (Get-QaWatchAction $null 2000 $st 0 1800).Reason 'block'

    Section 'Test-QaKit'
    $kit = Join-Path $tmp 'kit'
    New-Item -ItemType Directory -Force -Path (Join-Path $kit 'sub') | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $kit 'a.txt'), 'aaa', $utf8)
    [System.IO.File]::WriteAllText((Join-Path $kit 'sub\b.txt'), 'bbb', $utf8)
    $ha = (Get-FileHash -LiteralPath (Join-Path $kit 'a.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
    $hb = (Get-FileHash -LiteralPath (Join-Path $kit 'sub\b.txt') -Algorithm SHA256).Hash.ToLowerInvariant()
    [System.IO.File]::WriteAllText((Join-Path $kit 'kit-sha256.txt'), "$ha  a.txt`n$hb  sub/b.txt`n", $utf8)
    Check 'набор сходится' (Test-QaKit $kit).Ok
    [System.IO.File]::WriteAllText((Join-Path $kit 'a.txt'), 'xxx', $utf8)
    Check 'испорченный файл ловится' (-not (Test-QaKit $kit).Ok)
    Check 'нет списка хешей' (-not (Test-QaKit (Join-Path $tmp 'пусто')).Ok)

    Section 'Get-QaPaths'
    $pp = Get-QaPaths
    Check 'пути установки' ($pp.Exe -like '*Programs\Sosnina Albums\SosninaAlbums.exe' -and $pp.Config -like '*SosninaAlbums\config.json')

    Section 'Get-QaFeedName'
    Same 'feed без -Sabotage' (Get-QaFeedName 'feed' $false) 'feed'
    Same 'feed с -Sabotage - битый байт' (Get-QaFeedName 'feed' $true) 'feed-bad-byte'
    Same 'другие ленты не подменяются' (Get-QaFeedName 'feed-wrong-key' $true) 'feed-wrong-key'

    Section 'New-QaStandConfig'
    $cfgj = New-QaStandConfig | ConvertFrom-Json
    Check 'стенд: уровень, без стартового окна и вопроса о связи' ($cfgj.level -eq 'simple' -and $cfgj.hideStart -eq $true -and $cfgj.assoc -eq 'off')
    Check 'стенд: проверка при запуске выключена' ($cfgj.update.off -eq $true)
    Check 'стенд: туры закрыты' ($cfgj.tours.first -eq 'done' -and $cfgj.tours.export -eq 'done')
    $cfgp = New-QaStandConfig 'C:\a "b"\Alpha.salbum' | ConvertFrom-Json
    Same 'стенд: путь альбома с кавычкой и слэшами' $cfgp.lastProject 'C:\a "b"\Alpha.salbum'

    Section 'Test-QaUpdateChain'
    $dash = [string][char]0x2014
    $goodLog = "запуск, версия 0.25.0`nобновление: найдена версия 0.25.1`nобновление: версия 0.25.1 скачана и сверена`nобновление: запущен установщик версии 0.25.1 (install)`nзапуск, версия 0.25.1`nобновление: итог запуска установщика 0.25.1 $dash обновлено `"0.25.1`", не установилось `"`"`n"
    Check 'цепочка целиком' (Test-QaUpdateChain $goodLog '0.25.1').Ok
    $noRun = $goodLog.Replace('обновление: запущен установщик версии 0.25.1 (install)', 'x')
    $r1 = Test-QaUpdateChain $noRun '0.25.1'
    Check 'нет запуска установщика - ловится' (-not $r1.Ok -and $r1.Missing[0] -eq 'запущен установщик (install)')
    $swap = "обновление: итог запуска установщика 0.25.1 $dash обновлено `"0.25.1`"`nобновление: найдена версия 0.25.1`n"
    Check 'порядок строк важен' (-not (Test-QaUpdateChain $swap '0.25.1').Ok)
    $failed = $goodLog.Replace("обновлено `"0.25.1`", не установилось `"`"", "обновлено `"`", не установилось `"0.25.1`"")
    Check 'итог "не установилось" - не успех' (-not (Test-QaUpdateChain $failed '0.25.1').Ok)

    Section 'Test-QaOnlyData'
    Check 'только основной поток' (Test-QaOnlyData @(':$DATA')).Ok
    $zs = Test-QaOnlyData @(':$DATA', 'Zone.Identifier')
    Check 'метка скачанного ловится' (-not $zs.Ok -and $zs.Extra[0] -eq 'Zone.Identifier')

    Section 'Get-QaStandPaths'
    $sp = Get-QaStandPaths
    Check 'пути стенда qa' ($sp.Config -like '*SosninaAlbums-stand-qa\config.json' -and $sp.Updates -like '*SosninaAlbums-stand-qa\Updates')

    Section 'Get-QaSeedConfig, New-QaOpenConfig'
    Check 'сид фазы А: только выключенная проверка обновлений' (((Get-QaSeedConfig) | ConvertFrom-Json).update.off -eq $true)
    $oc = New-QaOpenConfig 'C:\a "b"\Beta.salbum' 'on' | ConvertFrom-Json
    Check 'конфиг Б: полный уровень, связь on, без стартового окна' ($oc.level -eq 'full' -and $oc.assoc -eq 'on' -and $oc.hideStart -eq $true)
    Same 'конфиг Б: путь с кавычкой и слэшами' $oc.lastProject 'C:\a "b"\Beta.salbum'
    Check 'конфиг Б: туры закрыты' ($oc.tours.first -eq 'done' -and $oc.tours.export -eq 'done' -and $oc.update.off -eq $true)
    $od = New-QaOpenConfig '' 'off' $true | ConvertFrom-Json
    Check 'конфиг Д: стартовое окно показано, связь off, без lastProject' ($null -eq $od.hideStart -and $od.assoc -eq 'off' -and $null -eq $od.lastProject)

    Section 'Test-QaAssocCommand'
    $exe = 'C:\Users\x\Sosnina Albums 0.25.0-rel\SosninaAlbums-0.25.0.exe'
    Check 'команда с кавычками' (Test-QaAssocCommand ('"' + $exe + '" "%1"') $exe)
    Check 'регистр пути не важен' (Test-QaAssocCommand ('"' + $exe.ToUpper() + '" "%1"') $exe)
    Check 'другой exe - нет' (-not (Test-QaAssocCommand '"C:\Programs\SosninaAlbums.exe" "%1"' $exe))
    Check 'без "%1" - нет' (-not (Test-QaAssocCommand ('"' + $exe + '"') $exe))
    Check 'пустая команда - нет' (-not (Test-QaAssocCommand '' $exe))

    Section 'Get-QaSpreadCount'
    $alb = Join-Path $tmp 'a.salbum'
    [System.IO.File]::WriteAllText($alb, '{"version":1,"spreads":[{"id":"a"},{"id":"b"},{"id":"c"}]}', $utf8)
    Same 'три разворота' (Get-QaSpreadCount $alb) 3
    [System.IO.File]::WriteAllText($alb, '{"version":1}', $utf8)
    Same 'нет массива - ноль' (Get-QaSpreadCount $alb) 0
    Same 'нет файла - минус один' (Get-QaSpreadCount (Join-Path $tmp 'нет.salbum')) -1
    [System.IO.File]::WriteAllText($alb, 'не json', $utf8)
    Same 'не json - минус один' (Get-QaSpreadCount $alb) -1

    Section 'Get-QaNewerLeadRegex, Get-QaNewerNextRegex'
    # строки собраны так же, как в frontend/src/shell/ask/newerask.ts
    $lead1 = '«NewerTest-0.25.1.salbum» сохранён в Sosnina Albums 0.25.1, а эта программа — 0.25.0. Открыть его может версия 0.25.1 или новее.'
    Check 'первая строка с номерами' ($lead1 -match (Get-QaNewerLeadRegex '0.25.1' '0.25.0'))
    Check 'первая строка: версия 0.26 не подходит под 0.25.1' (-not ('«x» сохранён в Sosnina Albums 0.26, а эта программа — 0.25.0. Открыть его может версия 0.26 или новее.' -match (Get-QaNewerLeadRegex '0.25.1' '0.25.0')))
    Check 'версия не размыта точкой' (-not ('«x» сохранён в Sosnina Albums 0x25x1, а эта программа — 0.25.0. Открыть его может версия 0x25x1 или новее.' -match (Get-QaNewerLeadRegex '0.25.1' '0.25.0')))
    Check 'версия в файле неизвестна' ('«NewerTest-unknown.salbum» сохранён более новой версией программы.' -match (Get-QaNewerLeadRegex ''))
    Check 'вторая строка: версия уже есть' ('Версия 0.25.1 уже есть на компьютере. Эта программа закроется, альбом откроется в 0.25.1.' -match (Get-QaNewerNextRegex '0.25.1'))
    Check 'вторая строка: версии нет - скачать' ('Скачайте новую версию — альбомы и настройки сохранятся.' -match (Get-QaNewerNextRegex ''))
    Check 'вторая строка: «есть» не подходит под «нет»' (-not ('Версия 0.25.1 уже есть на компьютере. Эта программа закроется, альбом откроется в 0.25.1.' -match (Get-QaNewerNextRegex '')))

    Section 'Get-QaShownVers'
    # таблица совпадает с TestShownVers в internal/app/newer_test.go (сверяет scripts/win/test_shownvers.py)
    foreach ($c in @(
        @('0.25.1', '0.25.0', '0.25.1', '0.25.0'),
        @('0.25.1-rc2+abc', '0.25.0-dev', '0.25.1', '0.25.0'),
        @('0.25.0', '0.25.0', '0.25.0', '0.25.0'),
        @('0.26.0', '0.25.3', '0.26', '0.25'),
        @('0.25', '0.25.0', '0.25', '0.25'),
        @('0.25.1', '0.25', '0.25', '0.25'),
        @('0.25.1', 'dev', '0.25', ''),
        @('', '0.25.0', '', '0.25'),
        @('dev', '', '', '')
    )) {
        $r = Get-QaShownVers $c[0] $c[1]
        Check ("версии '{0}', '{1}' -> '{2}', '{3}'" -f $c[0], $c[1], $c[2], $c[3]) ($r.Need -ceq $c[2] -and $r.Mine -ceq $c[3])
    }

    Section 'Test-QaTourLog'
    Same 'шаг из журнала' (Test-QaTourLog "x`nтур закрыт: Пропустить, шаг 3, запуск auto`ny") 3
    Same 'закрытие окна строку не пишет - ноль' (Test-QaTourLog "x`nзапуск, версия 0.25.0`ny") 0
    Same 'нет строки - ноль' (Test-QaTourLog 'запуск, версия 0.25.0') 0

    Section 'Get-QaDpiState (без побочных эффектов)'
    $st = Get-QaDpiState
    Check 'состояние масштаба - объект с Ok' ($st.Ok -is [bool])
} finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ("`nпрошло {0}, провалов {1}" -f $script:Pass, $script:Fail)
if ($script:Fail -gt 0) { exit 1 }
exit 0
