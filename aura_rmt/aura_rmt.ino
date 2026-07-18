/**
 * ASUS AURA ARGB — захват RMT RAW (RX) + генератор самопроверки (TX)
 * ========================================================================
 * Copyright (c) 2026 Vladislav Plotkin
 * SPDX-License-Identifier: MIT
 *
 * Нативный RMT HAL ESP32 Arduino Core 3.x (rmtInit/rmtWrite/rmtRead).
 *
 * RX: RMT на GPIO16 захватывает линию данных AURA как пары (HIGH длит, LOW длит)
 *     на бит WS2812B, в наносекундах. Без декодирования во время захвата.
 *
 * TX (самопроверка): RMT на GPIO17 воспроизводит известный кадр WS2812B
 *     (LED0 красный, LED1 зелёный, LED2 синий, LED3 белый, остальные чёрные). Соедините
 *     GPIO17 -> GPIO16, чтобы подать его на RX и проверить захват '0'/'1'.
 *
 * Использование:
 *   - Живой режим (по умолчанию): декодирует LED0 каждый кадр и управляет тремя PWM LED
 *     по цвету: красный канал (R) -> внешний красный LED на GPIO13, зелёный канал
 *     (G) -> внешний зелёный LED на GPIO12, синий канал (B) -> встроенный синий LED
 *     на GPIO2. Яркость каждого индикатора (PWM 0..255) следует за интенсивностью
 *     соответствующего канала, отправленной контроллером ARGB.
 *   - Отправить 'l' -> переключить живой монитор (выкл => режим гистограммы, LED выкл).
 *   - Отправить 'd' -> вывести один полный кадр RAW (H/L нс + уровни) + hex декодирование.
 *   - Отправить 't' -> переключить генератор самопроверки TX.
 *   - Отправить 'a' -> вывести информацию об авторе.
 *   - Самопроверка: ОТКЛЮЧИТЕ отвод AURA от GPIO16, соедините GPIO17->GPIO16.
 *
 * Ожидаемые декодированные байты (MSB-first, GRB):
 *   LED0 красный  : 00 FF 00
 *   LED1 зелёный  : FF 00 00
 *   LED2 синий    : 00 00 FF
 *   LED3 белый    : FF FF FF
 *   LED4          : 00 00 00
 */

#include <Arduino.h>
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include <string.h>

// ============================================================================
// Конфигурация
// ============================================================================

#define AURA_PIN    16
#define TX_PIN      17
#define LED_PIN     2        // встроенный LED (ESP32-DevKitC синий), активный HIGH
#define EXT_LED_PIN 13       // внешний индикаторный LED (красный), активный HIGH
#define GREEN_LED_PIN 12     // внешний индикаторный LED (зелёный), активный HIGH

// Пины 4-битного параллельного LCD (HD44780 совместимый) — все на одной физической стороне
// (левый ряд DevKit: 26, 25, 27, 33, 32) плюс D7 на GPIO14.
#define LCD_RS   14
#define LCD_EN   27
#define LCD_D4   32
#define LCD_D5   33
#define LCD_D6   25
#define LCD_D7   26
#define LCD_COLS 16
#define LCD_ROWS 2
#define LCD_SIZE (LCD_COLS * LCD_ROWS)  // 32
#define LCD_UPDATE_INTERVAL_MS 500     // обновлять LCD ~2 раз/сек (кадры приходят ~59 fps)

#define LEDC_FREQ   5000     // частота PWM для управления яркостью
#define LEDC_BITS   8        // разрешение PWM (0..255 скважность)
#define RMT_BAUD    921600
#define MAX_ITEMS   3000
#define N_LEDS_TEST 5
#define TICK_HZ     40000000UL   // 40 МГц -> 25 нс такт
#define IDLE_TICKS  4000         // 4000 * 25 нс = 100 мкс порог сброса-паузы

static const uint32_t tick_ns = 1000000000UL / TICK_HZ;   // = 25

