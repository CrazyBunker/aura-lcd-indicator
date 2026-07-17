#!/bin/bash
# Запустить все демо по очереди
# ./demo-all.sh          — запустить всё
# ./demo-all.sh rainbow  — запустить конкретное

CDIR="$(cd "$(dirname "$0")" && pwd)"
DEMOS=(
    demo-full.sh
    demo-lines.sh
    demo-leds.sh
    demo-moods.sh
    demo-quotes.sh
    demo-matrix.sh
    demo-game.sh
    demo-rainbow.sh
    demo-status.sh
)

if [ -n "$1" ]; then
    # Запустить конкретный
    name="demo-$1.sh"
    if [ -f "$CDIR/$name" ]; then
        echo "=== Running $name ==="
        bash "$CDIR/$name"
    else
        echo "Unknown demo: $1"
        echo "Available: ${DEMOS[*]}"
        exit 1
    fi
else
    # Запустить все
    for demo in "${DEMOS[@]}"; do
        echo ""
        echo "================================================"
        echo ">>> $demo"
        echo "================================================"
        bash "$CDIR/$demo"
        sleep 1
    done
    echo ""
    echo "All demos done!"
fi
