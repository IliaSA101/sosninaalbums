"""Вердикты win-qa «диск снова доступен» (спека 0.36/157, design §3): чистые функции над записями зонда volume_probe.js.
Без Windows и без запуска exe; тесты - test_volume_exe.py. Вердикт - (ok, подробности) или (ok, подробности, "skip")
для «не проверено». Времена записей - мс от старта зонда (поле t). Файл в UTF-8."""

import re

# Пороги design §3
V1_MISSING_MAX_MS = 3500  # недостающие видны до пробуждения тома (шов: 4 с), ждали ≈ 1,5-2 с
V1_CLEAR_FAIL_MS = 8000  # полоса через 8 с после открытия - провал (ждали ≈ 4-5 с)
V1_THUMBS_MS = 10000  # миниатюры с naturalWidth > 0 - в течение 10 с после тоста
V2_WANT_MS = 19500  # ждали: 15 с срок «мёртвого» + период проверки + пересчёт
V2_FAIL_MS = 25000  # полоса и через 25 с - провал
TOAST_RE = re.compile(r"Диск\s+([A-Za-z]):\s+снова доступен")


# ---------- записи зонда ----------

def step_of(recs, name):
    """Последняя запись шага или None."""
    for r in reversed(recs):
        if r.get("step") == name:
            return r
    return None


def first_of(recs, name):
    """Первая запись шага или None."""
    for r in recs:
        if r.get("step") == name:
            return r
    return None


def events(recs, name):
    """События Go (step=ev) с этим именем по порядку."""
    return [r for r in recs if r.get("step") == "ev" and r.get("name") == name]


def t_of(r):
    return (r or {}).get("t")


def sec(ms):
    return "%.1f с" % (ms / 1000.0)


def opening_t(recs):
    r = first_of(recs, "opening")
    return None if r is None else t_of(r)


def toast_times(recs, letter="Q"):
    """Времена появлений тоста «Диск Q: снова доступен…»: переход из «не он» в «он»."""
    out, prev = [], False
    for r in recs:
        if r.get("step") != "toast":
            continue
        m = TOAST_RE.search(r.get("text") or "")
        now = bool(m and m.group(1).upper() == letter.upper())
        if now and not prev:
            out.append(t_of(r))
        prev = now
    return out


def strip_at(recs, t):
    """Полоса недостающих в момент t (мс): последнее состояние не позже t; до первой записи - нет."""
    on = False
    for r in recs:
        if r.get("step") == "strip" and t_of(r) <= t:
            on = bool(r.get("on"))
    return on


def strip_span(recs):
    """(первое появление, последнее исчезновение) полосы, мс; None - не было."""
    ons = [t_of(r) for r in recs if r.get("step") == "strip" and r.get("on")]
    offs = [t_of(r) for r in recs if r.get("step") == "strip" and not r.get("on")]
    return (ons[0] if ons else None), (offs[-1] if offs else None)


def _no_open(recs):
    r = step_of(recs, "opened")
    if not r or not r.get("ok"):
        return "альбом не открылся: %s" % (r or {}).get("err", "нет записи opened")
    if opening_t(recs) is None:
        return "нет записи opening"
    return ""


# ---------- В1: сонный диск (шов SA_QA_VOLDELAY) ----------

def check_v1_missing(recs, photos=6):
    bad = _no_open(recs)
    if bad:
        return False, bad
    o = opening_t(recs)
    firsts = [r for r in events(recs, "missing") if t_of(r) >= o and r.get("n", 0) > 0]
    if not firsts:
        return False, "missing с недостающими не пришло: шов задержки не сработал или том ответил сразу"
    r = firsts[0]
    rel = t_of(r) - o
    why = []
    if r.get("n") != photos:
        why.append("недостающих %s, ждали %d" % (r.get("n"), photos))
    if rel > V1_MISSING_MAX_MS:
        why.append("через %s после открытия (ждали ≈ 1,5-2 с, до пробуждения тома)" % sec(rel))
    return not why, "; ".join(why) or "missing с %d к %s после открытия" % (r.get("n"), sec(rel))


def check_v1_clear(recs):
    """Полоса и список недостающих очищены к 8 с после открытия."""
    bad = _no_open(recs)
    if bad:
        return False, bad
    o = opening_t(recs)
    ms = [r for r in events(recs, "missing") if t_of(r) >= o]
    fails = [r for r in ms if r.get("n", 0) > 0]
    after = [r for r in ms if fails and t_of(r) > t_of(fails[0]) and r.get("n") == 0]
    if not after:
        return False, "список недостающих не опустел после появления (missing с 0 не пришло)"
    rel = t_of(after[0]) - o
    if rel > V1_CLEAR_FAIL_MS:
        return False, "missing пустое только через %s (провал с %s)" % (sec(rel), sec(V1_CLEAR_FAIL_MS))
    return True, "missing пустое к %s после открытия (ждали ≈ 4-5 с)" % sec(rel)


