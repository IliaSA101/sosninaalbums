# shots.ps1 - сценарий первого запуска на Windows 11 (спека 036, design §5.6): страница релиза в Edge ->
# предупреждение Edge -> «Сохранить» -> окно SmartScreen -> «Подробнее» -> «Выполнить в любом случае» -> программа;
# глава 2 - «Свойства» -> «Разблокировать». Пишет снимки png/win-NN-*.png, shots.json, журнал video/steps.jsonl
# (контракт design §2, его монтирует scripts/guide/overlay.py) и raw.mp4. Всегда выходит с кодом 0.
# Файл в UTF-8 с BOM (Windows PowerShell 5.1).
param(
    [Parameter(Mandatory)][string]$OutDir,
    [string]$Tag = '',
    [string]$Repo = '',
    [string]$ExePath = '',
    [switch]$Edge,
    [switch]$Video
)
$ErrorActionPreference = 'Continue'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here 'winlib.psm1') -Force -DisableNameChecking
Import-Module (Join-Path $here 'winui.psm1') -Force -DisableNameChecking
. (Join-Path $here 'record.ps1')

foreach ($d in 'png', 'video', 'logs') { New-Item -ItemType Directory -Force -Path (Join-Path $OutDir $d) | Out-Null }
Set-LogPath (Join-Path $OutDir 'logs\shots.txt')
Set-SummaryPath (Join-Path $OutDir 'summary.md')
Initialize-Dpi
Get-ChildItem -Path (Join-Path $OutDir 'video') -Filter 'probe-*.mp4' -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue

if ($Tag -and -not (Test-TagInput $Tag)) { Write-Log "tag '$Tag' не vX.Y.Z - игнорирую"; $Tag = ''; $Edge = $false }
if (-not $Tag) { $Edge = $false }   # без метки релиза нет страницы, с которой скачивать
$downloads = Join-Path $env:USERPROFILE 'Downloads'
New-Item -ItemType Directory -Force -Path $downloads | Out-Null
$ocrTmp = Join-Path $OutDir 'logs\ocr.png'
$desktopPng = Join-Path $OutDir 'png\win-00-desktop.png'
$build = Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion' -ErrorAction SilentlyContinue
$winVer = '{0}.{1}' -f $build.CurrentBuild, $build.UBR
$script:LastEnd = 0.0
$script:Recorder = 'none'
$assetRe = 'windows-x64\.exe'   # регулярка для имён UIA
$assetGlob = '*windows-x64.exe'   # маска для файлов и gh

# ---------- снимки и действия ----------

function Save-Shot([string]$Name, $Rect, [string]$Label) {
    $file = "png/$Name.png"
    $full = Join-Path $OutDir $file
    try {
        Save-Screen $full
        $changed = $null
        $ok = $true
        $why = ''
        if ($Name -ne 'win-00-desktop') {
            $changed = [math]::Round((Get-DiffRatio $desktopPng $full), 3)
            if ($changed -lt 0.02) { $ok = $false; $why = 'экран почти не отличается от пустого стола' }
        }
        Add-ShotRecord $Name $ok $why $file $Rect $Label $changed @{ windows = $winVer; lang = (Get-UICulture).TwoLetterISOLanguageName; screen = @(Get-ScreenSize) }
        $res = 'нет'
        if ($ok) { $res = 'да' }
        Add-SummaryRow "снимок $Name" $res ("changed=$changed" + $(if ($why) { "; $why" } else { '' }))
    } catch {
        Add-ShotRecord $Name $false $_.Exception.Message $file $null $Label $null @{}
        Add-SummaryRow "снимок $Name" 'нет' $_.Exception.Message
    }
}

# Снимка нет, потому что нужного окна не было
function Add-MissingShot([string]$Name, [string]$Why) {
    Add-ShotRecord $Name $false $Why "png/$Name.png" $null '' $null @{ windows = $winVer }
    Add-SummaryRow "снимок $Name" 'нет' $Why
    Add-JNote "нет $Name`: $Why"
}

function Reset-IdleStart { $script:LastEnd = Get-JTime }

function Write-IdleBefore {
    $now = Get-JTime
    if ($now - $script:LastEnd -ge 0.3) { Add-JSegment 'idle' $script:LastEnd $now }
}

