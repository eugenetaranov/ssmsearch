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
# Search parameters: prints "name value" for every match (all terms must match)
ssmsearch prod database host
ssmsearch prod database | grep -i password

# Interactive search (TUI): live filter, value preview, enter prints the value
ssmsearch -i
ssmsearch -i prod db          # start with a filter
ssmsearch -i -p /app/         # only keys under /app/
value=$(ssmsearch -i prod db) # UI draws on stderr, chosen value goes to stdout

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
| `-i` | Interactive search UI |
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

- **Interactive TUI** - Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea): type to filter, ↑/↓ to move, enter prints the value, ctrl+y copies it, ctrl+r refreshes the cache
- **Fuzzy search** - Match parameters by substring across name/value
- **Local caching** - Fast repeated searches without API calls
- **Tree view** - Hierarchical display of parameter paths
- **Copy/write** - Manage parameters with confirmation prompts
- **Multi-term AND** - All search terms must match
- **No value caching** - Only parameter names are cached locally in `~/.ssmsearch/<account_id>.cache`; values are always fetched from AWS

## License

MIT
