# shellcheck shell=bash
# Видео для инструкции (029 design §3.5): подключается из shots.sh. Запись идёт параллельно со всеми
# снимками (video_start до 01, video_stop после 08/11): в прогоне 1 отдельный сценарий висел на open,
# и ролики вышли застывшими. Основной вариант — v2 (ffmpeg avfoundation), запасной — v1 (кадры
# screencapture). Прогон 3: v2 писал живой экран (242 с, 14,7 кадр/с, в кадре курсор и «Настройки»), а
# вердикт «застыл» дала доля движения 0,099 — шаги в основном ждут. Прогон 4: screencapture -v (v3) и
# короткие пробы после выдачи прав записи экрана — screenrec.sh; из v2 вырезаются простои (v2-cut).
# Годность — ffprobe и движение (video_verdict в lib.sh). Обрезка до 20 с — вручную у оркестратора.
# Функции зовутся из shots.sh по имени
# shellcheck disable=SC2329

REC_MODE=""
REC_PID=""
REC_DESC=""
REC_WHY=""
REC_T0=0
HAVE_FFMPEG=0

ensure_ffmpeg() {
  command -v ffmpeg >/dev/null 2>&1 && return 0
  with_timeout 420 brew install ffmpeg >>"$OUT/logs/brew.txt" 2>&1
  command -v ffmpeg >/dev/null 2>&1
}

# cursor_to «x y w h» — курсор плавно к центру рамки: на видео видно, куда нажимать
cursor_to() {
  command -v cliclick >/dev/null 2>&1 || return 0
  # shellcheck disable=SC2086 # «x y w h» → четыре числа
  set -- $1
  [ "$#" -eq 4 ] || return 0
  cliclick -e 400 "m:$(($1 + $3 / 2)),$(($2 + $4 / 2))" 2>/dev/null
}

# stop_rec <pid> — запись останавливается по SIGINT (ffmpeg дописывает файл)
stop_rec() {
  local i
  kill -INT "$1" 2>/dev/null
  for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    alive "$1" || break
    sleep 1
  done
  alive "$1" && kill -TERM "$1" 2>/dev/null
  wait "$1" 2>/dev/null
}

# video_check <k> <файл> <описание> [<кадр/с съёмки>] — ffprobe, кадр из середины, доля движения
video_check() {
  local k="$1" f="$2" desc="$3" info dur fps motion mcount mb d="$WORK/vf$1" res w
  if [ ! -s "$f" ]; then
    row "видео v$k" "нет" "$desc: файла нет"
    return 1
  fi
  info="$(ffprobe -v error -select_streams v:0 -show_entries format=duration:stream=avg_frame_rate \
    -of default=nw=1 "$f" 2>>"$OUT/logs/ffprobe.txt")"
  dur="$(printf '%s\n' "$info" | awk -F= '/^duration=/ { print int($2); exit }')"
  fps="$(printf '%s\n' "$info" | awk -F= '/^avg_frame_rate=/ { split($2, a, "/"); if (a[2] + 0 > 0) printf "%.1f", a[1] / a[2]; else print 0; exit }')"
  [ -n "${4:-}" ] && fps="$4"
  ffmpeg -nostdin -y -loglevel error -ss "$((${dur:-0} / 2))" -i "$f" -frames:v 1 "$OUT/video/v$k-mid.png" 2>>"$OUT/logs/ffmpeg-mid.txt"
  rm -rf "$d"
  mkdir -p "$d"
  ffmpeg -nostdin -loglevel error -i "$f" -vf "fps=1,scale=480:-2" "$d/%04d.png" 2>>"$OUT/logs/ffmpeg-motion.txt"
  motion="$("$PY" "$MAC_DIR/macshots.py" motion "$d" --with-count 2>/dev/null)"
  mcount="${motion#* }"
  motion="${motion%% *}"
  mb=$(($(stat -f %z "$f") / 1048576))
  res="$(video_verdict "${dur:-0}" "${fps:-0}" "$motion" "$mb" "$mcount" "${VIDEO_MIN:-20}")"
  w="${res#*"$TAB"}"
  row "видео v$k" "${res%%"$TAB"*}" "$desc: ${dur:-?} с, ${fps:-?} кадр/с, движение ${motion:-?} (смен кадра ${mcount:-?}), $mb МБ${w:+; $w}"
  rm -rf "$d"
}

