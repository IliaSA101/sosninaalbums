# run.ps1 - вход задания win-qa (спека 118, design §1): подключает помощники и запускает сценарий блока.
# Запуск: run.ps1 -Block probe|install|update|open -Kit <набор qa-kit> -OutDir <папка итогов> [-Video] [-Dpi 125,150,175] [-Sabotage] [-Runner <имя>]
# Сторож (watchdog.ps1) снимает завис: при остановке пульса на минуту убивает этот процесс по PID (код тогда не 0).
# Всегда выходит с кодом 0: провал пункта - строка в results.jsonl, а не падение задания (красит прогон qa_report.py).
# Windows PowerShell 5.1, файл в UTF-8 с BOM.
param(
    [Parameter(Mandatory)][ValidateSet('probe', 'install', 'update', 'open')][string]$Block,
    [Parameter(Mandatory)][string]$Kit,
    [Parameter(Mandatory)][string]$OutDir,
    [switch]$Video,
    [string]$Dpi = '125,150,175',
    [switch]$Sabotage,
    [string]$Runner = ''
)
$ErrorActionPreference = 'Continue'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$win = Split-Path -Parent $here
foreach ($m in @((Join-Path $win 'winlib.psm1'), (Join-Path $win 'winui.psm1'), (Join-Path $win 'winzip.psm1'),
        (Join-Path $here 'winqa.psm1'), (Join-Path $here 'winqa-dpi.psm1'), (Join-Path $here 'winqa-wizard.psm1'), (Join-Path $here 'winqa-update.psm1'), (Join-Path $here 'winqa-open.psm1'))) {
    Import-Module $m -Global -Force -DisableNameChecking
}
# PerMonitorV2 до первого окна: иначе на масштабе 125-175 % снимки режут не то (Initialize-Dpi после этого - безвредный отказ)
[void](Enable-QaDpiAware)
Initialize-Dpi
Initialize-Qa -OutDir $OutDir -Job $Block -Runner $Runner -Video:$Video
# Сторож - отдельный процесс: если сценарий завис в блокирующем вызове, сам он ничего не сделает (watchdog.ps1)
$dog = $null
try {
    $dogArgs = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $here 'watchdog.ps1'), '-RunPid', $PID, '-OutDir', $OutDir, '-Job', $Block, '-Runner', $Runner)
    $dog = Start-Process -FilePath 'powershell.exe' -ArgumentList $dogArgs -PassThru -WindowStyle Hidden
    Write-Log "сторож запущен, PID $($dog.Id)"
} catch { Write-Log "сторож не запустился: $($_.Exception.Message)" }
Add-SummaryLine "## win-qa: блок $Block"
Add-SummaryLine ''
Add-SummaryLine ("Раннер: {0}; Windows {1}; набор: {2}" -f $Runner, [Environment]::OSVersion.Version, $Kit)

try {
    $mode = Set-ScreenResolution
    Write-Log "разрешение экрана: $mode"
    $check = Test-QaKit $Kit
    if (-not $check.Ok) {
        Write-Log "набор не сошёлся с kit-sha256.txt: $($check.Why)"
        Add-SummaryLine "**Набор повреждён:** $($check.Why). Сценарий не запускался."
    } else {
        $script = Join-Path $here "qa-$Block.ps1"
        if (-not (Test-Path -LiteralPath $script)) {
            Add-SummaryLine "Сценарий $script ещё не написан."
        } else {
            & $script -Kit $Kit -OutDir $OutDir -Dpi $Dpi -Sabotage:$Sabotage -RunnerTemp $env:RUNNER_TEMP
        }
    }
} catch {
    Write-Log ("сценарий прерван: {0}`n{1}" -f $_.Exception.Message, $_.ScriptStackTrace)
    Add-SummaryLine "**Сценарий прерван:** $($_.Exception.Message)"
}
if ($dog) { try { & taskkill.exe /PID $dog.Id /T /F | Out-Null } catch { } }
$rows = 0
$jsonl = Join-Path $OutDir 'results.jsonl'
if (Test-Path -LiteralPath $jsonl) { $rows = @(Get-Content -LiteralPath $jsonl -Encoding UTF8).Count }
Add-SummaryLine ''
Add-SummaryLine "Строк итогов: $rows (сводка по пунктам - задание report)."
exit 0
