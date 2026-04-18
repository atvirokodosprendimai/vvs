#!/usr/bin/env python3
"""mine-go.py — import Go AST chunks from gomine into MemPalace.

Usage:
    go run ./cmd/gomine [dir] | python3 scripts/mine-go.py [--dry-run]
    go run ./cmd/gomine ./internal ./cmd | python3 scripts/mine-go.py

Options:
    --dry-run   Print chunks without writing to palace
    --skip-tests  Skip *_test.go files
"""
import json
import sys
import os

DRY_RUN = "--dry-run" in sys.argv
SKIP_TESTS = "--skip-tests" in sys.argv

PALACE_PATH = os.path.expanduser("~/.mempalace/palace")
WING = "vvs"
AGENT = "gomine"


def room_for_file(path: str) -> str:
    """Map file path to a MemPalace room."""
    p = path.replace("\\", "/")
    if "/cmd/" in p:
        return "cmd"
    if "/e2e/" in p:
        return "e2e"
    # internal/modules/{module}/... → module name as room
    parts = p.split("/")
    if "modules" in parts:
        idx = parts.index("modules")
        if idx + 1 < len(parts):
            return parts[idx + 1]
    if "/internal/" in p:
        return "internal"
    return "general"


def format_content(chunk: dict) -> str:
    """Format chunk into human+LLM readable drawer content."""
    lines = []
    recv = f" (receiver: {chunk['receiver']})" if chunk.get("receiver") else ""
    lines.append(f"// {chunk['file']}:{chunk['start_line']}-{chunk['end_line']}")
    lines.append(f"// package {chunk['package']} | {chunk['kind']}: {chunk['symbol']}{recv}")
    if chunk.get("signature"):
        lines.append(f"// sig: {chunk['signature']}")
    if chunk.get("doc"):
        for dl in chunk["doc"].splitlines():
            lines.append(f"// {dl}")
    lines.append("")
    lines.append(chunk["body"])
    return "\n".join(lines)


def main():
    if not DRY_RUN:
        from mempalace.miner import add_drawer, get_collection
        collection = get_collection(PALACE_PATH)

    added = 0
    skipped = 0
    errors = 0

    for lineno, line in enumerate(sys.stdin):
        line = line.strip()
        if not line:
            continue
        try:
            chunk = json.loads(line)
        except json.JSONDecodeError as e:
            print(f"[warn] line {lineno}: bad JSON: {e}", file=sys.stderr)
            errors += 1
            continue

        if SKIP_TESTS and chunk.get("file", "").endswith("_test.go"):
            skipped += 1
            continue

        if not chunk.get("body"):
            skipped += 1
            continue

        room = room_for_file(chunk["file"])
        content = format_content(chunk)

        if DRY_RUN:
            print(f"[dry] {chunk['file']}:{chunk['start_line']} {chunk['kind']} {chunk['symbol']} → wing={WING} room={room}")
            added += 1
            continue

        try:
            add_drawer(
                collection=collection,
                wing=WING,
                room=room,
                content=content,
                source_file=chunk["file"],
                chunk_index=chunk["start_line"],
                agent=AGENT,
            )
            added += 1
            if added % 100 == 0:
                print(f"  {added} chunks imported...", file=sys.stderr)
        except Exception as e:
            print(f"[error] {chunk['file']}:{chunk['start_line']}: {e}", file=sys.stderr)
            errors += 1

    print(f"\ndone: {added} added, {skipped} skipped, {errors} errors", file=sys.stderr)


if __name__ == "__main__":
    main()