# avf_start [файл [hq]] — v2: ffmpeg avfoundation с курсором и щелчками. Кодер videotoolbox с -allow_sw
# (на 14 аппаратный не открылся: -12903), не поднялся — libx264. hq — сырая запись для монтажа (036): стоп-кадры
# берутся из неё, поэтому битрейт выше; без аргументов — v2.mp4
avf_start() {
  local idx enc f="${1:-$OUT/video/v2.mp4}" vt=4M crf=28 log
  local -a codec
  log="$OUT/logs/ffmpeg-$(basename "$f" .mp4).txt"
  [ "${2:-}" = hq ] && vt=6M && crf=20
  idx="$(ffmpeg -hide_banner -f avfoundation -list_devices true -i "" 2>&1 | tee "$OUT/logs/avf-devices.txt" | avf_screen_index)"
  if [ -z "$idx" ]; then
    REC_WHY="avfoundation: нет «Capture screen 0» (logs/avf-devices.txt)"
    return 1
  fi
  for enc in videotoolbox libx264; do
    if [ "$enc" = videotoolbox ]; then
      codec=(-c:v h264_videotoolbox -allow_sw 1 -b:v "$vt")
    else
      codec=(-c:v libx264 -preset ultrafast -crf "$crf")
    fi
    echo "== $enc" >>"$log"
    # -t — страховка: запись останавливает video_stop
    ffmpeg -nostdin -y -loglevel warning -f avfoundation -capture_cursor 1 -capture_mouse_clicks 1 \
      -framerate 15 -i "$idx:none" -t 1200 -vf "scale='min(1920,iw)':-2" "${codec[@]}" \
      -pix_fmt yuv420p "$f" 2>>"$log" &
    REC_PID=$!
    sleep 3
    if alive "$REC_PID"; then
      REC_DESC="ffmpeg avfoundation, $enc"
      return 0
    fi
    wait "$REC_PID" 2>/dev/null
  done
  REC_WHY="ffmpeg avfoundation вышел сразу и с videotoolbox, и с libx264 (${log#"$OUT/"})"
  return 1
}

# frames_start — v1: кадры screencapture каждые 0,2 с (нужен только screencapture)
frames_start() {
  local d="$WORK/frames"
  rm -rf "$d"
  mkdir -p "$d"
  (
    i=0
    while :; do
      screencapture -x -C -t jpg "$d/f$(printf '%05d' "$i").jpg" 2>/dev/null
      i=$((i + 1))
      sleep 0.2
    done
  ) </dev/null >/dev/null 2>&1 &
  REC_PID=$!
  REC_T0="$(date +%s)"
}

# frames_stop — кадры → mp4 в реальном времени (или zip кадров без ffmpeg)
frames_stop() {
  local d="$WORK/frames" t1 n fps
  t1="$(date +%s)"
  kill "$REC_PID" 2>/dev/null
  wait "$REC_PID" 2>/dev/null
  n="$(find "$d" -name 'f*.jpg' | wc -l | tr -d ' ')"
  fps="$(fps_of "$n" "$((t1 - REC_T0))")"
  if [ "$n" -lt 2 ]; then
    row "видео v1" "нет" "кадры → mp4: снято $n кадров"
    return 1
  fi
  if [ "$HAVE_FFMPEG" = 1 ]; then
    # Частота входа — фактическая: тогда ролик идёт в реальном времени
    ffmpeg -nostdin -y -loglevel error -framerate "$fps" -i "$d/f%05d.jpg" \
      -vf "scale='min(1920,iw)':-2,format=yuv420p" -c:v libx264 -crf 26 -r 10 "$OUT/video/v1.mp4" \
      2>>"$OUT/logs/ffmpeg-v1.txt"
    video_check 1 "$OUT/video/v1.mp4" "кадры → mp4 ($n кадров), параллельно со снимками" "$fps"
  else
    ditto -c -k "$d" "$OUT/video/v1-frames.zip"
    row "видео v1" "нет" "ffmpeg нет — $n кадров ($fps кадр/с) в video/v1-frames.zip"
  fi
  rm -rf "$d"
}

# video_start — запись в фоне основного процесса shots.sh (не в подоболочке шага: pid нужен video_stop)
video_start() {
  if ensure_ffmpeg; then
    HAVE_FFMPEG=1
  else
    row "ffmpeg" "нет" "не поставился (logs/brew.txt): v2 нет, v1 — zip кадров"
  fi
  if [ "$HAVE_FFMPEG" = 1 ] && avf_start; then
    REC_MODE=2
    note "видео: пишется v2 ($REC_DESC)"
    return 0
  fi
  [ -n "$REC_WHY" ] && row "видео v2" "нет" "$REC_WHY — пишется запасной v1"
  frames_start
  REC_MODE=1
  note "видео: пишется v1 (кадры screencapture)"
}

# video_cut <файл> <метка> — v<метка>-cut.mp4: кадры без изменений (ожидание шагов) выброшены, остаётся
# ход действий; кадр/с на выходе 15. mpdecimate роняет кадр, у которого почти все блоки 8×8 как в прошлом,
# курсор один блок не сдвинет, а окно или меню — сдвинет
video_cut() {
  local out="$OUT/video/v$2-cut.mp4"
  [ -s "$1" ] && command -v ffmpeg >/dev/null 2>&1 || return 0
  with_timeout 240 ffmpeg -nostdin -y -loglevel error -i "$1" \
    -vf "mpdecimate=hi=768:lo=320:frac=0.002,setpts=N/15/TB" -r 15 \
    -c:v libx264 -preset veryfast -crf 26 -pix_fmt yuv420p "$out" 2>>"$OUT/logs/ffmpeg-cut.txt"
  video_check "$2-cut" "$out" "v$2 без простоев (mpdecimate)" 15
}

video_stop() {
  case "$REC_MODE" in
    2)
      stop_rec "$REC_PID"
      video_check 2 "$OUT/video/v2.mp4" "$REC_DESC, параллельно со снимками"
      video_cut "$OUT/video/v2.mp4" 2
      ;;
    1) frames_stop ;;
  esac
  REC_MODE=""
  return 0
}