def check_v1_strip(recs):
    bad = _no_open(recs)
    if bad:
        return False, bad
    o = opening_t(recs)
    on, off = strip_span(recs)
    at8 = strip_at(recs, o + V1_CLEAR_FAIL_MS)
    last = [r for r in recs if r.get("step") == "strip"]
    final = bool(last[-1].get("on")) if last else False
    if at8:
        return False, "полоса недостающих ещё на экране через %s после открытия" % sec(V1_CLEAR_FAIL_MS)
    if final:
        return False, "полоса недостающих осталась на экране к концу прогона"
    if on is None:
        return True, "полоса не показывалась (опрос 100 мс мог пропустить короткий показ); в конце её нет"
    return True, "полоса показана с %s, убрана к %s после открытия; в конце её нет" % (
        sec(on - o), sec((off or on) - o))


def check_v1_back(recs, letter="Q", photos=6):
    bad = _no_open(recs)
    if bad:
        return False, bad
    o = opening_t(recs)
    backs = [r for r in events(recs, "drive:back") if t_of(r) >= o]
    if not backs:
        return False, "drive:back не пришло"
    if len(backs) > 1:
        return False, "drive:back пришло %d раза: %s" % (len(backs), [(b.get("drive"), b.get("photos")) for b in backs])
    b = backs[0]
    rel = t_of(b) - o
    why = []
    if str(b.get("drive", "")).upper() != letter:
        why.append("диск %r, ждали %r" % (b.get("drive"), letter))
    if b.get("photos") != photos:
        why.append("найдено %r, ждали %d" % (b.get("photos"), photos))
    if rel > V1_CLEAR_FAIL_MS:
        why.append("через %s (провал с %s)" % (sec(rel), sec(V1_CLEAR_FAIL_MS)))
    return not why, "; ".join(why) or "drive:back {%s, %s} к %s после открытия (ждали ≈ 4-5 с)" % (b.get("drive"), b.get("photos"), sec(rel))


def check_v1_toast(recs, letter="Q"):
    """Тост «Диск Q: снова доступен» ровно один."""
    bad = _no_open(recs)
    if bad:
        return False, bad
    ts = toast_times(recs, letter)
    if not ts:
        return False, "тоста «Диск %s: снова доступен» не было" % letter
    if len(ts) > 1:
        return False, "тост появлялся %d раза (t, мс: %s)" % (len(ts), ts)
    return True, "тост один, к %s после открытия" % sec(ts[0] - opening_t(recs))


def check_v1_thumbs(recs, photos=6):
    """Миниатюры с naturalWidth > 0 в течение 10 с после тоста (нет тоста - после drive:back)."""
    bad = _no_open(recs)
    if bad:
        return False, bad
    ts = toast_times(recs)
    backs = events(recs, "drive:back")
    base = ts[0] if ts else (t_of(backs[0]) if backs else None)
    if base is None:
        return False, "нет ни тоста, ни drive:back: отсчёта десяти секунд нет"
    th = [r for r in recs if r.get("step") == "thumbs"]
    good = [r for r in th if t_of(r) >= base and r.get("n", 0) >= photos]
    if good and t_of(good[0]) - base <= V1_THUMBS_MS:
        return True, "миниатюры (%d) загружены через %s после %s" % (good[0].get("n"), sec(max(0, t_of(good[0]) - base)), "тоста" if ts else "drive:back")
    # у уже загруженных до тоста записи нет: берём состояние на момент тоста
    cur = None
    for r in th:
        if t_of(r) <= base + V1_THUMBS_MS:
            cur = r
    if cur and cur.get("n", 0) >= photos:
        return True, "миниатюры (%d) уже загружены к тосту" % cur.get("n")
    return False, "за %s после %s миниатюр с naturalWidth > 0: %s из %s в ленте (ждали %d)" % (
        sec(V1_THUMBS_MS), "тоста" if ts else "drive:back", (cur or {}).get("n", 0), (cur or {}).get("total", 0), photos)


def check_v1_canvas(recs):
    """На холсте есть кадры с naturalWidth > 0 в конце прогона."""
    cv = [r for r in recs if r.get("step") == "canvas"]
    if not cv or cv[-1].get("imgs", 0) <= 0:
        last = cv[-1] if cv else {}
        return False, "кадров на холсте с naturalWidth > 0 нет (разворотов в DOM: %s)" % last.get("spreads", 0)
    return True, "кадров на холсте: %d (разворотов в DOM: %s)" % (cv[-1]["imgs"], cv[-1].get("spreads"))


# ---------- В2, В2б, В3: потеря и возврат буквы (Kick, фокус, NAS) ----------

