#!/bin/bash

DIALOGCMD="./susedialog"

# Build the binary if it doesn't exist
if [ ! -f "$DIALOGCMD" ]; then
	echo "Building $DIALOGCMD..."
  go build -o "$DIALOGCMD" . || exit 1
fi
$DIALOGCMD --msgbox "Hello openSUSE 🚀" 10 40
echo $?

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
