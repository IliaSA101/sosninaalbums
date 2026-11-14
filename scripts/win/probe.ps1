# probe.ps1 - пробы среды раннера Windows (спека 036, design §5.4): сеанс, экран, снимок, SmartScreen,
# Smart App Control, Edge, язык, OCR, способы записи. Каждая проба - строка summary.md и подробности в logs/probe.txt.
# Всегда выходит с кодом 0: проба не должна ронять прогон. Файл в UTF-8 с BOM.
param(
    [Parameter(Mandatory)][string]$OutDir,
    [switch]$Video,
    [switch]$TryRu
)
$ErrorActionPreference = 'Continue'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $here 'winlib.psm1') -Force -DisableNameChecking
Import-Module (Join-Path $here 'winui.psm1') -Force -DisableNameChecking
. (Join-Path $here 'record.ps1')

foreach ($d in 'png', 'video', 'logs') { New-Item -ItemType Directory -Force -Path (Join-Path $OutDir $d) | Out-Null }
Set-LogPath (Join-Path $OutDir 'logs\probe.txt')
Set-SummaryPath (Join-Path $OutDir 'summary.md')
Initialize-Dpi

$build = Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion' -ErrorAction SilentlyContinue
Add-SummaryLine '# Windows: снимки и видео (win-shots)'
Add-SummaryLine ''
Add-SummaryLine ('Раннер: {0}, {1}, сборка {2}.{3}, процессор {4}' -f $env:RUNNER_NAME, $build.ProductName, $build.CurrentBuild, $build.UBR, $env:PROCESSOR_ARCHITECTURE)
Add-SummarySection 'Пробы' @('проба', 'итог', 'почему')

function Add-Probe([string]$Name, [bool]$Ok, [string]$Why) {
    $r = 'нет'
    if ($Ok) { $r = 'да' }
    Add-SummaryRow $Name $r $Why
    Write-Log ('проба "{0}": {1} - {2}' -f $Name, $r, $Why)
}
function Get-RegValue([string]$Path, [string]$Name) {
    try { return (Get-ItemProperty -LiteralPath $Path -Name $Name -ErrorAction Stop).$Name } catch { return $null }
}

# 1. Сеанс: интерактивный сеанс с рабочим столом нужен для курсора, снимков и gdigrab
try {
    $me = Get-Process -Id $PID
    $expl = @(Get-Process -Name explorer -ErrorAction SilentlyContinue)
    $why = 'whoami={0}; UserInteractive={1}; SessionId={2}; explorer: {3} шт.; Runner.Listener: {4}' -f `
    (whoami), [Environment]::UserInteractive, $me.SessionId, $expl.Count, [bool](Get-Process -Name Runner.Listener -ErrorAction SilentlyContinue)
    Add-Probe 'Сеанс' ([Environment]::UserInteractive -and $me.SessionId -ne 0) $why
    Write-Log ((query user 2>&1) -join "`n")
} catch { Add-Probe 'Сеанс' $false $_.Exception.Message }

# 2. Экран: размер, масштаб; смена на 1920x1080
try {
    $before = (Get-ScreenSize) -join 'x'
    $set = Set-ScreenResolution
    $after = (Get-ScreenSize) -join 'x'
    Add-Probe 'Экран' ([bool]$set) ("было $before, стало $after" + $(if (-not $set) { '; разрешение не сменилось' } else { '' }))
    Set-SolidDesktop
    (New-Object -ComObject Shell.Application).MinimizeAll()
    Start-Sleep -Milliseconds 800
} catch { Add-Probe 'Экран' $false $_.Exception.Message }

# 3. Снимок экрана: рабочий стол есть, если картинка не чёрная
try {
    $png = Join-Path $OutDir 'png\00-probe-desktop.png'
    Save-Screen $png
    $black = Get-BlackRatio $png
    Add-Probe 'Снимок экрана' ($black -lt 0.9) ('чёрных точек {0:P0}' -f $black)
} catch { Add-Probe 'Снимок экрана' $false $_.Exception.Message }

