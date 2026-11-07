"""Вердикты win-qa «образцы наборов в exe» (спека 0.31/144, решение 0.31-144-59): чистые функции над записями зонда
library_probe.js и над состоянием папки стенда. Без Windows и без запуска exe; тесты - test_library_exe.py.
Файл в UTF-8."""

import base64
import hashlib
import json
import os
import re

VERSION = "0.25.1"  # qa-exe из набора: наборы библиотеки с minApp выше (0.26+) в нём серые и плиток не дают
PASSES = 40  # проходов по плиткам в одном круге листания
ROUND_MAX_MS = 90000

# Пределы памяти (МиБ). Частная память процессов: exe (Go) и всё дерево с msedgewebview2. «Не растёт без предела» -
# второй круг листания не добавляет к первому больше PLATEAU; рост за оба круга от начала - не больше GROW.
MIB = 1 << 20
PLATEAU_OWN = 100 * MIB
PLATEAU_TREE = 250 * MIB
GROW_OWN = 300 * MIB

# Варианты адреса после /media/lp/ ({A} - набор, чей лист на диске): всё, кроме годных, должно быть 404.
# Браузер схлопывает «..» и «%2e%2e» сам - такие варианты на службу не доходят, это видно по resolved в наблюдении.
PATHS = [
    "{A}/bg/99", "{A}/bg/01", "{A}/bg/-1", "{A}/bg/+1", "{A}/Bg/0", "{A}/bg/0/1", "nope-zz/bg/0", "CON/bg/0",
    "%2e%2e/bg/0", "%2E%2E/bg/0", "..", "../bg/0", "{A}/%2e%2e/bg/0", "%2e%2e%2fbg%2f0", "..%2Fbg%2F0",
    "{A}%2F..%2F..%2Fbg%2F0", "{A}%2Fbg%2F0%2F1", "%252e%252e/bg/0", "{A}/bg/0%00", "{A}/bg/0%20",
    "{A}%5Cbg%5C0", "%5C..%5Cbg%5C0", "{A}/bg", "{A}", "", "x", "{A}/sticker/9999", "{A}/bg/0/", "{A}//bg/0",
    "{A}/bg/0..", "t/x", "nul/bg/0", "{A}/bg/%2D1", "{A}/index.html", "index.html", "{A}/bg/index.html",
]




def file_sha(path):
    """sha256 файла по частям."""
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()

# ---------- записи зонда ----------

def step_of(recs, name):
    """Последняя запись шага или None."""
    for r in reversed(recs):
        if r.get("step") == name:
            return r
    return None


def steps_of(recs, name):
    return [r for r in recs if r.get("step") == name]


# ---------- вердикты: чистые функции над записями ----------

def check_ready(run):
    r = step_of(run["records"], "ready")
    if not r or not r.get("ok"):
        return False, "окно не отозвалось: %s; записей %d; exe жив: %s, код %s" % (
            (r or {}).get("err", "нет записи ready"), len(run["records"]), run["alive"], run["code"])
    return True, "связка Go и событий в окне за %s мс, запись зонда дошла до файла" % r.get("ms")


def check_update(recs):
    r = step_of(recs, "update")
    if not r or not r.get("ok"):
        return False, "UpdateLibrary: %s" % (r or {}).get("err", "нет записи")
    return True, "список с GitHub получен (попытка %s): можно скачать %s, не скачались %s" % (r.get("try"), r.get("available"), r.get("failed"))


def check_tiles(recs):
    t0, t1 = step_of(recs, "tiles0"), step_of(recs, "tiles1")
    if not t1 or not t1.get("ok"):
        return False, "DecorCatalog после образцов: %s" % (t1 or {}).get("err", "нет записи")
    c1 = t1.get("counts", {})
    why = []
    if not c1.get("bg"):
        why.append("плиток «Фонов» нет (набор, подходящий версии %s, с листом и без «удалён вами» должен быть)" % VERSION)
    if t0 and any(t0.get("counts", {}).values()):
        why.append("плитки были до задания образцов: %s" % t0.get("counts"))
    return not why, "; ".join(why) or "плитки до задания образцов: %s; после: %s; наборов с фоном: %s" % (
        (t0 or {}).get("counts"), c1, len(t1.get("packs", [])))