# Действие по design §4.1: ожидание -> стоп-кадр с рамкой -> плавный ход курсора -> щелчок -> пауза, чтобы было видно итог.
# Hit - результат поиска (или $null: подпись без стрелки и без щелчка). Снимок берётся до стоп-кадра, сцена не меняется.
function Invoke-Act($Hit, [string]$Caption, [string]$Mode = 'click', [string]$Snap = '', [double]$Pause = 1.2, [string]$Label = '') {
    $sz = Get-ScreenSize
    $rect = $null
    if ($Hit) { $rect = Limit-Rect $Hit.Rect $sz[0] $sz[1] }
    if ($Snap) { Save-Shot $Snap $rect $Label }
    Write-IdleBefore
    Add-JHold $rect $Caption
    if ($rect) {
        $cx = [int]($rect[0] + $rect[2] / 2); $cy = [int]($rect[1] + $rect[3] / 2)
        $t0 = Get-JTime
        Move-Cursor $cx $cy 0.6
        Add-JSegment 'move' $t0 (Get-JTime)
        Invoke-MouseClick $Mode
    } else {
        Add-JNote "без стрелки: $Caption"
    }
    Start-Sleep -Milliseconds ([int]($Pause * 1000))
    Reset-IdleStart
}

# Колесо мыши до появления цели; прокрутка идёт в реальном темпе (отрезок move), чтобы глаз видел движение страницы
function Invoke-ScrollTo([scriptblock]$Find, [int]$MaxTries = 14) {
    $hit = & $Find
    if ($hit) { return $hit }
    $sz = Get-ScreenSize
    Write-IdleBefore
    $t0 = Get-JTime
    Move-Cursor ([int]($sz[0] / 2)) ([int]($sz[1] / 2)) 0.4
    for ($i = 0; $i -lt $MaxTries -and -not $hit; $i++) {
        Invoke-Wheel (-3)
        Start-Sleep -Milliseconds 350
        $hit = & $Find
    }
    Add-JSegment 'move' $t0 (Get-JTime)
    Reset-IdleStart
    return $hit
}

# Результат функции с побочным выводом: булевым считается последнее значение
function Get-LastBool($Out) { $a = @($Out); return ($a.Count -gt 0 -and $a[-1] -eq $true) }

# ---------- Edge, загрузка, запуск ----------

function Set-EdgePolicies {
    $key = 'HKLM:\SOFTWARE\Policies\Microsoft\Edge'
    New-Item -Path $key -Force | Out-Null
    $dw = @{ HideFirstRunExperience = 1; PromptForDownloadLocation = 0; SmartScreenEnabled = 1; DefaultBrowserSettingEnabled = 0
        BrowserSignin = 0; SyncDisabled = 1; HubsSidebarEnabled = 0; StartupBoostEnabled = 0; BackgroundModeEnabled = 0
        ShowRecommendationsEnabled = 0; EdgeShoppingAssistantEnabled = 0
    }
    foreach ($k in $dw.Keys) { New-ItemProperty -Path $key -Name $k -Value $dw[$k] -PropertyType DWord -Force | Out-Null }
    New-ItemProperty -Path $key -Name ApplicationLocaleValue -Value 'ru' -PropertyType String -Force | Out-Null
    New-ItemProperty -Path $key -Name DownloadDirectory -Value $downloads -PropertyType String -Force | Out-Null
    # расширения имён в Проводнике: файл в «Свойствах» и в списке называется как на диске
    Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced' -Name HideFileExt -Value 0 -ErrorAction SilentlyContinue
}

function Get-EdgeExe {
    foreach ($p in "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe", "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe") {
        if (Test-Path -LiteralPath $p) { return $p }
    }
    return $null
}

function Start-EdgePage([string]$Url) {
    $exe = Get-EdgeExe
    if (-not $exe) { throw 'msedge.exe не найден' }
    Start-Process -FilePath $exe -ArgumentList @('--no-first-run', '--no-default-browser-check', '--force-renderer-accessibility', '--start-maximized', $Url)
}

