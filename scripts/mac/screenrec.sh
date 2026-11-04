# shellcheck shell=bash
# Права «Запись экрана» (TCC) и пробы видео (029, прогон 4). Подключается из shots.sh после video.sh.
# Прогоны 1–3: `screencapture -v` файла не давал. Гипотеза: у процесса без интерфейса нет права «Запись экрана».
# Пробы независимы, каждая пишет строку в summary.md (раздел «Видео: пробы») и подробности в logs/video-*.txt,
# ни одна не валит задание: csrutil → родители процесса → TCC до → screencapture -v до прав → выдача прав
# → TCC после → screencapture -v после прав → ffmpeg avfoundation. Читать итог: строки «видео v3», «видео v4»,
# «видео v5» (да/нет, движение, смен кадра), «csrutil», «родители», «TCC: …».
# Функции зовутся из shots.sh по имени
# shellcheck disable=SC2329,SC2024

TCC_SYS="/Library/Application Support/com.apple.TCC/TCC.db"
TCC_USER="$HOME/Library/Application Support/com.apple.TCC/TCC.db"

# proc_chain — предки этого shell вверх до launchd: pid, ppid, пользователь, путь исполняемого
proc_chain() {
  local p="$$" i=0
  while [ -n "$p" ] && [ "$p" -gt 1 ] && [ "$i" -lt 20 ]; do
    ps -o pid=,ppid=,user=,comm= -p "$p"
    p="$(ps -o ppid= -p "$p" | tr -d ' ')"
    i=$((i + 1))
  done
}

# chain_clients — пути исполняемых предков (запись экрана числится за «ответственным» процессом, не за bash)
chain_clients() {
  proc_chain | awk '{ $1 = $2 = $3 = ""; sub(/^ +/, ""); if ($0 ~ /^\//) print }' | sort -u
}

# tcc_dump <метка> — строки о записи экрана и событиях в обеих базах (в logs/video-tcc.txt)
tcc_dump() {
  local db
  for db in "$TCC_SYS" "$TCC_USER"; do
    {
      echo "== $1: $db"
      sudo -n sqlite3 -header "$db" "SELECT service, client, client_type, auth_value, auth_reason FROM access WHERE service IN ('kTCCServiceScreenCapture', 'kTCCServiceListenEvent', 'kTCCServicePostEvent', 'kTCCServiceAccessibility');" 2>&1
    } >>"$OUT/logs/video-tcc.txt"
  done
}

# tcc_grant <клиент> <0 bundle id | 1 путь> — право записи экрана в обе базы; 0, если хоть одна запись прошла
tcc_grant() {
  local ts db n v c ok=1
  ts="$(date +%s)"
  c="$(printf '%s' "$1" | sed "s/'/''/g")"
  for db in "$TCC_SYS" "$TCC_USER"; do
    n="$(sudo -n sqlite3 "$db" "SELECT COUNT(*) FROM pragma_table_info('access');" 2>>"$OUT/logs/tcc.txt")"
    if ! v="$(tcc_values "$n" kTCCServiceScreenCapture "$c" "$2" "$ts")"; then
      echo "схема $db: столбцов '$n'" >>"$OUT/logs/tcc.txt"
      continue
    fi
    sudo -n sqlite3 "$db" "INSERT OR REPLACE INTO access VALUES($v);" >>"$OUT/logs/tcc.txt" 2>&1 && ok=0
  done
  return "$ok"
}

# tcc_fix — права записи экрана клиентам: снимки, osascript, оболочки, ffmpeg и все исполняемые предки
# (на раннере это провайдер вроде sshd-keygen-wrapper / агента; кто именно — в logs/video-chain.txt)
tcc_fix() {
  local ff c ok=1
  ff="$(command -v ffmpeg 2>/dev/null)"
  [ -n "$ff" ] && ff="$("$PY" -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$ff")"
  while IFS= read -r c; do
    [ -n "$c" ] && tcc_grant "$c" 1 && ok=0
  done <<EOF
/usr/sbin/screencapture
/usr/bin/osascript
/bin/bash
/bin/sh
/bin/zsh
/usr/libexec/sshd-keygen-wrapper
$ff
$(chain_clients)
EOF
  tcc_grant com.apple.Terminal 0 && ok=0
  # tccd держит решения в памяти: после правки базы его перезапускают
  sudo -n killall tccd >>"$OUT/logs/tcc.txt" 2>&1
  killall tccd >>"$OUT/logs/tcc.txt" 2>&1
  return "$ok"
}

