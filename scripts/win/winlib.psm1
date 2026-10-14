# winlib.psm1 - чистые функции scripts/win (спека 036, design §2 и §5): журнал steps.jsonl, сводка,
# подписи, разбор входов, статистика кадров. Без ввода и UIA (это winui.psm1) - проверяет test_winlib.ps1.
# Windows PowerShell 5.1: файл в UTF-8 с BOM, иначе 5.1 читает кириллицу как ANSI.

Add-Type -AssemblyName System.Drawing

if (-not ('SaWin.Img' -as [type])) {
    Add-Type -ReferencedAssemblies System.Drawing -TypeDefinition @'
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;
namespace SaWin {
    // Попиксельные проверки в C#: в PowerShell цикл по 2 млн точек идёт минутами.
    public static class Img {
        static byte[] Bits(Bitmap b, out int stride) {
            var d = b.LockBits(new Rectangle(0, 0, b.Width, b.Height), ImageLockMode.ReadOnly, PixelFormat.Format32bppArgb);
            try {
                stride = d.Stride;
                var buf = new byte[Math.Abs(d.Stride) * b.Height];
                Marshal.Copy(d.Scan0, buf, 0, buf.Length);
                return buf;
            } finally { b.UnlockBits(d); }
        }
        // Доля почти чёрных точек (все каналы <= thr).
        public static double BlackRatio(Bitmap b, int thr) {
            int s; var p = Bits(b, out s); long n = 0;
            for (int y = 0; y < b.Height; y++)
                for (int x = 0; x < b.Width; x++) {
                    int i = y * s + x * 4;
                    if (p[i] <= thr && p[i + 1] <= thr && p[i + 2] <= thr) n++;
                }
            return (double)n / ((double)b.Width * b.Height);
        }
        // Доля точек, у которых сумма расхождений каналов больше thr. Размеры разные - 1.0.
        public static double DiffRatio(Bitmap a, Bitmap b, int thr) {
            if (a.Width != b.Width || a.Height != b.Height) return 1.0;
            int sa, sb; var pa = Bits(a, out sa); var pb = Bits(b, out sb); long n = 0;
            for (int y = 0; y < a.Height; y++)
                for (int x = 0; x < a.Width; x++) {
                    int i = y * sa + x * 4, j = y * sb + x * 4;
                    int d = Math.Abs(pa[i] - pb[j]) + Math.Abs(pa[i + 1] - pb[j + 1]) + Math.Abs(pa[i + 2] - pb[j + 2]);
                    if (d > thr) n++;
                }
            return (double)n / ((double)a.Width * a.Height);
        }
        // Средний цвет полосы шириной w у левого края: {R, G, B}. Так монтаж ищет метку синхронизации.
        public static double[] StripMean(Bitmap b, int w) {
            int s; var p = Bits(b, out s); long r = 0, g = 0, bl = 0, n = 0;
            int ww = Math.Min(w, b.Width);
            for (int y = 0; y < b.Height; y++)
                for (int x = 0; x < ww; x++) {
                    int i = y * s + x * 4;
                    bl += p[i]; g += p[i + 1]; r += p[i + 2]; n++;
                }
            if (n == 0) return new double[] { 0, 0, 0 };
            return new double[] { (double)r / n, (double)g / n, (double)bl / n };
        }
    }
}
'@
}

$script:Utf8 = New-Object System.Text.UTF8Encoding($false)
$script:J = $null
$script:LogPath = $null
$script:SumPath = $null
$script:Shots = New-Object System.Collections.ArrayList

# ---------- входы ----------

# \z, а не $: «$» пропускает перевод строки в конце значения
function Test-TagInput([string]$Value) { return ($Value -cmatch '^v\d+\.\d+\.\d+\z') }
function Test-RunIdInput([string]$Value) { return ($Value -cmatch '^\d+\z') }
function Test-RunnerInput([string]$Value) { return ($Value -cmatch '^windows-(11-arm|2025)\z') }

# ---------- журнал и сводка ----------

