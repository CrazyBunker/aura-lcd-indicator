#!/bin/bash
# Демо: радуга на индикаторе

CDIR="$(cd "$(dirname "$0")" && pwd)"
CTL="$CDIR/../aura-indicator/bin/aura-ctl"
pad() { local s="$1"; printf "%-16s" "${s:0:16}"; }
join() { echo "$(pad "$1")$(pad "$2")"; }

RAINBOW=(
    "255,0,0"
    "255,127,0"
    "255,255,0"
    "0,255,0"
    "0,0,255"
    "75,0,130"
    "143,0,255"
)

echo "=== Rainbow demo ==="

$CTL lcd "$(join "Rainbow!" "7 colors!")"

for color in "${RAINBOW[@]}"; do
    $CTL set led2 static "$color"
    sleep 0.7
done

$CTL lcd "$(join "Rainbow 2x" "fast!")"

for i in 1 2; do
    for color in "${RAINBOW[@]}"; do
        $CTL set led2 static "$color"
        sleep 0.3
    done
done

$CTL lcd "$(join "Blink rainbow!" "3 times each")"

for i in 1 2 3; do
    $CTL set led2 static 255,0,0
    sleep 0.2
    $CTL set led2 static 0,255,0
    sleep 0.2
    $CTL set led2 static 0,0,255
    sleep 0.2
done

$CTL off led2
echo "Rainbow done!"
