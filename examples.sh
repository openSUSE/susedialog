#!/bin/bash

set -u

DIALOGCMD="./susedialog"
TEXTBOX_FILE="$(mktemp)"
RESULT_FILE="$(mktemp)"

cleanup() {
  rm -f "$TEXTBOX_FILE" "$RESULT_FILE"
}
trap cleanup EXIT

print_section() {
  echo ""
  echo "== $1 =="
}

capture_dialog_output() {
  local label="$1"
  shift
  : > "$RESULT_FILE"
  "$DIALOGCMD" "$@" --output-fd 3 3>"$RESULT_FILE"
  local rc=$?
  echo "$label exit=$rc"
  if [ -s "$RESULT_FILE" ]; then
    echo "$label output:"
    cat "$RESULT_FILE"
  fi
}

# Build the binary if it doesn't exist
if [ ! -f "$DIALOGCMD" ]; then
  echo "Building $DIALOGCMD..."
  go build -o "$DIALOGCMD" . || exit 1
fi

po_count=0
pot_count=0
if [ -d "po" ]; then
  shopt -s nullglob
  po_files=(po/*.po)
  pot_files=(po/*.pot)
  shopt -u nullglob
  po_count=${#po_files[@]}
  pot_count=${#pot_files[@]}
fi

supported_languages=$((pot_count + po_count))
if [ "$supported_languages" -eq 0 ]; then
  # Source strings are English, so at least one language is always available.
  supported_languages=1
fi

cat > "$TEXTBOX_FILE" <<'EOF'
This is a textbox demo.

It shows file-backed content, wrapping, and scrolling.
Use arrow keys, PageDown, End and Enter to leave the widget.
EOF

print_section "Message Box"
"$DIALOGCMD" \
  --clear \
  --backtitle "openSUSE Demo Suite" \
  --title "Message" \
  --ok-label "Proceed" \
  --msgbox "Hello openSUSE\n\n**Bold markers** are supported too." 12 52
echo "MSGBOX_EXIT=$?"

print_section "Info Box"
"$DIALOGCMD" --title "Status" --infobox "Refreshing repositories..." 8 50
echo "INFOBOX_EXIT=$?"

print_section "Text Box"
"$DIALOGCMD" --title "Release Notes" --exit-label "Done" --textbox "$TEXTBOX_FILE" 14 60
echo "TEXTBOX_EXIT=$?"

print_section "Alignment"
"$DIALOGCMD" --align topleft --title "Alignment" --msgbox "Top-left aligned dialog" 10 50
"$DIALOGCMD" --align center --title "Alignment" --msgbox "Centered dialog" 10 50

print_section "Themes"
SUSEDIALOG_DEBUG_THEME=1 "$DIALOGCMD" --theme opensuse --msgbox "Theme demo: opensuse" 10 50
SUSEDIALOG_DEBUG_THEME=1 "$DIALOGCMD" --theme high-contrast --msgbox "Theme demo: high-contrast" 10 50
SUSEDIALOG_DEBUG_THEME=1 "$DIALOGCMD" --theme rainbow --msgbox "Theme demo: rainbow" 10 50

print_section "Localization (Czech Locale)"
LANG=cs_CZ.UTF-8 LANGUAGE=cs_CZ "$DIALOGCMD" \
  --title "Localization" \
  --msgbox "We support ${supported_languages} languages." 10 50
echo "LOCALIZATION_EXIT=$?"

print_section "Yes/No"
"$DIALOGCMD" \
  --title "Confirmation" \
  --yes-label "Continue" \
  --no-label "Abort" \
  --cancel-label "Abort" \
  --yesno "Continue with migration?" 10 50
echo "YESNO_EXIT=$?"

print_section "Input Box"
capture_dialog_output "INPUTBOX" \
  --title "Hostname" \
  --inputbox "Enter the system hostname:" 10 50

print_section "Password Box"
capture_dialog_output "PASSWORDBOX" \
  --title "Credentials" \
  --insecure \
  --passwordbox "Enter the registration code:" 10 50

print_section "Menu"
capture_dialog_output "MENU" \
  --title "Distribution" \
  --default-item 2 \
  --menu "Pick one" 15 50 5 \
  1 "Tumbleweed" \
  2 "Leap" \
  3 "Slowroll"

print_section "Checklist"
capture_dialog_output "CHECKLIST" \
  --title "Repositories" \
  --checklist "Select repos" 20 60 10 \
  packman "Multimedia repo" on \
  chrome "Google Chrome" off \
  vscode "VSCode repo" on

print_section "Radiolist"
capture_dialog_output "RADIOLIST" \
  --title "Desktop" \
  --default-item kde \
  --radiolist "Select default desktop" 18 60 8 \
  kde "KDE Plasma" on \
  gnome "GNOME" off \
  xfce "XFCE" off

print_section "Form"
capture_dialog_output "FORM" \
  --title "Registration" \
  --form "Enter credentials" 20 60 10 \
  "Email:" 1 1 "user@example.com" 1 25 25 50 \
  "Code:"  2 1 "ABC-123" 2 25 25 50

print_section "Mixed Form"
capture_dialog_output "MIXEDFORM" \
  --title "User Setup" \
  --mixedform "Create account" 20 60 10 \
  "Username:" 1 1 "demo" 1 25 25 50 0 \
  "Password:" 2 1 "secret" 2 25 25 50 1 \
  "Read-only note" 3 1 "Managed by installer" 3 25 25 50 2

print_section "Newline Expansion"
"$DIALOGCMD" --title "Expanded Newlines" --msgbox 'This expands to two lines: first\nsecond' 10 60

print_section "No-NL Expand"
"$DIALOGCMD" --title "Literal Newlines" --no-nl-expand --msgbox 'This keeps the literal sequence: first\nsecond' 10 60

print_section "No-Collapse"
"$DIALOGCMD" --title "Compatibility" --no-collapse --msgbox "Accepted for compatibility; currently has no layout effect." 10 60

print_section "Gauge"
"$DIALOGCMD" --title "Gauge" --gauge "Installing packages..." 10 50
echo "GAUGE_EXIT=$?"

print_section "Progress"
echo "Progress widget with animated tail for 4 seconds:"
if command -v timeout >/dev/null 2>&1; then
  timeout --foreground --signal=INT 4s "$DIALOGCMD" --progress "Downloading package..." 10 50 2>/dev/null
  rc=$?
  if [ "$rc" -eq 124 ]; then
    echo "progress demo finished"
  else
    echo "progress demo exited with code $rc"
  fi
else
  echo "No 'timeout' command available, auto-stopping after 4s"
  "$DIALOGCMD" --progress "Downloading package..." 10 50 &
  pid=$!
  sleep 4
  kill -INT "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  echo "progress demo stopped"
fi

print_section "Issue Tracker"
"$DIALOGCMD" \
  --theme rainbow \
  --title "Need Another Widget?" \
  --msgbox "If you're missing a feature, please open an issue at:\n\nhttps://github.com/openSUSE/susedialog/issues" 12 72
