# susedialog

`susedialog` is an openSUSE-focused, Bubble Tea-based compatibility shim for a small subset of `dialog`.

It is intentionally narrow: the initial target is `opensuse-migration-tool` and similar shell tools.

## Supported widgets in v0.1.0

- `--msgbox`
- `--menu`
- `--checklist`
- `--form`
- `--title`
- `--backtitle`
- `--clear` (accepted and ignored)

## Compatibility behavior

Like `dialog`, this tool:

- draws the UI on standard output
- writes the selected value(s) to standard error
- returns a non-zero exit status when cancelled

That means existing shell snippets such as this should keep working:

```bash
CHOICE=$(susedialog --clear \
  --title "Example" \
  --menu "Pick one:" \
  20 60 10 \
  1 "One" \
  2 "Two" \
  2>&1 >/dev/tty) || exit
```

## Build

```bash
go build .
```

## Notes

This is not meant to be a full clone of `dialog`. The goal is to provide a polished openSUSE-branded terminal UI for the specific widgets openSUSE tools actually use.
