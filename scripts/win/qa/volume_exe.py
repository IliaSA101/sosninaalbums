#!/usr/bin/env python3
"""Диск снова доступен в exe (win-qa, спека 0.36/157, design §3; образец - library_exe.py, 144): qa-exe на Windows-раннере.

Что видно только в exe: настоящая проба корня тома (os.Stat буквы диска) и её срок «мёртвого» в Windows, пересчёт недостающих
и событие drive:back, полоса и тост в окне WebView2. Окно снаружи запросов не принимает, поэтому наблюдения снимает зонд
внутри окна (volume_probe.js, SA_QA_PROBE); здесь - подготовка (subst, альбом, стенд), ход запусков, отчёт; вердикты -
volume_checks.py (чистые функции, их тесты - test_volume_exe.py).

Четыре независимых блока, каждый - свой запуск exe со стендом qa (SA_STAND=qa), сбой блока не прерывает остальные:
  В1   сонный диск: subst Q: <папка с 6 фото>, шов SA_QA_VOLDELAY=Q:4000 (первая проба Q: отвечает через 4 с);
  В2   буква пропала: без шва; subst Q: /D, один CheckFolders(true), subst Q: обратно, дальше CheckFolders(false) раз в 2,5 с (Kick);
  В2б  то же, но CheckFolders(true) раз в 2,5 с (фокус) - только запись времени;
  В3   NAS-петля: общая папка на этой машине, net use R:, Stop-Service LanmanServer - запись или «не проверено: окружение».
Один прогон собирает все провалы; в конце дамп состояния и код 1. Запуск: python volume_exe.py --kit <набор> --out <папка>.
Только Windows; stdlib. Файл в UTF-8 (запускается python, не PowerShell).
"""

import argparse
import importlib.util
import json
import os
import re
import shutil
import subprocess
import sys
import time
import traceback

HERE = os.path.dirname(os.path.abspath(__file__))


def _sibling(name):
    """Соседний модуль по пути: под `python -I` папка скрипта в sys.path не входит."""
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, name + ".py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


vc = _sibling("volume_checks")
lx = _sibling("library_exe")  # extract_exe, stand_config, read_log; тянет agents_exe и library_run
ax = lx.ax
rn = _sibling("library_run")
rn.PROBE_JS = os.path.join(HERE, "volume_probe.js")  # probe_script() читает этот путь при вызове

STAND = lx.STAND
DATA = lx.DATA
LETTER = "Q"  # subst-буква для В1 и В2
NAS_LETTER = "R"  # net use для В3
SHARE = "qavolume"
PHOTOS = 6
SPREADS = 3
DELAY_MS = 4000

# Настройки зонда. cutAfterMs: после отключения буквы живой кэш пробы тома (5 с) должен устареть, иначе Reachable
# ответит «жив» и неудачной пробы, с которой считается срок, не будет. periodMs - период сверки папок (2,5 с),
# limitMs - срок после неудачной пробы, noFailMs - срок ожидания самой неудачной пробы.
SLEEPY_CFG = {"mode": "sleepy", "capMs": 14000, "tailMs": 11500}
CUT_CFG = {"mode": "cut", "cutAfterMs": 6500, "periodMs": 2500, "limitMs": 28000, "noFailMs": 14000, "tailMs": 4000}


# ---------- отчёт ----------

class Report(ax.Report):
    def markdown(self):
        out = ["## win-qa: диск снова доступен в exe (157)", "", "| пункт | итог | подробности |", "|---|---|---|"]
        for rid, st, d in self.rows:
            out.append("| %s | %s | %s |" % (rid, st, d.replace("|", "/").replace("\n", " ")))
        return "\n".join(out + [""] + getattr(self, "notes", [])) + "\n"

    def judge(self, rid, verdict):
        """Вердикт (ok, текст) или (ok, текст, "skip") - «не проверено»."""
        self.add(rid, verdict[0], verdict[1], skip=len(verdict) > 2 and verdict[2] == "skip")


OWNER_LEFT = """### Остаток владельцу (в CI не проверяется)

- Настоящий сетевой диск (NAS, выдернутый кабель, Wi-Fi): здесь только `subst`, общая папка этой же машины и шов задержки.
- Выпускной exe: у него нет зонда и шва, `missing` прочесть нечем; его охраняет страж строк в бинарнике (`release.yml`).
- Тост и полоса глазами: зонд видит DOM (`.toast`, `[data-guide="missing-bar"]`), но не вид и не анимацию."""


