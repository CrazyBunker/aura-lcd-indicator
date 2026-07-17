#!/bin/bash
# Демо: знаменитые цитаты
# 16+16=32, слова не разрываются

CDIR="$(cd "$(dirname "$0")" && pwd)"
CTL="$CDIR/../aura-indicator/bin/aura-ctl"
pad() { local s="$1"; printf "%-16s" "${s:0:16}"; }
join() { echo "$(pad "$1")$(pad "$2")"; }

echo "=== Famous quotes ==="

$CTL lcd "$(join "Hello, World!" "")"              && sleep 2
$CTL lcd "$(join "To be or not" "to be")"         && sleep 2
$CTL lcd "$(join "I think" "therefore I am")"      && sleep 2
$CTL lcd "$(join "May the Force" "be with you")"   && sleep 2
$CTL lcd "$(join "Hasta la vista" "baby.")"        && sleep 2
$CTL lcd "$(join "Elementary, my" "dear Watson")"  && sleep 2
$CTL lcd "$(join "I'll be back." "")"              && sleep 2
$CTL lcd "$(join "Keep calm and" "carry on.")"     && sleep 2
$CTL lcd "$(join "Go rewrite" "done! all Go")"     && sleep 2

echo "Quotes done!"
