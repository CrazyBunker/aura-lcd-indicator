# AURA — ASUS AURA LED Controller Toolchain

**AURA** — это набор инструментов для управления LED-контроллером ASUS AURA (USB HID, VID:PID `0b05:19af`) и совместимыми клонами (AULA и др.). Позволяет управлять ARGB-подсветкой, выводить текст на 16×2 символьный LCD-дисплей через LED-ленту, а также декодировать сигнал WS2812B с помощью ESP32.

Проект состоит из трёх компонентов:

---

## 1. `aura-ctl` — CLI утилита (Go)

Полноценная замена Python-версии `aura_ctl.py`. Работает напрямую с HID-устройством через `/dev/hidraw*`, без внешних зависимостей.

**Возможности:**
- Обнаружение AURA-контроллера (сканирование sysfs)
- Установка режимов: static, rainbow, breathing, chase, off и ещё 14 эффектов
- Прямое управление каждым LED (direct mode)
- Вывод текста на 16×2 LCD-дисплей (full-screen и построчно)
- Мигание индикатором с настраиваемой скоростью
- Состояние LED и LCD сохраняется в JSON (`/tmp/aura_led2_state.json`)
- Индикатор (LED0) и LCD (LED1..LED32) работают независимо

**Использование:**
```bash
aura-ctl detect                      # найти устройство
aura-ctl status                      # статус и каналы
aura-ctl set led2 static 0,0,255     # синий индикатор
aura-ctl set led2 rainbow            # радуга
aura-ctl lcd "Hello, World!"         # текст на LCD
aura-ctl lcd-line 0 "First line"     # одна строка
aura-ctl blink 255,0,0 5 150         # мигание
aura-ctl off led2                    # выключить
```

---

## 2. `aura-indicator` — MCP сервер (Go)

JSON-RPC сервер по протоколу MCP (Model Context Protocol). Используется агентами OpenCode/Claude для визуальной индикации состояния на физическом устройстве.

**Инструменты:**
- `aura_notify_start` / `done` / `off` / `question` — цветовые уведомления
- `aura_notify_importance` / `progress` — яркость/прогресс (0-100%)
- `aura_lcd_print` — вывод текста на LCD (full-screen / строка)
- `aura_blink` — мигание с настраиваемыми параметрами

Работает через stdin/stdout, не требует Python.

---

## 3. `aura_rmt` — ESP32 firmware (Arduino)

Скетч для ESP32, который декодирует сигнал ARGB-ленты WS2812B с помощью RMT-периферии.

**Особенности:**
- Захват сигнала на GPIO16 (RMT)
- Декодирование LED0 из каждого кадра (GRB → 3 байта)
- Красный канал LED0 → GPIO13 (PWM, внешний красный LED)
- Зелёный канал LED0 → GPIO12 (PWM, внешний зелёный LED)
- Синий канал LED0 → GPIO2 (PWM, встроенный синий LED)
- Live-режим и histogram-режим
- Self-test: генератор тестового кадра на GPIO17 → GPIO16
- LCD-обновление не чаще 2 раз/с (anti-flicker)

---

## USB-HID протокол

Контроллер — HID-устройство (65-байтовые пакеты с префиксом `0xEC`). Протокол совместим с **OpenRGB** (AuraMainboardController).

```
Effect:  0xEC 0x35 [ch_type] 0x00 0x00 [mode]
Color:   0xEC 0x36 [mask_hi] [mask_lo] 0x00 [R] [G] [B]
Direct:  0xEC 0x40 [ch|0x80] [start] [count] [R G B...]
Commit:  0xEC 0x3F 0x55
```

---

## LCD через LED-ленту

Канал **led2** используется одновременно для индикатора (LED0) и 16×2 символьного дисплея (LED1..LED32). Каждый LED кодирует 3 символа (R, G, B каналы). Поддерживается только ASCII 0x20–0x7F.

- Индикатор и LCD работают независимо
- Строки обновляются независимо
- Состояние сохраняется и восстанавливается при перезапуске

---

## Сборка

```bash
make          # aura-ctl + aura-indicator → aura-indicator/bin/
make cross    # кросс-компиляция (Linux/Win/Mac, amd64/arm64)
make clean    # удалить bin/
```

Требуется **Go 1.25+**. Без внешних зависимостей (только стандартная библиотека).

---

## Системные требования

- **Linux** с ядром и поддержкой hidraw
- **Права доступа:** пользователь должен иметь доступ на чтение/запись к `/dev/hidraw*`
- **udev-правило:** `SUBSYSTEM=="hidraw", ATTRS{idVendor}=="0b05", MODE="0666"`
- Для ESP32: Arduino IDE или PlatformIO с поддержкой ESP32 Arduino Core

---

## Примеры

В каталоге `examples/` — 10 демо-скриптов с забавными текстами:

```bash
cd examples
bash demo-full.sh     # полный экран
bash demo-matrix.sh   # отсылки к фильму «Матрица»
bash demo-game.sh     # игровая сцена: загрузка → бой → победа
bash demo-all.sh      # все сразу
```

Каждый текст ровно 16+16=32 символа (full screen) или ≤16 символов (строка), слова не разрываются, только ASCII.

---

## Лицензия

MIT