def new_report():
    rep = Report()
    rep.notes = []
    return rep


# ---------- подготовка: альбом, стенд, буквы дисков ----------

def retarget_album(data, letter, folders):
    """Копия содержимого .salbum с фото на корне диска <letter>: и папками альбома folders (список путей).
    Пути фото из qa-albums - относительные (photos/photo-NN.jpg), в копии - абсолютные X:\\photo-NN.jpg."""
    out = json.loads(json.dumps(data))
    out["folders"] = [{"path": f} for f in folders]
    for ph in (out.get("photos") or {}).values():
        ph["path"] = "%s:\\%s" % (letter, os.path.basename(str(ph.get("path", "")).replace("\\", "/")))
    return out


def write_album(src, dst, letter, folders):
    with open(src, encoding="utf-8") as f:
        data = json.load(f)
    ax.write_text(dst, json.dumps(retarget_album(data, letter, folders), ensure_ascii=False, indent=1))
    return dst


def volume_config(album):
    """config.json стенда: как у library_exe (сеть библиотеки выключена), плюс альбом в «Последних»: зонд открывает OpenRecent."""
    cfg = json.loads(lx.stand_config(False))
    cfg["recent"] = [album]
    return json.dumps(cfg, ensure_ascii=False)


def make_set(kit, dest):
    """Фото и Alpha.salbum из tools/qa-albums.exe набора: (путь Alpha, папка photos) или (None, причина)."""
    gen = os.path.join(kit, "tools", "qa-albums.exe")
    if not os.path.isfile(gen):
        return None, "в наборе нет tools/qa-albums.exe"
    shutil.rmtree(dest, ignore_errors=True)
    r = subprocess.run([gen, "-out", dest, "-photos", str(PHOTOS), "-spreads", str(SPREADS)], capture_output=True, timeout=120)
    text = (r.stdout + r.stderr).decode("utf-8", "replace")
    m = re.search(r"(?m)^alpha=(.+?)\s*$", text)
    photos = os.path.join(dest, "photos")
    if r.returncode != 0 or not m or not os.path.isfile(m.group(1)):
        return None, "qa-albums: код %s: %s" % (r.returncode, text.strip()[:300])
    n = len([x for x in os.listdir(photos) if x.lower().endswith(".jpg")]) if os.path.isdir(photos) else 0
    if n != PHOTOS:
        return None, "qa-albums: в %s фото %d, ждали %d" % (photos, n, PHOTOS)
    return (m.group(1), photos), ""


def run_cmd(args, timeout=60):
    """(код, вывод) внешней команды; исключение запуска - в код -1."""
    try:
        r = subprocess.run(args, capture_output=True, timeout=timeout)
        return r.returncode, (r.stdout + r.stderr).decode("mbcs" if os.name == "nt" else "utf-8", "replace").strip()
    except Exception as e:
        return -1, "%s: %s" % (type(e).__name__, e)


def subst_on(letter, folder):
    return run_cmd(["subst", "%s:" % letter, folder])


def subst_off(letter):
    return run_cmd(["subst", "%s:" % letter, "/D"])


def subst_list():
    return run_cmd(["subst"])[1]


def ps_quote(s):
    """Строка PowerShell в одинарных кавычках: кавычка удваивается."""
    return "'" + s.replace("'", "''") + "'"


class NullSampler:
    """Память здесь не нужна, а Get-CimInstance раз в 2 с исказил бы времена: пустой снимок под интерфейс run_probe."""

    def __init__(self, pid):
        self.peak, self.n = {"own": 0, "tree": 0}, 0

    def start(self):
        pass

    def stop(self):
        pass

    def sample_now(self):
        return None


class CutHook:
    """Действия Python посреди запуска: по cut-now зонда - отключить том, по первому missing с недостающими - вернуть.
    Каждое действие пишется (имя, часы, код, вывод) в actions; неудача действия не прерывает запуск, а видна в отчёте."""

    def __init__(self, cut, restore):
        self.cut, self.restore = cut, restore
        self.stage = 0
        self.actions = []

    def act(self, name, fn):
        try:
            code, text = fn()
        except Exception as e:
            code, text = -1, "%s: %s" % (type(e).__name__, e)
        self.actions.append({"name": name, "w": int(time.time() * 1000), "code": code, "text": str(text)[:300]})

    def __call__(self, new):
        for r in new:
            if self.stage == 0 and r.get("step") == "cut-now":
                self.stage = 1
                self.act("cut", self.cut)
            elif self.stage == 1 and r.get("step") == "ev" and r.get("name") == "missing" and r.get("n", 0) > 0:
                self.stage = 2
                self.act("restore", self.restore)

    def failed(self):
        return [a for a in self.actions if a["code"] != 0]


