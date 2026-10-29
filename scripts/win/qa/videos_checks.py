"""Вердикты проверки роликов справочника F1 в собранной программе (спека 140, Т7): чистые функции над записями зонда
videos_probe.js, над журналом локального сервера каталога и над папкой кэша роликов. Без запуска программы;
тесты - test_videos_exe.py. Файл в UTF-8."""

import hashlib
import json
import os
import re

KIND = {"win32": "webm", "darwin": "mp4"}  # какой вид берёт Go (guidevideo pick: darwin - mp4, остальные - webm)
FRAME_W = 1280  # ширина кадра тестовых роликов (library-testvideos.mjs: testsrc 1280x720)
MIN_GROWTH = 1.2  # секунд времени в кадре за три секунды наблюдения (4 отсчёта через 1 с)
BAD_LOG = re.compile(r"panic|паник|goroutine \d+|fatal", re.I)
LEAK_LOG = re.compile(r"https?://|[A-Za-z]:\\|\\Users\\|/Users/", re.I)


def step_of(recs, name):
    """Последняя запись шага или None."""
    for r in reversed(recs):
        if r.get("step") == name:
            return r
    return None


def _need(recs, name):
    r = step_of(recs, name)
    if not r:
        return None, (False, "нет записи %s (зонд не дошёл до шага)" % name)
    if not r.get("ok"):
        return None, (False, "шаг %s: %s; в окне: %s" % (name, r.get("err", "?"), r.get("screen", "")[:200]))
    return r, None


def check_ready(run):
    r = step_of(run["records"], "ready")
    if not r or not r.get("ok"):
        return False, "окно не отозвалось: %s; записей %d; программа жива: %s, код %s" % (
            (r or {}).get("err", "нет записи ready"), len(run["records"]), run["alive"], run["code"])
    return True, "связка Go и событий в окне за %s мс (%s)" % (r.get("ms"), r.get("ua", ""))


def check_update(recs):
    for name in ("consent", "update"):
        r, bad = _need(recs, name)
        if bad:
            return bad
    return True, "согласие выдано, список с локального сервера получен"


def check_list_before(recs, ids):
    r, bad = _need(recs, "list-before")
    if bad:
        return bad
    got = {i["id"]: i for i in r["items"]}
    if sorted(got) != sorted(ids):
        return False, "в списке роликов %s, ждали %s" % (sorted(got), sorted(ids))
    ready = [i for i in got if got[i]["ready"]]
    return (not ready), "до скачивания готовых роликов нет" if not ready else "готовы до скачивания: %s" % ready


def check_download(recs, ids):
    r, bad = _need(recs, "download")
    if bad:
        return bad
    got = {i["id"]: i for i in r["items"]}
    miss = [i for i in ids if not (got.get(i) or {}).get("ready")]
    after, bad2 = _need(recs, "list-after")
    if bad2:
        return bad2
    again = {i["id"]: i for i in after["items"]}
    miss += [i for i in ids if not (again.get(i) or {}).get("ready") and i not in miss]
    if miss:
        return False, "не готовы после скачивания: %s" % miss
    return True, "скачано %d за %s мс, размеры: %s" % (len(ids), r.get("ms"), {i: got[i]["size"] for i in ids})


def sizes_of(recs):
    """{id: size} из списка после скачивания."""
    r = step_of(recs, "list-after") or {}
    return {i["id"]: i["size"] for i in r.get("items", [])}


def judge_playing(r, kind, size=None):
    """Запись наблюдения за <video> (playBlob или окно): (ok, подробности) - все условия сразу, провал называет каждое."""
    why = []
    if "status" in r and r["status"] != 200:
        why.append("ответ раздачи %s" % r["status"])
    if r.get("ct", "video/" + kind) != "video/" + kind:
        why.append("Content-Type %r, ждали video/%s" % (r.get("ct"), kind))
    if size is not None and r.get("bytes", size) != size:
        why.append("получено %s байт, в каталоге %s" % (r.get("bytes"), size))
    if r.get("error"):
        why.append("video.error %s" % json.dumps(r["error"], ensure_ascii=False))
    if r.get("readyState", 0) < 2:
        why.append("readyState %s (ждали >= 2)" % r.get("readyState"))
    times = r.get("times") or []
    grow = (times[-1] - times[0]) if len(times) >= 2 else 0
    if grow < MIN_GROWTH:
        why.append("время в кадре не идёт: %s" % times)
    if r.get("w") != FRAME_W or not r.get("h"):
        why.append("кадр %sx%s, ждали %dx..." % (r.get("w"), r.get("h"), FRAME_W))
    if r.get("playErr"):
        why.append("play(): %s" % r["playErr"])
    if why:
        return False, "; ".join(why)
    return True, "readyState %s, кадр %sx%s, %s с, время в кадре %s, событий %d" % (
        r.get("readyState"), r.get("w"), r.get("h"), r.get("duration"), times, len(r.get("events", [])))


def check_play(recs, id_, kind, size=None, name=None):
    """Игра ролика путём окна; name - имя шага (по умолчанию play:<id>)."""
    r, bad = _need(recs, name or "play:" + id_)
    return bad or judge_playing(r, kind, size)


def check_ui_play(recs, kind, name="ui-play"):
    r, bad = _need(recs, name)
    if bad:
        return bad
    ok, d = judge_playing(r, kind)
    if ok and not r.get("inDialog"):
        return False, "<video> есть, но не внутри окна ролика (Modal)"
    return ok, d


