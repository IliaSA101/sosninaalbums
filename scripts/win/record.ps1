# record.ps1 - запись экрана для win-shots (спека 036, design §5.5). Подключается через ". record.ps1":
# останавливать запись должен тот же процесс, что её начал (команда «q» идёт в stdin ffmpeg).
# Способы по порядку: gdigrab -> ddagrab -> кадры CopyFromScreen (как Mac v1). Файл в UTF-8 с BOM.

function Get-Ffmpeg {
    if ($env:WIN_FFMPEG -and (Test-Path -LiteralPath $env:WIN_FFMPEG)) { return $env:WIN_FFMPEG }
    $c = Get-Command ffmpeg -ErrorAction SilentlyContinue
    if ($c) { return $c.Source }
    return $null
}

# Чётные стороны для yuv420p; формат приводим явно
$script:RecVf = 'scale=trunc(iw/2)*2:trunc(ih/2)*2,format=yuv420p'

function Get-FfmpegArgs([string]$Method, [string]$OutFile, [int]$Fps) {
    $enc = "-c:v libx264 -preset ultrafast -crf 23 -vf `"$script:RecVf`" -r $Fps"
    switch ($Method) {
        'gdigrab' { return "-hide_banner -loglevel warning -y -f gdigrab -framerate $Fps -draw_mouse 1 -i desktop $enc `"$OutFile`"" }
        'ddagrab' { return "-hide_banner -loglevel warning -y -f lavfi -i `"ddagrab=framerate=${Fps}:draw_mouse=1,hwdownload,format=bgra`" $enc `"$OutFile`"" }
    }
    throw "Неизвестный способ записи: $Method"
}

