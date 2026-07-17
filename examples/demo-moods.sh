#!/bin/bash
# Демо: настроение + цвет индикатора
# 16+16=32, слова не разрываются

CDIR="$(cd "$(dirname "$0")" && pwd)"
CTL="$CDIR/../aura-indicator/bin/aura-ctl"
pad() { local s="$1"; printf "%-16s" "${s:0:16}"; }
join() { echo "$(pad "$1")$(pad "$2")"; }

echo "=== Mood ring demo ==="

$CTL lcd "$(join "I feel HAPPY!" "")"
$CTL set led2 static 255,255,0
sleep 2

$CTL lcd "$(join "Bits are heavy" "zzz")"
$CTL set led2 static 0,0,255
sleep 2

$CTL lcd "$(join "SEGFAULT AGAIN!" "")"
$CTL set led2 static 255,0,0
$CTL blink 255,0,0 2 100
sleep 2

$CTL lcd "$(join "Zzzzzzzzzzzzzz" "sleepy...")"
$CTL set led2 static 100,50,150
sleep 2

$CTL lcd "$(join "Need more" "coffee...")"
$CTL set led2 static 139,69,19
sleep 2

$CTL lcd "$(join "Hello from" "AURA!")"
$CTL set led2 static 0,255,200
sleep 2

$CTL lcd "$(join "PARTY MODE ON!" "ALL RGB!")"
$CTL blink 255,0,255 5 150
sleep 1

$CTL off led2
echo "Moods done!"
