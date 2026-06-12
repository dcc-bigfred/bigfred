# RailBOX RB 23XX — Configuration Reference

Reference documentation for RailBOX **RB 2300** and **RB 2310** DCC Wi-Fi sound decoders. This document is **not** tied to the BigFred or Loco runtime; it is intended as a future data source for decoder configuration.

| Item | Value |
|------|-------|
| Source | [RB 23XX manual (PDF)](https://www.railbox.pl/_files/ugd/6c739b_223b9a94c9d94cd18887f7087ab4c124.pdf) (RailBOX); [forum thread RB 2300](https://forum.modelarstwo.info/threads/dcc-wi-fi-dekoder-jazdy-d%C5%BAwi%C4%99kowy-rb-2300.60042/) (manufacturer posts, 2023–2025) |
| Models | **RB 2300** (PluX22 or NEM652), **RB 2310** (21MTC) |
| Scope | Motor / Back-EMF, shunting, AUX mapping, lighting effects, couplers, volume, sound-pack naming |

**Decoder highlights:** DCC addresses 1–10239, F0–F28 function outputs, F0–F63 sounds, 28 or 128 speed steps, RailCom, Back-EMF, 9 function outputs + 3 logic outputs, Wi-Fi sound upload, `map.txt` / `logic.txt` / `cv.txt` configuration files.

---

## Table of contents

1. [Acceleration, deceleration, and speed](#1-acceleration-deceleration-and-speed)
2. [Shunting mode](#2-shunting-mode)
3. [Output mapping, brightness, and lighting effects](#3-output-mapping-brightness-and-lighting-effects)
4. [Digital coupler (uncoupler)](#4-digital-coupler-uncoupler)
5. [Smoke generator](#5-smoke-generator)
6. [Volume regulation](#6-volume-regulation)
7. [Sound pack file naming](#7-sound-pack-file-naming)

**Appendices:** [D — Full CV table](#appendix-d--full-cv-table) · [E — Forum insights (RailBOX)](#appendix-e--forum-insights-railbox-manufacturer)

---

## 1. Acceleration, deceleration, and speed

### 1.1 Basic momentum (CV #3 and CV #4)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#3** | Acceleration time | 0–255 | 34 | Time from stop to maximum speed. **4 ≈ 1 s**, **8 ≈ 2 s** (linear scale in 0.25 s steps) |
| **#4** | Deceleration time | 0–255 | 25 | Time from maximum to minimum speed. Same encoding as CV #3 |

Acceleration and deceleration are configured independently.

**Firmware ≥ 1.3:** NMRA encoding changed — when migrating from older firmware or ESU-style values, convert: `new = 1020 / old` (see [Appendix E](#e1-sound-wi-fi-and-troubleshooting)).

### 1.2 Speed curve (CV #2, #5, #6)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#2** | Minimum speed (Vstart) | 0–127 | 4 | Starting voltage / minimum speed. Manufacturer (SW ≥ 1.10.3): often **0** for smooth gearbox; SW ≥ 1.4: **3** or **4–5** if step 1 jerks — tune **CV #51** / **CV #55** |
| **#5** | Maximum speed (Vmax) | 0–255 | 255 | Maximum speed as % of full scale |
| **#6** | Average speed (Vmid) | 10–200 | 127 | Mid-point of the speed curve together with CV #2 and CV #5 |

Together, CV #2, #5, and #6 define the locomotive speed characteristic (NMRA-style 3-point curve).

### 1.3 Speed steps and direction (CV #29)

| CV #29 bit | Function |
|------------|----------|
| **0** | Locomotive direction: 0 = normal, 1 = reversed |
| **1** | Speed steps: 0 = 14/27, 1 = 28/128 |
| **2** | RailCom: 0 = disabled, 1 = enabled |
| **3** | Address type: 0 = short (CV #1), 1 = long (CV #17/#18) |

### 1.4 Back-EMF and PID (CV #50–#55, #58–#60)

Factory defaults are tuned for typical HO motors. Adjust for specific motor types.

#### PID coefficients

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#50** | PID KP (fast driving) | 0–255 | 40 | Proportional gain at higher speeds |
| **#51** | PID KP (slow driving) | 0–255 | 130 | Proportional gain at low speed — keep higher for stable creep without oscillation |
| **#52** | PID KI (fast) | — | 0 | Integral — factory 0; no improvement observed in RailBOX tests |
| **#53** | PID KI (slow) | — | 0 | Same |
| **#54** | PID KD / KFF_A (fast) | 0–40 | 7 | Derivative; immediate voltage change on speed step changes |
| **#55** | PID KD / KFF_D (slow) | 0–40 | 12 | Same for low speed |

KFF_A / KFF_D (CV #54, #55) mainly matter for **high** acceleration and deceleration values.

#### Back-EMF system

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#58** | PID interval | 40–160 | 80 | Back-EMF measurement interval |
| **#59** | Measurement delay | 6–20 | 6 | Delay between EMF samples |
| **#60** | Voltage at maximum speed | 30–90 | 90 | Target motor voltage at full speed. If below the motor's physical maximum, track voltage may vary but speed stays constant |

CV #60 differs from CV #5: CV #5 limits the speed **curve**; CV #60 sets the **Back-EMF regulation voltage** at maximum speed.

### 1.5 Start delay (CV #63)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#63** | Start delay | 0–255 | 10 | Delay before movement begins. Unit: **value × 100 ms** |

### 1.6 Consist address (CV #19)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#19** | Consist / multiple-unit address | 0–127 | 0 | If > 0, speed and direction follow this address (advanced consist) |

### 1.7 Persisting motor CVs via `cv.txt`

Important motor CVs can be stored in a `cv.txt` file uploaded with the sound pack. After a factory reset the decoder reloads these defaults:

```
cv1=3
cv50=40
cv51=130
```

---

## 2. Shunting mode

### 2.1 Function key assignment (CV #165)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#165** | Shunting mode function | 0–28 | **6** | Function key that activates shunting mode (**F6** by default) |

### 2.2 Shunting momentum (CV #61, #62)

Separate acceleration and deceleration times apply while shunting mode is active:

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#61** | Acceleration (shunting) | 0–255 | 10 | Same encoding as CV #3 (**4 ≈ 1 s** to max speed) |
| **#62** | Deceleration (shunting) | 0–255 | 10 | Same encoding as CV #4 |

Factory defaults give faster response in shunting than normal running (CV #3 = 34, CV #4 = 25).

### 2.3 Shunting in `logic.txt`

Shunting mode (typically **F6**) is widely used as a **trigger function** in sound and lighting automation:

| Example rule | Effect |
|--------------|--------|
| `F6_DIM_F0_V50` | Dim F0 headlights to 50% while F6 (shunting) is on |
| `F6_BLOCK_F12` | Mute F12 wheel sound while shunting |
| `F10_LON_F6_ON_D4000` | Play coupling sound 4 s after F6 is turned on |

Use the web generator at [railbox.pl/sounds](https://www.railbox.pl/sounds/) to build custom shunting logic.

### 2.4 Default PIKO mappings using F6

**EP08 (PluX22):**
```
AUX5:F6>,F27>
AUX6:F6<,F27<
```

**BR232 (PluX22):**
```
AUX5:F5<>
AUX6:F6>,F27>
AUX7:F6<,F27<
```

F6 switches directional AUX outputs during shunting (e.g. shunting-step lights).

---

## 3. Output mapping, brightness, and lighting effects

### 3.1 Physical outputs

| Model | Connector | Outputs |
|-------|-----------|---------|
| **RB 2300** | PluX22 or NEM652 | 9 function outputs + 3 logic outputs |
| **RB 2310** | 21MTC | 9 function outputs + 3 logic outputs |

**NEM652 pin assignment (RB 2300):**

| Pin | Wire | Function |
|-----|------|----------|
| 1–2 | Black / Red | DCC track |
| 3 | White | Front light |
| 4 | Yellow | Rear light |
| 5 | Green | Cabin light (F1) |
| 2 | — | F2 (changeable) |
| 5 | Brown | F3 (changeable) |
| — | Grey / Blue | Motor |

**21MTC (RB 2310):** CV #209 selects connector standard — **0** = NEM660 (AUX3/AUX4 as logic outputs), **1** = MKL (AUX3/AUX4 as power outputs).

### 3.2 Output mapping — `map.txt` and mobile app

Output-to-function assignment is **not** done via NMRA CV #33–#46. Instead:

1. **`map.txt`** file uploaded via Wi-Fi web interface or RailBOX: Railroad Control app.
2. **Mobile app** — Loco Editor → Edit CV → **OUTPUTS MAPPING** → Read/Write on programming track.

**Syntax:**

```
AUXn:Fn>,Fn<,Fm<>
F0F:F0>          # forward headlight
F0R:F0<          # reverse headlight
```

| Symbol | Meaning |
|--------|---------|
| `F0`–`F28` | Function key |
| `>` | Active when direction = forward |
| `<` | Active when direction = reverse |
| `<>` | Active in **both** directions |
| `,` | Multiple functions on one output |

The same output can map to several functions and directions. When mixing direction modes on one output, use only `<>` and leave arrows blank.

Web generator: [railbox.pl/sounds](https://www.railbox.pl/sounds/)

**NEM652 note:** Decoders without a preloaded `map.txt` assign outputs sequentially from F0 by default.

### 3.3 Lighting effects (CV #112–#118, #212–#215)

Each of the **11 outputs** has an independent effect CV:

| Output | Effect CV |
|--------|-----------|
| 1–7 | **#112–#118** |
| 8–11 | **#212–#215** |

#### Base effect values

| Value | Effect |
|-------|--------|
| **0** | Light bulb (steady) |
| **1** | Flashing, frequency 1 (period in CV #133) |
| **2** | Flashing frequency 1, reversed phase |
| **3** | Flashing, frequency 2 (period in CV #134) |
| **4** | Flashing frequency 2, reversed phase |
| **5** | Short pulse (duration in CV #137) |
| **6** | First custom sequence (CV #139–#151) |
| **7** | Second custom sequence (CV #152–#164) |
| **9** | Servo mode |

#### Effect modifiers (add to base value)

| Add | Effect |
|-----|--------|
| **+16** | Fade-in over time from CV #135 |
| **+32** | Fade-in over time from CV #136 |
| **+64** | Fade-in over fixed **500 ms** |
| **+128** | Run custom sequence only **once** |

#### Timing CVs

| CV | Name | Range | Default | Unit |
|----|------|-------|---------|------|
| **#133** | Flash period 1 | 0–255 | 100 | × 10 ms |
| **#134** | Flash period 2 | 0–255 | 100 | × 10 ms |
| **#135** | Fade-in time 1 | 0–255 | 20 | — |
| **#136** | Fade-in time 2 | 0–255 | 50 | — |
| **#137** | Single flash duration | 0–255 | 1 | × 10 ms |
| **#138** | Custom sequence step time | 0–255 | 1 | — |

Factory custom sequences are preloaded in CV #139–#164 (written one byte at a time).

### 3.4 Brightness (CV #119–#128, #219–#222, #126–#132, #226–#229)

| Output | Max brightness CV | Min brightness CV | Default max | Default min |
|--------|-------------------|-------------------|-------------|-------------|
| 1 | **#119** | **#126** | 255 | 0 |
| 2 | **#120** | **#127** | 255 | 0 |
| 3 | **#121** | **#128** | 255 | 0 |
| 4 | **#122** | **#129** | 255 | 0 |
| 5 | **#123** | **#130** | 255 | 0 |
| 6 | **#124** | **#131** | 255 | 0 |
| 7 | **#125** | **#132** | 255 | 0 |
| 8 | **#219** | **#226** | 255 | 0 |
| 9 | **#220** | **#227** | 255 | 0 |
| 10 | **#221** | **#228** | 255 | 0 |
| 11 | **#222** | **#229** | 255 | 0 |

### 3.5 Dynamic dimming via `logic.txt` (software ≥ 1.3)

The **DIM** logic function reduces brightness of a mapped function while a trigger is active:

```
F6_DIM_F0_V50
```

| Parameter | Meaning |
|-----------|---------|
| Trigger function | First `Fn` — e.g. F6 (shunting) |
| Target function | Second `Fn` — e.g. F0 (headlights) |
| **V** | Brightness level in **percent** (50 = 50%) |

Requires `logic.txt` upload. Generator: [railbox.pl/sounds](https://www.railbox.pl/sounds/).

### 3.6 Servos

Up to **two servos** connect to S1 and S2 terminals (−, +, signal). Set the target output to **effect value 9 (Servo mode)** in the corresponding effect CV.

---

## 4. Digital coupler (uncoupler)

### 4.1 Wiring

Digital couplers connect between:

- the **`+`** (common / LED anode) terminal, and
- a selected **function output** (external pads on the decoder board, or AUX output on the locomotive PCB).

Two servos and one digital coupler can be installed per the connection diagram in the manual (§4). The coupler may also use a dedicated pad on the locomotive's factory board if available.

**Load limits:** Observe decoder maximum output current (0.5 A total function outputs; 1 A continuous motor). Avoid short circuits — outputs have protection but external overvoltage can cause damage.

### 4.2 Output assignment

Assign the coupler to an AUX output via **`map.txt`** or the mobile app, e.g.:

```
AUX4:F7>
```

No dedicated coupler PWM mode or automatic uncoupling sequence is documented (unlike ESU or ZIMO). Control is **on/off** through normal function output switching.

### 4.3 Sound integration

Coupler sounds are triggered via `logic.txt` rules, e.g.:

```
F10_LON_F6_ON_D4000
```

Plays the F10 coupling sound 4 seconds after F6 (shunting mode) is activated.

### 4.4 Automatic switch-off

Use the **lighting effect** and output timing features, or logic rules, to limit energisation time. ROCO-style couplers cannot tolerate continuous activation — the manual recommends timed switch-off for digital couplers in the ESU context; apply the same principle by releasing the function key promptly or using `logic.txt` automation.

---

## 5. Smoke generator

The RB 23XX manual **does not describe a dedicated smoke generator mode** or synchronised smoke control.

Smoke units (e.g. Seuthe) can be wired to any **free function output** mapped via `map.txt`, subject to:

- maximum output current (**0.5 A** total for function outputs),
- appropriate brightness CV for the assigned output (#119–#222),
- effect CV set to **0** (steady output) unless pulsed behaviour is desired.

For speed-synchronised smoke, consider a third-party smoke module with its own control logic, or a different decoder family with dedicated smoke support (e.g. ESU LokSound, ZIMO MS).

**Practical note:** If the smoke generator is chassis-referenced, wire the second pole to **`+`** (U+) rather than ground to avoid half-wave power loss on DCC — same guidance as for other decoder brands.

---

## 6. Volume regulation

### 6.1 Master volume (CV #203)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#203** | Master volume | 0–255 | 64 | Overall sound playback level. **Values above 64 may cause distortion/interference** |

### 6.2 Per-function volume (CV #192, #193)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#192** | Function number | 0–255 | 0 | Select function (F1–F28) to adjust |
| **#193** | Volume level | 0–200 | 100 | **1–200%** of factory level. **0** = factory default (100%) |

**Procedure:** Write the function number to CV #192, then set CV #193. Repeat for each function.

### 6.3 Mute function keys (CV #206, #207)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#206** | Mute braking sound | 0–100 | 22 | Function number that mutes wheel/brake sounds |
| **#207** | Mute all sounds | 0–100 | 23 | Function number that mutes **all** sounds at once |

### 6.4 Runtime volume via `logic.txt` (software ≥ 1.3)

The **VOL** logic function sets all sounds to a specified level while active:

```
F23_VOL_V50
```

Reduces all sounds to **50%** while F23 is on. Parameter **V** = target volume percent.

### 6.5 Sound smoothness (CV #204, #205)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#204** | Function sound smoothness | 0–100 | 35 | Transition smoothness for function sounds. Unit: **value × 10 ms** |
| **#205** | Engine sound smoothness | 0–100 | 95 | Engine sound transition smoothness (% of file length, not less than CV #204 value) |

### 6.6 Logic system disable (CV #208)

Individual automation features can be turned off:

| CV #208 bit | Function disabled when = 1 |
|-------------|----------------------------|
| **0** | All logical operations |
| **1** | Periodic sounds |
| **2** | Function blocking |
| **3** | Start sounds |
| **4** | Stop sounds |

### 6.7 Sound pack selection (CV #202)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#202** | Sound pack number | 1–3 | 1 | Active **sound pack / track** (up to 3 packs, 6 MB total) — not an individual sound file |

**F28** (default Wi-Fi / sound enable, CV #200) must be **on** for any sound playback or Wi-Fi access. Per-sound levels and assignments are defined inside the uploaded sound pack. Factory sound-slot layouts vary by project — download packs from [railbox.pl/sounds](https://www.railbox.pl/sounds/).

### 6.8 Chuff / loop sound frequency (CV #210)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#210** | Fx_LOOP_Px frequency base | 1–255 | 100 | Repetition period = **x × (CV #210 / 100) / speed** for "chiu-chiu" loop sounds |

---

## 7. Sound pack file naming

Sound files uploaded to the decoder (Wi-Fi web interface or RailBOX app) are assigned to behaviour by **filename**. Supported formats: **PCM**, **ADPCM**, **Vorbis (OGG)** — see technical parameters in the manual.

This naming scheme is **independent** of `logic.txt` automation ([Appendix C](#appendix-c--logic-function-reference-logictxt)): filenames define built-in playback rules inside the sound pack; `logic.txt` adds optional cross-function rules on top.

### 7.1 Filename structure

```
F{n}_[S{speed}_][D{delay}_]{TYPE}[{suffix}].wav
```

| Component | Meaning |
|-----------|---------|
| **`F{n}`** | Function key **F0–F63** that triggers or owns the sound |
| **`S{speed}`** | Optional minimum speed threshold in **percent** (e.g. `S40` = play only when speed ≥ 40%) |
| **`D{delay}`** | Optional delay in **milliseconds** — positive = after event; negative = before event (see examples) |
| **`{TYPE}`** | Playback type (table below); omit type = play once in full |
| **`{suffix}`** | Station index for **INFO** / **INFONEXT** sequences (`1`, `2`, …) |
| **`_V{n}`** | Per-file volume **percent** (firmware ≥ 1.3), e.g. `_V80` |
| **`_T{n}`** | **Actual** file duration in ms — required for **ADPCM LOOP** (ADPCM pads silence at end) |
| **`_E{n}`** | Minimum speed **percent** for some ON-type sounds, e.g. `F5_ON_S2_E50` |
| **`_M{n}`** | **TURBO** threshold on LOAD parameter (firmware ≥ 1.12), e.g. `F1_TURBO_M60` |
| **`_P{n}`** | Chuff / **LOOP_P** period index (steam), e.g. `F1_LOOP1_S1_P8000` |
| **`{letter}`** | Alternate sound **set** for random engine variants (firmware ≥ 1.12), e.g. `F2B_LOOP` alongside `F2_LOOP` |

**Extension:** `.wav` for PCM/ADPCM; OGG files use the same naming with `.ogg` (e.g. `F1_STOP_D3500_EN57.ogg`).

**Sample rate:** Do not mix **16 kHz** and **32 kHz ADPCM** files in one pack (forum, 2025). 32 kHz ADPCM packs are offered separately on [railbox.pl/sounds](https://www.railbox.pl/sounds/).

### 7.2 Playback types

| Type | Behaviour |
|------|-----------|
| *(omitted)* | No playback type — sound plays **once in full** when triggered |
| **ON** | Plays when the function is **turned on** |
| **OFF** | Plays when the function is **turned off** |
| **LOOP** | Loops while the function is **active** |
| **LOOPD** | Loops while the locomotive is **decelerating** |
| **ACCEL** | Plays during **acceleration**; when acceleration ends, the nearest **LOOP** sound is resumed |
| **DECEL** | Plays during **deceleration**; when deceleration ends, the nearest **LOOP** sound is resumed |
| **ACCELON** | Plays **in full** at each acceleration start (length ≥ 2× fade time from CV #204); LOOPs continue underneath (firmware ≥ 1.5) |
| **ACCEL_S**x / **DECEL_S**x | Speed-dependent accel/decel layers — file **x** matches current speed band; LOOPs continue underneath (firmware ≥ 1.5) |
| **DIR** | Direction-change sound, e.g. `F1_DIR.wav` (firmware ≥ 1.10.3) |
| **TURBO** | Plays while internal **LOAD** > `_M` threshold (firmware ≥ 1.12) |
| **LOOP_P** | Steam **chuff** loop; repetition period = **x × (CV #210 / 100) / speed** |
| **STOP** | Plays when the locomotive **comes to a stop** |
| **ESTOP** | Plays on **emergency stop** (e.g. double-tap STOP in RailBOX app) |
| **START** | Plays when the locomotive **begins moving** — minimum delay must exceed motor start delay ([CV #63](#15-start-delay-cv-63)); factory default start offset is **−1000 ms** |
| **INFO** | Train **current-station** announcement (paired with **INFONEXT**) |
| **INFONEXT** | Train **next-station** announcement (paired with **INFO**) |

#### ADPCM and LOOP

For **ADPCM** files with type **LOOP**, include parameter **`_T`** = real audio length in milliseconds (IMA ADPCM adds trailing silence). Parameter **`_S`** = minimum speed 1–100%. Use the converter / emulator from [railbox.pl/sounds](https://www.railbox.pl/sounds/).

### 7.3 Worked examples

| Filename | Effect |
|----------|--------|
| `F1_ON.wav` | One-shot sound when **F1** is turned on |
| `F0_S40_LOOP.wav` | Loops while **F0** is on **and** locomotive speed ≥ **40%** |
| `F0_STOP_D500.wav` | **STOP** sound: triggered on halt with **500 ms** delay so playback **finishes 500 ms after** the locomotive has fully stopped |
| `F0_START_D-1000.wav` | **START** sound: begins **1000 ms (1 s) before** movement starts |
| `F20_INFO1.wav` | Current-station announcement, station set **1** |
| `F20_INFONEXT1.wav` | Next-station announcement, station set **1** |
| `F20_INFO2.wav` | Current-station announcement, station set **2** |
| `F20_INFONEXT2.wav` | Next-station announcement, station set **2** |

### 7.4 Station announcements (INFO / INFONEXT)

Pair **INFO**n with **INFONEXT**n for each station index. Firmware ≥ 1.5 behaviour (manufacturer):

| Function key | Behaviour |
|--------------|-----------|
| **Fx** | Each **momentary** press advances to the **next** station in the sequence |
| **F(x+1)** | Each press plays the **previous** station in the sequence |
| **Fx + F(x+1) together** | Resets sequence position |

**Rule:** **INFONEXT**n always plays **before** **INFO**n within each station set.

Classic two-key layout (manual / early packs):

| Function key | Playback order |
|--------------|----------------|
| **F20** | Station sets **1 → 2** |
| **F21** | Station sets **2 → 1** |

Example layout for two stations:

```
F20_INFO1.wav
F20_INFONEXT1.wav
F20_INFO2.wav
F20_INFONEXT2.wav
```

Press **F20** → INFONEXT1, INFO1, INFONEXT2, INFO2.  
Press **F21** → INFONEXT2, INFO2, INFONEXT1, INFO1.

### 7.5 Relation to `logic.txt`

| Mechanism | Configured via | Use case |
|-----------|----------------|----------|
| **Filename types** (ON, LOOP, START, …) | Sound file names in pack folder | Built-in decoder behaviour per function |
| **Logic rules** (START, STOP, BLOCK, …) | `logic.txt` | Cross-function timing, blocking, DIM, VOL |

Both can coexist in one project. Prefer **filenames** for standard per-function sounds; use **`logic.txt`** when one function must trigger another function's sound with explicit delays (`L`, `D`, `R` parameters — see [Appendix C](#appendix-c--logic-function-reference-logictxt)).

---

## Appendix A — Configuration files

| File | Purpose |
|------|---------|
| **`map.txt`** | AUX-to-function mapping with direction |
| **`logic.txt`** | Sound automation, DIM, VOL, BLOCK, ACC/DCL triggers |
| **`cv.txt`** | Default CV values **per sound pack** — see below |
| **`functions.txt`** | Loco name + function labels for RailCom Plus / DCCA auto-discovery (firmware ≥ 1.11.1) |
| **Sound files** | `F{n}_…_{TYPE}.wav` — see [§7](#7-sound-pack-file-naming) |
| **Firmware `.bin`** | Upload unpacked firmware via Wi-Fi file browser (top upload button) |

Upload via Wi-Fi: enable **F28**, connect to `RB2300_XXXXX` (password `000000000`), browse to `http://192.168.4.1`.

**`cv.txt` format** (manufacturer, forum): lowercase keys, one per line, upload into the **sound pack folder** (not decoder root):

```
cv1=35
cv2=160
cv3=70
```

Values apply when that pack is active; they reload after factory reset if stored in the pack. Use as last resort when the command station cannot program CV #1 (e.g. Roco MultiMaus PoM limitation).

**`functions.txt`:** Generate in RailBOX: Railroad Control app — long-press **ZASTOSUJ** on an existing loco profile (firmware ≥ 1.11.1). Required for automatic loco registration on **RB1110** / PIKO WLAN (RailCom Plus / DCCA).

**Emulator:** Desktop emulator at [railbox.pl/sounds](https://www.railbox.pl/sounds/) previews packs **offline** — it does not connect to the decoder. After validation, upload the folder of `.wav` / `.ogg` files.

---

## Appendix B — Key default logic rules (factory PIKO packs)

**EP08:**
```
F2_L1500_ESTOP_D200
F4_BLOCK_F1
F6_BLOCK_F12
F9_BLOCKDRV … F26_BLOCKDRV
F17_L4000_DCL_V300
F21_ACCDCL_V500_L4000
F13_DCL_V200_L4000
```

**BR232:**
```
F2_L1500_ESTOP_D200
F6_BLOCK_F12
F9_BLOCKDRV … F20_BLOCKDRV
F17_L4000_DCL_V300
F19_L4000_ACC_V200
F21_L4000_ACCDCL_V500
F13_DCL_V200_L4000
```

---

## Appendix C — Logic function reference (`logic.txt`)

| Keyword | Trigger | Key parameters |
|---------|---------|----------------|
| **START** | Locomotive starts moving | L, D, optional R |
| **STOP** | Locomotive stops | L, D, optional R |
| **ESTOP** | Emergency stop (double-tap STOP in app) | L, D |
| **ON** | Trigger function turned on | L, D, trigger Fn |
| **OFF** | Trigger function turned off | L, D, trigger Fn |
| **ONOFF** | Trigger toggled on/off | L, D, R (odd preferred) |
| **LON** | Play full ON sound after trigger on | D, trigger Fn |
| **LOFF** | Play full OFF sound after trigger off | D, trigger Fn |
| **BLOCK** | Mute sound while trigger active | trigger Fn, blocked Fn |
| **BLOCKDRV** | Mute sound while driving | blocked Fn |
| **ACC** | Play when acceleration total reaches V% | L, V |
| **DCL** | Play when deceleration total reaches V% | L, V |
| **ACCDCL** | Play when combined accel+decel reaches V% | L, V |
| **DIM** | Reduce target **function** brightness (SW ≥ 1.3) | trigger Fn, target Fn, V% |
| **DIM_O** | Reduce **output** brightness (SW ≥ 1.8) | trigger Fn, output Ox, V% — e.g. `F5_DIM_O3_V40` |
| **VOL** | Set all sounds to V% (SW ≥ 1.3) | trigger Fn, V% |
| **RANDOM** | Random interval playback (SW ≥ 1.6.3) | S, E, L, optional `_INSTOP` / `_INMOVE` |
| **TOGETHER** | Link two functions (SW ≥ 1.8) | e.g. `F60_TOGETHER_F1` |
| **VOLSPD** | Volume follows speed (SW ≥ 1.12) | e.g. `F1_VOLSPD_V50` |
| **VOLLOAD** | Volume follows LOAD (SW ≥ 1.12) | e.g. `F1_VOLLOAD_V50` |
| **SPD** | Speed-dependent sound (SW ≥ 1.12) | e.g. `F1_SPD_S50` |
| **FAN** | Fan sound vs LOAD (SW ≥ 1.12) | e.g. `F1_FAN_M60` |
| **MINLOAD** | Minimum LOAD for sound (SW ≥ 1.12) | e.g. `F1_MINLOAD_M60` |

**Hall / GPIO input** (SW ≥ 1.10.0, CV #64 = 2): prefix **`I1_`** — e.g. `I1_TOGETHER_F13_INMOVE`, `F2_ON_I1_L2000`.

**SUSI** (SW ≥ 1.5): map **CLK** / **DAT** in `map.txt` like other outputs.

| Parameter | Meaning |
|-----------|---------|
| **L** | Sound length (ms) |
| **D** | Delay before playback (ms) |
| **R** | Repeat counter (R2 = every 2nd event, etc.) |
| **V** | Threshold % for ACC/DCL, brightness % for DIM, volume % for VOL |
| **S** / **E** | RANDOM interval start / end (seconds) |
| **Ox** | Physical output number for DIM_O |
| **M** | LOAD threshold % (FAN, MINLOAD, TURBO filenames) |

---

## Appendix D — Full CV table

Source: manual CV configuration table (pp. 13–17). Output mapping is primarily via `map.txt` / mobile app — not CV #33–#46.

### Address, motor, and speed

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#1** | Decoder address | 1–127 | 3 | Short address |
| **#2** | Minimum speed (Vstart) | 0–127 | 4 | Starting voltage |
| **#3** | Acceleration | 0–255 | 34 | **4 ≈ 1 s** from 0 to max speed |
| **#4** | Deceleration | 0–255 | 25 | **4 ≈ 1 s** from max to min speed |
| **#5** | Maximum speed | 0–255 | 255 | Max speed as % of full scale |
| **#6** | Average speed (Vmid) | 10–200 | 127 | Speed curve with CV #2 and #5 |
| **#7** | Software version | 0–255 | 172 | Read only |
| **#8** | Manufacturer ID / reset | 0–255 | **172** (SW ≥ 1.11.1) | Read: NMRA manufacturer ID; write **1** = factory reset |
| **#17** | Long address high byte | 192–231 | 192 | Long address with CV #18 (CV #29 bit 3) |
| **#18** | Long address low byte | 0–255 | 3 | |
| **#19** | Consist address | 0–127 | 0 | If > 0: speed/direction from consist address |
| **#110** | Product code XX | 0–255 | **23** (RB2300) | Read only; RB**XX**YY (SW ≥ 1.11.1) |
| **#111** | Product code YY | 0–255 | **0** (RB2300) | Read only |

### CV #28 — RailCom configuration (bit field)

| Bit | Value | Function |
|-----|-------|----------|
| 0 | 1 | CH1 address broadcast |
| 1 | 2 | CH2 data transmission |
| 3 | 8 | Automatic detection system |

### CV #29 — Decoder configuration (bit field)

| Bit | Value | Function |
|-----|-------|----------|
| 0 | 1 | Reversed direction |
| 1 | 2 | 28/128 speed steps (0 = 14/27) |
| 2 | 4 | RailCom enabled |
| 3 | 8 | Long address (CV #17/#18) |
| 4 | 16 | **28-point speed table** CV #67–#94 (SW ≥ 1.12) |

### Lighting effects (CV #112–#118, #212–#215)

Outputs **1–7** use CV #112–#118; outputs **8–11** use CV #212–#215. Range **0–135** each, default **0**.

| Base value | Effect |
|------------|--------|
| **0** | Light bulb (steady) |
| **1** | Flash frequency 1 (period CV #133) |
| **2** | Flash frequency 1, reversed phase |
| **3** | Flash frequency 2 (period CV #134) |
| **4** | Flash frequency 2, reversed phase |
| **5** | Short pulse (duration CV #137) |
| **6** | Custom sequence 1 (CV #139–#151) |
| **7** | Custom sequence 2 (CV #152–#164) |
| **9** | Servo mode |

**Modifiers** (add to base): **+16** fade-in (CV #135); **+32** fade-in (CV #136); **+64** fade-in 500 ms; **+128** run custom sequence once.

### Brightness per output

| Output | Max brightness CV | Default | Min brightness CV | Default |
|--------|-------------------|---------|---------------------|---------|
| 1 | **#119** | 255 | **#126** | 0 |
| 2 | **#120** | 255 | **#127** | 0 |
| 3 | **#121** | 255 | **#128** | 0 |
| 4 | **#122** | 255 | **#129** | 0 |
| 5 | **#123** | 255 | **#130** | 0 |
| 6 | **#124** | 255 | **#131** | 0 |
| 7 | **#125** | 255 | **#132** | 0 |
| 8 | **#219** | 255 | **#226** | 0 |
| 9 | **#220** | 255 | **#227** | 0 |
| 10 | **#221** | 255 | **#228** | 0 |
| 11 | **#222** | 255 | **#229** | 0 |

### Lighting timing and custom sequences

| CV | Name | Range | Default | Unit / notes |
|----|------|-------|---------|--------------|
| **#133** | Flash period 1 | 0–255 | 100 | × 10 ms |
| **#134** | Flash period 2 | 0–255 | 100 | × 10 ms |
| **#135** | Fade-in time 1 | 0–255 | 20 | |
| **#136** | Fade-in time 2 | 0–255 | 50 | |
| **#137** | Single flash time | 0–255 | 1 | × 10 ms |
| **#138** | Custom sequence step time | 0–255 | 1 | |
| **#139–#151** | Custom sequence 1 | 0–255 | factory | One byte per step; factory seq. in manual |
| **#152–#164** | Custom sequence 2 | 0–255 | factory | One byte per step |

### Back-EMF, PID, and shunting

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#50** | PID KP (fast driving) | 0–255 | 40 | Proportional gain |
| **#51** | PID KP (slow driving) | 0–255 | 130 | Proportional gain at low speed |
| **#54** | PID KD (fast driving) | 0–40 | 7 | Differential gain |
| **#55** | PID KD (slow driving) | 0–40 | 12 | |
| **#58** | Back-EMF PID interval | 40–160 | 80 | |
| **#59** | Back-EMF measurement delay | 6–20 | 6 | |
| **#60** | Back-EMF voltage at max speed | 30–90 | 90 | Target regulation voltage |
| **#61** | Acceleration (shunting) | 0–255 | 10 | Same encoding as CV #3 |
| **#62** | Deceleration (shunting) | 0–255 | 10 | Same encoding as CV #4 |
| **#63** | Start delay | 0–255 | 10 | × 100 ms before movement |
| **#165** | Shunting mode function | 0–28 | **6** | Function key for shunting (F6) |
| **#167** | Motor cut-off on power loss | 0–2 | **0** | Write **2** to re-enable NMRA motor cut-off (default off since SW 1.4) |

### CV #64 — Pin configuration (bit field)

| Bit | Value | Function |
|-----|-------|----------|
| 0 | 0 / 1 | SUSI: **0** = on, **1** = off |
| 1 | 2 | O12 (GPIO/C) as input IN1 |
| 3 | 8 | Invert O12 input |

### Volume and sound

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#192** | Function volume — function # | 0–68 | 1 | Write function number first |
| **#193** | Function volume — level | 0–200 | 100 | 1–200%; 0 = factory (100%) |
| **#194** | LOAD simulation — max | 0–255 | 255 | LOAD ceiling (SW ≥ 1.12) |
| **#195** | LOAD simulation — rate up | 0–255 | 10 | LOAD increase rate (SW ≥ 1.12) |
| **#196** | LOAD simulation — rate down | 0–255 | 10 | LOAD decrease rate (SW ≥ 1.12) |
| **#200** | Wi-Fi control function | 0–100 | 28 | Function key; value **> 68** disables Wi-Fi |
| **#201** | Wi-Fi TX power | 20–80 | 40 | 20 = 5 dBm … 80 = 20 dBm |
| **#202** | Sound pack number | 1–3 | 1 | Active sound project |
| **#203** | Master volume | 0–255 | 64 | **Values > 64 may distort** |
| **#204** | Function sound smoothness | 0–100 | 35 | × 10 ms transition |
| **#205** | Engine sound smoothness | 0–100 | 95 | % of file length (≥ CV #204) |
| **#206** | Mute braking sound | 0–100 | 22 | Function number |
| **#207** | Mute all sounds | 0–100 | 23 | Function number |
| **#210** | Fx_LOOP_Px frequency | 1–255 | 100 | Period = x × (CV #210 / 100) / speed |

### CV #208 — Logic system disable (bit field)

| Bit | Value | Function disabled when 1 |
|-----|-------|------------------------|
| 0 | 1 | All logical operations (`logic.txt`) |
| 1 | 2 | Periodic sounds |
| 2 | 4 | Function blocking |
| 3 | 8 | Start sounds |
| 4 | 16 | Stop sounds |

### Connector and misc

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#209** | 21MTC connector standard | 0–1 | 0 | 0 = NEM660 (AUX3/4 logic); 1 = MKL (AUX3/4 power) |

**LOAD simulation** (SW ≥ 1.12): internal parameter **LOAD** 0–255 rises with motor current and falls when coasting. Formula (manufacturer): `LOAD = CV194 × (motor_current / max_current)`. Tune **CV #195** / **#196** for response. Use with `VOLLOAD`, `FAN`, `MINLOAD`, `TURBO` filenames and logic rules.

**CV #3 / #4 after SW 1.3:** NMRA encoding changed — convert old values: `new = 1020 / old` (round to integer).

**CV #28** (SW ≥ 1.7): also enables **ABC** automatic braking when bit set per NMRA.

---

## Appendix E — Forum insights (RailBOX manufacturer)

Condensed from [55 pages](https://forum.modelarstwo.info/threads/dcc-wi-fi-dekoder-jazdy-d%C5%BAwi%C4%99kowy-rb-2300.60042/) of the modelarstwo.info RB 2300 thread (posts by **railbox**, 2023–2025). Use alongside the PDF manual when behaviour differs by firmware version.

### E.1 Sound, Wi-Fi, and troubleshooting

| Topic | Manufacturer guidance |
|-------|----------------------|
| **No sound** | **F28** (or CV #200 function) must be **on** — sound and Wi-Fi are gated. **CV #202** selects the **sound pack / track**, not an individual file; an empty pack is silent. |
| **Wi-Fi / firmware** | F28 on → SSID `RB2300_XXXXX`, password `000000000` → `http://192.168.4.1` → upload `.bin` via top upload control. Video: [YouTube update guide](https://youtu.be/GDzI6cEoNvA). |
| **CV programming** | Do **not** copy ESU LokSound CV defaults into RB decoders. After SW **1.3**, recalculate CV #3/#4: `new = 1020 / old`. |
| **CV #2 (Vstart)** | SW **1.10.3+**: **0** often best for smooth gearbox start. SW **1.4+**: **3** or **4–5** if step 1 is jerky. Tune PID **CV #51** / **CV #55** for creep. |
| **Dirty track** | SW **1.2+**: ~**1 s** stop on brief power loss (configurable; motor cut-off default changed in SW **1.4** — see CV #167). |
| **DCC glitches** | SW **1.10.0+** fixes harsh starts and random function activation from noisy DCC. |

### E.2 Hardware and connectors

| Topic | Detail |
|-------|--------|
| **Extra solder pads** | Pads on the Wi-Fi module side can wire a 4th output (default **F2**); NEM652 pins 1–3 are used on standard installs. |
| **Capacitor** | Optional flat **440 µF** (10×17×2.3 mm) if space is tight. |
| **Plux22** | Full connector: motor, lights, **speaker pins 15/17**, cap **pins 6/9/5**, **SUSI**, **GPIO/C**. Factory speaker + cap may be pre-soldered and removable. |
| **21MTC** | No dedicated cap pin. Early **RB2310** MKL wiring: hardware ties **AUX4** with **AUX5** — use **CV #209 = 0** (NEM660) unless MKL confirmed. |
| **PIKO EP08** | On-board cap may be **unconnected** unless jumper **R7** (V+) or **R8** (C+) is closed. |
| **Hall sensor** | On-board sensor (SOT-23) drives **GPIO/C** on Plux22. **CV #64 = 2** enables as **IN1**. Kits **RB2400 / RB2410 / RB2411** use magnets — [mounting video](https://youtu.be/yUGpkADMuC4). |
| **RB2302** (announced) | Nine **16 V** and five **5 V** outputs on decoder PCB, independent of Plux22 GPIO A/B. |

### E.3 Firmware changelog (highlights)

| Version | Notable changes (manufacturer) |
|---------|-------------------------------|
| **1.1** | Initial release |
| **1.2** | ~1 s stop on power interruption |
| **1.3** | New CV #3/#4 encoding; per-file `_V` volume; `DIM` / `VOL` logic |
| **1.4 / 1.4.1** | Motor cut-off default off (CV #167); CV #2 tuning; analog mode improvements |
| **1.5** | **SUSI**; `ACCELON`, `ACCEL_Sx` / `DECEL_Sx`; refined INFO / INFONEXT |
| **1.6.x** | EN57 pack; analog running; output **BLOCK** in logic |
| **1.6.3** | **RB2310**; **`RANDOM`** logic |
| **1.7 / 1.7.1** | Energy save; **ABC** braking (CV #28) |
| **1.8** | Speed memory; PWM lights; **`TOGETHER`**, **`DIM_O`** |
| **1.9** | Per-function volume via CV #192/#193; extended `map.txt` |
| **1.10.0 / 1.10.3** | DCC glitch fix; **Hall / I1_** logic; **`F1_DIR`** |
| **1.11.1** | **RailCom Plus**; **`functions.txt`**; CV #8 = **172**; CV #110/#111 product code |
| **1.12.0** | **LOAD** simulation (CV #194–#196); 28-step table (CV #29 bit 4); **`VOLSPD`**, **`VOLLOAD`**, **`SPD`**, **`FAN`**, **`MINLOAD`**, **`TURBO`**; random letter sound sets (`F2B_LOOP`); **`TOGETHER`** `_D` delay |

Current firmware: check [railbox.pl](https://www.railbox.pl) / decoder **CV #7**.

### E.4 Example logic snippets (forum)

**Hall — wheel squeal on curves while moving:**
```
I1_TOGETHER_F13_INMOVE
```

**Hall — 2 s horn when passing a magnet:**
```
F2_ON_I1_L2000
```

**Random compressor (10–30 s, only when stopped):**
```
F12_RANDOM_S10_E30_L4000_INSTOP
```

**Dim output 3 to 40% when F5 on:**
```
F5_DIM_O3_V40
```

**Link F60 with F1:**
```
F60_TOGETHER_F1
```

### E.5 Support

Manufacturer asks users to report bugs via e-mail (see [railbox.pl](https://www.railbox.pl)). Custom Wi-Fi function mapping (other than F28) may be programmed on order.