# motion_gen <сек> — заметное движение на экране для проб (курсор мал: его доля пикселей ниже порога
# движения): окно «Калькулятора» то открывается, то закрывается; плюс курсор, если есть cliclick
motion_gen() {
  local end i=0
  end=$(($(date +%s) + $1))
  while [ "$(date +%s)" -lt "$end" ]; do
    open -a Calculator 2>/dev/null
    command -v cliclick >/dev/null 2>&1 && cliclick -e 40 "m:$((200 + (i % 6) * 120)),$((300 + (i % 4) * 90))" 2>/dev/null
    sleep 1.5
    killall Calculator 2>/dev/null
    sleep 1
    i=$((i + 1))
  done
  killall Calculator 2>/dev/null
  return 0
}

# probe_sc_video <метка> <сек> <описание> — screencapture -v -V <сек> + движение; файл v<метка>.mov
probe_sc_video() {
  local k="$1" secs="$2" f="$OUT/video/v$1.mov" pid
  rm -f "$f"
  note "проба v$k: screencapture -v -V $secs"
  screencapture -v -V "$secs" -x "$f" </dev/null >>"$OUT/logs/video-screencapture.txt" 2>&1 &
  pid=$!
  motion_gen "$secs"
  # -V сам останавливает запись; если нет — останавливаем (SIGINT дописывает файл)
  sleep 2
  alive "$pid" && stop_rec "$pid"
  wait "$pid" 2>/dev/null
  if [ ! -s "$f" ]; then
    row "видео v$k" "нет" "$3: screencapture -v файла не дал (logs/video-screencapture.txt)"
    return 1
  fi
  VIDEO_MIN=5 video_check "$k" "$f" "$3: screencapture -v -V $secs"
}

# probe_avf_video <метка> <сек> <описание> — короткая запись ffmpeg avfoundation с движением
probe_avf_video() {
  local k="$1" secs="$2" f="$OUT/video/v$1.mp4" idx pid
  rm -f "$f"
  idx="$(ffmpeg -hide_banner -f avfoundation -list_devices true -i "" 2>&1 | avf_screen_index)"
  if [ -z "$idx" ]; then
    row "видео v$k" "нет" "$3: avfoundation не показал «Capture screen 0»"
    return 1
  fi
  ffmpeg -nostdin -y -loglevel warning -f avfoundation -capture_cursor 1 -framerate 15 -i "$idx:none" \
    -t "$((secs + 2))" -c:v libx264 -preset ultrafast -crf 28 -pix_fmt yuv420p "$f" \
    2>>"$OUT/logs/video-avf.txt" &
  pid=$!
  motion_gen "$secs"
  sleep 2
  alive "$pid" && stop_rec "$pid"
  wait "$pid" 2>/dev/null
  VIDEO_MIN=5 video_check "$k" "$f" "$3: ffmpeg avfoundation, ${secs} с с движением"
}

# video_probe — все пробы по порядку; зовётся до первого снимка при --video
video_probe() {
  local line
  section "Видео: пробы прав записи экрана (screenrec.sh)"
  line="$(csrutil status 2>&1 | tr '\n' ' ')"
  row "csrutil" "да" "${line:-нет вывода} (SIP включён — TCC.db пишется только через sudo; для системной базы это отдельный запрет)"
  proc_chain >"$OUT/logs/video-chain.txt" 2>&1
  row "родители" "да" "$(chain_clients | tr '\n' ',' | cut -c1-300) (полностью — logs/video-chain.txt)"
  tcc_dump "до"
  ensure_ffmpeg && HAVE_FFMPEG=1
  with_timeout 90 probe_sc_video 3 8 "до выдачи прав"
  if tcc_fix; then
    row "TCC: запись экрана" "да" "права вписаны клиентам и предкам (logs/tcc.txt)"
  else
    row "TCC: запись экрана" "нет" "sqlite3 не записал ни в одну базу (logs/tcc.txt): SIP/нет sudo"
  fi
  tcc_dump "после"
  sleep 2
  with_timeout 120 probe_sc_video 4 20 "после выдачи прав"
  [ "$HAVE_FFMPEG" = 1 ] && with_timeout 90 probe_avf_video 5 10 "после выдачи прав"
  # Итог: какая проба дала живую запись — её и обрезают; иначе ролик собирает place_run.py из снимков
  return 0
}