enum { NUM_BUCKETS = 6 };

// ============================================================================
// Вспомогательные функции
// ============================================================================

static uint8_t bucket(uint32_t ns) {
    if      (ns < 300)  return 0;
    else if (ns < 600)  return 1;
    else if (ns < 1000) return 2;
    else if (ns < 1500) return 3;
    else if (ns < 3000) return 4;
    else                return 5;
}

static void print_hist(const char *label, uint32_t c[6]) {
    Serial.printf("  %s: <300=%4lu 300-600=%4lu 600-1k=%4lu 1k-1.5k=%4lu 1.5k-3k=%4lu >3k=%4lu\n",
                  label, c[0], c[1], c[2], c[3], c[4], c[5]);
}

// ============================================================================
// Кадр самопроверки TX (символы rmt_data_t)
// ============================================================================

static rmt_data_t g_tx_items[N_LEDS_TEST * 24 + 2];
static int        g_tx_n = 0;
static bool       g_tx_on = true;

// ============================================================================
// Драйвер LCD (HD44780 4-бита)
// ============================================================================

static void build_test_frame() {
    // Цвета GRB для каждого LED
    uint8_t leds[N_LEDS_TEST][3] = {
        {0, 255, 0},     // красный
        {255, 0, 0},     // зелёный
        {0, 0, 255},     // синий
        {255, 255, 255}, // белый
        {0,0,0}
    };
    int idx = 0;
    for (int i = 0; i < N_LEDS_TEST; i++) {
        uint8_t bytes[3] = { leds[i][0], leds[i][1], leds[i][2] };  // порядок G,R,B
        for (int by = 0; by < 3; by++) {
            uint8_t b = bytes[by];
            for (int k = 7; k >= 0; k--) {            // старший бит первым
                int bit = (b >> k) & 1;
                rmt_data_t it;
                if (bit) {                             // '1': ~900 нс HIGH, ~350 нс LOW
                    it.duration0 = 36; it.level0 = 1; // 900 нс
                    it.duration1 = 14; it.level1 = 0; // 350 нс
                } else {                               // '0': ~350 нс HIGH, ~900 нс LOW
                    it.duration0 = 14; it.level0 = 1; // 350 нс
                    it.duration1 = 36; it.level1 = 0; // 900 нс
                }
                g_tx_items[idx++] = it;
            }
        }
    }
    // сброс-пауза: ~300 мкс LOW (idle_threshold 100 мкс -> здесь границы кадров RX)
    rmt_data_t gap;
    gap.duration0 = 12000; gap.level0 = 0;            // 300 мкс
    gap.duration1 = 0;     gap.level1 = 0;
    g_tx_items[idx++] = gap;
    g_tx_n = idx;
}

// ============================================================================
// Состояние
// ============================================================================

static bool       g_do_raw = false;
static bool       g_live   = true;   // живой режим LED0 -> встроенный монитор LED
static uint8_t    last_g = 255, last_r = 255, last_b = 255;
static uint32_t   g_frame  = 0;
static rmt_data_t g_rx_buf[MAX_ITEMS];

// Состояние LCD
static char     lcd_buf[LCD_SIZE];
static char     lcd_buf_prev[LCD_SIZE];
static uint8_t  lcd_colors[12][3];   // LED0..LED11, каждый R,G,B
static bool     lcd_dirty = false;
static unsigned long s_last_lcd_update = 0;  // время последнего обновления LCD (ms)

// ============================================================================
// Настройка
// ============================================================================

