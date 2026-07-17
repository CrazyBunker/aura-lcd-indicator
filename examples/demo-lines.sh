#!/bin/bash
# Демо: независимые строки + мигание

CDIR="$(cd "$(dirname "$0")" && pwd)"
CTL="$CDIR/../aura-indicator/bin/aura-ctl"
SLEEP=1.5

echo "=== Line-by-line demo ==="

$CTL lcd-line 0 "Coffee: ON"
$CTL lcd-line 1 "Sleep: OFF"
sleep $SLEEP

$CTL lcd-line 0 "Let's GO!"
$CTL lcd-line 1 "Debugging..."
sleep $SLEEP

$CTL lcd-line 0 "Live, Laugh,"
$CTL lcd-line 1 "L33T H4X0R"
sleep $SLEEP

$CTL lcd-line 0 "Top 1% dev:"
$CTL lcd-line 1 "procrastinat"
sleep $SLEEP

echo "=== Blink indicator ==="
$CTL lcd-line 0 "URGENT: blink"
$CTL lcd-line 1 "paging user!"
$CTL blink 255,0,0 5 200
sleep 1

$CTL lcd-line 0 "OK, you're"
$CTL lcd-line 1 "back, good!"
sleep 1

echo "Done!"