# ---------- запуск и дамп ----------

def reset_stand(data, album):
    shutil.rmtree(data, ignore_errors=True)
    ax.write_text(os.path.join(data, "config.json"), volume_config(album))


def go(ctx, name, cfg, album, env_extra=None, hook=None, timeout=150):
    """Один запуск exe с зондом на чистом стенде; результат run_probe плюс log стенда и действия hook."""
    reset_stand(ctx["data"], album)
    env = dict(ctx["env"], **(env_extra or {}))
    cfg = dict(cfg, album=album)
    run = rn.run_probe(ctx["exe"], env, ctx["work"], name, cfg, timeout, sampler_cls=NullSampler, poll=0.1, hook=hook)
    run["log"] = lx.read_log(ctx["data"])
    run["actions"] = list(hook.actions) if hook else []
    ctx["runs"][name] = run
    return run


def judge_run_basics(rep, tag, run):
    rep.judge("%s окно отвечает зонду, зонд дошёл до конца" % tag, vc_ready(run))
    ok, text = vc.check_log_panics(run["log"])
    rep.add("%s log.txt стенда без паник" % tag, ok, text)


def vc_ready(run):
    r = vc.step_of(run["records"], "ready")
    if not r or not r.get("ok"):
        return False, "окно не отозвалось: %s; записей %d; exe жив: %s, код %s" % ((r or {}).get("err", "нет записи ready"), len(run["records"]), run["alive"], run["code"])
    if not run["done"]:
        return False, "нет записи done за срок запуска (записей %d, exe жив: %s, код %s)" % (len(run["records"]), run["alive"], run["code"])
    return True, "связка Go и событий в окне за %s мс, запуск %s с" % (r.get("ms"), run["seconds"])


def action_verdict(hook):
    bad = hook.failed()
    if bad or len(hook.actions) < 2:
        return False, "действия Python: %s" % ("; ".join("%s: код %s %s" % (a["name"], a["code"], a["text"]) for a in bad) or "выполнено %d из 2" % len(hook.actions))
    return True, "отключение и возврат тома выполнены (%s)" % ", ".join(a["name"] for a in hook.actions)


# ---------- блоки ----------

def block_v1(ctx, rep):
    """В1: сонный диск через шов."""
    tag = "157-В1"
    made, why = make_set(ctx["kit"], os.path.join(ctx["work"], "v1set"))
    if not made:
        rep.add(tag + " подготовка", False, why)
        return
    alpha, photos = made
    album = write_album(alpha, os.path.join(ctx["work"], "v1", "Album.salbum"), LETTER, ["%s:\\" % LETTER])
    subst_off(LETTER)
    code, text = subst_on(LETTER, photos)
    if code != 0:
        rep.add(tag + " подготовка", False, "subst %s: не создан (код %s): %s" % (LETTER, code, text))
        return
    run = go(ctx, "v1", SLEEPY_CFG, album, {"SA_QA_VOLDELAY": "%s:%d" % (LETTER, DELAY_MS)})
    recs = run["records"]
    judge_run_basics(rep, tag, run)
    rep.judge(tag + " сонный диск: missing с 6 видно до пробуждения тома", vc.check_v1_missing(recs, PHOTOS))
    rep.judge(tag + " missing опустело, полоса не дольше 8 с", vc.check_v1_clear(recs))
    rep.judge(tag + " drive:back {Q, 6} один раз", vc.check_v1_back(recs, LETTER, PHOTOS))
    rep.judge(tag + " тост «Диск Q: снова доступен» один", vc.check_v1_toast(recs, LETTER))
    rep.judge(tag + " полосы недостающих нет", vc.check_v1_strip(recs))
    rep.judge(tag + " миниатюры загружены в течение 10 с после тоста", vc.check_v1_thumbs(recs, PHOTOS))
    rep.judge(tag + " на холсте фото", vc.check_v1_canvas(recs))