void setup() {
    Serial.begin(RMT_BAUD);
    Serial.setTxBufferSize(8192);   // избежать повреждения кольцевого буфера при больших дампах
    delay(100);

    // Индикаторы LED с управлением PWM (яркость через LEDC)
    ledcAttach(LED_PIN, LEDC_FREQ, LEDC_BITS);
    ledcAttach(EXT_LED_PIN, LEDC_FREQ, LEDC_BITS);
    ledcAttach(GREEN_LED_PIN, LEDC_FREQ, LEDC_BITS);
    ledcWrite(LED_PIN, 0);
    ledcWrite(EXT_LED_PIN, 0);
    ledcWrite(GREEN_LED_PIN, 0);
    // плавное изменение 0->255->0 на всех индикаторах для проверки работы пинов LED
    for (int v = 0; v <= 255; v += 8) { ledcWrite(LED_PIN, v); ledcWrite(EXT_LED_PIN, v); ledcWrite(GREEN_LED_PIN, v); delay(8); }
    for (int v = 255; v >= 0; v -= 8) { ledcWrite(LED_PIN, v); ledcWrite(EXT_LED_PIN, v); ledcWrite(GREEN_LED_PIN, v); delay(8); }

    Serial.println("\n========================================================");
    Serial.println("  AURA ARGB RMT Raw Capture (RX) + TX Self-Test");
    Serial.println("========================================================");
    Serial.printf("  RX GPIO%d   TX GPIO%d   RMT tick = %u ns (%u MHz)\n",
                  AURA_PIN, TX_PIN, tick_ns, TICK_HZ / 1000000);
    Serial.println("  LCD HD44780 4-bit: RS=26 EN=25 D4=27 D5=33 D6=32 D7=14");
    Serial.println("  Self-test TX frame (GRB, MSB-first):");
    Serial.println("    LED0 red  : 00 FF 00");
    Serial.println("    LED1 green: FF 00 00");
    Serial.println("    LED2 blue : 00 00 FF");
    Serial.println("    LED3 white: FF FF FF");
    Serial.println("    LED4      : 00 00 00");
    Serial.println("  Self-test: wire GPIO17 -> GPIO16 (AURA tap off GPIO16).");
    Serial.println("  Send 'd' = raw dump+hex.  Send 't' = toggle TX.  Send 'l' = toggle live/histogram.  Send 'a' = author info.\n");

    if (!rmtInit(AURA_PIN, RMT_RX_MODE, RMT_MEM_NUM_BLOCKS_6, TICK_HZ)) {
        Serial.println("[RMT] RX init FAILED!");
    }
    rmtSetRxMaxThreshold(AURA_PIN, IDLE_TICKS);

    if (!rmtInit(TX_PIN, RMT_TX_MODE, RMT_MEM_NUM_BLOCKS_2, TICK_HZ)) {
        Serial.println("[RMT] TX init FAILED!");
    }

    build_test_frame();

    // Инициализация LCD (HD44780 4-бита)
    lcd_init();
    Serial.println("[LCD] HD44780 4-bit initialized.");
    Serial.printf("[RMT] TX frame built: %d symbols.\n", g_tx_n);

    // Запуск самопроверки TX непрерывно на Core 0, чтобы (блокирующий) rmtRead в
    // loop() на Core 1 мог всегда захватить живой кадр. rmtWrite синхронный
    // и иначе завершился бы до того, как rmtRead начнёт прослушивание.
    xTaskCreatePinnedToCore(txTask, "tx", 4096, NULL, 1, NULL, 0);

    Serial.println("[RMT] RX running, TX self-test ON. LCD ready. Waiting for data...\n");
}

// Непрерывная передача известного кадра самопроверки на GPIO17 (Core 0).
void txTask(void *param) {
    while (true) {
        if (g_tx_on) {
            rmtWrite(TX_PIN, g_tx_items, g_tx_n, 100);
            delay(8);   // пауза LOW, чтобы RX мог разделять кадры
        } else {
            delay(10);
        }
    }
}

