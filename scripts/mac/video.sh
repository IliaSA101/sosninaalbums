# shellcheck shell=bash
# Видео для инструкции (029 design §3.5): подключается из shots.sh, video_all — после снимков.
# Три варианта по очереди, у каждого свой прогон сценария; годность — ffprobe и доля движения
# (video_verdict в lib.sh). Выбор лучшего и обрезка — вручную у оркестратора.
# Функции зовутся из shots.sh и через with_timeout по имени
# shellcheck disable=SC2329

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

# Сценарий: отказ Gatekeeper → «Готово» → «Конфиденциальность» → «Открыть всё равно» → Escape →
# «Терминал» с командой → запуск программы. Паузы 2–3 с, чтобы зритель успевал прочитать.
video_scenario() {
  local res
  reset_all
  fresh_download
  sleep 2
  open "$DST" 2>/dev/null
  sleep 4
  res="$(axfind com.apple.coreservices.uiagent find "Done" "Готово" "OK")"
  [ -n "$res" ] && cursor_to "${res%%"$TAB"*}"
  sleep 2
  axfind com.apple.coreservices.uiagent click "Done" "Готово" "OK" >/dev/null
  sleep 1
  killall CoreServicesUIAgent 2>/dev/null
  open_privacy >/dev/null
  res="$(find_open_anyway find)"
  [ -n "$res" ] && cursor_to "${res%%"$TAB"*}"
  sleep 3
  [ -n "$res" ] && axfind "$SETTINGS_ID" click "${OPEN_ANYWAY[@]}" >/dev/null
  sleep 3
  key_code 53
  sleep 1
  osascript -e "quit app id \"$SETTINGS_ID\"" >/dev/null 2>&1
  terminal_xattr >/dev/null
  sleep 4
  killall Terminal 2>/dev/null
  xattr -dr com.apple.quarantine "$DST" 2>/dev/null
  open "$DST" 2>/dev/null
  wait_app "$DST" 15 >/dev/null
  sleep 6
  reset_all
}

# stop_rec <pid> — запись останавливается по SIGINT (ffmpeg и screencapture дописывают файл)
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
  local k="$1" f="$2" desc="$3" info dur fps motion mb d="$WORK/vf$1" res w
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
  motion="$("$PY" "$MAC_DIR/macshots.py" motion "$d" 2>/dev/null)"
  mb=$(($(stat -f %z "$f") / 1048576))
  res="$(video_verdict "${dur:-0}" "${fps:-0}" "$motion" "$mb")"
  w="${res#*"$TAB"}"
  row "видео v$k" "${res%%"$TAB"*}" "$desc: ${dur:-?} с, ${fps:-?} кадр/с, движение ${motion:-?}, $mb МБ${w:+; $w}"
  rm -rf "$d"
}

# 1. Кадры screencapture каждые 0,2 с → mp4 (нужны только screencapture и ffmpeg)
video_frames() {
  local d="$WORK/frames" t0 t1 n fps loop
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
  loop=$!
  t0="$(date +%s)"
  with_timeout 180 video_scenario
  t1="$(date +%s)"
  kill "$loop" 2>/dev/null
  wait "$loop" 2>/dev/null
  n="$(find "$d" -name 'f*.jpg' | wc -l | tr -d ' ')"
  fps="$(fps_of "$n" "$((t1 - t0))")"
  if [ "$n" -lt 2 ]; then
    row "видео v1" "нет" "кадры → mp4: снято $n кадров"
    return 1
  fi
  if [ "$HAVE_FFMPEG" = 1 ]; then
    # Частота входа — фактическая: тогда ролик идёт в реальном времени
    ffmpeg -nostdin -y -loglevel error -framerate "$fps" -i "$d/f%05d.jpg" \
      -vf "scale='min(1920,iw)':-2,format=yuv420p" -c:v libx264 -crf 26 -r 10 "$OUT/video/v1.mp4" \
      2>>"$OUT/logs/ffmpeg-v1.txt"
    video_check 1 "$OUT/video/v1.mp4" "кадры → mp4 ($n кадров)" "$fps"
  else
    ditto -c -k "$d" "$OUT/video/v1-frames.zip"
    row "видео v1" "нет" "ffmpeg нет — $n кадров ($fps кадр/с) в video/v1-frames.zip"
  fi
  rm -rf "$d"
}

# 2. ffmpeg avfoundation: настоящее видео с курсором и щелчками
video_avf() {
  local idx fp
  idx="$(ffmpeg -hide_banner -f avfoundation -list_devices true -i "" 2>&1 | tee "$OUT/logs/avf-devices.txt" | avf_screen_index)"
  if [ -z "$idx" ]; then
    row "видео v2" "нет" "avfoundation: нет «Capture screen 0» (logs/avf-devices.txt)"
    return 1
  fi
  ffmpeg -nostdin -y -loglevel warning -f avfoundation -capture_cursor 1 -capture_mouse_clicks 1 \
    -framerate 15 -i "$idx:none" -t 240 -vf "scale='min(1920,iw)':-2" -c:v h264_videotoolbox -b:v 4M \
    -pix_fmt yuv420p "$OUT/video/v2.mp4" 2>"$OUT/logs/ffmpeg-v2.txt" &
  fp=$!
  sleep 3
  if ! alive "$fp"; then
    wait "$fp" 2>/dev/null
    row "видео v2" "нет" "ffmpeg avfoundation вышел сразу (logs/ffmpeg-v2.txt)"
    return 1
  fi
  with_timeout 180 video_scenario
  stop_rec "$fp"
  video_check 2 "$OUT/video/v2.mp4" "ffmpeg avfoundation"
}

# 3. screencapture -v: на 15+ может спросить выбор окна — у агента раннера это отключено (replayd)
video_sc() {
  local sp
  screencapture -v -V 240 -k -x "$OUT/video/v3.mov" </dev/null >/dev/null 2>"$OUT/logs/screencapture-v3.txt" &
  sp=$!
  sleep 3
  if ! alive "$sp"; then
    wait "$sp" 2>/dev/null
    row "видео v3" "нет" "screencapture -v вышел сразу (logs/screencapture-v3.txt)"
    return 1
  fi
  with_timeout 180 video_scenario
  stop_rec "$sp"
  if [ "$HAVE_FFMPEG" = 1 ]; then
    video_check 3 "$OUT/video/v3.mov" "screencapture -v"
  else
    row "видео v3" "предупреждение" "файл есть, без ffprobe не проверить"
  fi
}

video_all() {
  HAVE_FFMPEG=0
  if ensure_ffmpeg; then
    HAVE_FFMPEG=1
  else
    row "ffmpeg" "нет" "не поставился (logs/brew.txt): v1 — zip кадров, v2 нет"
  fi
  video_frames
  [ "$HAVE_FFMPEG" = 1 ] && video_avf
  video_sc
  return 0
}