def v2_facts(recs, photos=6):
    """Времена после отключения тома: {err, before, fail, clear, back, toast, dt_*} (мс от неудачной пробы)."""
    bad = _no_open(recs)
    if bad:
        return {"err": bad}
    cut = first_of(recs, "cut-now")
    if cut is None:
        return {"err": "зонд не дошёл до отключения тома (нет записи cut-now)"}
    tc = t_of(cut)
    ms = [r for r in events(recs, "missing")]
    before = [r for r in ms if t_of(r) < tc]
    f = {"before": before[-1].get("n") if before else None}
    fails = [r for r in ms if t_of(r) >= tc and r.get("n", 0) > 0]
    if not fails:
        return f
    f["fail"] = t_of(fails[0])
    f["fail_n"] = fails[0].get("n")
    clears = [r for r in ms if t_of(r) > f["fail"] and r.get("n") == 0]
    if clears:
        f["clear"] = t_of(clears[0])
        f["dt_clear"] = f["clear"] - f["fail"]
    backs = [r for r in events(recs, "drive:back") if t_of(r) >= f["fail"]]
    f["backs"] = backs
    if backs:
        f["back"] = t_of(backs[0])
        f["dt_back"] = f["back"] - f["fail"]
    ts = [t for t in toast_times(recs) if t >= f["fail"]]
    if ts:
        f["toast"] = ts[0]
        f["dt_toast"] = ts[0] - f["fail"]
    f["checks"] = len([r for r in recs if r.get("step") == "check" and t_of(r) >= tc])
    return f


def facts_text(f):
    """Факты В2/В2б/В3 одной строкой для отчёта: что и когда случилось от неудачной пробы."""
    def one(key, label):
        return "%s %s" % (label, sec(f[key])) if key in f else "%s нет" % label
    return "; ".join([
        "неудачная проба: missing %s" % f.get("fail_n"),
        one("dt_clear", "полоса исчезла через"),
        one("dt_back", "drive:back через"),
        one("dt_toast", "тост через"),
        "проверок папок после отключения: %s" % f.get("checks", 0),
    ])


def _v2_prelude(f):
    """Общие отказы подготовки (до отключения): None - можно судить по временам."""
    if f.get("err"):
        return f["err"]
    if f.get("before") not in (0, None):
        return "до отключения уже было недостающих: %s (фото на томе не найдены)" % f.get("before")
    return None


def check_v2(recs, photos=6):
    """В2/В2б (Kick): полоса исчезает и drive:back приходит; в срок ≤ 19,5 с от неудачной пробы, провал при 25 с."""
    f = v2_facts(recs, photos)
    bad = _v2_prelude(f)
    if bad:
        return False, bad
    if "fail" not in f:
        return False, "после отключения буквы недостающие не появились (CheckFolders(true) не дал missing): сценарий не состоялся"
    if "dt_clear" not in f:
        return False, "полоса не исчезла за %s после неудачной пробы; %s" % (sec(V2_FAIL_MS), facts_text(f))
    if f["dt_clear"] > V2_FAIL_MS:
        return False, "полоса исчезла только через %s (провал с %s); %s" % (sec(f["dt_clear"]), sec(V2_FAIL_MS), facts_text(f))
    why = []
    if len(f.get("backs", [])) != 1:
        why.append("drive:back пришло %d раз" % len(f.get("backs", [])))
    elif f["backs"][0].get("photos") != photos or str(f["backs"][0].get("drive", "")).upper() != "Q":
        why.append("drive:back {%s, %s}, ждали {Q, %d}" % (f["backs"][0].get("drive"), f["backs"][0].get("photos"), photos))
    elif f["dt_back"] > V2_FAIL_MS:
        why.append("drive:back через %s (провал с %s)" % (sec(f["dt_back"]), sec(V2_FAIL_MS)))
    late = "" if f["dt_clear"] <= V2_WANT_MS else " (дольше ожидаемых %s, но не провал)" % sec(V2_WANT_MS)
    return not why, "; ".join(why) or facts_text(f) + late


def check_v2_toast(recs):
    f = v2_facts(recs)
    bad = _v2_prelude(f)
    if bad or "fail" not in f:
        return False, bad or "неудачной пробы не было: тоста не ждали"
    ts = [t for t in toast_times(recs) if t >= f["fail"]]
    if not ts:
        return False, "тоста «Диск Q: снова доступен» не было за срок прогона; %s" % facts_text(f)
    if len(ts) > 1:
        return False, "тост появлялся %d раза" % len(ts)
    if ts[0] - f["fail"] > V2_FAIL_MS:
        return False, "тост через %s (провал с %s)" % (sec(ts[0] - f["fail"]), sec(V2_FAIL_MS))
    return True, "тост один, через %s после неудачной пробы" % sec(ts[0] - f["fail"])


def check_record(recs, label, env_err=""):
    """В2б и В3 - только запись времени (не валит). env_err - окружение не собралось: «не проверено»."""
    if env_err:
        return True, "не проверено: окружение: %s" % env_err, "skip"
    f = v2_facts(recs)
    if f.get("err"):
        return True, "не проверено: %s" % f["err"], "skip"
    if "fail" not in f:
        return True, "%s: после отключения недостающие не появились (быстрый отказ без сбоя пробы?); проверок папок %s" % (label, f.get("checks", 0))
    return True, "%s: %s" % (label, facts_text(f))


def check_log_panics(text):
    """В журнале стенда нет паник."""
    bad = [ln.strip()[:160] for ln in (text or "").splitlines() if re.search(r"\bpanic\b|goroutine \d+ \[", ln, re.I)]
    return not bad, "; ".join(bad[:3]) if bad else "паник в log.txt нет"