def block_cut(ctx, rep, tag, full, strict):
    """В2 (strict) и В2б: буква пропала и вернулась, сверка папок раз в 2,5 с (full - CheckFolders(true))."""
    made, why = make_set(ctx["kit"], os.path.join(ctx["work"], tag + "set"))
    if not made:
        rep.add(tag + " подготовка", False, why)
        return
    alpha, photos = made
    src = os.path.join(ctx["work"], tag + "src")  # папка-источник на C: - отпечаток папок не меняется, оживляет только Kick
    os.makedirs(src, exist_ok=True)
    album = write_album(alpha, os.path.join(ctx["work"], tag, "Album.salbum"), LETTER, [src])
    subst_off(LETTER)
    code, text = subst_on(LETTER, photos)
    if code != 0:
        rep.add(tag + " подготовка", False, "subst %s: не создан (код %s): %s" % (LETTER, code, text))
        return
    hook = CutHook(lambda: subst_off(LETTER), lambda: subst_on(LETTER, photos))
    run = go(ctx, tag, dict(CUT_CFG, full=full), album, hook=hook)
    recs = run["records"]
    judge_run_basics(rep, tag, run)
    rep.judge(tag + " отключение и возврат буквы", action_verdict(hook))
    if strict:
        rep.judge(tag + " полоса исчезла и drive:back пришло в срок (≤ 19,5 с, провал с 25 с)", vc.check_v2(recs, PHOTOS))
        rep.judge(tag + " тост «Диск Q: снова доступен» один", vc.check_v2_toast(recs))
    else:
        rep.judge(tag + " запись времени через фокус (CheckFolders(true) раз в 2,5 с)", vc.check_record(recs, "через фокус"))


def block_v3(ctx, rep):
    """В3: NAS-петля на этой машине. Любая ошибка настройки - «не проверено: окружение», задачу не валит."""
    tag = "157-В3"
    state = {"deps": []}
    try:
        made, why = make_set(ctx["kit"], os.path.join(ctx["work"], "v3set"))
        if not made:
            rep.judge(tag + " NAS-петля (запись времени)", vc.check_record([], "NAS", "подготовка: " + why))
            return
        alpha, photos = made
        share = os.path.join(ctx["work"], "v3set", "photos")
        steps = [
            ("общая папка", "New-SmbShare -Name %s -Path %s -FullAccess Everyone | Out-Null" % (SHARE, ps_quote(share))),
            ("диск", "net use %s: \\\\localhost\\%s /persistent:no" % (NAS_LETTER, SHARE)),
        ]
        for label, script in steps:
            code, text = ax.ps(script, timeout=90)
            if code != 0 or "error" in text.lower():
                rep.judge(tag + " NAS-петля (запись времени)", vc.check_record([], "NAS", "%s не создана (код %s): %s" % (label, code, " ".join(text.split())[:200])))
                return
        if not os.path.isfile("%s:\\photo-01.jpg" % NAS_LETTER):
            rep.judge(tag + " NAS-петля (запись времени)", vc.check_record([], "NAS", "диск %s: не показывает фото общей папки" % NAS_LETTER))
            return
        album = write_album(alpha, os.path.join(ctx["work"], "v3", "Album.salbum"), NAS_LETTER, ["%s:\\" % NAS_LETTER])

        def stop():
            code, text = ax.ps("(Get-Service LanmanServer).DependentServices | Where-Object { $_.Status -eq 'Running' } | ForEach-Object { 'dep=' + $_.Name }; "
                               "Stop-Service LanmanServer -Force -ErrorAction Stop; 'stopped'", timeout=120)
            state["deps"] = re.findall(r"(?m)^dep=(\S+)", text)
            return (0 if code == 0 and "stopped" in text else 1), " ".join(text.split())

        def start():
            return ax.ps(restore_script(state["deps"]), timeout=120)

        hook = CutHook(stop, start)
        run = go(ctx, "v3", dict(CUT_CFG, full=True), album, hook=hook, timeout=180)
        recs = run["records"]
        judge_run_basics(rep, tag, run)
        if hook.failed() or len(hook.actions) < 2:
            rep.judge(tag + " NAS-петля (запись времени)", vc.check_record(recs, "NAS", "службу не остановить или не запустить: " + action_verdict(hook)[1]))
        else:
            rep.judge(tag + " NAS-петля (запись времени)", vc.check_record(recs, "NAS"))
    finally:
        ax.ps(restore_script(state["deps"]), timeout=120)  # служба должна работать в любом случае
        ax.ps("net use %s: /delete /y" % NAS_LETTER, timeout=60)
        ax.ps("Remove-SmbShare -Name %s -Force -ErrorAction SilentlyContinue" % SHARE, timeout=60)


