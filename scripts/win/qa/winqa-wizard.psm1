# winqa-wizard.psm1 - мастер установки и удаления NSIS через UIA Win32 (спека 118, design §3: блок install).
# Окна NSIS - обычные окна Win32 без WebView2, поэтому не зависят от пробы П0. Имена кнопок - русские (installer.nsi, MUI Russian);
# запасные английские на случай, если язык мастера не подхватился. Windows PowerShell 5.1, файл в UTF-8 с BOM.

Import-Module (Join-Path $PSScriptRoot 'winqa.psm1') -DisableNameChecking

$script:WizBtn = @{
    Next    = '^(Установить|Далее.*|Install|Next.*)$'
    Finish  = '^(Готово|Finish)$'
    Retry   = '^(Повтор|Retry)$'
    Remove  = '^(Удалить|Uninstall)$'
    Close   = '^(Закрыть|Close|Готово|Finish)$'
}

function Get-WizButtons { return $script:WizBtn }

# Класс SaQa.Wiz (Win32: окна, кнопки, передний план) определён в winqa.psm1 - им пользуется и блок update.

# Чистая часть: из текстов кнопок (NSIS пишет "&Далее >", "&Установить") выбрать подходящий регулярке. & убираем, края обрезаем.
# Возвращает пару Index (в исходном списке) и Text (очищенный) или $null
function Select-WizButtonText([string[]]$Texts, [Parameter(Mandatory)][string]$Regex) {
    for ($i = 0; $i -lt $Texts.Count; $i++) {
        $clean = (([string]$Texts[$i]) -replace '&', '').Trim()
        if ($clean -and $clean -match $Regex) { return [pscustomobject]@{ Index = $i; Text = $clean } }
    }
    return $null
}

# Вывести окна мастера на передний план: иначе окно поверх (консоль раннера) перехватит щелчок (В4, прогон 6)
function Set-WizForeground([string[]]$ProcNames) {
    $ids = @()
    foreach ($pn in $ProcNames) { $ids += @(Get-Process -Name $pn -ErrorAction SilentlyContinue | ForEach-Object { $_.Id }) }
    if (-not $ids.Count) { return $false }
    $ok = $false
    foreach ($top in [SaQa.Wiz]::TopWindows([int[]]$ids)) { if ([SaQa.Wiz]::Foreground($top)) { $ok = $true } }
    Start-Sleep -Milliseconds 300
    return $ok
}

# Кнопка мастера по Win32 в окнах процессов: объект Hwnd, Text или $null. Нажатие - Invoke-WizWin32Click
function Find-WizWin32Button {
    param([string[]]$ProcNames, [Parameter(Mandatory)][string]$Regex)
    $ids = @()
    foreach ($pn in $ProcNames) { $ids += @(Get-Process -Name $pn -ErrorAction SilentlyContinue | ForEach-Object { $_.Id }) }
    if (-not $ids.Count) { return $null }
    foreach ($top in [SaQa.Wiz]::TopWindows([int[]]$ids)) {
        $raw = @([SaQa.Wiz]::Buttons($top))
        $texts = @($raw | ForEach-Object { ($_ -split '\|', 2)[1] })
        $pick = Select-WizButtonText $texts $Regex
        if ($pick) { return [pscustomobject]@{ Hwnd = [long](($raw[$pick.Index] -split '\|', 2)[0]); Text = $pick.Text } }
    }
    return $null
}
function Invoke-WizWin32Click($Btn) { return [SaQa.Wiz]::Click([long]$Btn.Hwnd) }

# Кнопка мастера: UIA, а при -Win32 - ещё и Win32. Цикл до Timeout. Результат: Kind (uia|win32), Target, Text или $null
function Find-WizTarget {
    param([string[]]$ProcNames, [Parameter(Mandatory)][string]$Regex, [double]$Timeout = 10, [bool]$Win32 = $false)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        $el = Find-AppEl -ProcNames $ProcNames -NameRegex $Regex -Type 'Button' -Timeout 0
        if ($el) { return [pscustomobject]@{ Kind = 'uia'; Target = $el; Text = '' } }
        if ($Win32) {
            $wb = Find-WizWin32Button -ProcNames $ProcNames -Regex $Regex
            if ($wb) { return [pscustomobject]@{ Kind = 'win32'; Target = $wb; Text = $wb.Text } }
        }
        Start-Sleep -Milliseconds 300
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $null
}
# Нажать найденное; возвращает путь: invoke/toggle/mouse (UIA) или win32. Окно мастера - на передний план; UIA - через
# InvokePattern, мышь только если Invoke недоступен
function Invoke-WizTarget($Tgt, [string[]]$ProcNames = @()) {
    if ($ProcNames.Count) { [void](Set-WizForeground $ProcNames) }
    if ($Tgt.Kind -eq 'win32') { [void](Invoke-WizWin32Click $Tgt.Target); return 'win32' }
    return (Invoke-AppEl -El $Tgt.Target)
}

