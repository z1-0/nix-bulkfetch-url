# nix-bulkfetch-url

A concurrent replacement for `nix-prefetch-url` that avoids Nix store locks.

## Why?

`nix-prefetch-url` writes to the Nix store during download, causing lock contention when running concurrently. This tool solves it by:

1. **Downloading/unpacking with Go** - no Nix store writes
2. **Hashing with `nix-hash`** - compatible output
3. **Concurrent worker pool** - efficient batch processing

## Install

```bash
go build -o nix-bulkfetch-url .
```

## Usage

```bash
# Read URLs from stdin
cat urls.txt | nix-bulkfetch-url

# Pipeline
grep -r "fetchurl" . | awk '{print $2}' | nix-bulkfetch-url

# Unpack mode (compute NAR hash of extracted contents)
cat urls.txt | nix-bulkfetch-url --unpack

# JSON output
cat urls.txt | nix-bulkfetch-url --json

# Custom concurrency
cat urls.txt | nix-bulkfetch-url -j 32
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-j` | 16 | Number of concurrent workers |
| `--type` | sha256 | Hash algorithm: md5, sha1, sha256, sha512, blake3 |
| `--unpack` | false | Unpack archive and compute NAR hash |
| `--json` | false | Output JSON format |
| `--timeout` | 300 | Download timeout in seconds |
| `--fail-fast` | false | Exit on first error |

## Input Format

One URL per line, supports empty lines and `#` comments:

```
# Nix source
https://github.com/NixOS/nix/archive/refs/tags/2.19.2.tar.gz
https://github.com/NixOS/nixpkgs/archive/refs/tags/23.11.tar.gz

# Other packages
https://example.com/package-1.0.tar.gz
```

## Output Format

### Text mode (default)

One hash per line:

```
0h2v8nd26brmrahfy7n5h6rqck0cnadb7y1z163s0sqrcz6k9yla
1878hsr34654xs9a3db57b6jqmxh1jmaplsd8c8hxyypx1s0m6mw
```

### JSON mode

```json
[
  {"url": "https://...", "hash": "sha256-xxx"},
  {"url": "https://...", "error": "download failed"}
]
```

## Exit Codes

- `0`: All succeeded
- `1`: Partial failure
- `2`: All failed

## Performance

- Default 16 concurrent workers
- Each worker handles: download -> unpack -> hash -> cleanup
- Retry: 3 attempts with exponential backoff (1s, 2s, 4s)

## Dependencies

- Go 1.21+
- `nix-hash` command (from Nix)

## Comparison with nix-prefetch-url

| Feature | nix-prefetch-url | nix-bulkfetch-url |
|---------|------------------|-------------------|
| Concurrent | No | Yes (worker pool) |
| Batch | No | Yes (stdin) |
| Nix store | Writes | Does not write |
| Hash | Compatible | Compatible |
