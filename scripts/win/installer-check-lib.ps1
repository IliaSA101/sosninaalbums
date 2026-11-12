# installer-check-lib.ps1 - помощники installer-check.ps1 (подключаются точкой в области сценария: $Inst, $script:Fail и др. - его переменные).
# Лежит рядом с installer-check.ps1 (в наборе qa-kit тоже). Windows PowerShell 5.1 и pwsh 7: UTF-8 с BOM.

function Check([string]$Name, $Cond, [string]$Detail = '') {
    $ok = ($Cond -is [bool]) -and $Cond
    if ($ok) { Write-Host "ok    $Name" } else { $script:Fail++; Write-Host "FAIL  $Name  $Detail" }
    [void]$script:Rows.Add([pscustomobject]@{ Name = $Name; Ok = $ok; Detail = $Detail })
}
function Same([string]$Name, $Got, $Want) { Check $Name ("$Got" -ceq "$Want") "получено '$Got', ожидалось '$Want'" }
function Section([string]$Title) { Write-Host "-- $Title" }

# Что в папке: «имя\» - каталог, «имя (N Б)» - файл; Skip не считаем
function Get-Listing([string]$Dir, [string]$Skip = '') {
    if (-not (Test-Path -LiteralPath $Dir)) { return 'папки нет' }
    $items = @(Get-ChildItem -LiteralPath $Dir -Force -ErrorAction SilentlyContinue | Where-Object { $_.Name -ne $Skip })
    if ($items.Count -eq 0) { return 'пусто' }
    return (($items | ForEach-Object { if ($_.PSIsContainer) { $_.Name + '\' } else { '{0} ({1} Б)' -f $_.Name, $_.Length } }) -join ', ')
}
# После удаления остаётся только uninstall.exe (при _?= его не стирают); лишнее - в строке FAIL
function Test-OnlyUninstaller([string]$Name) {
    $extra = Get-Listing $Inst 'uninstall.exe'
    Check $Name ((Test-Path -LiteralPath $UninstExe -PathType Leaf) -and ($extra -ceq 'пусто')) "в папке: $(Get-Listing $Inst)"
}
# Папка установки целиком. -Recurse обязателен: без него непустая папка просит подтверждения, а в неинтерактивном pwsh это NullReferenceException (-ErrorAction не гасит)
function Clear-OldImages { Get-ChildItem -LiteralPath $Inst -Filter 'SosninaAlbums.exe.old*' -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue }
function Clear-Inst {
    try { if (Test-Path -LiteralPath $Inst) { Remove-Item -LiteralPath $Inst -Recurse -Force -ErrorAction Stop } }
    catch { Check 'очистка папки установки' $false "$($_.Exception.Message); в папке: $(Get-Listing $Inst)" }
}
# Исключение блока: провал со строкой сценария, остальные блоки идут
function Add-Crash($Err) {
    $script:Fail++
    $detail = '{0} (строка {1}: {2})' -f $Err.Exception.Message, $Err.InvocationInfo.ScriptLineNumber, ("$($Err.InvocationInfo.Line)".Trim())
    Write-Host "FAIL  блок прерван: $detail"
    [void]$script:Rows.Add([pscustomobject]@{ Name = 'блок прерван'; Ok = $false; Detail = $detail })
}

# Значение ключа HKCU (имя '' - значение по умолчанию); $null - ключа нет
function Get-Reg([string]$Key, [string]$Name = '') {
    $k = Get-Item -LiteralPath "HKCU:\$Key" -ErrorAction SilentlyContinue
    if (-not $k) { return $null }
    return $k.GetValue($Name)
}
function Test-RegKey([string]$Key) { return [bool](Test-Path -LiteralPath "HKCU:\$Key") }

# Запуск установщика без ожидания потомков (Start-Process -Wait ждёт и программу, запущенную /relaunch)
function Start-Setup([string]$Exe, [string]$Arguments) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $Exe
    $psi.Arguments = $Arguments
    $psi.UseShellExecute = $false
    return [System.Diagnostics.Process]::Start($psi)
}
# Код выхода; -1 - не уложился в срок (процесс снят)
function Invoke-Setup([string]$Exe, [string]$Arguments, [int]$TimeoutSec = 180) {
    $p = Start-Setup $Exe $Arguments
    if (-not $p.WaitForExit($TimeoutSec * 1000)) {
        try { $p.Kill() } catch { }
        return -1
    }
    return $p.ExitCode
}

# Процессы программы из установленной папки
function Get-AppProc {
    return @(Get-Process -Name 'SosninaAlbums' -ErrorAction SilentlyContinue | Where-Object {
        try { $_.Path -ieq $ExePath } catch { $false }
    })
}
function Wait-AppProc([int]$TimeoutSec) {
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    do {
        $list = Get-AppProc
        if ($list.Count -gt 0) { return $list }
        Start-Sleep -Milliseconds 500
    } while ($sw.Elapsed.TotalSeconds -lt $TimeoutSec)
    return @()
}
# Закрыть по PID вместе с дочерними (WebView2); чужие процессы не трогаем
function Stop-App {
    foreach ($p in (Get-AppProc)) {
        try { & taskkill.exe /PID $p.Id /T /F | Out-Null } catch { Write-Host "taskkill: $($_.Exception.Message)" }
    }
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    while ((Get-AppProc).Count -gt 0 -and $sw.Elapsed.TotalSeconds -lt 20) { Start-Sleep -Milliseconds 300 }
}

# Манифест установщика пишется в ресурсы открытым XML: ищем строку в байтах
function Get-ManifestLevel([string]$Path) {
    $text = [System.Text.Encoding]::GetEncoding('iso-8859-1').GetString([System.IO.File]::ReadAllBytes($Path))
    $m = [regex]::Match($text, 'requestedExecutionLevel\s+level="([A-Za-z]+)"')
    if ($m.Success) { return $m.Groups[1].Value }
    return ''
}

# Читать log.txt при запущенной программе (она держит файл на запись)
function Read-Shared([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return '' }
    $fs = New-Object System.IO.FileStream($Path, 'Open', 'Read', 'ReadWrite')
    try {
        $sr = New-Object System.IO.StreamReader($fs, [System.Text.Encoding]::UTF8)
        return $sr.ReadToEnd()
    } finally { $fs.Dispose() }
}

function Write-Summary {
    $lines = @('### Установщик (win-check)', '', "Версия $Version, следующая $NextVersion.", '', '| Проверка | Итог |', '| --- | --- |')
    foreach ($r in $script:Rows) {
        $mark = 'ok'
        if (-not $r.Ok) { $mark = 'ПРОВАЛ ' + ($r.Detail -replace '\|', '/') }
        $lines += ('| {0} | {1} |' -f ($r.Name -replace '\|', '/'), $mark)
    }
    [System.IO.File]::WriteAllText($Summary, (($lines -join "`n") + "`n"), (New-Object System.Text.UTF8Encoding($false)))
}
