# test_winlib.ps1 - проверки чистых функций scripts/win без Pester (спека 036, design §1).
# Запуск: powershell -NoProfile -ExecutionPolicy Bypass -File scripts/win/test_winlib.ps1  (Windows PowerShell 5.1).
# Код возврата: 0 - всё прошло, 1 - есть провалы. Рабочего стола не трогает (курсор, экран, ввод - только на раннере).
# Файл в UTF-8 с BOM.
$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$root = Split-Path -Parent (Split-Path -Parent $here)
Import-Module (Join-Path $here 'winlib.psm1') -Force -DisableNameChecking

$script:Fail = 0
$script:Pass = 0
# Cond - строго [bool]: массив или строка (например, результат -match по списку) - провал, а не молчаливое «истина»
function Check([string]$Name, $Cond, [string]$Detail = '') {
    if ($Cond -is [bool] -and $Cond) { $script:Pass++; Write-Host "ok    $Name" }
    elseif ($Cond -is [bool]) { $script:Fail++; Write-Host "FAIL  $Name  $Detail" }
    else { $script:Fail++; Write-Host "FAIL  $Name  условие не bool ($($Cond.GetType().Name))" }
}
# Есть ли в списке строка, подходящая под шаблон (всегда bool; -match по массиву даёт массив строк)
function Has($Items, [string]$Pattern) { return (@(@($Items) | Where-Object { $_ -match $Pattern }).Count -gt 0) }
function Same([string]$Name, $Got, $Want) {
    Check $Name ("$Got" -ceq "$Want") "получено '$Got', ожидалось '$Want'"
}
function Section([string]$Title) { Write-Host "-- $Title" }