# Метка «скачано из интернета» (ZoneId=3): то же, что ставит браузер, - по ней SmartScreen и ругается
function Set-Zone([string]$Path, [string]$Url) {
    Set-Content -LiteralPath $Path -Stream Zone.Identifier -Value "[ZoneTransfer]`r`nZoneId=3`r`nHostUrl=$Url`r`nReferrerUrl=$Url"
}

function Find-Downloaded {
    $f = Get-ChildItem -LiteralPath $downloads -Filter $assetGlob -File -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($f) { return $f.FullName }
    return $null
}

# Запасной путь: файл из артефакта/релиза и метка вручную
function Get-ExeFallback {
    $exe = Find-Downloaded
    if ($exe) { return $exe }
    $src = $ExePath
    if (-not $src -or -not (Test-Path -LiteralPath $src)) {
        if ($Tag -and $Repo) {
            $tmp = Join-Path $env:TEMP 'sa-exe'
            New-Item -ItemType Directory -Force -Path $tmp | Out-Null
            & gh release download $Tag -R $Repo -p $assetGlob -D $tmp --clobber 2>&1 | ForEach-Object { Write-Log "gh: $_" }
            $got = Get-ChildItem -LiteralPath $tmp -Filter $assetGlob -File | Select-Object -First 1
            if ($got) { $src = $got.FullName }
        }
    }
    if (-not $src -or -not (Test-Path -LiteralPath $src)) { return $null }
    $dst = Join-Path $downloads (Split-Path -Leaf $src)
    Copy-Item -LiteralPath $src -Destination $dst -Force
    $url = 'https://github.com/'
    if ($Repo -and $Tag) { $url = "https://github.com/$Repo/releases/tag/$Tag" }
    Set-Zone $dst $url
    return $dst
}

$namesMore = @('Другие действия', 'Дополнительные действия', 'Ещё', 'Больше действий', 'More actions', 'More options')
$namesKeep = @('Сохранить', 'Keep')
$namesShow = @('Показать больше', 'Show more')
$namesAnyway = @('Всё равно сохранить', 'Все равно сохранить', 'Keep anyway')
$namesOpen = @('Открыть файл', 'Open file')
$namesInfo = @('Подробнее', 'More info')
$namesRun = @('Выполнить в любом случае', 'Run anyway')

# Шаг 1: страница релиза, щелчок по exe
function Invoke-EdgeDownload {
    Add-JStep 1 4 'Скачайте программу'
    Reset-IdleStart
    Start-EdgePage "https://github.com/$Repo/releases/tag/$Tag"
    $find = { Find-UiaOnce -Procs @('msedge') -NameRegex $assetRe -Type 'Hyperlink' }
    $hit = Wait-Target -Names @() -Procs @('msedge') -Timeout 60 -NameRegex $assetRe -Type 'Hyperlink'
    if (-not $hit) {
        Write-Log 'ссылка на exe не найдена на странице релиза - пробую список файлов (expanded_assets)'
        Start-EdgePage "https://github.com/$Repo/releases/expanded_assets/$Tag"
        $hit = Wait-Target -Names @() -Procs @('msedge') -Timeout 40 -NameRegex $assetRe -Type 'Hyperlink'
    }
    if ($hit) { $hit = Invoke-ScrollTo $find }
    if (-not $hit) { Add-SummaryRow 'Edge: страница релиза' 'нет' 'ссылка на exe не найдена (UIA)'; return $false }
    $cap = 'Нажмите на файл ' + ($hit.Name -replace '\s.*$', '')
    if ($cap.Length -gt 80) { $cap = $cap.Substring(0, 80) }
    Invoke-Act $hit $cap 'click' 'win-01-release' 0.5 $hit.Name
    Add-SummaryRow 'Edge: страница релиза' 'да' $hit.Name
    return $true
}

