#!/bin/bash
# Демо: светодиоды + LCD независимо

CDIR="$(cd "$(dirname "$0")" && pwd)"
CTL="$CDIR/../aura-indicator/bin/aura-ctl"
pad() { local s="$1"; printf "%-16s" "${s:0:16}"; }
join() { echo "$(pad "$1")$(pad "$2")"; }

echo "=== Indicator LED + LCD independent demo ==="

$CTL lcd "$(join "RGB party time!" "6 colors!")"
sleep 1

echo "Red indicator..."
$CTL set led2 static 255,0,0
sleep 1

echo "Green indicator..."
$CTL set led2 static 0,255,0
sleep 1

echo "Blue indicator..."
$CTL set led2 static 0,0,255
sleep 1

echo "Purple indicator..."
$CTL set led2 static 255,0,255
sleep 1

echo "Cyan indicator..."
$CTL set led2 static 0,255,255
sleep 1

echo "White indicator..."
$CTL set led2 static 255,255,255
sleep 1

echo "LCD still alive!"
$CTL lcd-line 0 "LCD survived!"
$CTL lcd-line 1 "all 6 colors!"
sleep 1

echo "Blink party..."
$CTL blink 255,0,0 3 150
sleep 0.5
$CTL blink 0,255,0 3 150
sleep 0.5
$CTL blink 0,0,255 3 150
sleep 0.5

$CTL lcd "$(join "Party over :(" "See you later!")"
$CTL off led2
echo "Done!"
