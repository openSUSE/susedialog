# susedialog

```text
+==============================================================+
||                      SUSEDIALOG 0.1                        ||
||                   openSUSE dialog shim                     ||
||                                                            ||
||   Widgets: [ msgbox ] [ menu ] [ checklist ]               ||
||            [ form ] [ mixedform ] [ progress ]             ||
||            [ inputbox ] [ passwordbox ] [ infobox ]        ||
||                                                            ||
||   > [ OK ]   [ Cancel ]   [ Help ]                         ||
||                                                            ||
||   Bubble Tea compatibility for openSUSE tooling            ||
||   Targets: opensuse-migration-tool, jeos-firstboot         ||
+==============================================================+
```
<img width="400" height="158" alt="examples" src="https://raw.githubusercontent.com/openSUSE/susedialog/main/examples.gif" />

`susedialog` is an openSUSE-focused, Bubble Tea-based compatibility shim for a small subset of `dialog`.

It is intentionally narrow: the initial target is openSUSE system tooling that currently includes:

- `opensuse-migration-tool`
- `jeos-firstboot`

## Supported widgets

- `--msgbox`
- `--infobox` (non-blocking, exits immediately with success)
- `--textbox`
- `--yesno`
- `--gauge` (currently mapped to the progress view)
- `--inputbox`
- `--passwordbox`
- `--menu`
- `--checklist`
- `--radiolist`
- `--form`
- `--mixedform`
- `--progress`

## Supported common options

- `--title`
- `--backtitle`
- `--ok-label`
- `--cancel-label`
- `--yes-label`
- `--no-label`
- `--exit-label`
- `--output-fd`
- `--default-item`
- `--theme`
- `--align`
- `--no-nl-expand`
- `--no-collapse` (accepted for compatibility)
- `--insecure`
- `--clear`

`--clear` is enabled by default in `susedialog` for screen-to-screen transitions.

## Compatibility behavior

Like `dialog`, this tool:

- draws the UI on standard output
- writes the selected value(s) to standard error
- returns a non-zero exit status when cancelled

Keyboard interrupt behavior matches `dialog` expectations:

- `Esc` follows the regular cancel path (exit status `1`)
- `Ctrl+C` is treated as an interrupt and exits with status `130`
- `Ctrl+C` propagation uses `SIGINT` process-group semantics so wrapper applications can react like with original `dialog`

For `--infobox`, `susedialog` renders a non-interactive status box using the provided height/width and exits immediately with status `0`.

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

And for non-interactive status updates:

```bash
susedialog --title "Status" --infobox "Refreshing repositories..." 8 50
```

## Build

```bash
go build .
```

To embed a commit hash for `--version` output:

```bash
go build -ldflags "-X main.gitCommit=$(git rev-parse --short HEAD)" .
```

## Localization

`susedialog` now includes gettext-style localization backed by PO files.

- Template catalog: `po/en.pot`

At runtime, locale selection follows `LC_ALL`, `LC_MESSAGES`, `LANGUAGE`, then `LANG`.
`susedialog` tries the full locale first (for example `cs_CZ`), then the language
code (`cs`), and finally falls back to source English `msgid` strings.

This layout is compatible with Weblate workflows based on `po/*.po` and a tracked
POT template.

You can contribute translations at <https://l10n.opensuse.org/projects/susedialog/>.

## Notes

This is not meant to be a full clone of `dialog`. The goal is to provide a polished openSUSE-branded terminal UI for the specific widgets openSUSE tools actually use.

Widgets are added in order of practical compatibility value: first the easiest widgets that reuse existing UI building blocks, then widgets that matter for openSUSE tooling and installer flows.

The following `dialog` widgets are intentionally not implemented at the moment:

- `--buildlist`
- `--calendar`
- `--dselect`
- `--editbox`
- `--fselect`
- `--inputmenu`
- `--mixedgauge`
- `--passwordform`
- `--pause`
- `--prgbox`
- `--programbox`
- `--progressbox`
- `--rangebox`
- `--tailbox`
- `--tailboxbg`
- `--timebox`
- `--treeview`

These widgets are omitted because we do not currently expect openSUSE consumers to need them. If you hit one in real usage, please open an issue and it can be added based on actual demand.

### UI details

- Title underline is currently a single PlumPurple line by default.
- Rainbow underline code is intentionally kept commented in the source for possible future return.
- `--textbox` keeps a stable box width while scrolling and wraps long lines.
- Password fields are masked in password dialogs and mixed/form password entries.

### Theming and Accessibility

`susedialog` now supports named themes, including:

- `opensuse` (default)
- `high-contrast` (accessibility-oriented)
- `rainbow` (multi-color accents and borders)

The `high-contrast` theme is informed by terminal palette legibility research from
https://inai.de/projects/consoleet/palette, favoring VGA-like high-separation
color choices for robust contrast across common terminal pairings.

Theme selection priority (highest to lowest):

1. CLI option: `--theme <name>`
2. Environment: `SUSEDIALOG_THEME`
3. User config: `~/.config/susedialog/config` (or `$XDG_CONFIG_HOME/susedialog/config`)
4. System config: `/etc/susedialog/config`
5. Built-in default: `opensuse`

Config files are read from `~/.config/susedialog/config` or `$XDG_CONFIG_HOME/susedialog/config` for the current user, with `/etc/susedialog/config` as the system-wide fallback.

Config files accept shell-style `key=value` entries, with or without a leading `export`. Supported keys are:

- `theme=<name>` or `SUSEDIALOG_THEME=<name>`
- `align=<topleft|center>` or `SUSEDIALOG_ALIGN=<topleft|center>`
- `theme_toggle_key=<key>` or `SUSEDIALOG_THEME_TOGGLE_KEY=<key>`

When the theme is changed at runtime with the toggle key, `susedialog` writes the selected theme back to the user config file as `SUSEDIALOG_THEME=<name>`. That makes the new theme persist across future sessions and override any system-wide default.

Theme toggle shortcut defaults to `ctrl+t` and can be configured with:

- `SUSEDIALOG_THEME_TOGGLE_KEY` (environment)
- `theme_toggle_key=<key>` (user/system config)

Dialog alignment defaults to `topleft` and can be configured with:

- CLI option: `--align <topleft|center>`
- `SUSEDIALOG_ALIGN` (environment)
- `align=<topleft|center>` (user/system config)

Bundled themes live in the project directory `themes/`.

At runtime, `susedialog` also checks `themes/` relative to the executing binary location and uses those files as overrides. This allows a `$SCRIPTDIR/themes` style deployment.

To debug key handling issues, enable key logging:

```bash
SUSEDIALOG_DEBUG_KEYS=1 ./susedialog --checklist "Select repos" 20 60 10 packman "Multimedia repo" off
```

This prints received key names to standard error while the dialog is running.
