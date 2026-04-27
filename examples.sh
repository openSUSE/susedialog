#!/bin/bash

DIALOGCMD="./susedialog"

# Build the binary if it doesn't exist
if [ ! -f "$DIALOGCMD" ]; then
	echo "Building $DIALOGCMD..."
  go build -o "$DIALOGCMD" . || exit 1
fi
$DIALOGCMD --msgbox "Hello openSUSE 🚀" 10 40
echo $?

echo ""
echo "Infobox demo (non-interactive, exits immediately):"
$DIALOGCMD --title "Status" --infobox "Refreshing repositories..." 8 50
echo "INFOBOX_EXIT=$?"

echo ""
echo "Alignment demo: topleft (default flow)"
$DIALOGCMD --align topleft --title "Alignment" --msgbox "Top-left aligned dialog" 10 50

echo ""
echo "Alignment demo: center (modal style)"
$DIALOGCMD --align center --title "Alignment" --msgbox "Centered dialog" 10 50

echo ""
echo "Theme demo: opensuse (debug enabled)"
SUSEDIALOG_DEBUG_THEME=1 $DIALOGCMD --theme opensuse --msgbox "Theme demo: opensuse" 10 50

echo ""
echo "Theme demo: high-contrast (debug enabled)"
SUSEDIALOG_DEBUG_THEME=1 $DIALOGCMD --theme high-contrast --msgbox "Theme demo: high-contrast" 10 50

echo ""
echo "Theme demo: rainbow (debug enabled)"
SUSEDIALOG_DEBUG_THEME=1 $DIALOGCMD --theme rainbow --msgbox "Theme demo: rainbow" 10 50

$DIALOGCMD --yesno "Continue with migration?" 10 50
echo "YESNO_EXIT=$?"

CHOICE=$($DIALOGCMD \
  --menu "Pick one" 15 50 5 \
  1 "Tumbleweed" \
  2 "Leap" \
  3 "Slowroll" \
  2>&1 >/dev/tty)

echo "CHOICE=$CHOICE"

CHOICE=$($DIALOGCMD \
  --checklist "Select repos" 20 60 10 \
  packman "Multimedia repo" on \
  chrome "Google Chrome" off \
  vscode "VSCode repo" on \
  2>&1 >/dev/tty)

echo "CHOICE=$CHOICE"

RESULT=$($DIALOGCMD \
  --form "Enter credentials" 20 60 10 \
  "Email:" 1 1 "" 1 25 25 50 \
  "Code:"  2 1 "" 2 25 25 50 \
  2>&1 >/dev/tty)

echo "RESULT:"
echo "$RESULT"

echo ""
echo "Progress widget with chameleon tail animation:"
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