def check_overlap(recs):
    r = step_of(recs, "overlap")
    if not r or not r.get("ok"):
        return False, "шаг не выполнен: %s" % (r or {}).get("err", "нет записи")
    why = []
    if r.get("derr"):
        why.append("«Скачать» вернул ошибку: %s" % r["derr"])
    if not r.get("picked"):
        why.append("набор не встал: %s" % (r.get("why") or "причина пуста"))
    if r.get("previewsTimeout"):
        why.append("событие library:previews не пришло")
    kb = (r.get("bytes") or 0) // 1024
    late = "да" if r.get("overlapped") else "нет (образцы кончились раньше начала «Скачать»)"
    return not why, "; ".join(why) or "набор «%s» (%d КБ) скачан за %s мс; листы: событие через %s мс, листов %s; перекрылись с задачей образцов: %s" % (
        r.get("pack"), kb, r.get("downloadMs"), r.get("previewsMs"), r.get("previewsReady"), late)


def _jpeg(p):
    return p.get("status") == 200 and p.get("ct") == "image/jpeg" and p.get("nosniff") == "nosniff" and p.get("cc") == "no-cache" and p.get("len", 0) > 0


def check_good(recs):
    r = step_of(recs, "good")
    if not r or not r.get("ok"):
        return False, "шаг не выполнен: %s" % (r or {}).get("err", "нет записи")
    d, e, h, p, g = (r.get(k, {}) for k in ("direct", "encoded", "head", "post", "range"))
    why = []
    if not _jpeg(d):
        why.append("годная клетка: статус %s, тип %s, nosniff %r, cache-control %r, %s Б" % (d.get("status"), d.get("ct"), d.get("nosniff"), d.get("cc"), d.get("len")))
    if e.get("status") != 200 or e.get("fnv") != d.get("fnv") or e.get("len") != d.get("len"):
        why.append("%%2F в пути: статус %s, тот же лист: %s" % (e.get("status"), e.get("fnv") == d.get("fnv")))
    if h.get("status") != 200 or h.get("len") != 0:
        why.append("HEAD: статус %s, тело %s Б" % (h.get("status"), h.get("len")))
    if p.get("status") != 405 or p.get("allow") != "GET, HEAD":
        why.append("POST: статус %s, Allow %r" % (p.get("status"), p.get("allow")))
    if g.get("status") != 200 or g.get("len") != d.get("len"):
        why.append("Range и If-None-Match: статус %s, %s Б вместо %s" % (g.get("status"), g.get("len"), d.get("len")))
    return not why, "; ".join(why) or "%s: 200 image/jpeg %d Б; %%2F - тот же лист; HEAD 200 без тела; POST 405 Allow GET, HEAD; Range - тело целиком" % (r.get("cell"), d.get("len", 0))