// Декодирование ПЕРВОГО LED (24 бита = GRB) из буфера захваченных символов.
// Возвращает true если было доступно >= 24 символов (выравнивание по началу кадра).
// WS2812B передаёт GRB, старший бит первым; импульс HIGH >= 600 нс = бит 1, иначе 0.
static bool decode_first_led(const rmt_data_t *buf, int total,
                              uint8_t *g, uint8_t *r, uint8_t *b) {
    int bits = 0, byte = 0, nbytes = 0;
    uint8_t out[3] = {0, 0, 0};
    for (int i = 0; i < total && nbytes < 3; i++) {
        uint32_t d0 = buf[i].duration0 * tick_ns;
        if (d0 >= 5000) break;            // сброс-пауза -> конец кадра
        int bit = (d0 >= 600) ? 1 : 0;    // HIGH >= 600 нс => бит 1
        byte = (byte << 1) | bit;
        if (++bits == 8) { out[nbytes++] = byte; byte = 0; bits = 0; }
    }
    if (nbytes < 3) return false;
    *g = out[0]; *r = out[1]; *b = out[2];    // GRB -> (g, r, b)
    return true;
}

// Декодирование ПОЛНОГО кадра (12 LED = 36 байт = 288 бит) из захваченных символов.
// Возвращает количество декодированных байт (ожидается 36).
// Формат WS2812B GRB на LED: G, R, B (старший бит первым).
static int decode_full_frame(const rmt_data_t *buf, int total, uint8_t out[36]) {
    int bits = 0, byte = 0, nbytes = 0;
    for (int i = 0; i < total && nbytes < 36; i++) {
        uint32_t d0 = buf[i].duration0 * tick_ns;
        if (d0 >= 5000) break;            // сброс-пауза -> конец кадра
        int bit = (d0 >= 600) ? 1 : 0;    // HIGH >= 600 нс => бит 1
        byte = (byte << 1) | bit;
        if (++bits == 8) { out[nbytes++] = byte; byte = 0; bits = 0; }
    }
    return nbytes;
}

// ============================================================================
// Драйвер LCD (HD44780 4-бита)
// ============================================================================

static void lcd_pulse_en(void) {
    digitalWrite(LCD_EN, HIGH);
    delayMicroseconds(1);
    digitalWrite(LCD_EN, LOW);
    delayMicroseconds(5);
}

static void lcd_write_nibble(uint8_t nib) {
    digitalWrite(LCD_D4, (nib & 0x01) ? HIGH : LOW);
    digitalWrite(LCD_D5, (nib & 0x02) ? HIGH : LOW);
    digitalWrite(LCD_D6, (nib & 0x04) ? HIGH : LOW);
    digitalWrite(LCD_D7, (nib & 0x08) ? HIGH : LOW);
    lcd_pulse_en();
}

static void lcd_write(uint8_t val, bool is_data) {
    digitalWrite(LCD_RS, is_data ? HIGH : LOW);
    lcd_write_nibble(val >> 4);        // старшая тетрада
    lcd_write_nibble(val & 0x0F);      // младшая тетрада
    delayMicroseconds(100);
}

static void lcd_init(void) {
    pinMode(LCD_RS, OUTPUT);
    pinMode(LCD_EN, OUTPUT);
    pinMode(LCD_D4, OUTPUT);
    pinMode(LCD_D5, OUTPUT);
    pinMode(LCD_D6, OUTPUT);
    pinMode(LCD_D7, OUTPUT);

    delay(150);   // ожидание стабилизации питания

    // Классическая последовательность инициализации HD44780 4-бита
    digitalWrite(LCD_RS, LOW);
    delayMicroseconds(100);
    
    // Первая последовательность сброса
    lcd_write_nibble(0x03);
    delayMicroseconds(4500);
    
    // Вторая последовательность сброса
    lcd_write_nibble(0x03);
    delayMicroseconds(150);
    
    // Третья последовательность сброса
    lcd_write_nibble(0x03);
    delayMicroseconds(100);
    
    // Установка 4-битного режима
    lcd_write_nibble(0x02);
    delayMicroseconds(100);

    // Настройка функций: 4-бита, 2 строки, 5x8 точек
    lcd_write(0x28, false);
    // Дисплей вкл: дисплей вкл, курсор выкл, мигание выкл
    lcd_write(0x0C, false);
    // Режим ввода: инкремент, без сдвига
    lcd_write(0x06, false);
    // Очистка дисплея
    lcd_write(0x01, false);
    delay(5);
}

