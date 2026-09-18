"""Explains why a Task/mise target is (or is not) up to date.

Neither runner tells you which files changed: go-task keeps one checksum per
task, mise one metadata hash, so both can only answer yes/no. This prints the
two things that produce the verdict: which files each sources/outputs glob
actually matches, and which sources changed since the last snapshot taken here.

Usage: python tools/task_why.py [--config taskfile.yml|mise.toml] [task ...] [-n]
"""

import glob
import hashlib
import json
import os
import sys

CONFIGS = ["taskfile.yml", "mise.toml"]
SNAPSHOT_DIR = os.path.join(".task", "why")


def load_tasks(path):
    if path.endswith(".toml"):
        import tomllib

        with open(path, "rb") as f:
            raw = tomllib.load(f).get("tasks", {})
        return {
            name: {
                "sources": task.get("sources") or [],
                "outputs": (
                    []
                    if isinstance(task.get("outputs"), dict)
                    else (task.get("outputs") or [])
                ),
                "auto": isinstance(task.get("outputs"), dict),
            }
            for name, task in raw.items()
        }

    import yaml

    with open(path, encoding="utf-8") as f:
        raw = yaml.safe_load(f).get("tasks", {})
    normalize = lambda entries: [  # noqa: E731
        "!" + e["exclude"] if isinstance(e, dict) else e for e in entries or []
    ]
    return {
        name: {
            "sources": normalize((task or {}).get("sources")),
            "outputs": normalize((task or {}).get("generates")),
            "auto": False,
        }
        for name, task in raw.items()
    }


def split_globs(entries):
    """Splits a sources/outputs list into (include, exclude) patterns."""
    include, exclude = [], []
    for entry in entries or []:
        (exclude if entry.startswith("!") else include).append(entry.lstrip("!"))
    return include, exclude


def expand(pattern):
    return sorted(f for f in glob.glob(pattern, recursive=True) if os.path.isfile(f))


def digest(path):
    h = hashlib.blake2b(digest_size=16)
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def report_globs(label, entries, empty_is_fatal):
    include, exclude = split_globs(entries)
    excluded = {f for pattern in exclude for f in expand(pattern)}
    matched, empty = set(), []

    print(f"  {label}:")
    for pattern in include:
        files = expand(pattern)
        matched.update(files)
        note = ""
        if not files:
            note = "  <- НИЧЕГО НЕ СОВПАЛО" + (
                ", таск всегда грязный" if empty_is_fatal else ""
            )
            empty.append(pattern)
        print(f"    {pattern:<24} {len(files):>5} шт{note}")
    for pattern in exclude:
        print(f"    exclude {pattern:<16} {len(expand(pattern)):>5} исключено")

    return sorted(matched - excluded), empty


def snapshot_path(name):
    return os.path.join(SNAPSHOT_DIR, name.replace(":", "_") + ".json")


def diff_snapshot(name, files, update):
    path = snapshot_path(name)
    current = {f: digest(f) for f in files}
    try:
        with open(path, encoding="utf-8") as f:
            previous = json.load(f)
   except FileNotFoundError:
        previous = None

    if update:
        os.makedirs(SNAPSHOT_DIR, exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            json.dump(current, f, indent=1, sort_keys=True)

    if previous is None:
        return None
    changed = [
        f"  M {f}"
        for f in sorted(current)
        if f in previous and previous[f] != current[f]
    ]
    changed += [f"  + {f}" for f in sorted(set(current) - set(previous))]
    changed += [f"  - {f}" for f in sorted(set(previous) - set(current))]
    return changed


def explain(name, task, update):
    print(f"\n{name}")
    if not task["sources"]:
        print("  без sources — запускается всегда")
        return

    sources, _ = report_globs("sources", task["sources"], empty_is_fatal=False)
    print(f"    {'':<24} {len(sources):>5} шт после exclude")
    empty = []
    if task["auto"]:
        print("  outputs: auto — выходы не проверяются, только источники")
    else:
        _, empty = report_globs("outputs", task["outputs"], empty_is_fatal=True)

    changed = diff_snapshot(name, sources, update)
    if changed is None:
        print(
            "\n  снимка ещё нет"
            + (": создан, изменения видны со следующего запуска" if update else "")
        )
    elif changed:
        print(f"\n  изменилось источников: {len(changed)}")
        print("\n".join(changed))
    else:
        print("\n  источники не менялись с прошлого снимка")

    if empty:
        print(
            f"  ВЕРДИКТ: пересборка при каждом запуске — пустые generates globs: {', '.join(empty)}"
        )
    elif changed:
        print("  ВЕРДИКТ: пересборка из-за изменившихся источников (см. список выше)")
    else:
        print("  ВЕРДИКТ: таск актуален")


def main():
    argv = sys.argv[1:]
    config = None
    if "--config" in argv:
        i = argv.index("--config")
        config = argv[i + 1]
        argv = argv[:i] + argv[i + 2 :]
    args = [a for a in argv if a not in ("-n", "--no-update")]
    update = len(args) == len(argv)

    os.chdir(os.environ.get("PROJECT_ROOT", "."))
    config = config or next((c for c in CONFIGS if os.path.exists(c)), None)
    if config is None:
        sys.exit(f"не нашёл конфиг ({', '.join(CONFIGS)})")

    tasks = load_tasks(config)
    names = args or [n for n, t in tasks.items() if t["sources"]]
    print(f"конфиг: {config}")
    for name in names:
        if name not in tasks:
            sys.exit(f"нет такого таска: {name}")
        explain(name, tasks[name], update)


if __name__ == "__main__":
    main()
