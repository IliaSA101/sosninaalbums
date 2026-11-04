# Основной режим: готовит .testphotos/sub/ (копии 5 stock-файлов, c-stock-NN.jpg), если её нет.
# Сами живые фото .testphotos/stock/stock-01..100.jpg (Pexels, список — docs/qa/stock-photos.md)
# в репозитории не лежат и скриптом не создаются; XMP-кейсы — make-test-xmp.ps1 -Src.
#
# -Synthetic: прежние 36 синтетических JPEG (3:2, 2:3, 1:1) img-NNN.jpg в .testphotos/ —
# для крупных и ровных кадров. Нужен ffmpeg в PATH.
#
# -Big: 600 фото 24 Мп в .testphotos-big/ (спека 020, QA на большом альбоме).
# Настоящих файлов 12 (разные ориентации и пропорции), остальные — жёсткие ссылки
# на них: на диске ~200 МБ, а библиотека видит 600 отдельных путей.
# -Count и -Dir — для проверки скрипта на малом числе (по умолчанию 600 и .testphotos-big).
param(
  [switch]$Big,
  [switch]$Synthetic,
  [int]$Count = 600,
  [string]$Dir = ''
)
$src = @('testsrc2', 'mandelbrot', 'smptehdbars', 'rgbtestsrc', 'cellauto',
  'gradients=c0=0x3366aa:c1=0xffcc88', 'gradients=c0=0x224422:c1=0xddffaa', 'life=mold=10:ratio=0.3')

function New-Frame($s, $z, $path) {
  $f = if ($s -match '^(gradients|life)') { "${s}:s=$z" } else { "$s=s=$z" }
  ffmpeg -v error -y -f lavfi -i $f -frames:v 1 -q:v 3 $path
}

if ($Big) {
  $out = if ($Dir) { $Dir } else { Join-Path $PSScriptRoot '..\.testphotos-big' }
  New-Item -ItemType Directory -Force $out | Out-Null
  # 24 Мп: 6000x4000, 4000x6000 и квадрат 4899x4899
  $bigSizes = @('6000x4000', '4000x6000', '6000x4000', '6000x4000', '4000x6000', '4899x4899')
  $uniq = 12
  $names = @()
  foreach ($i in 1..$Count) { $names += Join-Path $out ('img-{0:D3}.jpg' -f $i) }
  foreach ($i in 1..[Math]::Min($uniq, $Count)) {
    New-Frame $src[$i % $src.Count] $bigSizes[$i % $bigSizes.Count] $names[$i - 1]
  }
  for ($i = $uniq + 1; $i -le $Count; $i++) {
    Remove-Item -Force $names[$i - 1] -ErrorAction SilentlyContinue
    New-Item -ItemType HardLink -Path $names[$i - 1] -Target $names[($i - 1) % $uniq] | Out-Null
  }
  return
}

$out = Join-Path $PSScriptRoot '..\.testphotos'
New-Item -ItemType Directory -Force $out | Out-Null

if ($Synthetic) {
  $sizes = @('3000x2000', '2000x3000', '3000x2000', '3000x2000', '2000x3000', '2400x2400')
  foreach ($i in 1..36) {
    New-Frame $src[$i % $src.Count] $sizes[$i % $sizes.Count] (Join-Path $out ('img-{0:D3}.jpg' -f ($i * 7)))
  }
  return
}

# Основной набор — живые фото: stock/stock-01..100.jpg кладутся вручную (docs/qa/stock-photos.md),
# скрипт их не создаёт. Здесь только sub/: копии 5 первых stock-файлов, если папки ещё нет.
$stock = Join-Path $out 'stock'
$sub = Join-Path $out 'sub'
if (-not (Test-Path -LiteralPath $stock)) { throw "Missing $stock : put stock-01..100.jpg there (docs/qa/stock-photos.md)" }
# Синтетика img-NNN.jpg прежних запусков завышает счёт папки (QA 09, guide) — убрать.
Get-ChildItem -LiteralPath $out -File | Where-Object { $_.Name -match '^img-\d{3}\.jpg$' } | Remove-Item -Force
$need = 1..5 | ForEach-Object { Join-Path $stock ('stock-{0:D2}.jpg' -f $_) }
foreach ($f in $need) { if (-not (Test-Path -LiteralPath $f)) { throw "Missing $f" } }
# Прежняя sub/ держала c-img-*.jpg — пересоздать, если в ней нет копий stock.
if ((Test-Path -LiteralPath $sub) -and -not (Get-ChildItem -LiteralPath $sub -Filter 'c-stock-*.jpg')) { Remove-Item -LiteralPath $sub -Recurse -Force }
if (-not (Test-Path -LiteralPath $sub)) {
  New-Item -ItemType Directory -Force $sub | Out-Null
  foreach ($f in $need) {
    Copy-Item -LiteralPath $f -Destination (Join-Path $sub ('c-' + (Split-Path $f -Leaf))) -ErrorAction Stop
  }
}