static void lcd_set_cursor(uint8_t row, uint8_t col) {
    uint8_t addr = (row == 0 ? 0x80 : 0xC0) | (col & 0x0F);
    lcd_write(addr, false);
}

static void lcd_write_char(char c) {
    lcd_write((uint8_t)c, true);
}

static void lcd_update_display(void) {
    if (!lcd_dirty) return;
    
    lcd_set_cursor(0, 0);
    for (int i = 0; i < LCD_COLS; i++) {
        lcd_write_char(lcd_buf[i]);
    }
    lcd_set_cursor(1, 0);
    for (int i = LCD_COLS; i < LCD_SIZE; i++) {
        lcd_write_char(lcd_buf[i]);
    }
    lcd_dirty = false;
}

static void unpack_to_lcd(const uint8_t colors[][3]) {
    bool changed = false;
    for (int i = 0; i < LCD_SIZE; i++) {
        int led_idx = i / 3 + 1;      // 1..11
        int ch = i % 3;               // 0=R, 1=G, 2=B
        uint8_t code = colors[led_idx][ch];
        char c = (code >= 0x20 && code <= 0x7F) ? (char)code : ' ';
        if (c != lcd_buf[i]) changed = true;
        lcd_buf[i] = c;
    }
    
    if (changed) {
        memcpy(lcd_buf_prev, lcd_buf, LCD_SIZE);
        lcd_dirty = true;
    }
}

// ============================================================================
// Основной цикл
// ============================================================================

