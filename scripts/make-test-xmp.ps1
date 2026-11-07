# Генерирует JPEG с отметками Lightroom (XMP) в .testphotos/xmp/ для QA спеки 004.
# ffmpeg не нужен: XMP-пакет вставляется в APP1 вручную.
# Картинки: без -Src рисует System.Drawing (600x400); с -Src <папка> берутся её *.jpg
# (по кругу, если кейсов больше, чем файлов), из копий вырезаются их собственные XMP-пакеты
# (у Pexels они есть), чтобы отметки кейса были единственными.
# С -Src делается ещё rotated-6.jpg: копия первого файла с EXIF Orientation=6 (проверка
# поворота; у файлов stock тег есть, но равен 1).
# Повторный запуск перезаписывает только свои файлы (img-*.jpg, img-*.xmp, img-*.jpg.xmp, rotated-6.jpg в xmp/).
param(
  [string]$Dir = '',
  [string]$Src = ''
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$out = if ($Dir) { $Dir } else { Join-Path $PSScriptRoot '..\.testphotos\xmp' }
New-Item -ItemType Directory -Force $out | Out-Null
$out = (Resolve-Path $out).Path
Get-ChildItem -LiteralPath $out -File |
  Where-Object { $_.Name -match '^img-\d+.*\.(jpg|xmp)$' -or $_.Name -eq 'rotated-6.jpg' } | Remove-Item -Force

$srcFiles = @()
if ($Src) {
  $srcFiles = @(Get-ChildItem -LiteralPath $Src -Filter *.jpg -File | Sort-Object Name)
  if ($srcFiles.Count -eq 0) { throw "No *.jpg in -Src: $Src" }
}

$utf8 = New-Object System.Text.UTF8Encoding($false)
# Кириллица в строках — кодами: Windows PowerShell 5.1 читает .ps1 без BOM как ANSI.
$custom = -join ([char[]](0x041A, 0x043B, 0x0438, 0x0435, 0x043D, 0x0442, 0x0443))

# Пакет в формате Lightroom Classic: отметки — атрибутами rdf:Description (как пишет сам Lightroom).
function New-Xmp($f, [int]$padKb = 0) {
  $attr = ''
  if ($f.Contains('Rating')) { $attr += "`n   xmp:Rating=`"$($f.Rating)`"" }
  if ($f.Contains('Label')) { $attr += "`n   xmp:Label=`"$($f.Label)`"" }
  if ($f.Contains('Pick')) { $attr += "`n   xmpDM:pick=`"$($f.Pick)`"" }
  $pad = if ($padKb -gt 0) { "<!--" + ('x' * ($padKb * 1024)) + "-->`n" } else { '' }
  @"
<?xpacket begin="$([char]0xFEFF)" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/" x:xmptk="Adobe XMP Core 7.0-c000 1.000000, 0000/00/00-00:00:00        ">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about=""
    xmlns:xmp="http://ns.adobe.com/xap/1.0/"
    xmlns:xmpDM="http://ns.adobe.com/xmp/1.0/DynamicMedia/"
    xmlns:xmpMM="http://ns.adobe.com/xap/1.0/mm/"
    xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/"
   xmp:CreatorTool="Adobe Photoshop Lightroom Classic 13.5 (Windows)"
   xmp:ModifyDate="2026-06-01T12:00:00+03:00"
   xmp:MetadataDate="2026-06-01T12:00:00+03:00"$attr
   xmpMM:DocumentID="xmp.did:00000000-0000-0000-0000-000000000000"
   crs:Version="16.5"
   crs:ProcessVersion="15.4"
   crs:HasSettings="True"/>
 </rdf:RDF>
</x:xmpmeta>
$pad<?xpacket end="w"?>
"@
}

function New-Jpeg($path, $n) {
  $bmp = New-Object System.Drawing.Bitmap 600, 400
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $hue = ($n * 47) % 255
  $g.Clear([System.Drawing.Color]::FromArgb(60 + [int]($hue / 2), 90, 200 - [int]($hue / 2)))
  $font = New-Object System.Drawing.Font 'Segoe UI', 48
  $g.DrawString(('{0:D3}' -f $n), $font, [System.Drawing.Brushes]::White, 20, 150)
  $g.Dispose(); $font.Dispose()
  $bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Jpeg)
  $bmp.Dispose()
}

# Копирует исходный JPEG, вырезая из копии APP1 с XMP (Exif и прочие сегменты остаются).
function Copy-WithoutXmp($from, $to) {
  $b = [IO.File]::ReadAllBytes($from)
  $sig = [Text.Encoding]::ASCII.GetBytes('http://ns.adobe.com/xap/1.0/')
  $res = New-Object System.Collections.Generic.List[byte]
  $res.AddRange([byte[]]$b[0..1])
  $p = 2
  while ($p + 4 -le $b.Length -and $b[$p] -eq 0xFF -and $b[$p + 1] -ne 0xDA) {
    $len = $b[$p + 2] * 256 + $b[$p + 3]
    $isXmp = $false
    if ($b[$p + 1] -eq 0xE1 -and $len -gt (2 + $sig.Length)) {
      $isXmp = $true
      for ($k = 0; $k -lt $sig.Length; $k++) { if ($b[$p + 4 + $k] -ne $sig[$k]) { $isXmp = $false; break } }
    }
    if (-not $isXmp) { $res.AddRange([byte[]]$b[$p..($p + 1 + $len)]) }
    $p += 2 + $len
  }
  $res.AddRange([byte[]]$b[$p..($b.Length - 1)])
  [IO.File]::WriteAllBytes($to, $res.ToArray())
}

# Копия с EXIF Orientation=6 (показывать повёрнутой на 90 градусов по часовой). Тег правится
# в PropertyItem: у stock-файлов он есть (=1); если нет — PropertyItem создаётся через рефлексию
# (публичного конструктора у класса нет). Картинка перекодируется GDI+, XMP при этом не сохраняется.
function New-RotatedJpeg($from, $to) {
  $img = [System.Drawing.Image]::FromFile($from)
  try {
    $item = $img.PropertyItems | Where-Object { $_.Id -eq 0x0112 } | Select-Object -First 1
    if (-not $item) {
      $ctor = [System.Drawing.Imaging.PropertyItem].GetConstructor([Reflection.BindingFlags]'NonPublic,Instance', $null, [Type[]]@(), $null)
      $item = $ctor.Invoke(@())
      $item.Id = 0x0112; $item.Type = 3; $item.Len = 2
    }
    $item.Value = [byte[]](6, 0)
    $img.SetPropertyItem($item)
    $img.Save($to, [System.Drawing.Imaging.ImageFormat]::Jpeg)
  } finally { $img.Dispose() }
}

# Вставляет APP1 с XMP после SOI (и после APP0/JFIF, если он есть).
function Add-EmbeddedXmp($path, $xmpText) {
  $bytes = [IO.File]::ReadAllBytes($path)
  $pos = 2
  if ($bytes[2] -eq 0xFF -and $bytes[3] -eq 0xE0) { $pos = 2 + 2 + $bytes[4] * 256 + $bytes[5] }
  $head = [Text.Encoding]::ASCII.GetBytes("http://ns.adobe.com/xap/1.0/`0")
  $body = $utf8.GetBytes($xmpText)
  $len = 2 + $head.Length + $body.Length
  if ($len -gt 65533) { throw "XMP does not fit into APP1: $len bytes" }
  $res = New-Object System.Collections.Generic.List[byte]
  $res.AddRange([byte[]]$bytes[0..($pos - 1)])
  $res.AddRange([byte[]]@(0xFF, 0xE1, [byte]($len -shr 8), [byte]($len -band 0xFF)))
  $res.AddRange([byte[]]$head)
  $res.AddRange([byte[]]$body)
  $res.AddRange([byte[]]$bytes[$pos..($bytes.Length - 1)])
  [IO.File]::WriteAllBytes($path, $res.ToArray())
}

# Emb — XMP внутри JPEG; Side — сайдкар; Kind: 'name' -> ИМЯ.xmp, 'jpg' -> ИМЯ.jpg.xmp.
$cases = @(
  @{ N = 'rating-0'; Emb = @{ Rating = 0 } },
  @{ N = 'rating-1'; Emb = @{ Rating = 1 } },
  @{ N = 'rating-2'; Side = @{ Rating = 2 }; Kind = 'name' },
  @{ N = 'rating-3'; Side = @{ Rating = 3 }; Kind = 'jpg' },
  @{ N = 'rating-4'; Emb = @{ Rating = 4 } },
  @{ N = 'rating-5'; Side = @{ Rating = 5 }; Kind = 'name' },
  @{ N = 'rejected'; Emb = @{ Rating = -1 } },
  @{ N = 'rejected-sidecar'; Side = @{ Rating = -1 }; Kind = 'jpg' },
  @{ N = 'label-red'; Emb = @{ Label = 'Red' } },
  @{ N = 'label-yellow'; Side = @{ Label = 'Yellow' }; Kind = 'name' },
  @{ N = 'label-green'; Emb = @{ Label = 'Green' } },
  @{ N = 'label-blue'; Side = @{ Label = 'Blue' }; Kind = 'jpg' },
  @{ N = 'label-purple'; Emb = @{ Label = 'Purple' } },
  @{ N = 'label-custom'; Side = @{ Rating = 1; Label = $custom }; Kind = 'name' },
  @{ N = 'pick'; Emb = @{ Pick = 1 } },
  @{ N = 'pick-sidecar'; Side = @{ Rating = 5; Label = 'Purple'; Pick = 1 }; Kind = 'jpg' },
  # Сайдкар главнее встроенного по каждому полю: итог Rating 5, Label red (из JPEG)
  @{ N = 'conflict'; Emb = @{ Rating = 2; Label = 'Red' }; Side = @{ Rating = 5 }; Kind = 'name' },
  # Оба сайдкара сразу, поля разные: Rating 3 (ИМЯ.xmp) и Label green (ИМЯ.jpg.xmp)
  @{ N = 'both-sidecars'; Side = @{ Rating = 3 }; Kind = 'name'; Side2 = @{ Label = 'Green' } },
  # Битый сайдкар: фото читается, отметки из JPEG (Rating 3)
  @{ N = 'broken-sidecar'; Emb = @{ Rating = 3 }; Raw = '<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description xmp:Rating="5"'; Kind = 'name' },
  # Сайдкар больше 1 МБ: не читается, отметки из JPEG (Rating 3)
  @{ N = 'big-sidecar'; Emb = @{ Rating = 3 }; Side = @{ Rating = 5; Label = 'Red' }; PadKb = 1100; Kind = 'jpg' },
  # Без отметок
  @{ N = 'plain' }
)

$i = 0
foreach ($c in $cases) {
  $i++
  $name = 'img-{0:D3}-{1}' -f $i, $c.N
  $jpg = Join-Path $out "$name.jpg"
  if ($srcFiles.Count -gt 0) { Copy-WithoutXmp $srcFiles[($i - 1) % $srcFiles.Count].FullName $jpg } else { New-Jpeg $jpg $i }
  if ($c.Emb) { Add-EmbeddedXmp $jpg (New-Xmp $c.Emb) }
  $side = $null
  if ($c.Raw) { $side = $c.Raw } elseif ($c.Side) { $side = New-Xmp $c.Side ([int]$c.PadKb) }
  if ($side) {
    $sp = if ($c.Kind -eq 'jpg') { "$jpg.xmp" } else { Join-Path $out "$name.xmp" }
    [IO.File]::WriteAllText($sp, $side, $utf8)
  }
  if ($c.Side2) {
    [IO.File]::WriteAllText("$jpg.xmp", (New-Xmp $c.Side2), $utf8)
  }
}
if ($srcFiles.Count -gt 0) {
  New-RotatedJpeg $srcFiles[0].FullName (Join-Path $out 'rotated-6.jpg')
  Write-Host 'rotated-6.jpg: EXIF Orientation=6'
}
Write-Host "Done: $i JPEG in $out"
