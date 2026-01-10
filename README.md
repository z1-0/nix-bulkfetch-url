<!-- prettier-ignore -->
# nix-bulkfetch-url

Download files from URLs and print their hashes. Does not touch the Nix store. Downloads run in parallel.

## Features

- Concurrent with configurable worker pool (default: 16)
- Archive unpacking: supports `.tar.gz`, `.tar.xz`, `.tar.bz2`, `.tar.zst`, and `.zip`, computes NAR hashes via `nix-hash`
- Retry with exponential backoff (3 attempts by default)
- Fail-fast mode to abort on first error
- Zero dependencies: stdlib only, builds as a single static binary

## Install

Run directly without installing:

```bash
nix run github:z1-0/nix-bulkfetch-url
```

Or install permanently:

```bash
nix profile install github:z1-0/nix-bulkfetch-url
```

### As a flake input

Add to your flake:

```nix
inputs.nix-bulkfetch-url.url = "github:z1-0/nix-bulkfetch-url";
```

**NixOS system-wide** (in `configuration.nix`):

```nix
environment.systemPackages = [
  inputs.nix-bulkfetch-url.packages.${pkgs.system}.default
];
```

**Home Manager** (in `home.nix`):

```nix
home.packages = [
  inputs.nix-bulkfetch-url.packages.${pkgs.system}.default
];
```

## Usage

Feed URLs via stdin, one per line. Lines starting with `#` and empty lines are ignored.

```bash
echo "https://example.com/archive.tar.gz" | nix-bulkfetch-url
```

### Flags

| Flag          | Default  | Description                                                 |
| ------------- | -------- | ----------------------------------------------------------- |
| `-j`          | `16`     | Number of concurrent workers                                |
| `--type`      | `sha256` | Hash algorithm: `md5`, `sha1`, `sha256`, `sha512`, `blake3` |
| `--format`    | `sri`    | Hash output format: `base16`, `base32`, `base64`, `sri`     |
| `--unpack`    | `false`  | Unpack archive and compute NAR hash                         |
| `--json`      | `false`  | Output JSON format                                          |
| `--timeout`   | `300`    | Download timeout in seconds                                 |
| `--fail-fast` | `false`  | Exit on first error                                         |

### Exit codes

| Code | Meaning                          |
| ---- | -------------------------------- |
| `0`  | All URLs succeeded               |
| `1`  | Some URLs failed, some succeeded |
| `2`  | No URLs succeeded (or no input)  |

## Examples

### Hash a single URL

```bash
echo "https://github.com/user/repo/archive/v1.0.0.tar.gz" | nix-bulkfetch-url
```

### Hash multiple URLs from a file

```bash
cat urls.txt | nix-bulkfetch-url -j 8 --json
```

### Unpack and compute NAR hash

This downloads each archive, extracts it, and computes a NAR hash on the unpacked directory—the same hash you'd put in a Nix derivation's `sha256` field.

```bash
cat urls.txt | nix-bulkfetch-url --unpack
```

### Pipe URLs from a script

```bash
grep -oP 'https://[^\s"]+\.tar\.gz' packages.nix | nix-bulkfetch-url --type blake3
```

### JSON output for scripting

```bash
cat urls.txt | nix-bulkfetch-url --json | jq '.[] | select(.error == null)'
```

## How it works

1. Reads URLs from stdin
2. Downloads them concurrently using a pool of N workers
3. For each URL: fetches to a temp directory, unpacks if `--unpack` is set, then hashes with `nix-hash`
4. Prints one hash per line (or a JSON array with `--json`)