void loop() {
    while (Serial.available()) {
        int c = Serial.read();
        if (c == 'd' || c == 'D') g_do_raw = true;
        if (c == 't' || c == 'T') g_tx_on = !g_tx_on;
        if (c == 'l' || c == 'L') g_live = !g_live;
        if (c == 'a' || c == 'A') print_author();
    }

    if (g_do_raw) {
        int total = 0;
        bool got_gap = false;
        // Склейка нескольких вызовов rmtRead (HW буфер = 6 блоков = 384 символа)
        // в один кадр. Останов при обнаружении символа сброс-паузы (dur0 >= 5000 нс).
        while (total < MAX_ITEMS) {
            size_t num = MAX_ITEMS - total;
            if (!rmtRead(AURA_PIN, g_rx_buf + total, &num, 200)) break;  // таймаут/нет данных
            for (size_t i = 0; i < num; i++) {
                if (g_rx_buf[total + i].duration0 * tick_ns >= 5000) { got_gap = true; break; }
            }
            total += (int)num;
            // Останов при получении полного кадра (rmtRead ограничен 384-символьным
            // HW буфером; возврат < 384 означает конец кадра, а не заполнение буфера).
            if (got_gap || num < 384) break;
        }

        // Декодирование в байты (пропуск символа сброс-паузы: dur0 >= 5000 нс).
        uint8_t out[400];
        int nbytes = 0, bits = 0, byte = 0, ones = 0, zeros = 0;
        for (int i = 0; i < total; i++) {
            uint32_t d0 = g_rx_buf[i].duration0 * tick_ns;
            if (d0 >= 5000) break;                 // сброс-пауза -> конец кадра
            int b = (d0 >= 600) ? 1 : 0;           // HIGH >= 600 нс => бит 1
            if (b) ones++; else zeros++;
            byte = (byte << 1) | b;
            if (++bits == 8) { if (nbytes < 400) out[nbytes++] = byte; byte = 0; bits = 0; }
        }

        Serial.printf("=== RAW Frame %lu: %d symbols -> %d bytes ===\n", g_frame, total, nbytes);
        for (int i = 0; i < nbytes; i++) Serial.printf("%02X", out[i]);
        Serial.println();
        Serial.printf("  bits: ones=%d zeros=%d (test frame expect 48/192)\n\n", ones, zeros);

        g_do_raw = false;
        g_frame++;
        delay(1);
        return;
    }

    // ---- Живой режим: декодирование LED0, управление встроенным LED по синему каналу ----
    size_t num = MAX_ITEMS;
    if (rmtRead(AURA_PIN, g_rx_buf, &num, 100)) {
        g_frame++;
        if (g_live) {
            uint8_t all_leds[36];   // 12 LED * 3 байта (GRB)
            int n = decode_full_frame(g_rx_buf, (int)num, all_leds);
            
            // Маршрутизация LED0 по цвету: красный канал -> внешний красный LED (GPIO13),
            // зелёный канал -> внешний зелёный LED (GPIO12),
            // синий канал -> встроенный синий LED (GPIO2). Яркость индикатора
            // следует за интенсивностью канала, полученной от контроллера ARGB.
            // формат all_leds: [G0,R0,B0, G1,R1,B1, ...]
            // LEDCWrite ожидает R на EXT_LED_PIN, G на GREEN_LED_PIN, B на LED_PIN
            ledcWrite(EXT_LED_PIN,  all_leds[1]);   // R0
            ledcWrite(GREEN_LED_PIN, all_leds[0]);  // G0
            ledcWrite(LED_PIN,     all_leds[2]);    // B0
            
            // Заполнение lcd_colors для LED0..LED11 (порядок RGB для unpack_to_lcd)
            for (int i = 0; i < 12; i++) {
                int base = i * 3;   // формат GRB
                // Сохранение RGB для удобства: R=base+1, G=base+0, B=base+2
                lcd_colors[i][0] = all_leds[base + 1];   // R
                lcd_colors[i][1] = all_leds[base + 0];   // G
                lcd_colors[i][2] = all_leds[base + 2];   // B
            }
            
            // Декодирование и отображение на LCD — только 1 раз в секунду
            // (кадры приходят ~59 fps, LCD физически не успевает обновляться так часто)
            unsigned long now = millis();
            if (now - s_last_lcd_update >= LCD_UPDATE_INTERVAL_MS) {
                unpack_to_lcd(lcd_colors);
                lcd_update_display();
                s_last_lcd_update = now;
            }
            
            if (all_leds[0] != last_g || all_leds[1] != last_r || all_leds[2] != last_b) {
                Serial.printf("LED0 G=%3d R=%3d B=%3d -> GPIO%d(red)=%3d  GPIO%d(green)=%3d  GPIO%d(blue)=%3d\n",
                               all_leds[0], all_leds[1], all_leds[2],
                               EXT_LED_PIN, all_leds[1], GREEN_LED_PIN, all_leds[0], LED_PIN, all_leds[2]);
                last_g = all_leds[0]; last_r = all_leds[1]; last_b = all_leds[2];
            }
            
        } else {
            ledcWrite(LED_PIN, 0);
            ledcWrite(EXT_LED_PIN, 0);
            ledcWrite(GREEN_LED_PIN, 0);
            uint32_t h0[NUM_BUCKETS] = {0}, h1[NUM_BUCKETS] = {0};
            for (size_t i = 0; i < num; i++) {
                h0[bucket(g_rx_buf[i].duration0 * tick_ns)]++;
                h1[bucket(g_rx_buf[i].duration1 * tick_ns)]++;
            }
            Serial.printf("F%lu n=%d | ", g_frame, (int)num);
            print_hist("HIGH", h0);
            print_hist("LOW ", h1);
            Serial.println();
        }
    } else {
        delay(10);   // нет данных в течение 100 мс
    }
    delay(1);
}

// ============================================================================
// Author info
// ============================================================================

static void print_author(void) {
    Serial.println("\n----------------------------------------");
    Serial.println("  AURA ARGB RMT Capture");
    Serial.println("  Copyright (c) 2026 Vladislav Plotkin");
    Serial.println("  SPDX-License-Identifier: MIT");
    Serial.println("----------------------------------------\n");
}
