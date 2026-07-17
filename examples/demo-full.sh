#!/bin/bash
# Демо: полный экран 16x2
# Гарантированно 16+16=32 символа, слова не разрываются

CDIR="$(cd "$(dirname "$0")" && pwd)"
CTL="$CDIR/../aura-indicator/bin/aura-ctl"
pad() { local s="$1"; printf "%-16s" "${s:0:16}"; }
join() { echo "$(pad "$1")$(pad "$2")"; }

echo "=== AURA Display Demo ==="

$CTL lcd "$(join "Hello, World!" "This is AURA")"
sleep 2

$CTL lcd "$(join "AURA sees you" "and it waves")"
sleep 2

$CTL lcd "$(join "I'm a HID" "wizard!")"
sleep 2

$CTL lcd "$(join "LCD go brrrr" "working fine")"
sleep 2

$CTL lcd "$(join "ESP32 is love" "ESP32 is life")"
sleep 2

$CTL lcd "$(join "01001000 0110" "1110 01110100")"
sleep 2

$CTL lcd "$(join "All your base" "are belong to")"
sleep 2

$CTL lcd "$(join "Thank you and" "good night!")"
sleep 2

echo "Done!"