function Write-Log([string]$Message) {
    $line = '{0} {1}' -f (Get-Date -Format 'HH:mm:ss'), $Message
    Write-Host $line
    if ($script:LogPath) {
        try { [IO.File]::AppendAllText($script:LogPath, $line + "`n", $script:Utf8) } catch { }
    }
}

function Set-LogPath([string]$Path) {
    $dir = Split-Path -Parent $Path
    if ($dir -and -not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    $script:LogPath = $Path
}

function Set-SummaryPath([string]$Path) { $script:SumPath = $Path }

function ConvertTo-MdCell([string]$Text) {
    if ($null -eq $Text) { return '' }
    return (($Text -replace '\r?\n', ' ') -replace '\|', '\|').Trim()
}

function Add-SummaryLine([string]$Text) {
    if (-not $script:SumPath) { return }
    [IO.File]::AppendAllText($script:SumPath, $Text + "`n", $script:Utf8)
}

function Add-SummarySection([string]$Heading, [string[]]$Columns) {
    Add-SummaryLine ''
    Add-SummaryLine ('## ' + $Heading)
    Add-SummaryLine ''
    Add-SummaryLine ('| ' + ($Columns -join ' | ') + ' |')
    Add-SummaryLine ('|' + (($Columns | ForEach-Object { ' --- |' }) -join ''))
}

# Строка таблицы: проба | итог (да/нет/пропуск/без стрелки) | почему
function Add-SummaryRow([string]$Name, [string]$Result, [string]$Why = '') {
    Add-SummaryLine ('| {0} | {1} | {2} |' -f (ConvertTo-MdCell $Name), (ConvertTo-MdCell $Result), (ConvertTo-MdCell $Why))
}

# ---------- JSON ----------

function Format-JNum($Value) {
    $d = [double]$Value
    return $d.ToString('0.###', [Globalization.CultureInfo]::InvariantCulture)
}

function ConvertTo-JString([string]$Text) {
    $sb = New-Object System.Text.StringBuilder
    [void]$sb.Append('"')
    foreach ($ch in $Text.ToCharArray()) {
        switch ($ch) {
            '"' { [void]$sb.Append('\"') }
            '\' { [void]$sb.Append('\\') }
            "`n" { [void]$sb.Append('\n') }
            "`r" { [void]$sb.Append('\r') }
            "`t" { [void]$sb.Append('\t') }
            default {
                if ([int]$ch -lt 32) { [void]$sb.AppendFormat('\u{0:x4}', [int]$ch) } else { [void]$sb.Append($ch) }
            }
        }
    }
    [void]$sb.Append('"')
    return $sb.ToString()
}

# Свой сериализатор: ConvertTo-Json в 5.1 переставляет ключи, глотает одноэлементные массивы и считает по культуре.
function ConvertTo-JLine($Value) {
    if ($null -eq $Value) { return 'null' }
    if ($Value -is [bool]) { if ($Value) { return 'true' } else { return 'false' } }
    if ($Value -is [string]) { return (ConvertTo-JString $Value) }
    if ($Value -is [System.Collections.IDictionary]) {
        # GetEnumerator, а не .Keys: у словаря с ключом 'keys' (hold, keys) точка отдаёт значение ключа, а не список ключей
        $parts = @()
        foreach ($e in $Value.GetEnumerator()) { $parts += ((ConvertTo-JString ([string]$e.Key)) + ':' + (ConvertTo-JLine $e.Value)) }
        return '{' + ($parts -join ',') + '}'
    }
    if ($Value -is [System.Collections.IEnumerable]) {
        $parts = @()
        foreach ($v in $Value) { $parts += (ConvertTo-JLine $v) }
        return '[' + ($parts -join ',') + ']'
    }
    if ($Value -is [ValueType]) { return (Format-JNum $Value) }
    return (ConvertTo-JString ([string]$Value))
}

# ---------- журнал steps.jsonl (design §2) ----------

function Start-Journal([string]$Path, [System.Collections.Specialized.OrderedDictionary]$Meta) {
    $dir = Split-Path -Parent $Path
    if ($dir -and -not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
    [IO.File]::WriteAllText($Path, '', $script:Utf8)
    $script:J = @{ Path = $Path; Sw = [Diagnostics.Stopwatch]::StartNew(); Items = (New-Object System.Collections.ArrayList) }
    $m = [ordered]@{ ev = 'meta' }
    foreach ($e in $Meta.GetEnumerator()) { $m[[string]$e.Key] = $e.Value }
    Add-JEvent $m (-1e9)
}

# Секунды от начала журнала, с точностью до мс
function Get-JTime { return [math]::Round($script:J.Sw.Elapsed.TotalSeconds, 3) }

# Строка уходит в файл сразу (на случай обрыва); порядок по t выравнивает Close-Journal
function Add-JEvent([System.Collections.Specialized.OrderedDictionary]$Obj, [double]$Key) {
    $line = ConvertTo-JLine $Obj
    [IO.File]::AppendAllText($script:J.Path, $line + "`n", $script:Utf8)
    [void]$script:J.Items.Add([pscustomobject]@{ Key = $Key; Idx = $script:J.Items.Count; Line = $line })
}

function Add-JSync([string]$Color, [double]$Dur) {
    $t = Get-JTime
    Add-JEvent ([ordered]@{ ev = 'sync'; t = $t; kind = 'flash'; color = $Color; dur = $Dur }) $t
}
function Add-JChapter([string]$Title) {
    $t = Get-JTime
    Add-JEvent ([ordered]@{ ev = 'chapter'; t = $t; title = $Title }) $t
}
function Add-JStep([int]$N, [int]$Of, [string]$Title) {
    $t = Get-JTime
    Add-JEvent ([ordered]@{ ev = 'step'; t = $t; n = $N; of = $Of; title = $Title }) $t
}
# rect - массив [x,y,w,h] в пикселях экрана или $null (подпись внизу без стрелки)
function Add-JHold($Rect, [string]$Caption, $Keys = $null) {
    $t = Get-JTime
    $r = $null
    if ($null -ne $Rect) { $r = @([int[]]$Rect) }
    Add-JEvent ([ordered]@{ ev = 'hold'; t = $t; rect = $r; caption = $Caption; keys = $Keys }) $t
}
function Add-JSegment([string]$Ev, [double]$T0, [double]$T1) {
    Add-JEvent ([ordered]@{ ev = $Ev; t0 = $T0; t1 = $T1 }) $T0
}
function Add-JKeys([double]$T0, [double]$T1, [string[]]$Keys) {
    Add-JEvent ([ordered]@{ ev = 'keys'; t0 = $T0; t1 = $T1; keys = @($Keys) }) $T0
}
function Add-JNote([string]$Text) {
    $t = Get-JTime
    Add-JEvent ([ordered]@{ ev = 'note'; t = $t; text = $Text }) $t
}

# Конец записи; файл переписывается по порядку t (у отрезков по t0), при равенстве - по порядку появления
function Close-Journal {
    $t = Get-JTime
    Add-JEvent ([ordered]@{ ev = 'end'; t = $t }) 1e9
    # Idx вторым ключом: Sort-Object в 5.1 нестабилен, равные t иначе перемешаются
    $sorted = @($script:J.Items | Sort-Object -Property Key, Idx)
    $text = (($sorted | ForEach-Object { $_.Line }) -join "`n") + "`n"
    [IO.File]::WriteAllText($script:J.Path, $text, $script:Utf8)
    return $script:J.Path
}

# Проверки контракта design §2; пустой результат - журнал годен. Ошибка - «строка N: ...»
function Test-JournalLines([string[]]$Lines) {
    $errs = New-Object System.Collections.ArrayList
    $screen = $null
    $prev = -1e9
    $busy = New-Object System.Collections.ArrayList   # отрезки move/keys/type
    $idles = New-Object System.Collections.ArrayList
    $n = 0
    foreach ($line in $Lines) {
        $n++
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        try { $o = $line | ConvertFrom-Json } catch { [void]$errs.Add("строка ${n}: не JSON"); continue }
        $ev = [string]$o.ev
        if ($n -eq 1 -and $ev -ne 'meta') { [void]$errs.Add("строка ${n}: первой должна быть meta") }
        if ($n -gt 1 -and $ev -eq 'meta') { [void]$errs.Add("строка ${n}: meta не первая") }
        $key = $null
        if ($null -ne $o.PSObject.Properties['t']) { $key = [double]$o.t }
        elseif ($null -ne $o.PSObject.Properties['t0']) { $key = [double]$o.t0 }
        if ($null -ne $key) {
            if ($key -lt $prev) { [void]$errs.Add("строка ${n}: время убывает ($key < $prev)") }
            $prev = $key
        }
        switch ($ev) {
            'meta' { if ($null -ne $o.screen) { $screen = @([double]$o.screen[0], [double]$o.screen[1]) } }
            'hold' {
                if ([string]::IsNullOrWhiteSpace([string]$o.caption)) { [void]$errs.Add("строка ${n}: пустая подпись") }
                elseif (([string]$o.caption).Length -gt 80) { [void]$errs.Add("строка ${n}: подпись длиннее 80 знаков") }
                if ($null -ne $o.rect) {
                    $x = [double]$o.rect[0]; $y = [double]$o.rect[1]; $w = [double]$o.rect[2]; $h = [double]$o.rect[3]
                    if ($w -le 0 -or $h -le 0 -or $x -lt 0 -or $y -lt 0 -or ($screen -and ($x + $w -gt $screen[0] -or $y + $h -gt $screen[1]))) {
                        [void]$errs.Add("строка ${n}: рамка вне экрана")
                    }
                }
            }
            { $_ -in 'move', 'keys', 'type', 'idle' } {
                if ([double]$o.t1 -lt [double]$o.t0) { [void]$errs.Add("строка ${n}: t1 < t0") }
                if ($ev -eq 'idle') { [void]$idles.Add(@([double]$o.t0, [double]$o.t1, $n)) }
                else { [void]$busy.Add(@([double]$o.t0, [double]$o.t1, $n)) }
            }
        }
    }
    foreach ($i in $idles) {
        foreach ($b in $busy) {
            if ($i[0] -lt $b[1] -and $b[0] -lt $i[1]) { [void]$errs.Add("строка $($i[2]): idle пересекается с отрезком из строки $($b[2])") }
        }
    }
    return @($errs)
}

# Рамку в пределах экрана; вырожденная - $null (подпись тогда без стрелки)
function Limit-Rect($Rect, [int]$ScreenW, [int]$ScreenH) {
    if ($null -eq $Rect) { return $null }
    $x0 = [math]::Max(0, [int]$Rect[0]); $y0 = [math]::Max(0, [int]$Rect[1])
    $x1 = [math]::Min($ScreenW, [int]$Rect[0] + [int]$Rect[2]); $y1 = [math]::Min($ScreenH, [int]$Rect[1] + [int]$Rect[3])
    if ($x1 - $x0 -lt 2 -or $y1 - $y0 -lt 2) { return $null }
    return @($x0, $y0, ($x1 - $x0), ($y1 - $y0))
}

# ---------- подписи ----------

# «Нажмите «Подробнее»»; если окно английское (нашли не русское имя) - «Нажмите «Подробнее» (More info)»
function Get-Caption([string]$Ru, [string]$Found = '', [string]$Verb = 'Нажмите') {
    $c = '{0} «{1}»' -f $Verb, $Ru
    $f = ''
    if ($Found) { $f = $Found.Trim() }
    if ($f -and $f -ne $Ru -and $f -notmatch '[а-яА-ЯёЁ]' -and $f.Length -le 40) { $c += " ($f)" }
    return $c
}

# ---------- OCR: поиск фразы среди слов ----------

# Words - объекты Text, X, Y, W, H в порядке чтения. Фраза - слова подряд; результат - объединение рамок
function Find-PhraseRect($Words, [string]$Phrase) {
    $norm = { param($s) ([string]$s).Trim().Trim('.', ',', ':', ';', '«', '»', '"', '(', ')').ToLowerInvariant() }
    $tokens = @($Phrase -split '\s+' | Where-Object { $_ } | ForEach-Object { & $norm $_ })
    $list = @($Words)
    if ($tokens.Count -eq 0 -or $list.Count -lt $tokens.Count) { return $null }
    for ($i = 0; $i -le $list.Count - $tokens.Count; $i++) {
        $ok = $true
        for ($j = 0; $j -lt $tokens.Count; $j++) {
            if ((& $norm $list[$i + $j].Text) -ne $tokens[$j]) { $ok = $false; break }
        }
        if (-not $ok) { continue }
        $x0 = 1e9; $y0 = 1e9; $x1 = -1e9; $y1 = -1e9
        for ($j = 0; $j -lt $tokens.Count; $j++) {
            $w = $list[$i + $j]
            $x0 = [math]::Min($x0, $w.X); $y0 = [math]::Min($y0, $w.Y)
            $x1 = [math]::Max($x1, $w.X + $w.W); $y1 = [math]::Max($y1, $w.Y + $w.H)
        }
        return @([int]$x0, [int]$y0, [int]($x1 - $x0), [int]($y1 - $y0))
    }
    return $null
}

# ---------- кадры ----------

function Use-Bitmap([string]$Path, [scriptblock]$Body) {
    $bmp = New-Object System.Drawing.Bitmap $Path
    try { return (& $Body $bmp) } finally { $bmp.Dispose() }
}

function Get-BlackRatio([string]$Path) { return (Use-Bitmap $Path { param($b) [SaWin.Img]::BlackRatio($b, 12) }) }

function Get-StripColor([string]$Path, [int]$Width = 64) {
    return (Use-Bitmap $Path { param($b) , [SaWin.Img]::StripMean($b, $Width) })
}

# Доля изменившихся точек между двумя PNG; один путь пуст - 1.0
function Get-DiffRatio([string]$PathA, [string]$PathB) {
    if (-not (Test-Path -LiteralPath $PathA) -or -not (Test-Path -LiteralPath $PathB)) { return 1.0 }
    $a = New-Object System.Drawing.Bitmap $PathA
    try {
        $b = New-Object System.Drawing.Bitmap $PathB
        try { return [SaWin.Img]::DiffRatio($a, $b, 24) } finally { $b.Dispose() }
    } finally { $a.Dispose() }
}

# Список кадров для ffmpeg concat: реальные длительности вместо номинальных 5 кадр/с
function Format-ConcatList($Frames) {
    $sb = New-Object System.Text.StringBuilder
    $last = $null
    foreach ($f in $Frames) {
        $p = ([string]$f.Path) -replace '\\', '/'
        [void]$sb.Append("file '").Append(($p -replace "'", "'\''")).Append("'`n")
        [void]$sb.Append('duration ').Append((Format-JNum $f.Dur)).Append("`n")
        $last = $p
    }
    # concat теряет длительность последнего файла без повторной строки file
    if ($last) { [void]$sb.Append("file '").Append(($last -replace "'", "'\''")).Append("'`n") }
    return $sb.ToString()
}

# ---------- shots.json (029 design §3.2) ----------

function Add-ShotRecord([string]$Name, [bool]$Ok, [string]$Why, [string]$File, $Rect, [string]$Label, $Changed, $Extra) {
    $o = [ordered]@{ name = $Name; ok = $Ok; why = $Why; file = $File; os = 'windows' }
    if ($null -ne $Extra) { foreach ($e in $Extra.GetEnumerator()) { $o[[string]$e.Key] = $e.Value } }
    $o['scale'] = 1
    $o['target'] = $null
    if ($null -ne $Rect) { $o['target'] = [ordered]@{ label = $Label; rect = @([int[]]$Rect) } }
    $o['changed'] = $Changed
    [void]$script:Shots.Add($o)
}

function Save-ShotsJson([string]$Path) {
    $lines = @($script:Shots | ForEach-Object { '  ' + (ConvertTo-JLine $_) })
    [IO.File]::WriteAllText($Path, "[`n" + ($lines -join ",`n") + "`n]`n", $script:Utf8)
}

function Get-ShotRecords { return @($script:Shots) }