# 4. Защита от неизвестных программ (политики, умный контроль приложений) и версия браузера Edge
try {
    $pol = Get-RegValue 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\System' 'EnableSmartScreen'
    $exp = Get-RegValue 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer' 'SmartScreenEnabled'
    $proc = [bool](Get-Process -Name smartscreen -ErrorAction SilentlyContinue)
    $sas = Get-RegValue 'HKLM:\SYSTEM\CurrentControlSet\Control\CI\Policy' 'VerifiedAndReputablePolicyState'
    $sasText = 'не читается'
    if ($null -ne $sas) { $sasText = @{ 0 = 'выкл'; 1 = 'вкл'; 2 = 'оценка' }[[int]$sas] + " ($sas)" }
    Add-Probe 'SmartScreen (реестр)' $true ("EnableSmartScreen=$pol; Explorer\SmartScreenEnabled=$exp; smartscreen.exe запущен: $proc")
    Add-Probe 'Smart App Control' $true $sasText
    try { Write-Log ((Get-MpComputerStatus | Format-List AMServiceEnabled, RealTimeProtectionEnabled, AntivirusEnabled | Out-String)) } catch { Write-Log "Get-MpComputerStatus: $($_.Exception.Message)" }
    $edge = Get-RegValue 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{56EB18F8-B008-4CBD-B6D2-8C97FE7E9062}' 'pv'
    if (-not $edge) { $edge = Get-RegValue 'HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{56EB18F8-B008-4CBD-B6D2-8C97FE7E9062}' 'pv' }
    Add-Probe 'Edge' ([bool]$edge) ("версия $edge")
} catch { Add-Probe 'SmartScreen / Edge' $false $_.Exception.Message }

# 5. Язык системы и OCR
try {
    $langs = (Get-OcrLanguages) -join ', '
    Add-Probe 'Язык интерфейса' $true ("UICulture={0}; системный язык {1}" -f (Get-UICulture).Name, (Get-WinSystemLocale).Name)
    Add-Probe 'OCR' ([bool]$langs) ("языки: $langs")
} catch { Add-Probe 'Язык / OCR' $false $_.Exception.Message }

# 6. Запись: gdigrab -> ddagrab -> кадры. Выбранный способ - в video/method.txt (его читает shots.ps1)
if ($Video) {
    $ff = Get-Ffmpeg
    if (-not $ff) {
        Add-Probe 'ffmpeg' $false 'не найден (WIN_FFMPEG / PATH)'
    } else {
        Add-Probe 'ffmpeg' $true ((& $ff -version 2>&1 | Select-Object -First 1) -join '')
        $chosen = ''
        foreach ($m in 'gdigrab', 'ddagrab', 'frames') {
            $r = Test-RecMethod $m (Join-Path $OutDir 'video')
            Add-Probe "Запись: $m" $r.Ok $r.Why
            if ($r.Ok -and -not $chosen) { $chosen = $m; break }
        }
        [IO.File]::WriteAllText((Join-Path $OutDir 'video\method.txt'), $chosen)
    }
}

# 7. (try_ru) Русский язык: ожидаем «нет» - язык применяется после повторного входа (design §5.4 п. 6)
if ($TryRu) {
    try {
        Set-Service -Name wuauserv -StartupType Manual -ErrorAction Stop
        $job = Start-Job { Install-Language -Language ru-RU -ErrorAction Stop }
        if (Wait-Job $job -Timeout 900) { Receive-Job $job | Out-Null } else { Stop-Job $job; throw 'Install-Language не уложился в 15 мин' }
        Set-WinUILanguageOverride -Language ru-RU
        $p = Start-Process -FilePath notepad.exe -PassThru
        Start-Sleep -Seconds 2
        Save-Screen (Join-Path $OutDir 'png\00-probe-ru.png')
        Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        Add-Probe 'Русский язык (try_ru)' $true 'установлен, проверить png/00-probe-ru.png: окно Блокнота русское?'
    } catch { Add-Probe 'Русский язык (try_ru)' $false $_.Exception.Message }
}

exit 0