$tmp = Join-Path ([IO.Path]::GetTempPath()) ('sa-winlib-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
    Section 'JSON'
    Same 'строка: кавычка, обратная косая, перевод строки' (ConvertTo-JLine "a`"b\c`nd") '"a\"b\\c\nd"'
    Same 'кириллица без экранирования' (ConvertTo-JLine 'Нажмите «Готово»') '"Нажмите «Готово»"'
    Same 'управляющий знак -> \u' (ConvertTo-JLine ([string][char]1)) '"\u0001"'
    Same 'null, true, false' ((ConvertTo-JLine $null) + (ConvertTo-JLine $true) + (ConvertTo-JLine $false)) 'nulltruefalse'
    Same 'пустой массив' (ConvertTo-JLine @()) '[]'
    Same 'одноэлементный массив остаётся массивом' (ConvertTo-JLine ([ordered]@{ a = @(5) })) '{"a":[5]}'
    Same 'порядок ключей сохраняется' (ConvertTo-JLine ([ordered]@{ ev = 'x'; t = 1.5; a = 'z' })) '{"ev":"x","t":1.5,"a":"z"}'
    Same 'вложенный массив рамки' (ConvertTo-JLine ([ordered]@{ rect = @(1, 2, 3, 4) })) '{"rect":[1,2,3,4]}'
    $old = [Threading.Thread]::CurrentThread.CurrentCulture
    [Threading.Thread]::CurrentThread.CurrentCulture = [Globalization.CultureInfo]::GetCultureInfo('ru-RU')
    Same 'число не зависит от культуры (запятая)' (ConvertTo-JLine 0.25) '0.25'
    Same 'округление до мс' (ConvertTo-JLine 3.14159) '3.142'
    [Threading.Thread]::CurrentThread.CurrentCulture = $old

    Section 'Журнал: круг записи и проверки'
    $jp = Join-Path $tmp 'video\steps.jsonl'
    Start-Journal $jp ([ordered]@{ os = 'windows'; ver = '26100.1'; lang = 'en'; space = 'pixels'; screen = @(1920, 1080); scale = 1; recorder = 'gdigrab'; script = 'test' })
    Add-JSync '#d81f26' 0.8
    Add-JChapter 'Первый запуск'
    Add-JStep 1 2 'Скачайте программу'
    # отрезок пишется после конца, но по t0 раньше уже записанных строк - файл обязан выровнять порядок
    $t0 = Get-JTime
    Start-Sleep -Milliseconds 30
    Add-JHold @(100, 200, 300, 40) 'Нажмите «Готово»'
    Start-Sleep -Milliseconds 30
    Add-JSegment 'idle' $t0 (Get-JTime)
    Add-JHold $null 'Подпись без стрелки'
    $m0 = Get-JTime
    Start-Sleep -Milliseconds 20
    Add-JSegment 'move' $m0 (Get-JTime)
    Add-JKeys $m0 (Get-JTime) @('Alt', 'Enter')
    Add-JNote 'заметка'
    [void](Close-Journal)
    $bytes = [IO.File]::ReadAllBytes($jp)
    Check 'файл без BOM' (-not ($bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB))
    Check 'переводы строк LF' (-not ([Text.Encoding]::UTF8.GetString($bytes)).Contains("`r"))
    $lines = [IO.File]::ReadAllLines($jp, [Text.Encoding]::UTF8)
    Same 'первая строка - meta' (($lines[0] | ConvertFrom-Json).ev) 'meta'
    Same 'последняя строка - end' (($lines[-1] | ConvertFrom-Json).ev) 'end'
    Same 'число строк' $lines.Count 11
    $holdLine = $lines | Where-Object { $_ -like '*"ev":"hold"*' } | Select-Object -First 1
    Same 'кириллица переживает круг записи' (($holdLine | ConvertFrom-Json).caption) 'Нажмите «Готово»'
    $errs = @(Test-JournalLines $lines)
    Same 'годный журнал без ошибок' $errs.Count 0
    # hold и keys несут ключ 'keys' - по нему сериализатор раньше терял поля (точка у словаря отдаёт значение ключа)
    Same 'hold со словарём-ключом keys сериализуется' (ConvertTo-JLine ([ordered]@{ ev = 'hold'; keys = $null })) '{"ev":"hold","keys":null}'
    Same 'keys с двумя клавишами' (ConvertTo-JLine ([ordered]@{ ev = 'keys'; keys = @('Alt', 'Enter') })) '{"ev":"keys","keys":["Alt","Enter"]}'
    Same 'в файле все hold и keys на месте' (@($lines | Where-Object { $_ -like '*"ev":"hold"*' -or $_ -like '*"ev":"keys"*' }).Count) 3
    # idle (t0 раньше hold) должен встать перед hold
    $ev = @($lines | ForEach-Object { ($_ | ConvertFrom-Json).ev })
    Check 'выровнен порядок: idle раньше hold' ([array]::IndexOf($ev, 'idle') -lt [array]::IndexOf($ev, 'hold'))

    Section 'Журнал: края'
    $jp2 = Join-Path $tmp 'empty.jsonl'
    Start-Journal $jp2 ([ordered]@{ os = 'windows'; space = 'pixels'; screen = @(1280, 720); scale = 1 })
    [void](Close-Journal)
    $l2 = [IO.File]::ReadAllLines($jp2)
    Same 'пустой журнал: meta и end' $l2.Count 2
    Same 'пустой журнал годен' (@(Test-JournalLines $l2)).Count 0
    $meta = '{"ev":"meta","screen":[1920,1080]}'
    Check 'убывающее t -> ошибка со строкой' (Has (Test-JournalLines @($meta, '{"ev":"note","t":5,"text":"a"}', '{"ev":"note","t":4,"text":"b"}')) 'строка 3')
    Check 'пустая подпись -> ошибка' (Has (Test-JournalLines @($meta, '{"ev":"hold","t":1,"rect":null,"caption":" ","keys":null}')) 'строка 2: пустая подпись')
    $long = 'я' * 81
    Check 'подпись длиннее 80 -> ошибка' (Has (Test-JournalLines @($meta, ('{"ev":"hold","t":1,"rect":null,"caption":"' + $long + '","keys":null}'))) 'длиннее 80')
    Same 'подпись ровно 80 знаков - годна' (@(Test-JournalLines @($meta, ('{"ev":"hold","t":1,"rect":null,"caption":"' + ('я' * 80) + '","keys":null}')))).Count 0
    Check 'рамка за правым краем -> ошибка' (Has (Test-JournalLines @($meta, '{"ev":"hold","t":1,"rect":[1900,10,50,20],"caption":"x","keys":null}')) 'вне экрана')
    Check 'рамка нулевой ширины -> ошибка' (Has (Test-JournalLines @($meta, '{"ev":"hold","t":1,"rect":[10,10,0,20],"caption":"x","keys":null}')) 'вне экрана')
    Same 'рамка вплотную к краю годна' (@(Test-JournalLines @($meta, '{"ev":"hold","t":1,"rect":[1870,1060,50,20],"caption":"x","keys":null}'))).Count 0
    Check 'meta не первая -> ошибка' (Has (Test-JournalLines @('{"ev":"note","t":1,"text":"a"}', $meta)) 'meta')
    Check 'idle пересекает move -> ошибка' (Has (Test-JournalLines @($meta, '{"ev":"idle","t0":1,"t1":5}', '{"ev":"move","t0":3,"t1":4}')) 'idle пересекается')
    Same 'idle встык с move годен' (@(Test-JournalLines @($meta, '{"ev":"idle","t0":1,"t1":3}', '{"ev":"move","t0":3,"t1":4}'))).Count 0
    Check 'не JSON -> ошибка' (Has (Test-JournalLines @($meta, 'ну это не json')) 'строка 2: не JSON')
    Check 'несколько ошибок сразу' (@(Test-JournalLines @($meta, '{"ev":"hold","t":2,"rect":null,"caption":"","keys":null}', '{"ev":"hold","t":1,"rect":[5000,0,1,1],"caption":"y","keys":null}')).Count -ge 3)

    Section 'Рамки и подписи'
    Same 'Limit-Rect: внутри не меняется' ((Limit-Rect @(10, 20, 30, 40) 100 100) -join ',') '10,20,30,40'
    Same 'Limit-Rect: обрезка за краем' ((Limit-Rect @(90, 90, 30, 30) 100 100) -join ',') '90,90,10,10'
    Same 'Limit-Rect: отрицательный угол' ((Limit-Rect @(-10, -5, 30, 30) 100 100) -join ',') '0,0,20,25'
    Check 'Limit-Rect: вне экрана -> null' ($null -eq (Limit-Rect @(200, 200, 30, 30) 100 100))
    Check 'Limit-Rect: null -> null' ($null -eq (Limit-Rect $null 100 100))
    Same 'подпись русского окна' (Get-Caption 'Подробнее' 'Подробнее') 'Нажмите «Подробнее»'
    Same 'подпись английского окна' (Get-Caption 'Подробнее' 'More info') 'Нажмите «Подробнее» (More info)'
    Same 'подпись без найденного имени' (Get-Caption 'Готово' '') 'Нажмите «Готово»'
    Same 'подпись с другим глаголом' (Get-Caption 'Свойства' 'Properties' 'Выберите') 'Выберите «Свойства» (Properties)'
    Same 'длинное английское имя не дописывается' (Get-Caption 'Х' ('a' * 60)) 'Нажмите «Х»'
    Same 'другое русское имя не дописывается' (Get-Caption 'Сохранить' 'Сохранить как') 'Нажмите «Сохранить»'

    Section 'OCR: поиск фразы'
    $words = @(
        [pscustomobject]@{ Text = 'Windows'; X = 100; Y = 50; W = 70; H = 20 },
        [pscustomobject]@{ Text = 'protected'; X = 175; Y = 52; W = 80; H = 20 },
        [pscustomobject]@{ Text = 'More'; X = 100; Y = 200; W = 40; H = 18 },
        [pscustomobject]@{ Text = 'info'; X = 145; Y = 200; W = 30; H = 18 },
        [pscustomobject]@{ Text = 'More'; X = 100; Y = 400; W = 40; H = 18 })
    Same 'фраза из двух слов - объединённая рамка' ((Find-PhraseRect $words 'More info') -join ',') '100,200,75,18'
    Same 'регистр и пунктуация не важны' ((Find-PhraseRect $words 'windows PROTECTED,') -join ',') '100,50,155,22'
    Check 'фразы нет -> null' ($null -eq (Find-PhraseRect $words 'Run anyway'))
    Check 'слова не подряд -> null' ($null -eq (Find-PhraseRect $words 'Windows info'))
    Check 'фраза длиннее списка -> null' ($null -eq (Find-PhraseRect @($words[0]) 'Windows protected'))
    Check 'пустой список слов -> null' ($null -eq (Find-PhraseRect @() 'More'))
    Check 'пустая фраза -> null' ($null -eq (Find-PhraseRect $words ''))
    Same 'одно слово: берётся первое совпадение' ((Find-PhraseRect $words 'More') -join ',') '100,200,40,18'

    Section 'Кадры'
    function New-Png([string]$Path, [int]$W, [int]$H, [scriptblock]$Paint) {
        $b = New-Object System.Drawing.Bitmap $W, $H
        $g = [System.Drawing.Graphics]::FromImage($b)
        & $Paint $g
        $g.Dispose(); $b.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png); $b.Dispose()
    }
    $black = Join-Path $tmp 'black.png'; $red = Join-Path $tmp 'red.png'; $half = Join-Path $tmp 'half.png'; $small = Join-Path $tmp 'small.png'
    New-Png $black 200 100 { param($g) $g.Clear([System.Drawing.Color]::Black) }
    New-Png $red 200 100 { param($g) $g.Clear([System.Drawing.ColorTranslator]::FromHtml('#d81f26')) }
    New-Png $half 200 100 { param($g) $g.Clear([System.Drawing.Color]::Black); $g.FillRectangle([System.Drawing.Brushes]::White, 0, 0, 100, 100) }
    New-Png $small 20 10 { param($g) $g.Clear([System.Drawing.Color]::Black) }
    Same 'чёрный кадр: доля чёрного 1' (Get-BlackRatio $black) 1
    Same 'красный кадр: чёрного 0' (Get-BlackRatio $red) 0
    Same 'половина белая: чёрного 0.5' (Get-BlackRatio $half) 0.5
    Same 'одинаковые кадры: разницы 0' (Get-DiffRatio $black $black) 0
    Same 'чёрный и красный: разница 1' (Get-DiffRatio $black $red) 1
    Same 'половина изменилась: 0.5' (Get-DiffRatio $black $half) 0.5
    Same 'разные размеры: 1' (Get-DiffRatio $black $small) 1
    Same 'нет файла: 1' (Get-DiffRatio $black (Join-Path $tmp 'нет.png')) 1
    $rgb = Get-StripColor $red 64
    Check 'цвет полосы у края - метка синхронизации' ([math]::Abs($rgb[0] - 216) -lt 1 -and [math]::Abs($rgb[1] - 31) -lt 1 -and [math]::Abs($rgb[2] - 38) -lt 1) ($rgb -join ',')
    $rgbHalf = Get-StripColor $half 64
    Same 'полоса шире картинки не падает: белая половина' ($rgbHalf[0]) 255

    Section 'Список кадров ffmpeg'
    $cl = Format-ConcatList @([pscustomobject]@{ Path = 'C:\a\f1.jpg'; Dur = 0.2 }, [pscustomobject]@{ Path = "C:\a\o'k.jpg"; Dur = 1.5 })
    Same 'concat: пути со слэшами, длительности, повтор последнего' ($cl.TrimEnd() -replace "`n", '|') "file 'C:/a/f1.jpg'|duration 0.2|file 'C:/a/o'\''k.jpg'|duration 1.5|file 'C:/a/o'\''k.jpg'"
    Same 'concat: пустой список - пусто' (Format-ConcatList @()) ''

    Section 'Сводка и shots.json'
    Set-SummaryPath (Join-Path $tmp 'summary.md')
    Add-SummarySection 'Пробы' @('проба', 'итог', 'почему')
    Add-SummaryRow 'a|b' 'да' "две`nстроки"
    $sum = [IO.File]::ReadAllText((Join-Path $tmp 'summary.md'), [Text.Encoding]::UTF8)
    Check 'ячейка: | экранирован, перевод строки убран' ($sum.Contains('| a\|b | да | две строки |'))
    Check 'заголовок таблицы' ($sum.Contains('| проба | итог | почему |') -and $sum.Contains('| --- | --- | --- |'))
    Add-ShotRecord 'win-04-smartscreen' $true '' 'png/win-04-smartscreen.png' @(10, 20, 30, 40) 'Подробнее' 0.31 @{ windows = '26100.1'; screen = @(1920, 1080) }
    Add-ShotRecord 'win-05-x' $false 'нет окна' 'png/win-05-x.png' $null '' $null @{}
    Save-ShotsJson (Join-Path $tmp 'shots.json')
    $sj = [IO.File]::ReadAllText((Join-Path $tmp 'shots.json'), [Text.Encoding]::UTF8) | ConvertFrom-Json
    Same 'shots.json: две записи' @($sj).Count 2
    Same 'shots.json: рамка цели' ($sj[0].target.rect -join ',') '10,20,30,40'
    Same 'shots.json: os' $sj[0].os 'windows'
    Check 'shots.json: цели нет - null' ($null -eq $sj[1].target)
    Check 'shots.json: ok=false' ($sj[1].ok -eq $false)

    Section 'Способ записи (method.txt)'
    $mf = Join-Path $tmp 'method.txt'
    $def = @('gdigrab', 'ddagrab', 'frames')
    Same 'файла нет - порядок по умолчанию' ((Get-RecMethods $mf $def) -join ',') 'gdigrab,ddagrab,frames'
    Same 'путь $null - по умолчанию, не падает' ((Get-RecMethods $null $def) -join ',') 'gdigrab,ddagrab,frames'
    Same 'путь пустой - по умолчанию' ((Get-RecMethods '' @('frames')) -join ',') 'frames'
    Same 'нет ни файла, ни умолчания - пусто' (@(Get-RecMethods $mf)).Count 0
    [IO.File]::WriteAllText($mf, '')
    Same 'пустой файл (0 байт, как на ARM) - записи нет, не падает' (@(Get-RecMethods $mf $def)).Count 0
    [IO.File]::WriteAllText($mf, "  `r`n `n")
    Same 'только пробелы и переводы строк - записи нет' (@(Get-RecMethods $mf $def)).Count 0
    [IO.File]::WriteAllText($mf, 'frames')
    Same 'один способ без перевода строки' ((Get-RecMethods $mf $def) -join ',') 'frames'
    [IO.File]::WriteAllText($mf, "ddagrab`r`n")
    Same 'способ с CRLF в конце' ((Get-RecMethods $mf $def) -join ',') 'ddagrab'
    [IO.File]::WriteAllText($mf, "`r`n gdigrab `r`nframes")
    Same 'несколько строк - первая непустая, без пробелов' ((Get-RecMethods $mf $def) -join ',') 'gdigrab'
    [IO.File]::WriteAllText($mf, 'frames', (New-Object System.Text.UTF8Encoding($true)))
    Same 'BOM в файле не попадает в имя' ((Get-RecMethods $mf $def) -join ',') 'frames'
    Same 'результат - ровно один способ' (@(Get-RecMethods $mf $def)).Count 1

    Section 'Запуск exe: «canceled by the user» = окно SmartScreen'
    Check 'Win32Exception 1223' (Test-LaunchCanceled (New-Object System.ComponentModel.Win32Exception 1223))
    $inv = New-Object System.InvalidOperationException 'This command cannot be run due to the error: The operation was canceled by the user.'
    Check 'сообщение Start-Process (английское)' (Test-LaunchCanceled $inv)
    $rec = New-Object System.Management.Automation.ErrorRecord $inv, 'InvalidOperationException', ([System.Management.Automation.ErrorCategory]::InvalidOperation), $null
    Check 'ErrorRecord (как $_ в catch)' (Test-LaunchCanceled $rec)
    $wrapped = New-Object System.InvalidOperationException 'Не удалось запустить', (New-Object System.ComponentModel.Win32Exception 1223)
    Check 'код 1223 во вложенном исключении' (Test-LaunchCanceled $wrapped)
    Check 'русское сообщение' (Test-LaunchCanceled (New-Object System.Exception 'Операция была отменена пользователем.'))
    Check 'cancelled с двумя l' (Test-LaunchCanceled (New-Object System.Exception 'The operation was cancelled by the user.'))
    Check 'null - не отмена, не падает' (-not (Test-LaunchCanceled $null))
    Check 'другая ошибка Win32 (5) - не отмена' (-not (Test-LaunchCanceled (New-Object System.ComponentModel.Win32Exception 5)))
    Check 'файл не найден - не отмена' (-not (Test-LaunchCanceled (New-Object System.InvalidOperationException 'The system cannot find the file specified')))
    Check 'ErrorRecord с другой ошибкой - не отмена' (-not (Test-LaunchCanceled (New-Object System.Management.Automation.ErrorRecord (New-Object System.Exception 'boom'), 'x', ([System.Management.Automation.ErrorCategory]::NotSpecified), $null)))

    Section 'Входы workflow'
    Check 'tag vX.Y.Z' ((Test-TagInput 'v0.9.1') -and -not (Test-TagInput 'v0.9') -and -not (Test-TagInput 'v0.9.1; calc') -and -not (Test-TagInput 'V0.9.1') -and -not (Test-TagInput ''))
    Check 'tag: перевод строки не проходит' (-not (Test-TagInput "v1.2.3`nx"))
    Check 'run_id число' ((Test-RunIdInput '1234567890') -and -not (Test-RunIdInput '12a') -and -not (Test-RunIdInput '') -and -not (Test-RunIdInput '1 2'))
    Check 'раннер из списка' ((Test-RunnerInput 'windows-11-arm') -and (Test-RunnerInput 'windows-2025') -and -not (Test-RunnerInput 'windows-latest') -and -not (Test-RunnerInput 'windows-11-arm,x'))
    $yml = Join-Path $root '.github\workflows\win-shots.yml'
    if (Test-Path -LiteralPath $yml) {
        $y = [IO.File]::ReadAllText($yml, [Text.Encoding]::UTF8)
        Check 'workflow: regex tag совпадает с winlib' ($y.Contains('^v[0-9]+\.[0-9]+\.[0-9]+$'))
        Check 'workflow: regex run_id' ($y.Contains('^[0-9]+$'))
        Check 'workflow: regex раннера совпадает с winlib' ($y.Contains('^windows-(11-arm|2025)$'))
        Check 'workflow: права contents: read' ($y -match '(?m)^permissions:\s*\r?\n\s+contents: read')
        Check 'workflow: ручной запуск, без push/pull_request' ($y -match '(?m)^on:\s*\r?\n\s+workflow_dispatch:' -and $y -notmatch '(?m)^\s+(push|pull_request|pull_request_target):')
        # подстановка inputs/needs допустима только в строке вида «ключ: ${{ ... }}» (env, if, with, outputs), не в теле run
        $bad = @($y -split '\r?\n' | Where-Object { $_ -match '\$\{\{\s*(inputs|needs)\.' -and $_ -notmatch '^\s+[A-Za-z_-]+:\s+(\$\{\{.*\}\}|".*\$\{\{.*)$' -and $_ -notmatch '^\s*#' })
        Check 'workflow: входы не подставляются в run напрямую' ($bad.Count -eq 0) ($bad -join ' // ')
        $unpinned = @($y -split '\r?\n' | Where-Object { $_ -match '^\s*-?\s*uses:' -and $_ -notmatch '@[0-9a-f]{40}\b' })
        Check 'workflow: действия закреплены по SHA' ($unpinned.Count -eq 0) ($unpinned -join ' // ')
    } else { Write-Host 'пропуск  win-shots.yml ещё нет' }

    Section 'Файлы scripts/win'
    $files = @(Get-ChildItem -LiteralPath $here -File | Where-Object { $_.Extension -in '.ps1', '.psm1' })
    foreach ($f in $files) {
        $tokens = $null; $perr = $null
        [void][System.Management.Automation.Language.Parser]::ParseFile($f.FullName, [ref]$tokens, [ref]$perr)
        Check "разбор без ошибок: $($f.Name)" ($perr.Count -eq 0) (($perr | ForEach-Object { $_.Message }) -join '; ')
        $b = [IO.File]::ReadAllBytes($f.FullName)
        Check "BOM (кириллица в 5.1): $($f.Name)" ($b.Length -gt 3 -and $b[0] -eq 0xEF -and $b[1] -eq 0xBB -and $b[2] -eq 0xBF)
        $text = [Text.Encoding]::UTF8.GetString($b)
        Check "строк <= 600 и размер <= 30 КБ: $($f.Name)" (($text -split "`n").Count -le 600 -and $b.Length -le 30720)
        if ($f.Name -ne 'test_winlib.ps1') {
            Check "нет Invoke-Expression и загрузки кода из сети: $($f.Name)" ($text -notmatch 'Invoke-Expression|\biex\b|DownloadString|Invoke-WebRequest|Invoke-RestMethod')
        }
    }
    # регрессии прогона 38538798248: пустой method.txt на ARM и ожидание ответа SmartScreen в Start-Process
    $shotsText = [IO.File]::ReadAllText((Join-Path $here 'shots.ps1'), [Text.Encoding]::UTF8)
    Check 'shots.ps1: нет Get-Content -Raw с .Trim() ($null на пустом файле)' ($shotsText -notmatch 'Get-Content[^\r\n]*-Raw[^\r\n]*\)\.Trim\(\)')
    Check 'shots.ps1: способ записи читает Get-RecMethods' ($shotsText.Contains('Get-RecMethods'))
    Check 'shots.ps1: exe запускается через Start-ExeNoWait, а не Start-Process -FilePath $Exe' ($shotsText.Contains('Start-ExeNoWait $Exe') -and -not $shotsText.Contains('Start-Process -FilePath $Exe'))
    Check 'в папке <= 20 файлов кода' ((Get-ChildItem -LiteralPath $here -File).Count -le 20)

    Section 'Структура DEVMODE и OCR (на этой машине)'
    Import-Module (Join-Path $here 'winui.psm1') -Force -DisableNameChecking
    $mar = [Runtime.InteropServices.Marshal]
    Same 'DEVMODE занимает 220 байт (как DEVMODEW)' ($mar::SizeOf([type]'SaWin.DEVMODE')) 220
    foreach ($o in @(@('dmSize', 68), @('dmFields', 72), @('dmPositionX', 76), @('dmColor', 92), @('dmFormName', 102),
                     @('dmLogPixels', 166), @('dmBitsPerPel', 168), @('dmPelsWidth', 172), @('dmPelsHeight', 176),
                     @('dmDisplayFrequency', 184), @('dmPanningHeight', 216))) {
        Same "смещение $($o[0])" ([int]$mar::OffsetOf([type]'SaWin.DEVMODE', $o[0])) $o[1]
    }
    $mode = Get-CurrentMode
    $liveSize = [int]$mode.dmSize -gt 0 -and [int]$mode.dmSize -le 220   # Windows пишет свой размер (188)
    Check 'dmSize режима в пределах 1..220' ($liveSize -or $env:CI -eq 'true') "dmSize=$($mode.dmSize)"
    $live = $mode.dmPelsWidth -gt 0 -and $mode.dmPelsHeight -gt 0 -and $mode.dmDisplayFrequency -gt 0
    Check 'EnumDisplaySettings читает текущий режим' ($live -or $env:CI -eq 'true') "$($mode.dmPelsWidth)x$($mode.dmPelsHeight) @ $($mode.dmDisplayFrequency)"
    $langs = @(Get-OcrLanguages)
    if ($langs.Count -gt 0) {
        $ocrPng = Join-Path $tmp 'ocr.png'
        New-Png $ocrPng 600 200 { param($g)
            $g.Clear([System.Drawing.Color]::White)
            $f = New-Object System.Drawing.Font('Segoe UI', 28)
            $g.DrawString('Windows protected your computer', $f, [System.Drawing.Brushes]::Black, 20, 30)
            $g.DrawString('More info', $f, [System.Drawing.Brushes]::Blue, 20, 110)
        }
        $w = Get-OcrWords $ocrPng
        $r = Find-PhraseRect $w 'More info'
        Check 'OCR находит «More info» на синтетическом снимке' ($null -ne $r -and $r[0] -lt 60 -and $r[1] -gt 90 -and $r[1] -lt 150) (($w | ForEach-Object { $_.Text }) -join ' ')
    } else { Write-Host 'пропуск  OCR: нет языков распознавания' }
} finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ("`nпрошло {0}, провалов {1}" -f $script:Pass, $script:Fail)
if ($script:Fail -gt 0) { exit 1 }
exit 0