# Нажатие с проверкой: кнопка должна пропасть за 5 с (страница сменилась), иначе запасной Win32 BM_CLICK.
# Возвращает путь нажатия (например invoke, затем win32)
function Invoke-WizPress {
    param([string[]]$ProcNames, $Tgt, [Parameter(Mandatory)][string]$Regex, [bool]$Win32 = $false)
    $how = Invoke-WizTarget $Tgt $ProcNames
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt 5) {
        if (-not (Get-Process -Name $ProcNames -ErrorAction SilentlyContinue)) { return $how }
        $still = Find-AppEl -ProcNames $ProcNames -NameRegex $Regex -Type 'Button' -Timeout 0
        if (-not $still -and $Win32) { $still = Find-WizWin32Button -ProcNames $ProcNames -Regex $Regex }
        if (-not $still) { return $how }
        Start-Sleep -Milliseconds 300
    }
    [void](Set-WizForeground $ProcNames)
    $wb = Find-WizWin32Button -ProcNames $ProcNames -Regex $Regex
    if ($wb) { [void](Invoke-WizWin32Click $wb); return "$how+win32" }
    # кнопки нет и для Win32: страница могла смениться как раз за эти секунды - перепроверяем, а не врём в детали
    if (-not (Get-Process -Name $ProcNames -ErrorAction SilentlyContinue)) { return "$how (мастер закрылся)" }
    if (-not (Find-AppEl -ProcNames $ProcNames -NameRegex $Regex -Type 'Button' -Timeout 0)) { return "$how (страница сменилась с задержкой)" }
    return "$how (страница не сменилась)"
}

# Ждёт появления окна мастера (видимого, не пустого)
function Wait-WizWindow {
    param([string[]]$ProcNames, [double]$Timeout = 40)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        if (Get-QaWinRect -ProcNames $ProcNames) { return $true }
        Start-Sleep -Milliseconds 400
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $false
}

function Find-WizButton {
    param([string[]]$ProcNames, [Parameter(Mandatory)][string]$Regex, [double]$Timeout = 10)
    return (Find-AppEl -ProcNames $ProcNames -NameRegex $Regex -Type 'Button' -Timeout $Timeout)
}

# Флажок "Запустить программу" на странице завершения: снять (запуск программы - отдельный пункт, не побочный эффект)
function Clear-WizRunCheckbox([string[]]$ProcNames) {
    $cb = Find-AppEl -ProcNames $ProcNames -Type 'CheckBox' -Timeout 2
    if (-not $cb) { return $false }
    try {
        $p = $null
        if ($cb.TryGetCurrentPattern([System.Windows.Automation.TogglePattern]::Pattern, [ref]$p) -and $p.Current.ToggleState -eq 'On') { $p.Toggle() }
        return $true
    } catch { return $false }
}

function Format-WizText([string[]]$Texts) {
    $t = (($Texts | Where-Object { $_ }) -join ' / ') -replace '\s+', ' '
    if ($t.Length -gt 400) { $t = $t.Substring(0, 400) }
    return $t
}

function Wait-ProcGone {
    param([string[]]$ProcNames, [double]$Timeout = 30)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    do {
        if (-not (Get-Process -Name $ProcNames -ErrorAction SilentlyContinue)) { return $true }
        Start-Sleep -Milliseconds 400
    } while ($sw.Elapsed.TotalSeconds -lt $Timeout)
    return $false
}

