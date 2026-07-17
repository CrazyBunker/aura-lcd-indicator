#!/bin/bash
# Демо: игровая сцена
# 16+16=32, слова не разрываются

CDIR="$(cd "$(dirname "$0")" && pwd)"
CTL="$CDIR/../aura-indicator/bin/aura-ctl"
pad() { local s="$1"; printf "%-16s" "${s:0:16}"; }
join() { echo "$(pad "$1")$(pad "$2")"; }

echo "=== Game state demo ==="

$CTL lcd "$(join "LOADING..." "0%")"
$CTL blink 255,255,0 4 300
sleep 1

$CTL lcd "$(join "LOADING..." "42%")"
sleep 1

$CTL lcd "$(join "LOADING..." "99%")"
sleep 1

$CTL lcd "$(join "1. Play" "2. Exit")"
$CTL set led2 static 0,0,255
sleep 2

$CTL lcd "$(join "Score: 00000" "Level 1")"
$CTL set led2 static 0,255,0
sleep 2

$CTL lcd "$(join "Score: 00010" "Level 1")"
sleep 1
$CTL lcd "$(join "Score: 00100" "Level 2")"
sleep 1
$CTL lcd "$(join "Score: 01000" "Level 3")"
sleep 1

$CTL lcd "$(join "OUCH! -50 HP" "HP: 50/100")"
$CTL set led2 static 255,0,0
$CTL blink 255,0,0 3 80
sleep 2

$CTL lcd "$(join "POWER UP!" "HP: 150/100")"
$CTL set led2 static 0,255,255
$CTL blink 0,255,255 3 150
sleep 2

$CTL lcd "$(join "YOU WIN! GG!" "Score: 99999")"
$CTL set led2 static 255,215,0
$CTL blink 255,215,0 5 200
sleep 2

$CTL lcd "$(join "GAME OVER" "Score: 01000")"
$CTL set led2 static 255,0,0
sleep 2

$CTL off led2
echo "Game over!"