# Шаг 2: предупреждение Edge. Порядок кнопок на разных версиях Edge разный, поэтому цикл по приоритету:
# «Открыть файл» > «Всё равно сохранить» > «Показать больше» > «Сохранить» > «…»
function Invoke-EdgeKeep {
    Add-JStep 2 4 'Сохраните файл'
    Reset-IdleStart
    $all = $namesMore + $namesKeep + $namesShow + $namesAnyway + $namesOpen
    $sw = [Diagnostics.Stopwatch]::StartNew()
    $clicked = @{}
    $viaMenu = $false
    $snappedWarn = $false
    $opened = $false
    while ($sw.Elapsed.TotalSeconds -lt 75 -and -not $opened) {
        $hits = Find-UiaOnce -Names $all -Procs @('msedge') -All
        if (-not $hits -or $hits.Count -eq 0) {
            # файл уже скачан: предупреждения может не быть (у файла есть репутация) - ждать нечего
            if ((Find-Downloaded) -and $sw.Elapsed.TotalSeconds -gt 10) { break }
            Start-Sleep -Milliseconds 700
            continue
        }
        if (-not $snappedWarn) {
            $snappedWarn = $true
            Save-Shot 'win-02-edge-warning' $null ''
        }
        $pick = $null
        $kind = ''
        foreach ($k in 'open', 'anyway', 'show', 'keep', 'more') {
            $set = switch ($k) { 'open' { $namesOpen } 'anyway' { $namesAnyway } 'show' { $namesShow } 'keep' { $namesKeep } 'more' { $namesMore } }
            $h = $hits | Where-Object { $set -contains $_.Name } | Select-Object -First 1
            if ($h -and -not $clicked.ContainsKey($k)) { $pick = $h; $kind = $k; break }
        }
        if (-not $pick) { Start-Sleep -Milliseconds 700; continue }
        $clicked[$kind] = $true
        switch ($kind) {
            'more' { $viaMenu = $true; Invoke-Act $pick (Get-Caption '…' $pick.Name) 'click' '' 1.0 '…' }
            'keep' {
                $verb = 'Нажмите'
                if ($viaMenu) { $verb = 'Выберите' }
                Invoke-Act $pick (Get-Caption 'Сохранить' $pick.Name $verb) 'click' '' 1.2 'Сохранить'
            }
            'show' { Invoke-Act $pick (Get-Caption 'Показать больше' $pick.Name) 'click' '' 1.0 'Показать больше' }
            'anyway' { Invoke-Act $pick (Get-Caption 'Всё равно сохранить' $pick.Name) 'click' 'win-03-edge-keep' 1.5 'Всё равно сохранить' }
            'open' { $script:OpenHit = $pick; $opened = $true }
        }
    }
    # предупреждение Edge необязательно: нет окна - «нет предупреждения браузера», сценарий идёт дальше без провала
    $noWarn = 'нет предупреждения браузера'
    if (-not $snappedWarn) { Add-MissingShot 'win-02-edge-warning' $noWarn }
    if (-not $clicked.ContainsKey('anyway')) { Add-MissingShot 'win-03-edge-keep' $noWarn }
    $res = 'пропуск'
    if ($snappedWarn) { $res = 'да' } elseif (-not (Find-Downloaded)) { $res = 'нет' }
    Add-SummaryRow 'Edge: предупреждение' $res ("нажато: " + (($clicked.Keys | Sort-Object) -join ', ') + $(if (-not $snappedWarn) { "; $noWarn" } else { '' }))
    return $opened
}

