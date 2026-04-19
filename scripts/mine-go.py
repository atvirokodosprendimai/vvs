#!/usr/bin/env python3
"""mine-go.py — import Go AST chunks into a dedicated ChromaDB collection with Ollama embeddings.

Uses a separate collection `go_code` so it doesn't conflict with MemPalace's
default all-MiniLM-L6-v2 embeddings (768-dim vs 384-dim).

Usage:
    go run ./cmd/gomine ./internal ./cmd | python3 scripts/mine-go.py   # import
    python3 scripts/mine-go.py --search "mark invoice paid"              # search
    go run ./cmd/gomine ./internal | python3 scripts/mine-go.py --dry-run

Options:
    --dry-run       Print chunks without writing
    --skip-tests    Skip *_test.go files
    --search QUERY  Search existing collection (rest of argv = query)
    --n N           Number of search results (default 5)
"""
import hashlib
import json
import sys
import os

DRY_RUN = "--dry-run" in sys.argv
SKIP_TESTS = "--skip-tests" in sys.argv
SEARCH_MODE = "--search" in sys.argv

PALACE_PATH = os.path.expanduser("~/.mempalace/palace")
COLLECTION_NAME = "go_code"
AGENT = "gomine"
OLLAMA_BASE_URL = os.environ.get("OLLAMA_BASE_URL", "http://100.64.79.36:11434")
EMBED_MODEL = os.environ.get("EMBED_MODEL", "nomic-embed-text")
# ChromaDB OllamaEmbeddingFunction uses the old /api/embeddings path
OLLAMA_EMBED_URL = f"{OLLAMA_BASE_URL}/api/embeddings"


def get_collection(create=False):
    import chromadb
    from chromadb.utils.embedding_functions import OllamaEmbeddingFunction
    ef = OllamaEmbeddingFunction(url=OLLAMA_EMBED_URL, model_name=EMBED_MODEL)
    client = chromadb.PersistentClient(path=PALACE_PATH)
    if create:
        return client.get_or_create_collection(
            COLLECTION_NAME,
            embedding_function=ef,
            metadata={"hnsw:space": "cosine"},
        )
    return client.get_collection(COLLECTION_NAME, embedding_function=ef)


def room_for_file(path: str) -> str:
    p = path.replace("\\", "/")
    if "/cmd/" in p:
        return "cmd"
    if "/e2e/" in p:
        return "e2e"
    parts = p.split("/")
    if "modules" in parts:
        idx = parts.index("modules")
        if idx + 1 < len(parts):
            return parts[idx + 1]
    if "/internal/" in p:
        return "internal"
    return "general"


def format_content(chunk: dict) -> str:
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


def chunk_id(source_file: str, start_line: int) -> str:
    h = hashlib.sha256(f"{source_file}:{start_line}".encode()).hexdigest()[:24]
    return f"go_{h}"


def do_import():
    collection = get_collection(create=True)
    added = skipped = errors = 0

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

        if DRY_RUN:
            room = room_for_file(chunk["file"])
            print(f"[dry] {chunk['file']}:{chunk['start_line']} {chunk['kind']} {chunk['symbol']} → room={room}")
            added += 1
            continue

        content = format_content(chunk)
        doc_id = chunk_id(chunk["file"], chunk["start_line"])
        try:
            collection.upsert(
                documents=[content],
                ids=[doc_id],
                metadatas=[{
                    "file": chunk["file"],
                    "package": chunk["package"],
                    "symbol": chunk["symbol"],
                    "kind": chunk["kind"],
                    "receiver": chunk.get("receiver", ""),
                    "room": room_for_file(chunk["file"]),
                    "start_line": chunk["start_line"],
                    "end_line": chunk["end_line"],
                    "added_by": AGENT,
                }],
            )
            added += 1
            if added % 100 == 0:
                print(f"  {added} chunks...", file=sys.stderr)
        except Exception as e:
            print(f"[error] {chunk['file']}:{chunk['start_line']}: {e}", file=sys.stderr)
            errors += 1

    print(f"\ndone: {added} added, {skipped} skipped, {errors} errors", file=sys.stderr)


def do_search(query: str, n: int = 5):
    collection = get_collection(create=False)
    results = collection.query(query_texts=[query], n_results=n)
    docs = results.get("documents", [[]])[0]
    metas = results.get("metadatas", [[]])[0]
    dists = results.get("distances", [[]])[0]
    for doc, meta, dist in zip(docs, metas, dists):
        sim = 1 - dist
        print(f"\n── {meta['file']}:{meta['start_line']} [{meta['kind']}:{meta['symbol']}] sim={sim:.3f}")
        # Print first 10 lines of body
        lines = doc.splitlines()
        print("\n".join(lines[:12]))
        if len(lines) > 12:
            print(f"  ... ({len(lines)-12} more lines)")


def main():
    if SEARCH_MODE:
        idx = sys.argv.index("--search")
        query_parts = sys.argv[idx+1:]
        n = 5
        if "--n" in sys.argv:
            ni = sys.argv.index("--n")
            n = int(sys.argv[ni+1])
            query_parts = [p for p in query_parts if p not in ("--n", sys.argv[ni+1])]
        query = " ".join(query_parts)
        if not query:
            print("Usage: mine-go.py --search <query>", file=sys.stderr)
            sys.exit(1)
        do_search(query, n)
    else:
        do_import()


if __name__ == "__main__":
    main()
