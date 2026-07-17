#!/bin/bash
# Демо: матрица из фильма
# 16+16=32, слова не разрываются

CDIR="$(cd "$(dirname "$0")" && pwd)"
CTL="$CDIR/../aura-indicator/bin/aura-ctl"
pad() { local s="$1"; printf "%-16s" "${s:0:16}"; }
join() { echo "$(pad "$1")$(pad "$2")"; }

echo "=== Matrix demo ==="

$CTL lcd "$(join "Wake up, Neo.." "The Matrix has")"
$CTL set led2 static 0,255,0
sleep 2

$CTL lcd "$(join "01001000 0110" "Follow me...")"
$CTL blink 0,255,0 3 100
sleep 2

$CTL lcd "$(join "01101000 0110" "Knock knock...")"
sleep 2

$CTL lcd "$(join "I know kung-fu" "dodges bullets")"
sleep 2

$CTL lcd "$(join "There is no" "spoon.")"
$CTL set led2 static 0,100,0
sleep 2

$CTL lcd "$(join "Free your mind" "Neo.")"
$CTL blink 0,255,0 5 80
sleep 2

$CTL lcd "$(join "01101001 0110" "Its the Matrix")"
sleep 2

$CTL off led2
echo "Done!"