def restore_script(deps):
    """Запуск LanmanServer и зависимых служб, которые работали до остановки."""
    names = [d for d in deps if re.fullmatch(r"[A-Za-z0-9_.-]+", d)]
    return "Start-Service LanmanServer -ErrorAction SilentlyContinue; " + "; ".join("Start-Service %s -ErrorAction SilentlyContinue" % n for n in names) + "; 'started'"


# ---------- прогон ----------

def dump_text(ctx):
    parts = ["состояние на конец прогона", "subst: %r" % subst_list(), "папка стенда %s: %s" % (ctx["data"], ax.listing(ctx["data"]))]
    for name, run in ctx["runs"].items():
        parts.append("запуск %s: done=%s alive=%s code=%s %s с" % (name, run["done"], run["alive"], run["code"], run["seconds"]))
        parts.append("действия %s: %s" % (name, json.dumps(run.get("actions"), ensure_ascii=False)))
        parts.append("записи %s:\n%s" % (name, "\n".join(json.dumps(r, ensure_ascii=False)[:400] for r in run["records"][:300])))
        parts.append("log.txt %s (хвост):\n%s" % (name, "\n".join(run["log"].splitlines()[-40:])))
    return "\n".join(parts)


def scenario(kit, out, data, rep, runs):
    work = os.path.join(out, "work")
    exe = lx.extract_exe(kit, os.path.join(work, "exe"))
    env = {k: v for k, v in os.environ.items() if k not in ("SA_LIBRARY_URL", "SA_LIBRARY_KEY", "SA_QA_PROBE", "SA_QA_PROBE_OUT", "SA_QA_VOLDELAY")}
    env["SA_STAND"] = STAND
    ctx = {"kit": kit, "work": work, "exe": exe, "env": env, "data": data, "runs": runs}
    blocks = [
        ("157-В1", lambda: block_v1(ctx, rep)),
        ("157-В2", lambda: block_cut(ctx, rep, "157-В2", False, True)),
        ("157-В2б", lambda: block_cut(ctx, rep, "157-В2б", True, False)),
        ("157-В3", lambda: block_v3(ctx, rep)),
    ]
    for tag, fn in blocks:  # блок падает сам по себе, остальные идут дальше
        try:
            fn()
        except Exception:
            rep.add(tag + " сценарий", False, "исключение: %s" % traceback.format_exc().replace("\n", " / "))
        finally:
            subst_off(LETTER)
    return ctx


def main():
    ax.use_utf8_streams()
    ap = argparse.ArgumentParser()
    ap.add_argument("--kit", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    if os.name != "nt":
        print("только Windows", file=sys.stderr)
        return 2
    out = os.path.abspath(a.out)
    os.makedirs(out, exist_ok=True)
    roaming = os.environ.get("APPDATA", "")
    if not roaming:
        print("нет APPDATA", file=sys.stderr)
        return 2
    data = os.path.join(roaming, DATA)
    rep, runs, ctx = new_report(), {}, None
    try:
        ctx = scenario(a.kit, out, data, rep, runs)
    except Exception:  # сбой самого сценария (набор, exe) - в итог, дамп всё равно соберётся
        rep.add("сценарий", False, "исключение: %s" % traceback.format_exc().replace("\n", " / "))
    return finish(rep, out, ctx or {"data": data, "runs": runs})


def finish(rep, out, ctx):
    rep.notes.append(OWNER_LEFT)
    dump = dump_text(ctx)
    ax.write_text(os.path.join(out, "state-dump.txt"), dump)
    ax.write_text(os.path.join(out, "summary.md"), rep.markdown())
    if rep.failed():
        print("\n" + dump, flush=True)
    skipped = [r for r in rep.rows if r[1] == "skip"]
    print("\nпунктов: %d, провалов: %d, не проверено: %d" % (len(rep.rows), len(rep.failed()), len(skipped)), flush=True)
    return 1 if rep.failed() else 0


if __name__ == "__main__":
    sys.exit(main())
