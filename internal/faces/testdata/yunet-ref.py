#!/usr/bin/env python3
"""Эталон для TestYuNetParity: onnxruntime гоняет YuNet на синтетическом входе (та же формула,
что в parity_test.go) и пишет по каждому из 12 выходов 64 значения и среднее.

Разовый запуск, в check не входит; нужен numpy и onnxruntime (venv-media), из папки testdata:
    python yunet-ref.py ../face_detection_yunet_2023mar.onnx ref.json            # вход 640x640
    python yunet-ref.py ../face_detection_yunet_2023mar.onnx ref-448x640.json 448 640   # рабочий вход
Размер входа в файле модели — 640x640 (onnxruntime другой не принимает). Для другого размера
скрипт правит в памяти две цифры формы входа (640 -> H, W): сеть свёрточная, веса те же;
предупреждения onnxruntime о несовпадении форм выходов с записанными в файле ожидаемы.
Среднее считается во float64: сумма float32 у Go и onnxruntime расходится в последних разрядах,
поэлементно же выходы совпадают до 1e-5.
"""
import hashlib
import json
import sys

import numpy as np
import onnxruntime as ort

NATIVE = 640  # размер входа, записанный в модели


def synthetic(h, w):
    # целочисленная формула: вход одинаков до бита в Go и здесь
    x = np.arange(w, dtype=np.int64)[None, :]
    y = np.arange(h, dtype=np.int64)[:, None]
    planes = [((x * 3 + y * 5 + c * 61 + ((x * y) >> 6)) & 255).astype(np.float32) for c in range(3)]
    return np.stack(planes)[None]


def pick(flat):
    # 48 равномерных позиций и 16 самых больших по модулю: края диапазона важнее середины
    n = flat.size
    even = np.linspace(0, n - 1, 48).astype(np.int64)
    big = np.argsort(-np.abs(flat), kind="stable")[:64]
    idx = list(dict.fromkeys(list(even) + list(big)))[:64]
    return sorted(int(i) for i in idx)


def varint(v):
    out = bytearray()
    while v > 0x7F:
        out.append((v & 0x7F) | 0x80)
        v >>= 7
    out.append(v)
    return bytes(out)


def resized_model(raw, h, w):
    # Форма входа — два Dimension{dim_value: 640} подряд (H, W): тег 0x0a, длина 3, поле 1 varint.
    # Двухбайтовые varint меняются на месте, длины сообщений не трогаются.
    pat = b"\x0a\x03\x08" + varint(NATIVE)
    if raw.count(pat) != 2 or not (128 <= h < 16384 and 128 <= w < 16384):
        sys.exit("форма входа в модели не найдена или размер вне 128..16383")
    i = raw.find(pat)
    j = raw.find(pat, i + 1)
    b = bytearray(raw)
    b[i : i + 5] = b"\x0a\x03\x08" + varint(h)
    b[j : j + 5] = b"\x0a\x03\x08" + varint(w)
    return bytes(b)


def main(model, out, h=NATIVE, w=NATIVE):
    raw = open(model, "rb").read()
    sess = ort.InferenceSession(
        raw if (h, w) == (NATIVE, NATIVE) else resized_model(raw, h, w), providers=["CPUExecutionProvider"]
    )
    res = sess.run(None, {"input": synthetic(h, w)})
    doc = {
        "model_sha256": hashlib.sha256(raw).hexdigest(),
        "onnxruntime": ort.__version__,
        "h": h,
        "w": w,
        "outputs": {},
    }
    for meta, arr in zip(sess.get_outputs(), res):
        flat = arr.reshape(-1)
        idx = pick(flat)
        doc["outputs"][meta.name] = {
            "shape": list(arr.shape),
            "mean": float(flat.astype(np.float64).mean()),
            "idx": idx,
            "val": [float(f"{flat[i]:.9g}") for i in idx],
        }
    with open(out, "w", encoding="utf-8") as f:
        json.dump(doc, f, ensure_ascii=False, separators=(",", ":"))
        f.write("\n")


if __name__ == "__main__":
    if len(sys.argv) not in (3, 5):
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2], *map(int, sys.argv[3:]))
