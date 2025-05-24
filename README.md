# cx

A command line tool for cut and paste operations on files and directories.

## Installation

```bash
go install github.com/pkitazos/cx@latest
```

## Usage

Cut a file or directory:
```bash
cx /path/to/file
```

Paste (move) the most recent item:
```bash
cx paste
```

Keep pasting (copy) the most recent item:
```bash
cx paste --persist
```

List clipboard contents:
```bash
cx list
```

Clear clipboard:
```bash
cx clear
```

## Commands

- `cx [path]` - Cut a file or directory to clipboard
- `cx paste` - Paste most recent clipboard entry (moves file)
- `cx paste -p` - Paste most recent clipboard entry (copies file)
- `cx list` - Show all clipboard entries
- `cx clear` - Clear all clipboard entries

Files are stored in `~/.cx_clipboard.json` and persist between sessions.