"""Статические проверки PowerShell-скриптов scripts/win (прогон 2 спеки 118):
1) имена переменных не различают регистр - параметр функции нельзя присваивать в теле ($last = ... затирает $Last);
   исключение - строка с комментарием `# param-reassign-ok`;
2) Start-Process -PassThru без -Wait: перед чтением .ExitCode нужен `$null = $<var>.Handle`, иначе ExitCode пустой."""

import glob
import os
import re
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
FILES = sorted(
    glob.glob(os.path.join(HERE, "**", "*.ps1"), recursive=True)
    + glob.glob(os.path.join(HERE, "**", "*.psm1"), recursive=True)
)


def read(path):
    with open(path, encoding="utf-8-sig") as f:
        return f.read()


def match_close(text, i, op, cl):
    """Индекс закрывающей скобки для открывающей в позиции i; кавычки и комментарии не разбираются."""
    depth = 0
    for k in range(i, len(text)):
        c = text[k]
        if c == op:
            depth += 1
        elif c == cl:
            depth -= 1
            if depth == 0:
                return k
    return -1


FUNC = re.compile(r"^function\s+([\w-]+)\s*(\(|\{)", re.M | re.I)


def functions(text):
    """(имя, параметры, тело) для каждой function верхнего уровня."""
    for m in FUNC.finditer(text):
        name = m.group(1)
        if m.group(2) == "(":
            end = match_close(text, m.end() - 1, "(", ")")
            params_src = text[m.end() : end]
            b = text.find("{", end)
            body_from = b
        else:
            b = m.end() - 1
            pm = re.match(r"\s*param\s*\(", text[b + 1 :], re.I)
            params_src = ""
            body_from = b
            if pm:
                ps = b + 1 + pm.end() - 1
                pe = match_close(text, ps, "(", ")")
                params_src = text[ps + 1 : pe]
                body_from = pe  # сам param(...) телом не считается: там значения по умолчанию с "="
        e = match_close(text, b, "{", "}")
        if b < 0 or e < 0:
            continue
        # имена параметров: $Имя, за которым запятая, "=", скобка или конец строки; $script:X в значениях по умолчанию не берём
        names = set(re.findall(r"(?:^|[\s,\]\(])\$([A-Za-z_]\w*)(?=\s*(?:[,=\)\r\n]|$))", params_src))
        yield name, names, text[body_from : e + 1]


class PsLint(unittest.TestCase):
    def test_files_found(self):
        self.assertGreater(len(FILES), 5)

    def test_param_not_reassigned(self):
        bad = []
        for path in FILES:
            for name, params, body in functions(read(path)):
                for line in body.splitlines():
                    if "param-reassign-ok" in line:
                        continue
                    for p in params:
                        if re.search(r"(?<![\w$:.])\$" + re.escape(p) + r"\s*(?:[-+*/]|\?\?)?=(?!=)", line, re.I):
                            bad.append(f"{os.path.relpath(path, HERE)}: {name}: параметр ${p} присваивается: {line.strip()}")
        self.assertEqual(bad, [], "\n".join(bad))

    def test_detects_known_collision(self):
        src = "function F {\n param([string]$Last = 'a')\n $last = $null\n}\n"
        fn = list(functions(src))
        self.assertEqual(fn[0][1], {"Last"})
        self.assertTrue(re.search(r"\$Last\s*=", fn[0][2], re.I))

    def test_passthru_exitcode_needs_handle(self):
        bad = []
        for path in FILES:
            text = read(path)
            for m in re.finditer(r"^\s*\$(\w+)\s*=\s*Start-Process\b([^\r\n]*)", text, re.M | re.I):
                var, rest = m.group(1), m.group(2)
                if not re.search(r"-PassThru\b", rest, re.I) or re.search(r"-Wait\b", rest, re.I):
                    continue  # с -Wait ExitCode заполняется сам
                if not re.search(r"\$" + var + r"\.ExitCode\b", text, re.I):
                    continue
                if not re.search(r"\$null\s*=\s*\$" + var + r"\.Handle\b", text, re.I):
                    bad.append(f"{os.path.relpath(path, HERE)}: ${var} - нет `$null = ${var}.Handle`")
        self.assertEqual(bad, [], "\n".join(bad))


if __name__ == "__main__":
    unittest.main()