def check_paths(recs):
    r = step_of(recs, "paths")
    if not r or not r.get("ok"):
        return False, "шаг не выполнен: %s" % (r or {}).get("err", "нет записи")
    base, items = r.get("baseline", {}), r.get("items", [])
    if base.get("status") != 404 or not base.get("body", "").startswith("404"):
        return False, "эталон (неизвестный набор): статус %s, тело %r" % (base.get("status"), base.get("body"))
    # Адрес с «/» или «/index.html» в конце: ассет-сервер Wails подменяет 404 своей HTML-страницей, поэтому media отвечает 400 (свой эталон)
    slash = r.get("baselineSlash", {})
    if slash.get("status") != 400 or not slash.get("body"):
        return False, "эталон адреса с «/» в конце: статус %s, тело %r (ожидалось 400 от media, не страница Wails)" % (slash.get("status"), slash.get("body"))
    sent, collapsed, why = 0, [], []
    for it in items:
        rp = it.get("resolved", "")
        wails = rp.endswith("/") or rp.endswith("/index.html")  # такие пути Wails подменяет своей страницей: у media на них 400
        if not rp.startswith("/media/lp/") and not (rp.startswith("/media/") and wails):
            collapsed.append("%r -> %s" % (it.get("p"), rp))
            continue
        sent += 1
        ref, code = (slash, 400) if wails else (base, 404)
        if it.get("status") == 200 and (it.get("ct") or "").startswith("text/html"):
            why.append("%r: подмена ассет-сервера Wails (HTML, %s Б), не данные и не наш %d" % (it.get("p"), it.get("len"), code))
        elif it.get("status") != code:
            why.append("%r: статус %s вместо %s, %s Б" % (it.get("p"), it.get("status", it.get("err")), code, it.get("len")))
        elif (it.get("ct"), it.get("body")) != (ref.get("ct"), ref.get("body")):
            why.append("%r: ответ не службы /media (тип %r, тело %r)" % (it.get("p"), it.get("ct"), it.get("body")))
    if not sent:
        why.append("ни один вариант не дошёл до /media/: проверять нечего")
    return not why, "; ".join(why[:6]) or "вариантов %d: до /media/ дошло %d - 404 (или 400 для адресов с «/» в конце, см. media) с ответом службы, не HTML-страница Wails; схлопнул браузер до отправки: %d (%s)" % (
        len(items), sent, len(collapsed), "; ".join(collapsed[:4]) or "нет")


def check_stress(recs, passes=PASSES):
    rounds = steps_of(recs, "round")
    fin = step_of(recs, "final")
    why = []
    if len(rounds) < 2:
        why.append("кругов листания %d из 2" % len(rounds))
    for r in rounds:
        if not r.get("ok"):
            why.append("круг: %s" % r.get("err"))
        elif r.get("errors") or r.get("bad") or not r.get("got"):
            why.append("круг %s: ошибки запросов %s, статусы не 200: %s, ответов 200: %s" % (r.get("n"), r.get("errors"), r.get("bad"), r.get("got")))
        elif r.get("done", 0) < min(passes, 10):
            why.append("круг %s: пройдено %s проходов за %s мс" % (r.get("n"), r.get("done"), r.get("ms")))
    if not fin or not (fin.get("probe") or {}).get("status") == 200:
        why.append("клетка после листания: %s" % ((fin or {}).get("probe") or {}).get("status", "нет записи"))
    tot = sum(r.get("requests", 0) for r in rounds)
    okn = sum(r.get("got", 0) for r in rounds)
    return not why, "; ".join(why) or "кругов %d, запросов %d (ответили 200: %d, оборвано нами: %d), ошибок и не-200 нет; клетка после листания - 200" % (
        len(rounds), tot, okn, sum(r.get("aborted", 0) for r in rounds))


def check_alive(run):
    ok = run["done"] and run["alive"]
    return ok, "exe %s к концу сценария (код выхода %s), зонд дошёл до конца: %s, %s с" % ("жив" if run["alive"] else "завершился", run["code"], run["done"], run["seconds"])


def _mib(n):
    return "%.0f МиБ" % (n / MIB)