# Проход мастера по страницам: первая кнопка (First) -> ожидание последней (Last) -> нажать её.
# Каждая страница - снимок <Id>-p<N>-<слог>; -OnRetry получает окно "Повтор" один раз, пока мастер ждёт (В10).
# Результат: Ok, Why, Pages (Slug, Text, Shot), Retried, RetryText
function Invoke-QaWizard {
    param(
        [Parameter(Mandatory)][string[]]$ProcNames, [Parameter(Mandatory)][string]$Id,
        [string]$First = $script:WizBtn.Next, [string]$Last = $script:WizBtn.Finish,
        [scriptblock]$OnRetry = $null, [double]$Timeout = 150
    )
    $res = [pscustomobject]@{ Ok = $false; Why = ''; Pages = @(); Retried = $false; RetryText = ''; Via = @() }
    $add = {
        param($slug, $n)
        $shot = Save-QaShot -Id $Id -Slug ("p$n-$slug") -ProcNames $ProcNames
        $res.Pages += [pscustomobject]@{ Slug = $slug; Text = (Format-WizText (Get-QaTexts -ProcNames $ProcNames)); Shot = $shot }
    }
    if (-not (Wait-WizWindow -ProcNames $ProcNames)) { $res.Why = 'окно мастера не появилось'; return $res }
    # Половину срока ищем кнопку через UIA, затем подключаем Win32 (UIA окна, запущенного самой программой, бывает слепым)
    $half = 7.5
    $win32 = $false
    $tgtFirst = Find-WizTarget -ProcNames $ProcNames -Regex $First -Timeout $half
    if (-not $tgtFirst) {
        Write-QaUiaDiag -ProcNames $ProcNames -Tag "$Id-first"
        $win32 = $true
        $tgtFirst = Find-WizTarget -ProcNames $ProcNames -Regex $First -Timeout $half -Win32 $true
    }
    & $add 'first' 1
    if (-not $tgtFirst) { $res.Why = "нет кнопки первой страницы ($First)"; return $res }
    $res.Via += ('first=' + (Invoke-WizPress -ProcNames $ProcNames -Tgt $tgtFirst -Regex $First -Win32 $true))
    Start-Sleep -Seconds 1
    & $add 'progress' 2
    $sw = [Diagnostics.Stopwatch]::StartNew()
    $tgtLast = $null
    while ($sw.Elapsed.TotalSeconds -lt $Timeout) {
        if ($sw.Elapsed.TotalSeconds -gt ($Timeout / 2)) { $win32 = $true }
        $tgtLast = Find-WizTarget -ProcNames $ProcNames -Regex $Last -Timeout 1 -Win32 $win32
        if ($tgtLast) { break }
        $retry = Find-WizTarget -ProcNames $ProcNames -Regex $script:WizBtn.Retry -Timeout 0.5 -Win32 $win32
        if ($retry -and $OnRetry -and -not $res.Retried) {
            $res.Retried = $true
            $res.RetryText = Format-WizText (Get-QaTexts -ProcNames $ProcNames)
            & $OnRetry $retry
            $retry = Find-WizTarget -ProcNames $ProcNames -Regex $script:WizBtn.Retry -Timeout 5 -Win32 $win32
            if ($retry) { $res.Via += ('retry=' + (Invoke-WizPress -ProcNames $ProcNames -Tgt $retry -Regex $script:WizBtn.Retry -Win32 $win32)) }
        } elseif ($retry) {
            $res.Why = 'окно "Повтор" не ожидалось: ' + (Format-WizText (Get-QaTexts -ProcNames $ProcNames))
            return $res
        }
    }
    if (-not $tgtLast) { $res.Why = "последняя кнопка ($Last) не появилась за $Timeout с"; return $res }
    & $add 'last' 3
    [void](Clear-WizRunCheckbox $ProcNames)
    $res.Via += ('last=' + (Invoke-WizPress -ProcNames $ProcNames -Tgt $tgtLast -Regex $Last -Win32 $true))
    if (-not (Wait-ProcGone -ProcNames $ProcNames)) { $res.Why = 'мастер не закрылся после последней кнопки'; return $res }
    $res.Ok = $true
    return $res
}

# Подпись для пункта: страницы по порядку, коротко
function Get-WizSummary($Res) {
    $pages = (($Res.Pages | ForEach-Object { '{0}: {1}' -f $_.Slug, $_.Text }) -join ' || ')
    return ($pages + ' || нажато: ' + (@($Res.Via) -join ', '))
}