def check_corrupt_blob(recs, kind):
    """Испорченный файл при том же размере: Go его отдал, а окно играть не должно - иначе сбой не попал бы в onfail."""
    r, bad = _need(recs, "play-corrupt")
    if bad:
        return bad
    if r.get("status") != 200:
        return False, "раздача ответила %s вместо 200 (файл испорчен при том же размере, Go сверяет только размер)" % r.get("status")
    broken = bool(r.get("error")) or r.get("readyState", 0) < 2 or (r.get("w") or 0) == 0
    if not broken:
        return False, "испорченный ролик сыграл: readyState %s, кадр %sx%s, error нет" % (r.get("readyState"), r.get("w"), r.get("h"))
    return True, "не открылся: error %s, readyState %s, кадр %sx%s" % (json.dumps(r.get("error")), r.get("readyState"), r.get("w"), r.get("h"))


def check_ui_broken(recs):
    r, bad = _need(recs, "ui-broken")
    if bad:
        return bad
    why = []
    if not r.get("broken"):
        why.append("нет строки «Ролик не открылся в окне программы»")
    if not r.get("reveal"):
        why.append("нет «Показать файл»")
    if not r.get("redo"):
        why.append("нет «Скачать заново»")
    if r.get("watch"):
        why.append("«Смотреть» осталась в строке (0.30-140-28: её нет, пока справочник не открыт заново)")
    if not r.get("windowGone"):
        why.append("окно ролика не закрылось")
    return (not why), "; ".join(why) or "строка «Ролик не открылся…», «Показать файл», «Скачать заново», окно закрылось"


def check_redo(recs, victim, size):
    r, bad = _need(recs, "ui-redo")
    if bad:
        return bad
    got = {i["id"]: i for i in r.get("items", [])}
    it = got.get(victim) or {}
    why = []
    if not r.get("back"):
        why.append("строка не вернулась к «Смотреть»")
    if not it.get("ready"):
        why.append("файл не готов после замены")
    if it.get("size") != size:
        why.append("размер %s, ждали %s" % (it.get("size"), size))
    return (not why), "; ".join(why) or "«Скачать заново» заменила файл, строка вернулась к «Смотреть»"


# ---------- журнал локального сервера и папка кэша ----------

def check_requests(reqs, ids, kind):
    """reqs - пути GET, дошедшие до сервера. Видов не путаем: mac берёт только mp4, остальные только webm."""
    other = "mp4" if kind == "webm" else "webm"
    wrong = [p for p in reqs if p.endswith("." + other)]
    miss = [i for i in ids if not any(re.search(r"/%s-\d+\.%s$" % (re.escape(i), kind), p) for p in reqs)]
    cat = [p for p in reqs if p.endswith("catalog.json")]
    why = (["запрошен чужой вид: %s" % wrong[:3]] if wrong else []) + (["ролики не запрашивались: %s" % miss] if miss else []) + (
        [] if cat else ["каталог не запрашивался"])
    return (not why), "; ".join(why) or "каталог и ролики вида %s, чужого вида запросов нет (всего GET: %d)" % (kind, len(reqs))


def file_sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def check_cache(cache_dir, site_dir, ids, kind):
    """В кэше ровно файлы выбранного вида, байты равны файлам раздачи, остатков gv-*.tmp нет."""
    try:
        names = sorted(os.listdir(cache_dir))
    except OSError as e:
        return False, "папка кэша роликов не читается: %s" % e
    why = []
    tmp = [n for n in names if n.startswith("gv-") or n.endswith(".tmp")]
    if tmp:
        why.append("остатки скачивания: %s" % tmp)
    for i in ids:
        mine = [n for n in names if re.fullmatch(re.escape(i) + r"-\d+\." + kind, n)]
        if len(mine) != 1:
            why.append("%s: файлов вида %s %d (%s)" % (i, kind, len(mine), names))
            continue
        site = os.path.join(site_dir, mine[0])
        if not os.path.isfile(site):
            why.append("%s: в раздаче нет %s" % (i, mine[0]))
        elif file_sha(os.path.join(cache_dir, mine[0])) != file_sha(site):
            why.append("%s: байты в кэше не равны раздаче" % i)
    other = [n for n in names if n.endswith(".mp4" if kind == "webm" else ".webm")]
    if other:
        why.append("файлы чужого вида: %s" % other)
    return (not why), "; ".join(why) or "в кэше %s, байты равны раздаче, остатков нет" % names


def cache_file(cache_dir, id_, kind):
    """Путь файла ролика в кэше или ''."""
    try:
        for n in os.listdir(cache_dir):
            if re.fullmatch(re.escape(id_) + r"-\d+\." + kind, n):
                return os.path.join(cache_dir, n)
    except OSError:
        pass
    return ""


def corrupt_file(path, n=256):
    """Портит начало файла при том же размере (Go сверяет размер, не sha256): первые n байт инвертируются."""
    with open(path, "r+b") as f:
        head = f.read(n)
        f.seek(0)
        f.write(bytes(b ^ 0xFF for b in head))
    return len(head)


def check_log(text):
    """log.txt стенда: ни паник, ни адресов и путей в строках про ролики."""
    bad, leak = [], []
    for ln in text.splitlines():
        if BAD_LOG.search(ln):
            bad.append(ln.strip()[:160])
        if "ролик" in ln.lower() and LEAK_LOG.search(ln):
            leak.append(ln.strip()[:160])
    why = (["паники: %s" % bad[:3]] if bad else []) + (["адрес или путь в строке про ролик: %s" % leak[:3]] if leak else [])
    return (not why), "; ".join(why) or "паник нет, в строках про ролики адресов и путей нет (строк: %d)" % len(text.splitlines())