def check_memory(recs, run):
    marks = {r.get("name"): r.get("mem") for r in steps_of(recs, "mark")}
    miss = [k for k in ("baseline", "r1", "r2") if not marks.get(k)]
    if miss:
        return False, "память не снята на отметках: %s" % ", ".join(miss)
    b, r1, r2 = marks["baseline"], marks["r1"], marks["r2"]
    why = []
    if r2["own"] - r1["own"] > PLATEAU_OWN:
        why.append("exe: второй круг добавил %s (предел %s)" % (_mib(r2["own"] - r1["own"]), _mib(PLATEAU_OWN)))
    if r2["tree"] - r1["tree"] > PLATEAU_TREE:
        why.append("дерево с WebView2: второй круг добавил %s (предел %s)" % (_mib(r2["tree"] - r1["tree"]), _mib(PLATEAU_TREE)))
    if r2["own"] - b["own"] > GROW_OWN:
        why.append("exe: рост от начала %s (предел %s)" % (_mib(r2["own"] - b["own"]), _mib(GROW_OWN)))
    nums = "exe, частная память: до %s, после круга 1 %s, после круга 2 %s, пик %s; дерево (процессов %d): %s -> %s -> %s, пик %s" % (
        _mib(b["own"]), _mib(r1["own"]), _mib(r2["own"]), _mib(run["peak"]["own"]), r2["procs"],
        _mib(b["tree"]), _mib(r1["tree"]), _mib(r2["tree"]), _mib(run["peak"]["tree"]))
    return not why, "; ".join(why + [nums]) if why else nums


def memory_note(recs, run):
    """Таблица памяти для summary.md: три отметки и пик."""
    marks = {r.get("name"): r.get("mem") for r in steps_of(recs, "mark")}
    rows = ["### Память процессов при листании", "",
            "| отметка | exe, частная | exe, рабочий набор | дерево с WebView2, частная | процессов |", "|---|---|---|---|---|"]
    for k, title in (("baseline", "до листания"), ("r1", "после круга 1"), ("r2", "после круга 2")):
        m = marks.get(k)
        cells = (_mib(m["own"]), _mib(m["own_ws"]), _mib(m["tree"]), m["procs"]) if m else ("-",) * 4
        rows.append("| %s | %s | %s | %s | %s |" % ((title,) + cells))
    rows.append("| пик за запуск | %s | | %s | |" % (_mib(run["peak"]["own"]), _mib(run["peak"]["tree"])))
    return "\n".join(rows) + "\n\nПределы: второй круг добавляет к первому не больше %s (exe) и %s (дерево); рост за оба круга - не больше %s." % (
        _mib(PLATEAU_OWN), _mib(PLATEAU_TREE), _mib(GROW_OWN))


def check_sheets_dir(path):
    """library-previews\\ есть, в нём листы <sha256>.jpg, и содержимое каждого сходится с именем."""
    names = [n for n in os.listdir(path) if re.fullmatch(r"[0-9a-f]{64}\.jpg", n)] if os.path.isdir(path) else []
    if not names:
        return False, "в %s нет листов (папки нет или пуста)" % path
    bad = [n for n in names if not sheet_matches(os.path.join(path, n), n[:-4])]
    return not bad, "листов %d, у которых sha256 не равен имени: %s" % (len(names), bad[:3]) if bad else "листов %d, sha256 каждого равен имени файла" % len(names)


def check_revoke(recs):
    r = step_of(recs, "revoke")
    if not r or not r.get("ok"):
        return False, "шаг не выполнен: %s" % (r or {}).get("err", "нет записи")
    n = sum((r.get("counts") or {}).values())
    st = (r.get("probe") or {}).get("status")
    return n == 0 and st == 404, "после «Не обращаться к сети»: плиток %s, клетка - статус %s (ожидалось 0 и 404)" % (n, st)


def check_corrupt(recs, sheet_exists):
    b, c, a = step_of(recs, "c-before"), step_of(recs, "c-cell"), step_of(recs, "c-after")
    if not (b and c and a) or not (b.get("ok") and c.get("ok") and a.get("ok")):
        return False, "шаги не выполнены: %s" % [(x or {}).get("err", "нет записи") for x in (b, c, a)]
    st = (c.get("probe") or {}).get("status")
    why = []
    if not b.get("n"):
        why.append("плиток набора до показа нет (лист не должен отвергаться по Lstat: размер тот же)")
    if st != 404:
        why.append("первый показ подменённого листа - статус %s вместо 404" % st)
    if a.get("n"):
        why.append("после отказа плитки набора остались: %s" % a.get("n"))
    if sheet_exists:
        why.append("испорченный лист остался на диске (os.Remove не прошёл?)")
    return not why, "; ".join(why) or "плиток до показа %s; показ - 404; плиток после 0; лист удалён с диска" % b.get("n")


