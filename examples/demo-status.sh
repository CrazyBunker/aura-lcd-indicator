#!/bin/bash
# Демо: имитация прогресса / статусов

CDIR="$(cd "$(dirname "$0")" && pwd)"
CTL="$CDIR/../aura-indicator/bin/aura-ctl"
pad() { local s="$1"; printf "%-16s" "${s:0:16}"; }
join() { echo "$(pad "$1")$(pad "$2")"; }

echo "=== Status simulation ==="

$CTL lcd "$(join "thinking..." "step 1/5")"
sleep 2

$CTL lcd "$(join "planning" "step 2/5")"
$CTL set led2 static 0,0,255
sleep 2

$CTL lcd "$(join "coding..." "step 3/5")"
sleep 2

$CTL lcd "$(join "reviewing" "step 4/5")"
$CTL blink 255,255,0 3 300
sleep 2

$CTL lcd "$(join "testing..." "step 5/5")"
$CTL set led2 static 255,165,0
sleep 2

$CTL lcd "$(join "ALL DONE!" "SUCCESS!")"
$CTL set led2 static 0,255,0
$CTL blink 0,255,0 3 200
sleep 1

$CTL off led2
echo "Done! (green = success)"
