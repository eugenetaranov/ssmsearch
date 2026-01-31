# ssmsearch

Fast CLI for AWS SSM Parameter Store with fuzzy search and local caching.

## Installation

```bash
# Quick install
curl -fsSL https://raw.githubusercontent.com/eugenetaranov/ssmsearch/main/install.sh | bash

# Homebrew
brew install eugenetaranov/tap/ssmsearch

# From source
go install github.com/eugenetaranov/ssmsearch/cmd/ssmsearch@latest
```

## Usage

```bash
# Search parameters (fuzzy match, all terms must match)
ssmsearch -s prod database host

# List all parameter names
ssmsearch -l

# List parameters under a path
ssmsearch -l -p /app/

# Search and list names only (no values)
ssmsearch -s prod -l

# Tree view
ssmsearch -t
ssmsearch -t -s prod

# Copy parameter to new path(s)
ssmsearch -cp /app/prod/db-host /app/staging/db-host

# Write/update parameter
ssmsearch -w /app/config/key "value"
echo "value" | ssmsearch -w /app/config/key

# Refresh cache
ssmsearch -r
ssmsearch -s -r term   # refresh and search
```

## Flags

| Flag | Description |
|------|-------------|
| `-s` | Search mode - terms as positional args |
| `-l` | List parameter names only (no values) |
| `-t` | Display as tree |
| `-r` | Refresh cache |
| `-p` | Path prefix (default: `/`) |
| `-cp` | Copy parameter: `-cp src dest1 [dest2 ...]` |
| `-w` | Write parameter: `-w /path [value]` |
| `-y` | Skip confirmation prompts |
| `-d` | Decrypt SecureString values (default: true) |
| `-region` | AWS region |
| `-profile` | AWS profile |
| `-v` | Show version |

## Features

- **Fuzzy search** - Match parameters by substring across name/value
- **Local caching** - Fast repeated searches without API calls
- **Tree view** - Hierarchical display of parameter paths
- **Copy/write** - Manage parameters with confirmation prompts
- **Multi-term AND** - All search terms must match

## License

MIT