def check_refetch(recs, sheet_ok):
    f, t, c = step_of(recs, "r-fetch"), step_of(recs, "r-tiles"), step_of(recs, "r-cell")
    if not (f and t and c) or not (f.get("ok") and t.get("ok") and c.get("ok")):
        return False, "шаги не выполнены: %s" % [(x or {}).get("err", "нет записи") for x in (f, t, c)]
    why = []
    if f.get("timeout"):
        why.append("событие library:previews не пришло за срок")
    if not t.get("n"):
        why.append("плитки набора не вернулись")
    if not _jpeg(c.get("probe") or {}):
        why.append("клетка после докачки: статус %s" % (c.get("probe") or {}).get("status"))
    if not sheet_ok:
        why.append("лист на диске не равен листу каталога (sha256)")
    return not why, "; ".join(why) or "лист докачан (листов %s), плитки вернулись (%s), клетка - 200, sha256 листа сошёлся" % (f.get("ready"), t.get("n"))


BAD_LOG = re.compile(r"panic|паник|сбой \(|goroutine \d+|fatal", re.I)
LEAK_LOG = re.compile(r"https?://|[A-Za-z]:\\|\\Users\\", re.I)


def check_log(text):
    """log.txt стенда: ни паник, ни сбоев; в строках «библиотека: …» ни адресов, ни путей."""
    bad, leak = [], []
    for ln in text.splitlines():
        if BAD_LOG.search(ln):
            bad.append(ln.strip()[:160])
        if "библиотека" in ln and LEAK_LOG.search(ln):
            leak.append(ln.strip()[:160])
    why = (["сбои: %s" % bad[:3]] if bad else []) + (["адрес или путь в строке библиотеки: %s" % leak[:3]] if leak else [])
    return not why, "; ".join(why) or "паник и сбоев нет, в строках «библиотека» адресов и путей нет (строк в журнале: %d)" % len(text.splitlines())


# ---------- состояние на диске ----------

def catalog_pack(data, pack_id):
    """Строка набора из сохранённого списка (конверт {sig, body-base64}); None - нет файла или набора."""
    try:
        with open(os.path.join(data, "library-catalog.json"), encoding="utf-8") as f:
            env = json.load(f)
        body = json.loads(base64.b64decode(env["body"]))
    except (OSError, ValueError, KeyError, TypeError):
        return None
    for p in body.get("packs", []):
        if p.get("id") == pack_id:
            return p
    return None


def valid_sha(sha):
    """64 строчные hex-цифры: sha256 из каталога (чужие данные) попадает в имя файла, другого вида в пути быть не должно."""
    return isinstance(sha, str) and re.fullmatch(r"[0-9a-f]{64}", sha) is not None


def sheet_path(data, sha):
    if not valid_sha(sha):
        raise ValueError("sha256 листа не 64 hex-цифры: %r" % (sha,))
    return os.path.join(data, "library-previews", sha + ".jpg")


def sheet_matches(path, sha):
    try:
        return file_sha(path) == sha
    except OSError:
        return False


def corrupt_sheet(path):
    """Подменить один байт листа, размер не меняется (sha256 не сойдётся, Lstat ничего не заметит)."""
    with open(path, "rb") as f:
        b = bytearray(f.read())
    if len(b) <= 100:
        raise ValueError("лист слишком мал: %d Б" % len(b))
    b[100] ^= 0xFF
    with open(path, "wb") as f:
        f.write(b)


TMP_RE = re.compile(r"^\.dl-.*\.tmp$")


def tmp_leftovers(roots):
    """Остатки .dl-*.tmp под папками данных стенда."""
    found = []
    for root in roots:
        for d, _, files in os.walk(root):
            found += [os.path.join(d, n) for n in files if TMP_RE.match(n)]
    return found


