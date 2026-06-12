# ESU LokSound 5 — Configuration Reference

Reference documentation for ESU **LokSound 5** sound decoders (all form factors: H0, micro, L, XL, etc.). This document is **not** tied to the BigFred or Loco runtime; it is intended as a future data source for decoder configuration.

| Item | Value |
|------|-------|
| Source | [LokSound 5 Instruction Manual, Edition 15](https://www.esu.eu/) (ESU, 51989) |
| Scope | LokSound 5 family — motor control, function mapping, outputs, couplers, smoke, volume |

---

## Table of contents

1. [Acceleration, deceleration, and speed](#1-acceleration-deceleration-and-speed)
2. [Shunting mode](#2-shunting-mode)
3. [Output mapping, brightness, and lighting effects](#3-output-mapping-brightness-and-lighting-effects)
4. [Digital coupler (uncoupler)](#4-digital-coupler-uncoupler)
5. [Smoke generator](#5-smoke-generator)
6. [Volume regulation](#6-volume-regulation)

**Appendices:** [B — Full CV table](#appendix-b--full-cv-table-cv-1255) · [C — Indexed CVs](#appendix-c--indexed-cvs-257511) · [D — Long address](#appendix-d--long-address-calculation)

---

## 1. Acceleration, deceleration, and speed

### 1.1 Basic momentum (CV #3 and CV #4)

Acceleration and brake time are set **independently**:

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#3** | Acceleration time | 0–63 | 0 = no delay |
| **#4** | Deceleration / brake time | 0–63 | 0 = no delay |

The times are **speed-dependent**: at higher speeds the acceleration and braking **distances** are longer (the faster the locomotive moves, the longer the distance until it stops).

For braking distance independent of speed, see [§1.6](#16-constant-brake-distance-cv-254-cv-253).

### 1.2 Switching acceleration / deceleration on and off

LokSound decoders can **deactivate** acceleration and deceleration via a function button (logic function *Disable Acceleration & Braking times* in function mapping). The locomotive then responds directly to the throttle — useful for shunting.

### 1.3 Minimum speed, maximum speed, and speed curve

LokSound 5 decoders use **256 internal speed steps**, mapped to 14, 28, or 128 external steps.

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#2** | Vmin (start voltage) | 1–255 | Minimum speed; scales the lower end of the curve |
| **#5** | Vmax (top speed) | 0–255 | Maximum speed limit; scales the upper end of the curve |
| **#67–#94** | 28-point speed curve | 0–255 each | Internal speed step for each of 28 external steps |

**Speed curve behaviour:**

- CV #67 and CV #94 are fixed at **1** and **255** respectively; intermediate values (CV #68–#93) can be distributed freely.
- The speed curve **cannot be switched off**.
- CV #2 and CV #5 act as **scale factors**: reducing CV #5 squeezes the curve to a lower maximum while preserving its shape; increasing CV #2 raises the lower end.

#### 3-point speed table (NMRA alternative)

When **CV #29 bit 4 = 0**, the NMRA 3-point table is active:

| CV | Role |
|----|------|
| **#2** | Start voltage |
| **#5** | Maximum speed |
| **#6** | Speed at medium step — defines a "kink" in the curve |

Always maintain: **start voltage < mid speed < maximum speed**. Violating this order causes erratic driving.

### 1.4 Brake functions (CV #179–#184)

Three brake functions can shorten the effective braking time and/or impose a speed limit:

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#179** | Brake function 1 — time reduction | 0–255 | Percentage deducted from CV #4 (0 = 0%, 255 = 100%) |
| **#180** | Brake function 2 — time reduction | 0–255 | Same |
| **#181** | Brake function 3 — time reduction | 0–255 | Same |
| **#182** | Brake function 1 — speed limit | 0–255 | Maximum speed while active (0 = decelerate to stop) |
| **#183** | Brake function 2 — speed limit | 0–255 | Same |
| **#184** | Brake function 3 — speed limit | 0–255 | Same |

Brake functions are **cumulative** — more active functions = shorter braking time. Normally they only affect braking times, not initiate braking.

**Example:** CV #4 = 60. CV #179 = 90 → effective braking time = 60 × (255 − 90) / 255 ≈ 39.

### 1.5 Load simulation (CV #103, #104)

Two logical functions — *Optional Load* and *Primary Load* — scale momentum:

| CV | Name | Description |
|----|------|-------------|
| **#103** | Optional load factor | 128 = unchanged; <128 = lighter; >128 = heavier |
| **#104** | Primary load factor | Same |

**Formulas:**

```
Acceleration time = CV #3 × (load value / 128)
Braking time     = CV #4 × (load value / 128)
```

Only one load function is active at a time; if both are pressed, **Primary Load** wins. Load functions also affect sound when modelled in the sound project.

### 1.6 Constant brake distance (CV #254, CV #253)

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#254** | Constant brake distance | 0–255 | Higher value = longer brake distance. 0 = normal time mode (CV #3/#4) |
| **#253** | Braking profile | 0–255 | 0 = linear braking from sector entry; >0 = continue driving briefly then brake with constant effort |
| **#255** | Reverse brake distance | 0–255 | If >0, CV #254 applies forward and CV #255 reverse (push-pull trains) |

Active only in **brake sectors**. When throttle returns to 0 outside a brake sector, deceleration follows CV #4.

Set **CV #27 bit 7** to apply constant brake distance when speed step 0 is commanded (useful for computer control without physical brake sections).

### 1.7 Analogue operation speed (CV #125–#130)

Acceleration and maximum speed can be set separately for analogue DC and AC:

| Mode | Start speed CV | Max speed CV |
|------|----------------|--------------|
| **DC analogue** | #125 | #126 |
| **AC analogue** | #127 | #128 |

CV #129 and CV #130 define voltage offsets for function activation and motor cut-off relative to the start voltage.

---

## 2. Shunting mode

Shunting support combines **speed reduction** and optional **momentum deactivation**.

### 2.1 Speed trim (CV #101)

| CV | Name | Description |
|----|------|-------------|
| **#101** | Shunting mode trim | Reduces speed for each speed step when shunting mode is active |

**Factory default:** shunting mode reduces speed to approximately **50%** per step, giving finer control in the lower speed range — especially important in **14-speed-step** mode.

### 2.2 Activation

Shunting mode is a **logic function** (*Shunting Mode On*, mapping value **2**) assigned to any function key via ESU function mapping (see [§3.1](#31-esu-function-mapping-overview)).

When active, the locomotive moves at the speed defined by CV #101.

### 2.3 Complementary shunting features

| Feature | Mapping logic function | Effect |
|---------|------------------------|--------|
| **Disable acceleration & braking** | Value 16 | Locomotive responds instantly to throttle |
| **Drive Hold** | Value 128 (CV O block) | Throttle changes affect sound only; speed stays constant |
| **Uncoupling cycle** | Value 64 (CV O block) | Automated push-back uncoupling sequence ([§4.3](#43-automatic-uncoupling-cycle)) |

Typical shunting setup: assign one key to *Shunting Mode* and optionally a second to *Disable Acceleration & Braking*.

---

## 3. Output mapping, brightness, and lighting effects

LokSound 5 decoders provide up to **22 physical function outputs** (Headlight, Rearlight, AUX1–AUX18). Headlight and Rearlight are dedicated to lighting; remaining outputs are freely assignable.

### 3.1 ESU function mapping overview

Unlike simple NMRA bit-mapping, LokSound 5 uses an extended **mapping table** with **72 rows**. Each row has:

- **Conditions block** (input): which state triggers the action — e.g. "F3 on", "locomotive stationary, forward, F8 on".
- **Output block**: what happens — physical outputs, logic functions, or sound slots.

The decoder evaluates rows **1 → 72** continuously (hundreds of times per second). A row's output actions execute **only when** its conditions are met.

**Capabilities:**

- Any function button can switch multiple outputs.
- Any output can be activated by multiple buttons.
- Buttons can be combined (AND) or inverted (NOT).
- Direction of travel, movement state (moving / stopped), and up to **5 external sensors** can be conditions.

#### Index CV access (CV #257–#511)

CVs #257–#511 are **indexed** — their meaning depends on index registers:

| CV | Role |
|----|------|
| **#31** | Index register (always **16** for mapping) |
| **#32** | Page selector: **0, 1, 2, 3, or 4** |

Always set CV #31 = 16 and the correct CV #32 **before** reading or writing indexed CVs. CVs #1–#256 are not affected by the index.

Each mapping row uses **20 control CVs** (A–T): 10 for conditions, 10 for outputs. The manual's master table (pp. 75–78) lists the exact CV numbers per row.

**Recommended tool:** ESU **LokProgrammer** (software ≥ 5.0.0) — graphical mapping editor. Project-specific sound-slot assignments are listed at [projects.esu.eu](http://projects.esu.eu).

### 3.2 Physical output mapping values

Physical outputs are controlled via control CVs K, L, M (bit-sum encoding):

| Output | Value |
|--------|-------|
| Headlight [Config 1] | 1 |
| Rearlight [Config 1] | 2 |
| AUX1 [Config 1] | 4 |
| AUX2 [Config 1] | 8 |
| AUX3 | 16 |
| AUX4 | 32 |
| AUX5–AUX10 | 64, 128, 1, 2, 4, 8 |
| AUX11–AUX18 | 16, 32, 64, 128, 1, 2, 4, 8 |
| Headlight [Config 2] | 1 |
| Rearlight [Config 2] | 2 |
| AUX1 [Config 2] | 4 |
| AUX2 [Config 2] | 8 |

Headlight, Rearlight, AUX1, and AUX2 each have **two configurations** — e.g. bright upper beam (Config 1) and dim lower beam (Config 2), selected by mapping.

### 3.3 Function output configuration

Before use, each output must be **configured** with 7 CVs per output. Set **CV #31 = 16, CV #32 = 0** before changing output configuration CVs.

| Parameter | CV role | Description |
|-----------|---------|-------------|
| **Mode Select** | Primary mode CV | Selects lighting effect or special function (see tables below) |
| **Brightness** | Brightness CV | Output intensity in **32 steps (0–31)** |
| **Special Function CV 1** | Options bitmask | Phase, Grade Crossing, Rule 17, Dimmer response, LED mode (+128) |
| **Special Function CV 2** | Effect-specific | Varies by mode (flash timing, smoke heater power, servo positions, etc.) |
| **Special Function CV 3** | Effect-specific | Varies by mode |
| **Switch-on/off delay** | Delay CV | Encoded: `switch-off × 16 + switch-on` (each 0–15) |
| **Automatic switch-off** | Auto-off CV | Time until forced off (unit **0.4 s**); 0 = disabled |

#### Configuration procedure

1. Look up **Mode Select** value for the desired effect.
2. Compute **Special Function CV 1** by adding option bit values.
3. Set **Brightness** (0–31).
4. Write values to the CVs for the target output (table on manual p. 85).

**Example — Double Strobe on AUX4 with LED:**

```
CV #299 (Mode Select) = 6      # Double Strobe
CV #302 (Brightness)   = 25
CV #303 (Special CV 1) = 128   # LED mode
```

### 3.4 Available lighting effects

| Mode Select | Effect |
|-------------|--------|
| **1** | Dimmable light |
| **2** | Dimmable headlight with fade in/out |
| **3** | Firebox |
| **4** | Intelligent firebox (intensity varies with global *Firebox* logic function) |
| **5** | Single strobe |
| **6** | Double strobe |
| **7** | Rotary beacon |
| **8** | Prime Stratolight |
| **9** | Ditch light type 1 (steady on when not flashing) |
| **10** | Ditch light type 2 (off when not flashing) |
| **11** | Oscillator (US warning signal) |
| **12** | Flashing light |
| **13** | Mars light |
| **14** | Gyra light |
| **15** | FRED (flashing end-of-train device) |
| **16** | Fluorescent lamp simulation |
| **17** | Energy-saving lamp simulation |
| **18** | Single strobe random |
| **21** | ESU coupler 1 / 2 (compatibility) |

Dimmable effects reduce to ~**50%** brightness when the global *Dimmer* logic function is active (if the output is configured for dimming).

### 3.5 Special output functions (non-lighting)

| Mode Select | Function | Notes |
|-------------|----------|-------|
| **22** | Fan control | Motor ramp up/down |
| **23** | Single strobe random | |
| **26** | Smoke unit (sound controlled) | |
| **27** | Steam trigger output | Pulse for KM-1 / Massoth clocked smoke |
| **28** | Smoke unit with external control | Special CV2: 0 = KM-1 BR41/44, 1 = other KM-1, 2 = Kiss, 3 = ESU |
| **30** | Heater control | Smoke generator heating |
| **31** | Fan control (smoke) | Blower speed via Brightness CV |
| **—** | Seuthe smoke unit | Intensity reduced when stationary |
| **—** | Conventional coupler | Krois couplers; supports auto push/remove |
| **—** | ROCO coupler | ROCO couplers; supports auto push/remove |
| **—** | Panto control | ESU pantograph locomotives |
| **—** | PowerPack control | External energy storage |
| **—** | Servo | RC servo (specific AUX outputs only) |

### 3.6 Special Function CV 1 — global interaction bits

| Bit value | Option | Effect |
|-----------|--------|--------|
| **1** | Rule 17 Forward | Dims to ~60% when stopped; full brightness forward |
| **2** | Rule 17 Reverse | Same, but full brightness in reverse |
| **4** | Phase select | 180° phase offset vs. other effects |
| **8** | Grade crossing | Active only when global *Grade Crossing* flag is set |
| **16** | Dimmer | Dims to ~60% when global *Dimmer* flag is set |
| **128** | LED mode | Adjust effects for LED loads (default outputs assume incandescent bulbs) |

### 3.7 Global lighting parameters

| CV | Name | Description |
|----|------|-------------|
| **#112** | Flash rate | Global flash/strobe rate. Time = value × 0.065536 s (default 20 → 1.00 s) |
| **#132** | Grade crossing hold time | Time global grade-crossing flag stays active after key off. Time = value × 0.065 s (default 80 → 5.2 s) |

### 3.8 Global logic functions affecting outputs

| Logic function | Mapping value (CV N/O/P) | Effect on outputs |
|----------------|--------------------------|-------------------|
| **Firebox** | 1 (CV P) | Intelligent firebox brightness variation |
| **Dimmer** | 2 (CV P) | ~60% dim on outputs configured for dimming |
| **Grade Crossing** | 4 (CV P) | Enables grade-crossing-gated effects |

---

## 4. Digital coupler (uncoupler)

LokSound 5 decoders can drive **Krois**, **ROCO**, and modern **Telex** digital couplers directly, plus run an **automatic uncoupling cycle**.

### 4.1 Coupler output mode

Set the function output to **Conventional coupler function** (Krois) or **ROCO coupler function** via Mode Select in output configuration.

**PWM behaviour:** The output turns on at **100% for 250 ms**, then switches to a high-frequency PWM signal to avoid burning the coupler coil.

| Parameter | Control |
|-----------|---------|
| **Hold power** | *Brightness* value 0–31 — ratio of off/on time in PWM phase (0 = off, 31 = continuous 100%) |

Use **automatic switch-off** (auto-off CV) for couplers that cannot handle permanent activation — ROCO couplers in particular. Unit: **0.4 s** per step.

### 4.2 Automatic uncoupling — CV settings

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#246** | Uncoupling drive speed | 0–255 | Speed for automated movement. **0 = function disabled** |
| **#247** | Pull-back time | 0–255 | Duration of reverse / pull-away phase |
| **#248** | Push time | 0–255 | Duration of push-into-consist phase |

**Pull-back time should exceed push time** so the locomotive stops at a safe distance from the train.

The coupler output must be configured correctly (coupler mode on the appropriate AUX output).

### 4.3 Automatic uncoupling cycle

The **Uncoupling cycle** logic function (mapping value **128** in CV O) triggers a full sequence:

1. Locomotive pushes backward against the train.
2. Coupler activates.
3. Locomotive pulls away.

Timing is governed by CV #246–#248. Map the logic function to the desired function key via function mapping.

### 4.4 Wiring notes

- Connect the coupler solenoid to any configured AUX output.
- Observe decoder maximum current per output (see manual §5.1.11).
- For Krois/ROCO couplers, use the dedicated coupler mode — do not use plain dimmable light mode.

---

## 5. Smoke generator

### 5.1 Output types and wiring

Smoke generators connect to function outputs (AUX1–AUX18). Choose the output mode matching the hardware:

| Type | Mode / configuration | Connection notes |
|------|----------------------|------------------|
| **Seuthe (basic)** | Seuthe smoke unit mode | Intensity reduced when stationary. Set output to *Dimmer* + full *Brightness* for best results |
| **Seuthe No. 10** | Dimmable output | High current; may trigger overload protection — use ESU relay (51963) or reduce brightness |
| **Seuthe No. 11** | Wire to U+ (not chassis) | Avoids asymmetric track signal; recommended when possible |
| **ESU synchronized** | Logic function *Smoke Units (ESU, KM-1, Kiss)* + Mode 28 / 30 / 31 | ESU, KM-1, or Kiss units with temperature sensor |
| **KM-1 / Massoth clocked** | Steam trigger output (Mode 27) | Generates chuff-synchronised control pulses |
| **Sound-controlled** | Mode 26 | Smoke intensity follows sound / driving state |

**LokSound 5 L** supports a dedicated 6-pin synchronized smoke unit (2× fan, 2× heater, 2× temperature sensor). The decoder reads the temperature sensor and regulates heating — preventing burn-through on empty tanks. Use only ESU spare-part generators or equivalent units with temperature sensors.

### 5.2 Smoke configuration CVs (per output)

When a smoke mode is selected, Special Function CVs control behaviour:

| Parameter | Typical CV | Range | Description |
|-----------|------------|-------|-------------|
| **Heater power at Vmin** | Special CV 2 | 0–31 | Heating at speed step 1 |
| **Heater power at Vmax** | Special CV 2 | 0–31 | Heating at full speed |
| **Blower strength** | Brightness CV | 0–31 | Fan power |
| **Acceleration time** | Special CV 2 | 0–31 | Ramp behaviour (fan control mode) |
| **Timeout** | Special CV 2 | 0–31 | Auto shut-off |
| **Steam chuff strength** | Special CV 2 | 0–31 | KM-1 / clocked units |
| **Run time A / B** | Brightness / Special CV 3 | 0–63 | Fan run phases (× 0.25 s) |

**Smoke unit with external control** (Mode 28) — set Special Function CV 2:

| Value | Locomotive type |
|-------|-----------------|
| **0** | KM-1 BR 41/44 |
| **1** | Other KM-1 locomotives |
| **2** | Kiss locomotives |
| **3** | ESU smoke units |

### 5.3 Activation via function mapping

Map the logic function **Smoke Units (ESU, KM-1, Kiss)** (value **32** in CV P) to a function key to enable synchronized smoke generators.

For Seuthe units on a plain AUX output, configure the output mode directly (no logic function required) and assign the AUX to a function key in mapping.

### 5.4 Practical tuning factors

Smoke amount depends heavily on:

1. **Track voltage** — even 1 V difference between command stations matters.
2. **Generator tolerance** — Seuthe units vary significantly; distillate type and fill level matter.
3. **Output brightness** — primary decoder-side control (0–31).
4. **Wiring** — chassis-referenced wiring receives power only every second half-cycle on DCC; U+ wiring avoids this.

---

## 6. Volume regulation

### 6.1 Master volume (CV #63)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#63** | Master volume | 0–192 | 180 | Controls **all** sound effects. 0 = mute |

Effective volume of each sound = master volume × individual sound volume.

If speakers distort (especially sugar-cube types), reduce CV #63 or lower the speaker baffle height.

### 6.2 Per-sound volume (indexed CVs)

Each sound occupies a **sound slot** (1–32). Individual volumes are set via indexed CVs:

- Set **CV #31 = 16, CV #32 = 1** before changing per-sound volume CVs.
- Range per sound: **0–128** (factory default typically **128** or **99** depending on slot).
- Sound-slot-to-CV mapping depends on the loaded **sound project** — refer to [projects.esu.eu](http://projects.esu.eu) for project-specific tables (steam / diesel / electric / special).

**Avoid clipping:** Do not set all individual volumes to maximum if many sounds play simultaneously — summed signals can clip (clicking/popping). Balance volumes for sounds that overlap.

#### Example sound-slot CV layout (steam project)

| Function | Sound slot | CV (index 1) | Default |
|----------|------------|--------------|---------|
| Sound on/off | 1 | #259 | 99 |
| Boiler noise | 2 | #267 | 99 |
| Whistle #1 | 3 | #275 | 128 |
| Bell | 4 | #283 | 128 |
| Air pump | 6 | #299 | 128 |
| Coupler sound | 8 | #315 | 128 |
| Brake set/release | 13 | #355 | 128 |
| … | … | … | … |

Diesel and electric projects use the same indexing scheme with different function names (fan motor, compressor, doors, etc.).

### 6.3 Volume control during operation

#### Logic function — Volume control

Map the **Volume control** logic function (value **64** in CV P) to a function key:

- Each button press (press and release) reduces volume by one of **four steps** down to minimum.
- Further presses cycle back up to maximum.

Useful for on-layout adjustment without reprogramming.

#### Sound fader (CV #133)

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#133** | Sound fader level | 0–255 | Volume when *Soundfader* logic function is active |

| Value | Effect |
|-------|--------|
| **0–127** | Lower than normal |
| **128** | Equal to normal (fader has no effect) |
| **129–255** | Higher than normal |

Map *Soundfader* (value **32** in CV P) for tunnel simulation or temporary mute.

### 6.4 External volume control (hardware)

| Decoder | Potentiometer | Notes |
|---------|---------------|-------|
| **LokSound 5 XL** | 100 kΩ log, ≥ 0.1 W (e.g. Piher PT 10 LV) | One pot per speaker output; keep wires short |
| **LokSound 5 L** | 10 kΩ log, ≥ 0.1 W | Single pot for both speaker outputs |

Wiring diagrams: manual Figures 30 and 32.

### 6.5 Related sound CVs

| CV | Feature |
|----|---------|
| **#64** | Braking sound threshold — when wheel-sync brake sound starts (default 100 ≈ speed step 48 of 128) |
| **#65** | Braking sound end fine-tuning |
| **#124 bit 3** | Prime mover startup delay — delete bit 3 to move immediately on throttle (sound may desync) |
| **#249** | Minimum distance between steam chuffs at high speed (unit 1 ms) |

---

## Appendix A — LokSound 5 decoder variants (overview)

| Form factor | Typical outputs | Notes |
|-------------|-----------------|-------|
| **LokSound 5** | Up to 22 | Standard HO / N |
| **LokSound 5 micro** | Reduced set | Compact installs |
| **LokSound 5 L** | Extended | Synchronized smoke unit, 10 kΩ ext. volume pot |
| **LokSound 5 XL** | Extended + 4 servos | Dual 10 V speaker outputs, 100 kΩ ext. volume pots |
| **LokSound 5 DCC / micro DCC / L DCC** | DCC only | CV #47 has no effect on protocol selection |

---

## Appendix B — Full CV table (CV #1–#255)

Source: manual chapter 21. LokSound 5 DCC uses **0.896 s** per unit for CV #3/#4 (multiprotocol decoders: × 0.25 s). Defaults are factory values; sound-project CVs (#155–#170) vary by project.

### Address and basic motor

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#1** | Loco address | 1–127 (1–255 Motorola) | 3 | Short address |
| **#2** | Start voltage (Vmin) | 1–255 | 3 | Minimum speed |
| **#3** | Acceleration | 0–255 | 28 | Time stop → max speed |
| **#4** | Deceleration | 0–255 | 21 | Time max speed → stop |
| **#5** | Maximum speed (Vmax) | 0–255 | 255 | Top speed |
| **#6** | Medium speed (Vmid) | 0–255 | 151 | 3-point speed table only (CV #29 bit 4 = 0) |
| **#7** | Version number | — | — | Read only |
| **#8** | Manufacturer ID | — | — | ESU ID; write **8** = factory reset |
| **#9** | Motor PWM frequency | 10–50 | 40 | Multiple of 1000 Hz |
| **#17** | Long address high byte | 0–255 | 192 | With CV #18; activate via CV #29 bit 5 |
| **#18** | Long address low byte | 0–255 | 3 | See [Appendix D](#appendix-d--long-address-calculation) |
| **#19** | Consist address | 0–255 | 0 | 0/128 = off; 1–127 fwd; 129–255 rev |
| **#21** | Consist mode F1–F8 | 0–255 | 0 | Function bits as CV #13 |
| **#22** | Consist mode F0, F9–F12 | 0–255 | 0 | Function bits as CV #14 |

### Momentum adjustment and speed table

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#23** | Adjust acceleration | 0–127 (+128 subtract) | 0 | Added to CV #3 |
| **#24** | Adjust deceleration | 0–127 (+128 subtract) | 0 | Added to CV #4 (manual typo: says CV #3) |
| **#66** | Forward trim | 0–255 | 128 | Factor n/128 forward; 0 = off |
| **#67–#94** | Speed table | 0–255 each | — | 28-point curve (CV #29 bit 4 = 1) |
| **#95** | Reverse trim | 0–255 | 128 | Factor n/128 reverse; 0 = off |
| **#101** | Shunting mode trim | 0–128 | 64 | Factor n/128 when shunting active |

### Configuration registers

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#27** | Brake mode | 0–255 | 28 | Bit sum: ABC L/R (1/2), ZIMO HLU (4), DC brake (8/16), Selectrix (32/64), const. brake at speed 0 (128) |
| **#28** | RailCom configuration | 0–255 | — | Bit 0 = CH1 address (1); bit 1 = CH2 data (2); bit 7 = RailCom Plus (128) |
| **#29** | Configuration register | 0–255 | 12 | Bit 0 = direction rev (1); bit 1 = 28/128 steps (2); bit 2 = analog off (4); bit 3 = RailCom off (8); bit 4 = 3-point curve (16); bit 5 = long address (32) |
| **#47** | Protocol selection | 0–255 | 15 | DCC (1), M4 (2), Motorola (4), Selectrix (8) — sum bits; DCC-only decoders ignore |
| **#49** | Extended configuration #1 | 0–255 | 19 | Bit 0 = load control off (1); bit 3/7 = Märklin consecutive addr (8/128); bit 4 = auto speed-step detect (16); bit 5 = LGB pulse mode (32) |
| **#50** | Analogue mode | 0–3 | 3 | Bit 0 = AC analog (1); bit 1 = DC analog (2) |
| **#124** | Extended configuration #2 | 0–255 | 21 | Bit 0 = directional bit (1); bit 1 = decoder lock (2); bit 2 = startup delay off (4); bit 3 = SUSI off (8); bit 4 = AUX9/wheel sensor (16); bit 5 = motor overload prot. (32); bit 6 = parking brake (64) |

### Index registers and programming assist

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#31** | Index register H | 0–16 | 16 | Page select for CV #257–#511 |
| **#32** | Index register L | 0–16 | 0 | Sub-page for indexed CVs |
| **#96** | Address offset (assist) | 0–9 | — | ROCO Multimaus: CV hundreds digit |
| **#97** | Address (assist) | 0–99 | — | ROCO: CV tens/ones digit |
| **#98** | Value offset (assist) | 0–9 | — | LokMaus II: value hundreds digit |
| **#99** | Value (assist) | 0–255 | — | ROCO/LokMaus: writes target CV when programmed |

### Back-EMF / PID

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#51** | «K slow» cutoff | 0–255 | 10 | Internal speed step until «K slow» applies |
| **#52** | BEMF «K slow» | 0–255 | 10 | PI gain at low speed |
| **#53** | Control reference voltage | 0–255 | 130 | Back-EMF voltage at max speed |
| **#54** | Load control «K» | 0–255 | 50 | PI proportional component |
| **#55** | Load control «I» | 0–255 | 100 | PI integral component |
| **#56** | BEMF influence at Vmin | 1–255 | 255 | 0–100% at minimum step |
| **#116** | Slow BEMF sampling period | 50–200 | 50 | × 0.1 ms at speed step 1 |
| **#117** | Full-speed BEMF sampling | 50–200 | 150 | × 0.1 ms at step 255 |
| **#118** | BEMF gap length Vmin | 10–20 | 15 | × 0.1 ms at step 1 |
| **#119** | BEMF gap length Vmax | 0–255 | 100 | × 0.1 ms at step 255 |

### Analogue operation

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#13** | Analog mode F1–F8 | 0–255 | 1 | Function bitmask |
| **#14** | Analog mode F0, F9–F15 | 0–63 | 1 | Function bitmask |
| **#125** | Start voltage analog DC | 0–255 | 90 | |
| **#126** | Max speed analog DC | 0–255 | 130 | |
| **#127** | Start voltage analog AC | 0–255 | 90 | |
| **#128** | Max speed analog AC | 0–255 | 130 | |
| **#129** | Analog function hysteresis | 0–255 | 15 | Offset for functions on |
| **#130** | Analog motor hysteresis | 0–255 | 5 | Offset for motor off |

### Braking, load, and consist

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#102** | Brake mode exit delay | 0–255 | 12 | × 16 ms before leaving brake sector |
| **#103** | Optional load factor | 0–255 | 0 | n/128 scales CV #3/#4 |
| **#104** | Primary load factor | 0–255 | 255 | n/128 scales CV #3/#4 |
| **#111** | Gearbox backlash | 0–255 | 0 | × 16 ms min speed after direction change |
| **#123** | ABC slow approach speed | 0–255 | 15 | Speed in ABC slow section |
| **#149** | ABC shuttle hold time | 0–255 | 255 | Seconds before direction reversal |
| **#150–#154** | HLU speed limits 1–5 | 0–255 | 42/85/127/170/212 | Internal speed steps |
| **#179–#181** | Brake function 1–3 decel | 0–255 | 80/40/40 | % deducted from CV #4 |
| **#182–#184** | Brake function 1–3 speed limit | 0–126 | 0/126/126 | Max speed when active |
| **#253** | Constant brake mode | 0–255 | 0 | 0 = linear; >0 = constant linear effort |
| **#254** | Constant brake distance fwd | 0–255 | 0 | Active in brake sectors |
| **#255** | Constant brake distance rev | 0–255 | 0 | Push-pull; 0 = use CV #254 |

### Sound and volume

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#57** | Steam chuff sync #1 | 1–255 | 30 | See [§6.5](#65-related-sound-cvs) |
| **#58** | Steam chuff sync #2 | 1–255 | 20 | Gear factor |
| **#63** | Master volume | 0–192 | 128 | All sounds |
| **#64** | Brake sound «on» threshold | 0–255 | 60 | Speed step |
| **#65** | Brake sound «off» threshold | 0–255 | 7 | Speed step |
| **#105–#106** | User CV #1 / #2 | 0–255 | 0 | Free storage |
| **#133** | Sound fader level | 0–255 | 128 | Volume when fader active |
| **#155–#170** | Sound CV 1–16 | 0–255 | 0 | Sound selection per project |

### Lighting (non-indexed)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#112** | Flash frequency global | 0–255 | 32 | × 0.065536 s |
| **#113** | Power fail bypass | — | — | PowerPack bridge time (× 0.032768 s) |
| **#132** | Grade crossing hold | 0–255 | 80 | × 0.065 s |
| **#134** | ABC detection threshold | 4–32 | 10 | Asymmetry sensitivity |

### Smoke generator

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#138** | Smoke fan trim | 0–255 | 128 | n/128 fan speed |
| **#139** | Smoke temperature trim | 0–255 | 128 | n/128 heater |
| **#140** | Smoke timeout | 0–255 | 255 | Auto shut-off |
| **#141** | Smoke chuff min | 0–255 | 10 | × 0.041 s |
| **#142** | Smoke chuff max | 0–255 | 125 | × 0.041 s |
| **#143** | Smoke chuff length | 0–255 | 100 | n/128 vs. trigger pulse |
| **#144** | Smoke preheat temperature | 0–255 | 150 | °C, secondary generator |
| **#249** | Min. steam chuff distance | 0–255 | 0 | 1 ms units |
| **#250** | Secondary chuff trigger | 0–255 | 0 | ‰ shorter than primary |

### Coupler and decoder lock

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#15–#16** | Decoder lock | 0–255 | 0 | NMRA decoder lock |
| **#246** | Auto decoupling speed | 0–255 | 0 | 0 = off |
| **#247** | Decoupling pull-away time | 0–255 | 0 | × 0.016 s |
| **#248** | Decoupling push time | 0–255 | 0 | × 0.016 s |

---

## Appendix C — Indexed CVs (#257–#511)

CVs **#257–#511** share physical addresses; meaning depends on **CV #31** (always **16**) and **CV #32**:

| CV #32 | Purpose | CV range used |
|--------|---------|---------------|
| **0** | Function output configuration (Mode, brightness, special CVs per AUX) | #257–#511 |
| **1** | Per-sound volume (sound slots 1–32) | #259–#443 |
| **3–5** | Function mapping — conditions block (72 rows) | #257–#506 |
| **8–10** | Function mapping — output block (physical, logic, sound) | #257–#506 |

**Rules:**

- Always set **CV #31 = 16** before accessing indexed CVs.
- The same CV number (e.g. #257) has **different content** at different CV #32 pages.
- Function mapping uses 72 rows × 20 control CVs (A–T); see manual pp. 75–78.
- Output configuration uses 7 CVs per physical output (Headlight, Rearlight, AUX1–AUX18); see [§3.3](#33-function-output-configuration).

Per-sound volume (CV #32 = 1): set CV #32 = 1, then CV #259+ control slot volumes 0–128. Sound-slot mapping is **project-specific** — see [projects.esu.eu](http://projects.esu.eu).

---

## Appendix D — Long address calculation

Long address = base (from CV #17) + CV #18. CV #17 selects the 256-address block (e.g. 207 → addresses 3840–4095). Example: address **4007** → CV #17 = **207**, CV #18 = **167**. See manual Figure 50.
