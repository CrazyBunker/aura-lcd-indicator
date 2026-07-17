#!/usr/bin/env bash
# Случайный демо-скрипт из examples/
set -e

DIR="$(cd "$(dirname "$0")" && pwd)"

# Собираем все demo-*.sh, исключая себя
scripts=()
for f in "$DIR"/demo-*.sh; do
    name="$(basename "$f")"
    [ "$name" = "demo-random.sh" ] && continue
    scripts+=("$f")
done

if [ ${#scripts[@]} -eq 0 ]; then
    echo "Нет demo-скриптов для запуска."
    exit 1
fi

# Случайный индекс
idx=$(( RANDOM % ${#scripts[@]} ))
chosen="${scripts[$idx]}"

echo "=== Запуск: $(basename "$chosen") ==="
exec bash "$chosen"