# Запуск через cmd: stderr ffmpeg уходит в файл (не нужно читать поток, иначе буфер встаёт и запись зависает),
# а stdin остаётся нашим - в него пишем «q».
function Start-Rec([string]$OutFile, [string]$Method = 'gdigrab', [int]$Fps = 15) {
    $dir = Split-Path -Parent $OutFile
    if ($dir -and -not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    if (Test-Path -LiteralPath $OutFile) { Remove-Item -LiteralPath $OutFile -Force }
    $rec = @{ Method = $Method; OutFile = $OutFile; Fps = $Fps; Proc = $null; Runspace = $null; Shell = $null; State = $null; Log = ($OutFile + '.log') }
    if ($Method -eq 'frames') { return (Start-FrameRec $rec) }
    $ff = Get-Ffmpeg
    if (-not $ff) { throw 'ffmpeg не найден' }
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $env:ComSpec
    $psi.Arguments = "/c `"`"$ff`" $(Get-FfmpegArgs $Method $OutFile $Fps) 2> `"$($rec.Log)`"`""
    $psi.UseShellExecute = $false
    $psi.RedirectStandardInput = $true
    $psi.CreateNoWindow = $true
    $rec.Proc = [System.Diagnostics.Process]::Start($psi)
    Start-Sleep -Milliseconds 800
    if ($rec.Proc.HasExited) { throw "ffmpeg ($Method) завершился сразу, код $($rec.Proc.ExitCode)" }
    return $rec
}

function Start-FrameRec($rec) {
    $dir = Join-Path (Split-Path -Parent $rec.OutFile) 'frames'
    if (Test-Path -LiteralPath $dir) { Remove-Item -LiteralPath $dir -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $state = [hashtable]::Synchronized(@{ Stop = $false; Frames = (New-Object System.Collections.ArrayList); Dir = $dir; Error = '' })
    $rs = [runspacefactory]::CreateRunspace()
    $rs.Open()
    $ps = [powershell]::Create()
    $ps.Runspace = $rs
    [void]$ps.AddScript({
            param($s)
            try {
                Add-Type -AssemblyName System.Drawing
                Add-Type -AssemblyName System.Windows.Forms
                $sw = [Diagnostics.Stopwatch]::StartNew()
                $i = 0
                while (-not $s.Stop) {
                    $t = $sw.Elapsed.TotalSeconds
                    $b = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
                    $bmp = New-Object System.Drawing.Bitmap $b.Width, $b.Height
                    $g = [System.Drawing.Graphics]::FromImage($bmp)
                    $g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size)
                    $g.Dispose()
                    $p = Join-Path $s.Dir ('f{0:D6}.jpg' -f $i)
                    $bmp.Save($p, [System.Drawing.Imaging.ImageFormat]::Jpeg)
                    $bmp.Dispose()
                    [void]$s.Frames.Add([pscustomobject]@{ Path = $p; T = $t; Dur = 0.2 })
                    $i++
                    $next = $i * 0.2
                    $wait = $next - $sw.Elapsed.TotalSeconds
                    if ($wait -gt 0.005) { Start-Sleep -Milliseconds ([int]($wait * 1000)) }
                }
            } catch { $s.Error = $_.Exception.Message }
        }).AddArgument($state)
    $rec.Runspace = $rs
    $rec.Shell = $ps
    $rec.State = $state
    $rec.Async = $ps.BeginInvoke()
    Start-Sleep -Milliseconds 600
    if ($state.Error) { throw "кадры: $($state.Error)" }
    return $rec
}

# Останавливает запись и собирает файл. $true - файл есть и читается
function Stop-Rec($rec) {
    if ($rec.Method -eq 'frames') { return (Stop-FrameRec $rec) }
    $p = $rec.Proc
    try {
        if (-not $p.HasExited) {
            $p.StandardInput.WriteLine('q')
            $p.StandardInput.Flush()
            $p.StandardInput.Close()
        }
    } catch { }
    if (-not $p.WaitForExit(15000)) {
        Write-Log 'ffmpeg не остановился за 15 с - убиваю'
        & taskkill.exe /T /F /PID $p.Id | Out-Null
    }
    return (Test-VideoFile $rec.OutFile)
}

function Stop-FrameRec($rec) {
    $s = $rec.State
    $s.Stop = $true
    [void]$rec.Shell.EndInvoke($rec.Async)
    $rec.Shell.Dispose()
    $rec.Runspace.Close()
    $frames = @($s.Frames)
    if ($frames.Count -lt 2) { return $false }
    # длительность кадра - до начала следующего; последний держится 0.2 с
    for ($i = 0; $i -lt $frames.Count - 1; $i++) { $frames[$i].Dur = [math]::Max(0.02, $frames[$i + 1].T - $frames[$i].T) }
    $list = Join-Path $s.Dir 'list.txt'
    [IO.File]::WriteAllText($list, (Format-ConcatList $frames), (New-Object System.Text.UTF8Encoding($false)))
    $ff = Get-Ffmpeg
    $ffArgs = "-hide_banner -loglevel warning -y -f concat -safe 0 -i `"$list`" -c:v libx264 -preset veryfast -crf 24 -vf `"fps=$($rec.Fps),$script:RecVf`" `"$($rec.OutFile)`""
    $proc = Start-Process -FilePath $ff -ArgumentList $ffArgs -NoNewWindow -Wait -PassThru -RedirectStandardError $rec.Log
    if ($proc.ExitCode -eq 0) { Remove-Item -LiteralPath $s.Dir -Recurse -Force -ErrorAction SilentlyContinue }
    return (Test-VideoFile $rec.OutFile)
}

function Test-VideoFile([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return $false }
    return ((Get-Item -LiteralPath $Path).Length -gt 2048)
}

# Кадр видео в PNG (секунда от начала); $false - не вышло
function Export-VideoFrame([string]$Video, [double]$At, [string]$Png) {
    $ff = Get-Ffmpeg
    if (-not $ff -or -not (Test-Path -LiteralPath $Video)) { return $false }
    $a = "-hide_banner -loglevel error -y -ss $(Format-JNum $At) -i `"$Video`" -frames:v 1 `"$Png`""
    $p = Start-Process -FilePath $ff -ArgumentList $a -NoNewWindow -Wait -PassThru
    return (($p.ExitCode -eq 0) -and (Test-Path -LiteralPath $Png))
}

# Проба способа: 3 с записи, посередине красная вспышка. Кадр до вспышки не чёрный, а на вспышке сильно другой - способ годен.
# Возвращает @{ Ok; Why }.
function Test-RecMethod([string]$Method, [string]$Dir) {
    $out = Join-Path $Dir "probe-$Method.mp4"
    $rec = $null
    try {
        $rec = Start-Rec $out $Method 15
        Start-Sleep -Milliseconds 1200
        Show-Flash '#d81f26' 0.8
        Start-Sleep -Milliseconds 800
        $ok = Stop-Rec $rec
        if (-not $ok) { return @{ Ok = $false; Why = 'файл не получился' } }
    } catch {
        if ($rec) { try { [void](Stop-Rec $rec) } catch { } }
        return @{ Ok = $false; Why = $_.Exception.Message }
    }
    $a = Join-Path $Dir "probe-$Method-a.png"
    $b = Join-Path $Dir "probe-$Method-b.png"
    if (-not (Export-VideoFrame $out 0.4 $a) -or -not (Export-VideoFrame $out 2.1 $b)) {
        return @{ Ok = $false; Why = 'кадры не извлеклись' }
    }
    $black = Get-BlackRatio $a
    $diff = Get-DiffRatio $a $b
    $why = 'чёрных {0:P0}, вспышка изменила {1:P0}' -f $black, $diff
    return @{ Ok = (($black -lt 0.9) -and ($diff -gt 0.5)); Why = $why }
}
