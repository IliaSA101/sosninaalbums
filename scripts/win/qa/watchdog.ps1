# watchdog.ps1 - сторож задания win-qa (спека 118, design §1): отдельный процесс, который снимает завис сценария.
# Запускает run.ps1 и останавливает в конце. Раз в 2 с читает logs/current.json (пульс: пункт, шаг, срок).
# Срок пункта вышел: снимок экрана png/<id>-hang.png, строка fail "завис на шаге ...", попытка закрыть окна AES/SmartScreen кнопкой
# Cancel/Отмена. Пульс не сдвинулся ещё 60 с: logs/abort.txt (id, шаг) и taskkill run.ps1 по PID (только по PID, никогда по имени).
# Срок блока (30 мин) - то же с причиной "блок остановлен сторожем". Windows PowerShell 5.1, файл в UTF-8 с BOM.
param(
    [Parameter(Mandatory)][int]$RunPid,
    [Parameter(Mandatory)][string]$OutDir,
    [Parameter(Mandatory)][string]$Job,
    [string]$Runner = '',
    [int]$BlockMin = 30,
    [int]$PollSec = 2,
    [int]$StallSec = 60
)
$ErrorActionPreference = 'Continue'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$win = Split-Path -Parent $here
foreach ($m in @((Join-Path $win 'winlib.psm1'), (Join-Path $win 'winui.psm1'), (Join-Path $here 'winqa.psm1'), (Join-Path $here 'winqa-dpi.psm1'))) {
    Import-Module $m -Global -Force -DisableNameChecking
}
Initialize-Dpi
Initialize-Qa -OutDir $OutDir -Job $Job -Runner $Runner
# свой журнал: общий logs/<блок>.txt открыт сценарием
Set-LogPath (Join-Path $OutDir 'logs\watchdog.txt')
$pulsePath = Join-Path $OutDir 'logs\current.json'
$abortPath = Join-Path $OutDir 'logs\abort.txt'
$state = @{ HangAt = 0.0; HangId = ''; HangSeq = -1; HangStep = ''; Block = $false }
$blockStart = Get-QaUnixNow
$reported = @{}
Write-Log "сторож: run.ps1 PID $RunPid, срок блока $BlockMin мин, ожидание после срока $StallSec с"

# Закрыть окна, на которых завис сценарий: AES Проводника и SmartScreen (кнопки отмены)
function Close-HangDialogs {
    $aes = Find-QaAesWindow
    if ($aes) {
        $b = Find-QaWindowButton $aes @('Cancel', 'Отмена')
        if ($b) { try { Invoke-AppEl -El $b.Element -Mouse | Out-Null; Write-Log 'сторож: окно проверки безопасности закрыто кнопкой Cancel' } catch { Write-Log "сторож: Cancel не нажат: $($_.Exception.Message)" } }
    }
}

while ($true) {
    Start-Sleep -Seconds $PollSec
    if (-not (Get-Process -Id $RunPid -ErrorAction SilentlyContinue)) { Write-Log 'сторож: run.ps1 завершился'; break }
    $pulse = $null
    if (Test-Path -LiteralPath $pulsePath) { try { $pulse = Get-Content -LiteralPath $pulsePath -Raw -Encoding UTF8 | ConvertFrom-Json } catch { $pulse = $null } }
    $now = Get-QaUnixNow
    $act = Get-QaWatchAction -Pulse $pulse -Now $now -State $state -BlockStart $blockStart -BlockSec ($BlockMin * 60) -StallSec $StallSec
    if ($act.Action -eq 'hang') {
        $id = $act.Id; if (-not $id) { $id = 'блок' }
        $shotId = $id; if ($id -eq 'блок') { $shotId = 'BLOCK' }
        $shot = Save-QaShot -Id $shotId -Slug 'hang'
        if ($act.Reason -eq 'block') { $why = "блок остановлен сторожем: прошло $BlockMin мин; шаг '$($act.Step)'" }
        else { $why = "завис на шаге '$($act.Step)' дольше срока пункта" }
        # пункт, ожив и встав снова, не плодит строки: провал записывается один раз
        if ($id -ne 'блок') { if (-not $reported.ContainsKey($id)) { $reported[$id] = $true; Add-QaResult -Id $id -Status fail -Detail $why -Shots $shot } }
        else { Write-Log "сторож: $why" }
        $state.HangAt = $now; $state.HangId = [string]$pulse.id; $state.HangSeq = [long]$pulse.seq; $state.HangStep = [string]$act.Step
        $state.Block = ($act.Reason -eq 'block')
        Close-HangDialogs
    } elseif ($act.Action -eq 'abort') {
        $idText = $state.HangId; if (-not $idText) { $idText = 'блок' }
        [System.IO.File]::WriteAllText($abortPath, "$idText`n$($state.HangStep)`n", (New-Object System.Text.UTF8Encoding($false)))
        Write-Log "сторож: пульс стоит $StallSec с, run.ps1 (PID $RunPid) снимается; abort.txt записан"
        & taskkill.exe /PID $RunPid /T /F | Out-Null
        break
    }
}
exit 0
