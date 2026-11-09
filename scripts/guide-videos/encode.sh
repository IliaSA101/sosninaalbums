#!/usr/bin/env bash
# Ролик справочника F1 (спека 140, design §5–6): сырой webm Playwright → два вида (WebM VP9, MP4 H.264), sha256, размер.
#
#   encode.sh <raw.webm> [--id ID] [--version N] [--out ДИР]
#
# Рядом с raw.webm может лежать <имя>.json (его пишет gv.py): {start, seconds} — окно обрезки. Нет файла — весь ролик.
# Файлы: <out>/<id>-<версия>.webm и .mp4 (имена — как в релизе videos-1). Последняя строка stdout — JSON
# {"video": <строка `videos` каталога>, "guide": {"id","seconds","hash"}} (запись для GUIDE_VIDEOS; хеш — articlehash.mjs).
# Переменные: MIN_APP (0.30.0), VIDEO_BASE_URL (релиз videos-1 библиотеки). Каталог не подписывается и не публикуется.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
raw="${1:-}"
[[ -n "$raw" && -f "$raw" ]] || { echo "encode.sh: нужен путь к webm" >&2; exit 2; }
shift
id="$(basename "$raw" .webm)"; version=1; out="$(dirname "$raw")/.."
while [[ $# -gt 0 ]]; do
  case "$1" in
    --id) id="${2:-}"; shift 2 ;;
    --version) version="${2:-}"; shift 2 ;;
    --out) out="${2:-}"; shift 2 ;;
    *) echo "encode.sh: неизвестный аргумент $1" >&2; exit 2 ;;
  esac
done
[[ "$id" =~ ^[a-z0-9][a-z0-9-]{2,39}$ ]] || { echo "encode.sh: id $id не подходит" >&2; exit 2; }
[[ "$version" =~ ^[1-9][0-9]{0,5}$ ]] || { echo "encode.sh: версия $version не подходит" >&2; exit 2; }
[[ -n "$out" ]] || { echo "encode.sh: пустой --out" >&2; exit 2; }
mkdir -p "$out"

# Окно обрезки из спутника: -ss до -i (быстро), -t — длина; без спутника — весь файл.
trim=()
meta="${raw%.webm}.json"
if [[ -f "$meta" ]]; then
  read -r ss dur < <(node -e 'const m=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log(m.start??0,m.seconds??0)' "$meta")
  trim=(-ss "$ss"); [[ "$dur" != "0" ]] && trim+=(-t "$dur")
fi

base="$out/$id-$version"
vf="scale=1280:-2,fps=15"
ffmpeg -v error -y "${trim[@]}" -i "$raw" -an -vf "$vf" -c:v libvpx-vp9 -crf 40 -b:v 0 -row-mt 1 "$base.webm"
ffmpeg -v error -y "${trim[@]}" -i "$raw" -an -vf "$vf" -c:v libx264 -crf 28 -preset slow -pix_fmt yuv420p -movflags +faststart "$base.mp4"

secs="$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$base.webm")"
hash="$(node "$here/articlehash.mjs" "$id" | awk '{print $2}')"
node -e '
const [id, ver, secs, hash, base, minApp, url] = process.argv.slice(1)
const fs = require("fs"), cr = require("crypto")
const file = (ext) => { const p = `${base}.${ext}`, b = fs.readFileSync(p)
  return { size: b.length, sha256: cr.createHash("sha256").update(b).digest("hex"), url: `${url}/${id}-${ver}.${ext}` } }
const seconds = Math.max(1, Math.round(Number(secs)))
console.log(JSON.stringify({
  video: { id, version: Number(ver), minApp, seconds, files: { webm: file("webm"), mp4: file("mp4") } },
  guide: { id, seconds, hash },
}))
' "$id" "$version" "$secs" "$hash" "$base" "${MIN_APP:-0.30.0}" "${VIDEO_BASE_URL:-https://github.com/IliaSA101/sosninaalbums-library/releases/download/videos-1}"
