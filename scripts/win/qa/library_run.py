"""Запуск exe с зондом win-qa «образцы наборов» (спека 0.31/144): процесс, чтение файла записей, снятие памяти.
Только stdlib и agents_exe (ps, write_text). Файл в UTF-8."""

import importlib.util
import json
import os
import subprocess
import threading
import time

HERE = os.path.dirname(os.path.abspath(__file__))


def _sibling(name):
    """Соседний модуль по пути: под `python -I` папка скрипта в sys.path не входит."""
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, name + ".py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


ax = _sibling("agents_exe")
PROBE_JS = os.path.join(HERE, "library_probe.js")

def read_new(path, offset):
    """Новые полные строки файла записей с байта offset: ([запись], новый offset). Нет файла - пусто.
    Неполная последняя строка остаётся до следующего чтения; негодная строка - запись step=unparsable."""
    try:
        with open(path, "rb") as f:
            f.seek(offset)
            chunk = f.read()
    except OSError:
        return [], offset
    end = chunk.rfind(b"\n")
    if end < 0:
        return [], offset
    recs = []
    for raw in chunk[:end].split(b"\n"):
        if not raw.strip():
            continue
        try:
            obj = json.loads(raw.decode("utf-8"))
            recs.append(obj if isinstance(obj, dict) else {"step": "unparsable", "raw": str(obj)[:200]})
        except ValueError:
            recs.append({"step": "unparsable", "raw": raw[:200].decode("utf-8", "replace")})
    return recs, offset + end + 1


# ---------- память процессов ----------

PROC_PS = ("Get-CimInstance Win32_Process | ForEach-Object { '{0}|{1}|{2}|{3}|{4}' -f "
           "$_.ProcessId, $_.ParentProcessId, $_.Name, $_.PrivatePageCount, $_.WorkingSetSize }")


def parse_proc_rows(text):
    """Строки pid|ppid|имя|private|ws -> [(pid, ppid, name, private, ws)]; мусор и пустые числа пропускаются."""
    rows = []
    for ln in text.splitlines():
        parts = ln.strip().split("|")
        if len(parts) != 5:
            continue
        try:
            rows.append((int(parts[0]), int(parts[1]), parts[2], int(parts[3] or 0), int(parts[4] or 0)))
        except ValueError:
            continue
    return rows


def mem_from_rows(rows, root):
    """Память exe (root) и всего его дерева потомков; None - процесса нет в таблице."""
    by_parent = {}
    own = None
    for r in rows:
        by_parent.setdefault(r[1], []).append(r)
        if r[0] == root:
            own = r
    if own is None:
        return None
    tree, todo, seen = [own], [root], {root}
    while todo:
        for r in by_parent.get(todo.pop(), []):
            if r[0] not in seen:
                seen.add(r[0])
                tree.append(r)
                todo.append(r[0])
    return {"own": own[3], "own_ws": own[4], "tree": sum(r[3] for r in tree), "tree_ws": sum(r[4] for r in tree), "procs": len(tree)}


def mem_sample(pid):
    code, out = ax.ps(PROC_PS, timeout=60)
    return mem_from_rows(parse_proc_rows(out), pid) if code == 0 else None


class MemSampler:
    """Пик памяти за весь запуск: поток снимает значение раз в period секунд; sample_now - разовое чтение для отметок."""

    def __init__(self, pid, sample=mem_sample, period=2.0):
        self.pid, self._sample, self.period = pid, sample, period
        self.peak = {"own": 0, "tree": 0}
        self.n = 0
        self._stop = threading.Event()
        self._th = threading.Thread(target=self._loop, daemon=True)

    def start(self):
        self._th.start()

    def stop(self):
        self._stop.set()
        self._th.join(10)

    def _take(self):
        try:
            m = self._sample(self.pid)
        except Exception:  # снятие памяти не должно ронять прогон
            m = None
        if m:
            self.n += 1
            self.peak["own"] = max(self.peak["own"], m["own"])
            self.peak["tree"] = max(self.peak["tree"], m["tree"])
        return m

    def sample_now(self):
        return self._take()

    def _loop(self):
        while not self._stop.wait(self.period):
            self._take()


# ---------- запуск exe с зондом ----------

def kill_tree(proc):
    """Снять процесс и его дерево по PID, который запустили сами (по имени не убиваем)."""
    if proc.poll() is not None:
        return
    if os.name == "nt":
        subprocess.run(["taskkill", "/PID", str(proc.pid), "/T", "/F"], capture_output=True)
    else:  # тесты на Linux
        proc.kill()
    try:
        proc.wait(15)
    except subprocess.TimeoutExpired:
        pass


def probe_script(cfg):
    """Текст для SA_QA_PROBE: функция main из library_probe.js и её вызов с настройками (JSON - допустимый JS)."""
    with open(PROBE_JS, encoding="utf-8") as f:
        return f.read() + "\nmain(" + json.dumps(cfg, ensure_ascii=False) + ");\n"


def run_probe(exe, env, workdir, name, cfg, timeout, sampler_cls=MemSampler, poll=0.25):
    """Один запуск exe с зондом; ждёт запись step=done, выход exe или timeout. Результат:
    {records, done, alive (exe жив к концу), code, peak, started_at, seconds}. На каждой отметке mark в запись кладётся mem."""
    os.makedirs(workdir, exist_ok=True)
    js, out = os.path.join(workdir, name + ".js"), os.path.join(workdir, name + ".jsonl")
    ax.write_text(js, probe_script(cfg))
    if os.path.exists(out):
        os.remove(out)
    penv = dict(env, SA_QA_PROBE=js, SA_QA_PROBE_OUT=out)
    t0 = time.monotonic()
    proc = subprocess.Popen([exe], env=penv, cwd=os.path.dirname(exe))
    sampler = sampler_cls(proc.pid)
    sampler.start()
    recs, offset, done = [], 0, False
    try:
        while time.monotonic() - t0 < timeout and not done:
            new, offset = read_new(out, offset)
            for r in new:
                if r.get("step") == "mark":
                    r["mem"] = sampler.sample_now()
                recs.append(r)
                done = done or r.get("step") == "done"
            if not done and proc.poll() is not None:
                new, offset = read_new(out, offset)
                recs += new
                break
            if not done:
                time.sleep(poll)
        alive = proc.poll() is None
    finally:
        sampler.stop()
        code = proc.poll()
        kill_tree(proc)
    return {"records": recs, "done": done, "alive": alive, "code": code, "peak": dict(sampler.peak), "samples": sampler.n,
            "seconds": round(time.monotonic() - t0, 1)}