# Запуск exe без ожидания. Для файла с меткой «из интернета» ShellExecute показывает окно SmartScreen и может вернуть
# «The operation was canceled by the user» - это признак окна, а не сбой. Через explorer.exe вызов не ждёт ответа
# и не бросает; прямой Start-Process - запасной. Результат: explorer | direct | canceled (окно, по версии системы) | failed
function Start-ExeNoWait([string]$Path) {
    try {
        Start-Process -FilePath explorer.exe -ArgumentList @("`"$Path`"") -ErrorAction Stop
        return 'explorer'
    } catch { Write-Log "explorer.exe не запустил exe: $($_.Exception.Message)" }
    try {
        Start-Process -FilePath $Path -ErrorAction Stop
        return 'direct'
    } catch {
        if (Test-LaunchCanceled $_) { Write-Log 'запуск отменён системой - считаю, что окно SmartScreen показано'; return 'canceled' }
        Write-Log "запуск exe: $($_.Exception.Message)"
        return 'failed'
    }
}

# Шаг 3: открыть файл (или запустить его из Загрузок) и пройти окно SmartScreen
function Invoke-RunExe([string]$Exe, [bool]$ViaEdgeOpen, [int]$StepN, [int]$Of) {
    Add-JStep $StepN $Of 'Запустите программу'
    Reset-IdleStart
    $launch = ''
    if ($ViaEdgeOpen -and $script:OpenHit) {
        Invoke-Act $script:OpenHit (Get-Caption 'Открыть файл' $script:OpenHit.Name) 'click' '' 0.5 'Открыть файл'
    } else {
        Write-Log "запуск $Exe"
        $launch = Start-ExeNoWait $Exe
        Write-Log "запуск: $launch"
    }
    $info = Wait-Target -Names $namesInfo -Timeout 45 -Ocr @('More info', 'Подробнее') -Tmp $ocrTmp -Skip @('msedge')
    if (-not $info) {
        $started = (Get-AppProcess).Count -gt 0
        $ssProc = [bool](Get-Process -Name smartscreen -ErrorAction SilentlyContinue)
        $why = 'SmartScreen не появился'
        if ($launch -eq 'canceled') { $why = 'запуск вернул «отменено пользователем», но окно SmartScreen не найдено (UIA и OCR)' }
        if ($started) { $why += '; программа запустилась сразу (у файла есть репутация или SmartScreen выключен)' } else { $why += '; программа тоже не запустилась' }
        if (-not $ssProc) { $why += '; smartscreen.exe не запущен' }
        try { Save-Screen (Join-Path $OutDir 'logs\smartscreen-miss.png') } catch { }
        Add-MissingShot 'win-04-smartscreen' $why
        Add-MissingShot 'win-05-smartscreen-more' $why
        Add-SummaryRow 'SmartScreen' 'нет' $why
        return
    }
    Add-SummaryRow 'SmartScreen' 'да' "окно найдено через $($info.Source): $($info.Name)"
    Invoke-Act $info (Get-Caption 'Подробнее' $info.Name) 'click' 'win-04-smartscreen' 1.0 'Подробнее'
    $run = Wait-Target -Names $namesRun -Timeout 10 -Ocr @('Run anyway', 'Выполнить в любом случае') -Tmp $ocrTmp -Skip @('msedge')
    if (-not $run) {
        Add-MissingShot 'win-05-smartscreen-more' 'кнопка «Выполнить в любом случае» не появилась (Smart App Control или политика?)'
        Add-SummaryRow 'SmartScreen: кнопка запуска' 'нет' 'после «Подробнее» кнопки нет'
        return
    }
    Invoke-Act $run (Get-Caption 'Выполнить в любом случае' $run.Name) 'click' 'win-05-smartscreen-more' 1.5 'Выполнить в любом случае'
    Add-SummaryRow 'SmartScreen: кнопка запуска' 'да' $run.Name
}

function Get-AppProcess { return @(Get-Process | Where-Object { $_.ProcessName -like 'SosninaAlbums*' -or $_.ProcessName -eq 'sosninaalbums' }) }

# Шаг 4: окно программы
function Invoke-AppShot([int]$StepN, [int]$Of) {
    Add-JStep $StepN $Of 'Готово'
    Reset-IdleStart
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ((Get-AppProcess).Count -eq 0 -and $sw.Elapsed.TotalSeconds -lt 45) { Start-Sleep -Milliseconds 700 }
    if ((Get-AppProcess).Count -eq 0) {
        Add-MissingShot 'win-06-app' 'программа не запустилась'
        Add-SummaryRow 'Окно программы' 'нет' 'процесс не найден за 45 с'
        return
    }
    Start-Sleep -Seconds 5   # WebView2 рисует окно не сразу
    Invoke-Act $null 'Программа открыта. Дальше она открывается сразу' 'click' 'win-06-app' 2.0
    Add-SummaryRow 'Окно программы' 'да' (((Get-AppProcess) | ForEach-Object { $_.ProcessName }) -join ', ')
    Get-AppProcess | Stop-Process -Force -ErrorAction SilentlyContinue
}

# Глава 2: «Свойства» -> «Разблокировать»
function Invoke-Unblock([string]$Exe) {
    Add-JChapter 'Если кнопки «Выполнить в любом случае» нет'
    Reset-IdleStart
    Add-JStep 1 1 'Разблокируйте файл'
    Reset-IdleStart
    $dir = Join-Path $downloads 'unblock'
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $copy = Join-Path $dir (Split-Path -Leaf $Exe)
    Copy-Item -LiteralPath $Exe -Destination $copy -Force
    Set-Zone $copy 'https://github.com/'
    Start-Process -FilePath explorer.exe -ArgumentList @("/select,`"$copy`"")
    $base = [regex]::Escape([IO.Path]::GetFileNameWithoutExtension($copy))
    $item = Wait-Target -Names @() -Procs @('explorer') -Timeout 25 -NameRegex "^$base" -Type 'ListItem'
    if (-not $item) { $item = Wait-Target -Names @() -Procs @('explorer') -Timeout 8 -NameRegex "^$base" -Type 'DataItem' }
    if (-not $item) {
        Add-MissingShot 'win-07-unblock' 'файл не найден в окне Проводника (UIA)'
        Add-SummaryRow 'Разблокировать' 'нет' 'элемент файла в Проводнике не найден'
        return
    }
    Invoke-Act $item 'Щёлкните правой кнопкой по файлу' 'right' '' 1.0 ''
    $props = Wait-Target -Names @('Свойства', 'Properties') -Procs @('explorer') -Timeout 6
    if ($props) {
        Invoke-Act $props (Get-Caption 'Свойства' $props.Name 'Выберите') 'click' '' 1.5 'Свойства'
    } else {
        Write-Log 'пункта «Свойства» в меню не нашли - Alt+Enter'
        Add-JNote 'пункт «Свойства» не найден, Alt+Enter'
        Send-Keys @('Alt', 'Enter')
        Start-Sleep -Seconds 2
    }
    $box = Wait-Target -Names @('Разблокировать', 'Unblock') -Procs @() -Timeout 12 -Skip @('msedge')
    if (-not $box) {
        Add-MissingShot 'win-07-unblock' 'флажок «Разблокировать» не найден в окне «Свойства»'
        Add-SummaryRow 'Разблокировать' 'нет' 'в окне «Свойства» нет флажка (метки нет или окно не открылось)'
        Send-Keys @('Esc')
        return
    }
    Invoke-Act $box (Get-Caption 'Разблокировать' $box.Name 'Поставьте флажок') 'click' 'win-07-unblock' 1.0 'Разблокировать'
    $ok = Wait-Target -Names @('OK', 'ОК') -Timeout 5 -Skip @('msedge')
    if ($ok) { Invoke-Act $ok (Get-Caption 'ОК' $ok.Name) 'click' '' 1.2 'ОК' } else { Send-Keys @('Enter') }
    $left = $true
    try { [void](Get-Content -LiteralPath $copy -Stream Zone.Identifier -ErrorAction Stop) } catch { $left = $false }
    $res = 'нет'
    if (-not $left) { $res = 'да' }
    Add-SummaryRow 'Разблокировать' $res $(if ($left) { 'метка Zone.Identifier осталась' } else { 'метка снята' })
}

# ---------- основной ход ----------

$rec = $null
$exe = $null
$journalOpen = $false
try {
    Set-EdgePolicies
    $set = Set-ScreenResolution
    Set-SolidDesktop
    (New-Object -ComObject Shell.Application).MinimizeAll()
    Start-Sleep -Seconds 1
    $sz = Get-ScreenSize
    Write-Log ("экран {0}x{1}, смена разрешения: {2}" -f $sz[0], $sz[1], $set)
    Add-SummarySection 'Сценарий' @('шаг', 'итог', 'почему')
    Save-Shot 'win-00-desktop' $null ''

    # способ записи: выбрал probe.ps1 (video/method.txt), иначе по порядку
    # method.txt пуст - проба не нашла рабочего способа (так на ARM): запись пропускаем, сценарий идёт дальше
    $methods = @(Get-RecMethods (Join-Path $OutDir 'video\method.txt') @('gdigrab', 'ddagrab', 'frames'))
    $rawMp4 = Join-Path $OutDir 'video\raw.mp4'
    if ($Video) {
        foreach ($m in $methods) {
            try { $rec = Start-Rec $rawMp4 $m 15; $script:Recorder = $m; break } catch { Write-Log "запись $m не стартовала: $($_.Exception.Message)" }
        }
        if (-not $rec) {
            $why = 'ни один способ не стартовал'
            if ($methods.Count -eq 0) { $why = 'проба не нашла рабочего способа записи (method.txt пуст)' }
            Add-SummaryRow 'Запись экрана' 'нет' $why
        }
    }
    $lang = (Get-UICulture).TwoLetterISOLanguageName
    Start-Journal (Join-Path $OutDir 'video\steps.jsonl') ([ordered]@{
            os = 'windows'; ver = $winVer; lang = $lang; space = 'pixels'; screen = @($sz[0], $sz[1]); scale = 1
            recorder = $script:Recorder; script = 'shots.ps1 1' })
    $journalOpen = $true

    Start-Sleep -Seconds 2
    Show-Flash '#d81f26' 0.8 { Add-JSync '#d81f26' 0.8 }
    Start-Sleep -Milliseconds 600
    [void][SaWin.Native]::SetCursorPos([int]($sz[0] * 0.7), [int]($sz[1] * 0.8))
    Add-JChapter 'Первый запуск на Windows 11'
    Reset-IdleStart

    $viaEdge = $false
    $stepsTotal = 2
    if ($Edge) {
        $stepsTotal = 4
        $got = $false
        try { $got = Get-LastBool (Invoke-EdgeDownload) } catch { Write-Log "Edge, шаг 1: $($_.Exception.Message)" }
        if ($got) { try { $viaEdge = Get-LastBool (Invoke-EdgeKeep) } catch { Write-Log "Edge, шаг 2: $($_.Exception.Message)" } }
        Start-Sleep -Seconds 1
        $exe = Find-Downloaded
        if (-not $exe) { Add-SummaryRow 'Edge: файл в Загрузках' 'нет' 'exe не скачался - иду запасным путём (Zone.Identifier)' }
        else { Add-SummaryRow 'Edge: файл в Загрузках' 'да' $exe }
    }
    if (-not $exe) {
        $exe = Get-ExeFallback
        if ($exe) { Add-JNote 'запасной путь: файл скопирован, метка Zone.Identifier поставлена вручную' }
        if ($Edge) { $stepsTotal = 4 }
    }
    if (-not $exe) {
        Add-SummaryRow 'Файл программы' 'нет' 'exe не найден ни в Edge, ни по tag/run_id'
    } else {
        $stepN = $stepsTotal - 1
        Invoke-RunExe $exe $viaEdge $stepN $stepsTotal
        Invoke-AppShot $stepsTotal $stepsTotal
        Invoke-Unblock $exe
    }
} catch {
    Write-Log "сбой сценария: $($_.Exception.Message)`n$($_.ScriptStackTrace)"
    Add-SummaryRow 'Сценарий' 'нет' $_.Exception.Message
} finally {
    Get-Process -Name msedge -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Get-AppProcess | Stop-Process -Force -ErrorAction SilentlyContinue
    if ($rec) {
        $okRec = Stop-Rec $rec
        $mb = 0
        if (Test-Path -LiteralPath $rawMp4) { $mb = [math]::Round((Get-Item -LiteralPath $rawMp4).Length / 1MB, 1) }
        Add-SummaryRow 'Запись экрана' $(if ($okRec) { 'да' } else { 'нет' }) "способ $($script:Recorder), raw.mp4 $mb МБ"
    }
    if ($journalOpen) {
        $jp = Close-Journal
        $lines = [IO.File]::ReadAllLines($jp, [Text.Encoding]::UTF8)
        $errs = @(Test-JournalLines $lines)
        Add-SummaryRow 'Журнал steps.jsonl' $(if ($errs.Count -eq 0) { 'да' } else { 'нет' }) $(if ($errs.Count) { $errs -join '; ' } else { "$($lines.Count) строк" })
        $holds = @($lines | Where-Object { $_ -like '{"ev":"hold"*' })
        $arrows = @($holds | Where-Object { $_ -notlike '*"rect":null*' })
        $steps = @($lines | Where-Object { $_ -like '{"ev":"step"*' })
        Add-SummaryLine ''
        Add-SummaryLine ("ролик: шагов {0}, со стрелкой {1} из {2} стоп-кадров" -f $steps.Count, $arrows.Count, $holds.Count)
    }
    Save-ShotsJson (Join-Path $OutDir 'shots.json')
}
exit 0
