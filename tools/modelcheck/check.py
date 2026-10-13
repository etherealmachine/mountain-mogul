"""Compare model OBJs against a reference set: bounds, triangle count,
colours and metadata. Used when porting or reworking models.

    python tools/modelcheck/check.py <reference-dir> <obj>...
"""
import os
import sys


def read(path):
    meta, verts, cols, tris = [], [], set(), 0
    for line in open(path):
        if line.startswith("# slot") or line.startswith("# footprint") or line.startswith("# part"):
            meta.append(line.split()[:2] + [round(float(x), 2) if _num(x) else x for x in line.split()[2:]])
        elif line.startswith("v "):
            f = [float(x) for x in line.split()[1:]]
            verts.append(f[:3])
            if len(f) >= 6:
                cols.add(tuple(round(c, 2) for c in f[3:6]))
        elif line.startswith("f "):
            tris += 1
    lo = [min(v[i] for v in verts) for i in range(3)] if verts else [0] * 3
    hi = [max(v[i] for v in verts) for i in range(3)] if verts else [0] * 3
    return meta, lo, hi, cols, tris


def _num(x):
    try:
        float(x)
        return True
    except ValueError:
        return False


def main():
    ref = sys.argv[1]
    bad = 0
    for path in sys.argv[2:]:
        name = os.path.basename(path)
        rp = os.path.join(ref, name)
        m, lo, hi, cols, tris = read(path)
        if not os.path.exists(rp):
            print(f"{name}: new, {tris} tris, bounds {fmt(lo)}..{fmt(hi)}")
            continue
        rm, rlo, rhi, rcols, rtris = read(rp)
        issues = []
        d = max(max(abs(a - b) for a, b in zip(lo, rlo)), max(abs(a - b) for a, b in zip(hi, rhi)))
        size = max(max(b - a for a, b in zip(rlo, rhi)), 0.01)
        if d > max(0.05, 0.03 * size):
            issues.append(f"bounds {fmt(lo)}..{fmt(hi)} vs {fmt(rlo)}..{fmt(rhi)}")
        if m != rm:
            issues.append(f"meta {m} vs {rm}")
        if tris > max(3 * rtris, rtris + 200):
            issues.append(f"tris {tris} vs {rtris}")
        missing = [c for c in rcols if not any(max(abs(a - b) for a, b in zip(c, n)) < 0.02 for n in cols)]
        if missing:
            issues.append(f"colours missing {missing}")
        status = "OK " if not issues else "BAD"
        bad += bool(issues)
        print(f"{status} {name}: {tris} tris (was {rtris}){'; ' + '; '.join(issues) if issues else ''}")
    sys.exit(1 if bad else 0)


def fmt(v):
    return "(" + ", ".join(f"{x:.2f}" for x in v) + ")"


main()
