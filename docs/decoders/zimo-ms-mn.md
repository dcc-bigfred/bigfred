# ZIMO MS / MN / FS Decoders — Configuration Reference

Reference documentation for ZIMO **MS** (sound), **MN** (non-sound), and **FS** (function-sound) decoder families. This document is **not** tied to the BigFred or Loco runtime; it is intended as a future data source for decoder configuration.

| Item | Value |
|------|-------|
| Source | [MS-MN-Decoders_EN.pdf](https://www.zimo.at/web2010/documents/MS-MN-Decoders_EN.pdf) (ZIMO, 2026) |
| Software version | 5.27.14 |
| Scope | MS sound decoders (primary), MN non-sound (identical motor/function behaviour), FS function-sound decoders |

MS and MN decoders share the same hardware (except sound components) and software. Sections marked *sound only* can be ignored for MN decoders.

---

## Table of contents

1. [Acceleration, deceleration, and speed](#1-acceleration-deceleration-and-speed)
2. [Shunting mode](#2-shunting-mode)
3. [Output mapping, brightness, and lighting effects](#3-output-mapping-brightness-and-lighting-effects)
4. [Digital coupler (uncoupler)](#4-digital-coupler-uncoupler)
5. [Smoke generator](#5-smoke-generator)
6. [Volume regulation](#6-volume-regulation)

**Appendix:** [C — Full CV table](#appendix-c--full-cv-table)

---

## 1. Acceleration, deceleration, and speed

### 1.1 Basic momentum (CV #3 and CV #4)

The fundamental acceleration and deceleration times are set with **CV #3** (acceleration) and **CV #4** (deceleration), per NMRA convention:

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#3** | Acceleration time | 0–255 | 2 | Value × 0.9 = seconds from stop to full speed |
| **#4** | Deceleration time | 0–255 | 1 | Value × 0.9 = seconds from full speed to stop |

**Practical guidance:**

- Use values **3 or higher** for smooth driving; start with **5** for very slow starts/stops.
- Values above **30** are rarely useful except with the brake key.
- On **sound decoders**, the loaded sound project usually defines its own CV #3/#4 defaults. Changing them too far from the project values can break sound synchronisation.

**Important difference from MX decoders:** On MS/MN decoders, acceleration/deceleration follows the **speed table** (including interpolation), not 255 equidistant steps. An exponential speed curve therefore produces exponential acceleration behaviour. MX decoders needed CV #121/#122 for this; MS/MN do not.

**CV #49 / #50 interaction (HLU/ABC):** For signal-controlled operation, only the **higher** of CV #3 or CV #49 (acceleration) and CV #4 or CV #50 (deceleration) is used — values are **not added** (unlike MX decoders).

### 1.2 Temporary momentum variation (CV #23, #24)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#23** | Acceleration variation | 0–255 | 0 | Temporary increase/decrease of CV #3 (bit 7 = 0/1) |
| **#24** | Deceleration variation | 0–255 | 0 | Temporary increase/decrease of CV #4 (bit 7 = 0/1) |

### 1.3 Adaptive acceleration (CV #123) — SW 6.00+

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#123** | Adaptive acceleration/deceleration | 0–99 | 0 | Tens digit: acceleration effect (1 = strong); ones digit: deceleration effect. Value 0 = disabled. Value **11** = strongest effect. Transition to the next internal speed step occurs only when the preceding step is nearly reached. |

### 1.4 Speed curve selection (CV #29, bit 4)

| CV #29 bit 4 | Curve type | Defining CVs |
|--------------|------------|--------------|
| **0** | 3-point speed curve | CV #2, #5, #6 |
| **1** | 28-point freely programmable curve | CV #67–#94 |

#### 3-point speed curve

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#2** | Vstart (start voltage) | 1–255 | 1 | Internal speed step applied as lowest external step (= step 1). Value 1 = lowest possible speed. |
| **#5** | Vhigh (top speed) | 0–255 | 255 | Internal speed step for highest external step (14/28/128 depending on mode). 0 and 1 both equal 255. |
| **#6** | Vmid (medium speed) | 1–255 | 1 | Internal speed step for medium external step (7/14/64). Default characteristic: medium speed ≈ ⅓ of top speed. If CV #5 = 255, curve equals CV #6 = 85. Curve is automatically smoothed (no sharp bends). |

#### 28-point speed curve

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#67–#94** | Free speed table | 0–255 each | Internal speed step (1–255) for each of 28 external steps. Applies to 14, 28, and 128-step modes. In 128-step mode, intermediate values are interpolated. Default curve emphasises the lower speed range. |

#### Directional speed trimming

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#66** | Forward trim | 0–127 | 0 | Speed step multiplication by n/128 in forward direction |
| **#95** | Reverse trim | 0–127 | 0 | Same for reverse direction |

### 1.5 Voltage reference and motor regulation (CV #57, #58, #9, #56, #147–#149)

#### CV #57 — Voltage reference

Specifies the reference voltage for motor regulation. The decoder tries to deliver the exact fraction of this voltage to the motor regardless of track voltage fluctuations.

| Value | Meaning |
|-------|---------|
| **0** | Relative reference — auto-adapts to track voltage. Only useful with stabilised track output (all ZIMO systems qualify). Not recommended on unstable third-party systems. |
| **100–255** | Absolute voltage in tenths of a volt at full throttle (e.g. 140 = 14 V). Useful range: 10–24 V. Set ~2 V below expected track voltage. Does not work on 5 V motor output decoders (e.g. MN250). |

#### CV #58 — BEMF / load compensation intensity

| Value | Effect |
|-------|--------|
| **0** | No back-EMF (unregulated behaviour) |
| **150** | Medium compensation |
| **255** | Maximum compensation (default) |

Useful range: 100–200. Locomotives in consists should **never** run at 100% compensation across the full speed range — this causes fighting and derailments.

For precise load compensation across the full range, use CV #58 together with CV #113 (3-point curve). *Note: CV #10 on MX decoders; on MS decoders CV #10 serves another function.*

#### Motor control PID (CV #9, #56, #147–#149)

| CV | Name | Default | Description |
|----|------|---------|-------------|
| **#9** | Motor control period / EMF sampling | 55 | Hundreds digit 1: coreless motor settings. Tens digit 1–4: lower sampling rate; 6–9: higher (anti-judder). Ones digit 1–4: shorter EMF time; 5–9: longer. |
| **#56** | P and I value (legacy) | 55 | Tens digit = proportional; ones digit = integral. Only effective if CV #147–#149 = 0. Values 10, 20, … 90 are **not allowed**. |
| **#147** | Integral (PID) | 100 | Full-resolution integral. Auto-synced from CV #56. |
| **#148** | Differential (PID) | 100 | Differential component. |
| **#149** | Proportional (PID) | 100 | Proportional component. |

**Recommended starting values by motor type:**

| Motor type | CV #9 | CV #147–#149 |
|------------|-------|--------------|
| 3-pole / ringfield | 78, 88, 98 | 150–200 |
| 5-pole (e.g. Roco) | 38, 48, 58 | 100–150 |
| Faulhaber / coreless | 172, 182, 192 | 30–60 |
| Modern powertrain | — | #147=65, #148=45, #149=65 |

**Tuning procedure for CV #56:** Start at 11; drive slowly into an obstacle. Regulation should compensate within 0.5 s — if not, increase ones digit (12, 13, …). Then increase tens digit (23, 33, …) until judder appears, then step back.

#### CV #146 — Gear backlash compensation

Compensates for gear backlash on direction changes. Motor turns at minimum speed (CV #2) for a set duration before accelerating.

| Value | Approximate effect |
|-------|-------------------|
| **50** | ~½ turn or max ½ second |
| **100** | ~1 turn or max 1 second (typical) |
| **200** | ~2 turns or max 2 seconds |

Only useful with CV #58 = 200–255. CV #2 must be set so the train moves correctly at step 1.

### 1.6 Motor brake (CV #151) — SW 6.00+

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#151** | Motor brake / consist regulation | 0–99 | 0 | **Motor brake:** When PWM = 0 but speed not yet reached, motor is shorted through amplifier (1–8 s). **Consist mode:** Tens digit 1–9 reduces CV #58 regulation to 10–90% when consist key active. |

Useful for non-worm-gear locomotives to prevent rolling on grades.

### 1.7 Brake key (CV #309, #349)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#309** | Brake key | 0, 1–29 | 0 | Function key acting as brake key. 0 = off; 1 = F1 … 28 = F28; 29 = F0. Uses deceleration from CV #349, ignoring CV #4. |
| **#349** | Brake time for brake key | 0–255 | 0 | Brake deceleration time. Set CV #4 very high (50–250) and CV #349 low (5–20) for coast-then-brake effect. |

### 1.8 Emergency stop (CV #111)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#111** | Emergency delay time | 0–255 | 0 | Valid for emergency stop (single and collective) instead of CV #4. |

### 1.9 Distance-controlled stopping (CV #140, #141, #830–#833)

Alternative to time-controlled braking (CV #4):

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#140** | Constant braking distance selection | 0, 1, 2, 3, 11, 12, 13 | 0 | 1 = auto stop (HLU/ABC); 2 = manual; 3 = both. Values 11–13 = immediate braking on section entry. |
| **#141** | Braking distance | 0–255 | 20 | CV #141 = 255 ≈ 500 m prototype (6 m HO); CV #141 = 50 ≈ 100 m (1.2 m HO). |
| **#830–#833** | Direction-dependent braking distance | 0–255 each | 0 | High/low byte forward and backward. Factor 16× vs CV #141. Only effective if CV #141 = 0. |
| **#143** | HLU compensation | 0–255 | 0 | Detection delay compensation for HLU method. |

**Variant 1** (CV #140 = 1, 2, 3): At lower entry speed, train continues briefly then brakes normally — recommended.

**Variant 2** (CV #140 = 11, 12, 13): Immediate braking even at low entry speed.

### 1.10 Signal-controlled speed influence (HLU / ABC)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#49** | HLU/ABC acceleration | 0–255 | 0 | Value × 0.4 = seconds. Higher of CV #3 or #49 used. |
| **#50** | HLU/ABC deceleration | 0–255 | 0 | Value × 0.4 = seconds. Higher of CV #4 or #50 used. |
| **#51–#55** | Speed limits (U, L, intersteps) | 0–255 | 20/40/70/110/180 | Internal speed steps for 5 HLU speed stages |
| **#59** | HLU/ABC delay | 0–255 | 5 | Tenths of a second before acceleration after higher speed limit command |

Signal-controlled times in CV #49/#50 are **added** to CV #3/#4/#121/#122 curves and can only be equal or slower, never faster.

### 1.11 Analog operation speed (CV #14, #179)

| CV | Name | Description |
|----|------|-------------|
| **#14** | Analog acceleration/deceleration | Bit 0 = 1, bit 6 = 0: momentum per CV #3/#4 in analog mode. Bit 6 = 1: no momentum (classical analog). |
| **#179** | Increased speed with rail tension | 0–255. Sets maximum speed in analog mode (controlled and uncontrolled). Default 128. From SW 5.15. |

### 1.12 Advanced / planned features (SW 6.00 preview)

| CV | Feature |
|----|---------|
| **#135, #136** | km/h regulation operating state |
| **#394** | Faster acceleration and sound on rapid throttle advance (bit 4); impedes acceleration with brake key active (bit 6) |
| **#246, #348** | Special acceleration possibilities for diesel-mechanical locos |
| **#364, #365** | Acceleration interruption for diesel-mechanical |
| **#399** | Speed-dependent high beam (Rule 17) |

---

## 2. Shunting mode

Shunting (manoeuvring) support allows temporarily reducing acceleration/braking times and limiting the speed range — essential when the prototypical momentum from CV #3/#4 is a hindrance during yard work.

### 2.1 Primary configuration — CV #124

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#124** | Shunt key functions | Bits | 0 | Combined bit field for half-speed key and acceleration deactivation |

**Bit assignments in CV #124** (only when CV #155 = 0 and CV #156 = 0):

| Bits | Function |
|------|----------|
| **4 = 1** (bit 3 = 0) | **F3** as half-speed key |
| **3 = 1** (bit 4 = 0) | **F7** as half-speed key |
| **2 = 0, 6 = 0** | MAN key as acceleration deactivation |
| **2 = 1, 6 = 0** | **F4** as acceleration deactivation |
| **6 = 1** (bit 2 irrelevant) | **F3** as acceleration deactivation |

**Acceleration deactivation type** (bits 1, 0 — always apply, including with CV #155/#156):

| Bits 1,0 | Effect |
|----------|--------|
| **00** | No influence on acceleration times |
| **10** | Reduces acceleration/deceleration to **¼** of CV #3/#4 values |
| **11** | **Completely deactivates** acceleration/deceleration |

**Examples:**

| CV #124 value | Configuration |
|---------------|---------------|
| **16** | F3 as half-speed key |
| **23** | F3 half-speed + F4 full acceleration deactivation (bits 0,1,2,4 = 1) |
| **83** | F3 half-speed + F3 acceleration deactivation (bits 0,1,4,6 = 1) |
| **3** | Typical: full deactivation (unless other bits set) |

### 2.2 Extended half-speed key — CV #155 (preferred for new projects)

Alternative to CV #124 bit selections for half-speed:

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#155** | Function key for half speed | 0, 1–28, 29, 30 | 0 | 0 = CV #124 valid; 1–28 = F1–F28; 29 = F0; 30 = MAN key. If > 0, CV #124 half-speed assignment is overridden. |

**Half-speed multiplier** (bits 7, 6, 5 of CV #155):

| Bits 7:5 | Multiplier applied to speed step |
|----------|----------------------------------|
| **000** | × 0.625 |
| **001–100** | × 0.125 … × 0.5 |
| **100–111** | × 0.5 … × 0.875 |

### 2.3 Extended acceleration deactivation key — CV #156 (preferred for new projects)

Alternative to CV #124 for acceleration deactivation key selection:

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#156** | Function key for accel/decel deactivation | 0, 1–28, 29, 30, 129–158 | 0 | 0 = CV #124 valid; 1–28 = F1–F28; 29 = F0; 30 = MAN key. Bit 7 = 1: suppress light switching on direction change. |

The **type** of deactivation (bits 1,0 of CV #124) still applies when CV #156 selects the key.

### 2.4 MAN function — CV #157

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#157** | MAN-function key (non-ZIMO controllers) | 0, 1–28, 29 | 0 | Assigns a function key to cancel HLU/ABC stop and speed-limit commands on controllers without a dedicated MAN key. |

Originally designed for ZIMO HLU; expanded to include Lenz ABC asymmetrical DCC signal stops.

### 2.5 Interaction rules

- If **CV #155 > 0**, any half-speed assignment in CV #124 is ineffective.
- If **CV #156 > 0**, any acceleration deactivation key assignment in CV #124 is ineffective.
- Bits 2, 3, 4, 6 in CV #124 (shunt key function selection) only apply when CV #155 = 0 and CV #156 = 0.
- Bits 0, 1 (deactivation **type**) always apply regardless of CV #155/#156.

### 2.6 Sound-related shunting features *(sound decoders)*

| CV | Feature |
|----|---------|
| **#312** | Blow-off key (default F13) — blow-off noise for shunting with open valves |
| **#288** | Minimum drive time before brake squeal — suppresses squeal on short shunting runs without cars |
| **#267 / #268** | Blow-off effect cancelled when shunting function with motor load is active |

---

## 3. Output mapping, brightness, and lighting effects

ZIMO small-scale decoders provide **4–12 function outputs (FO)**. Large-scale decoders provide more. Consumers (lights, smoke, couplers, etc.) connect to FO1–FO8 (and beyond on large decoders).

### 3.1 NMRA function mapping (CV #33–#46)

Standard NMRA mapping: each function key (F0–F12) has one 8-bit register selecting which outputs it controls.

| Function key | ZIMO key # | CV | Default value |
|--------------|------------|-----|---------------|
| F0 forward | 1 (L) fw | #33 | 1 |
| F0 reverse | 1 (L) rev | #34 | 2 |
| F1 | 2 | #35 | 4 |
| F2 | 3 | #36 | 8 |
| F3 | 4 | #37 | 2 |
| F4 | 5 | #38 | 4 |
| F5 | 6 | #39 | 8 |
| F6 | 7 | #40 | 16 |
| F7 | 8 | #41 | 4 |
| F8–F12 | 9–3 | #42–#46 | (powers of 2) |

Each bit in the CV corresponds to one FO (bit 0 = FO1, bit 1 = FO2, …). Factory default: Fn controls FOn.

**Limitations:** Only 8 outputs per function key; only F0 is direction-dependent in standard NMRA mapping.

**Example remapping** (F2 also switches FO4; F3/F4 switch FO7/FO8 for couplers):

```
CV #36 = 40   (F2 → FO4 + FO6)
CV #37 = 32   (F3 → FO6)
CV #38 = 64   (F4 → FO7)
```

#### Extended NMRA mapping (CV #61)

| CV #61 | Effect |
|--------|--------|
| **0** | Standard NMRA mapping with left-shift |
| **97** | NMRA mapping **without left-shift** — higher F-keys can access lower FOs (e.g. F4 → FO1) |

### 3.2 Input mapping (CV #400–#428)

Remaps external function keys to internal decoder functions without changing sound projects:

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#400** | Input mapping for internal F0 | 0, 1–28, 29, 30–187, 254, 255 | 0 = F0→F0; 1 = F1→F0; … 28 = F28→F0; 30–57 = F1–F28→F0 forward only; 58–86 = F0–F28→F0 reverse only; 101–187 = inverted keys; 254/255 = direction bit → F0 |
| **#401–#428** | Input mapping for internal F1–F28 | Same pattern | E.g. CV #403 = 9 maps F9 → internal F3 |

### 3.3 Swiss mapping (CV #430–#507, #800–#823)

Complex function mapping for multi-state locomotive lighting (Swiss prototype, also useful elsewhere). **17 CV groups × 6 CVs = 102 CVs.**

Each group structure:

| CV offset | Role |
|-----------|------|
| +0 | F-key number (1–28, 29 = F0, 129–157 with bit 7 options) |
| +1 | M-key (master ON/OFF, often F0 = 157) |
| +2 | A1 forward — outputs to switch ON |
| +3 | A2 forward — additional outputs |
| +4 | A1 reverse |
| +5 | A2 reverse |

**Special M-key value CV #431 = 255:** Full-beam override — outputs at full intensity via normal mapping, dimmed per CV #60 otherwise.

**Dimming CVs #508–#512:** Five dimming levels (0 = dark, 31 = full). Selected per group via bits 5–7 in A1/A2 CVs.

**Speed-dependent high beam (CV #399):** From SW 6.00. High beam only above configured internal speed step (Rule 17).

### 3.4 Global brightness / PWM dimming (CV #60, #114, #152)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#60** | PWM voltage reduction (all outputs) | 0–255 | 0 | 0 or 255 = full voltage; 170 = ⅔; 204 = 80%. Affects all function outputs. |
| **#114** | Dim mask 1 | Bits 0–7 | 0 | Exclude outputs from CV #60 dimming. Bit 0 = front headlight, bit 1 = rear, bits 2–7 = FO1–FO6. |
| **#152** | Dim mask 2 | Bits 0–5, 6, 7 | 0 | Bits 0–5 = FO7–FO12. Bit 6: direction bits on FO3/FO4. Bit 7: direction bit for FO9 forward. |

**Connection notes:**

- Bulbs rated **12 V or higher** can be dimmed via PWM even at high track voltage.
- Bulbs **below 12 V** (5 V, 1.2 V) must use the decoder's **stabilised low-voltage pin**, not PWM dimming.
- LEDs always need a series resistor; if resistor drops to 5 V, PWM dimming works (e.g. CV #60 = 50 for 25 V track).

**Alternative dimming via low-voltage pin:** Connect positive side to the decoder's stabilised low-voltage supply (see installation chapter) for constant voltage regardless of track voltage.

### 3.5 Low / high beam (CV #119, #120)

| CV | Name | Description |
|----|------|-------------|
| **#119** | Low beam mask for **F6** | Bits 0–6 select outputs dimmed to CV #60 value when F6 active. Bit 7 = 1: inverted (dim when F6 off). |
| **#120** | Low beam mask for **F7** | Same as CV #119 with F7 as key. |

Example: CV #119 = 131 → headlights switch between high/low beam with F6.

### 3.6 Second dimming value (CV #115)

If more outputs need individual dimming than CV #60 allows, and uncoupler is not needed:

Configure the output's effect CV (#127–#132, #159, #160) as uncoupler (effect code 48), then use CV #115 ones digit for PWM reduction (0–90%). Hundreds digit sets wait time before driving away (if used as uncoupler).

### 3.7 Light suppression (CV #107, #108, #109, #110)

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#107** | Turn off lights at cab 1 | 0–220 | Value = (FO number 1–6) × 32 + F-key number. Turns off front headlights AND defined FO on cab 1 side. |
| **#108** | Turn off lights at cab 2 | 0–255 | Same for cab 2 (rear). |
| **#109** | Additional FO suppression cab 1 | 1–6 | FO turned off together with CV #107. |
| **#110** | Additional FO suppression cab 2 | 1–6 | FO turned off together with CV #108. |

### 3.8 Direction-dependent outputs via effect CVs

Since NMRA mapping only makes F0 directional, use **effect CVs with effect code 000000xx** (direction only, no effect) for direction-dependent FO control.

**Example 1 — directional taillights on F1:**
```
CV #35 = 12   (F1 → FO1 + FO2)
CV #127 = 1   (FO1: forward only)
CV #128 = 2   (FO2: reverse only)
```

**Example 2 — unilateral light suppression:**
```
CV #33 = 1, CV #34 = 8
CV #35 = 6
CV #126 = 1, CV #127 = 2
```

### 3.9 Special lighting effects (CV #125–#132, #159, #160, #195–#200, #205–#207)

Each effect CV = **6-bit effect code** (bits 7–2) + **2-bit direction code** (bits 1–0):

| Direction bits | Meaning |
|----------------|---------|
| **00** | Bidirectional |
| **01** | Forward only (+0 to code) |
| **10** | Reverse only (+2 to code) |

#### Effect code table

| Code (bits 7–2) | Effect | Example values |
|-----------------|--------|----------------|
| 000000 | Direction only (no effect) | 0, 1, 2 |
| 000001 | Mars light | 4, 5, 6 |
| 000010 | Random flicker (charcoal) | 8, 9, 10 |
| 000011 | Flashing headlight | 12, 13, 14 |
| 000100 | Single pulse strobe | 16, 17, 18 |
| 000101 | Double pulse strobe | 20, 21, 22 |
| 000110 | Rotary beacon simulator | 24, 25, 26 |
| 000111 | Gyralite | 28, 29, 30 |
| 001000 | Ditch light type 1, right | 32, 33, 34 |
| 001001 | Ditch light type 1, left | 36, 37, 38 |
| 001010 | Ditch light type 2, right | 40, 41, 42 |
| 001011 | Ditch light type 2, left | 44, 45, 46 |
| 001100 | **Uncoupler** | 48, 49, 50 |
| 001101 | Soft start (slow brightening) | 52, 53, 54 |
| 001110 | Automatic brake light (tram) | 56, 57, 58 |
| 001111 | Auto OFF at speed > 0 (cab light) | 60, 61, 62 |
| 010000 | Auto OFF after 5 minutes | 64, 65, 66 |
| 010001 | Auto OFF after 10 minutes | 68, 69, 70 |
| 010010 | Speed/load-dependent smoke (steam) | 72, 73, 74, 116, 124, 132 |
| 010011 | SUSI smoke generator | 76 |
| 010100 | Driving-state smoke (diesel) | 80, 81, 82, 120, 128, 135 |
| 010110 | Slow brightening & dimming | 88, 89, 90 |
| 010111 | Fluorescent tube effect | 92, 93, 94 |
| 011000 | Sparks on heavy braking | 96, 97, 98 |
| 011010 | Dimming (value in CV #192) | 104, 105, 106 |
| 011011 | Firebox effect (steam project) | 108, 109, 110 |
| 011100 | Servo protection relay | 112 |

#### Effect CV assignment by output

| Output | Effect CV |
|--------|-----------|
| Front headlight | #125 |
| Rear headlight | #126 |
| FO1–FO6 | #127–#132 |
| FO7–FO8 | #159–#160 |
| FO9–FO14 | #195–#200 |
| FO15–FO17 | #205–#207 |

#### Effect modification CVs

| CV | Name | Description |
|----|------|-------------|
| **#62** | Afterglow brake light | 0–255 tenths of a second (0–25 s) afterglow at standstill |
| **#63** | Lighting effect modifications | Tens digit: cycle time (0–9, default 5). Ones digit: extended off-time. Also soft-start brightening time for code 001101. |
| **#64** | Ditch light modifications | Bits 7–4: ditch light key (function key+1)×16. Bits 3–0: OFF time in seconds. |
| **#190** | Brightening-up time (effects 88–90) | 0–100 = 0–1 s; 101–200 = 1–100 s; 201–255 = 100–320 s |
| **#191** | Dimming time (effects 88–90) | Same ranges as CV #190 |
| **#192** | Dimming value (effect 011010) | 0–255 percent (127 = 50%) |
| **#393** | ZIMO Config 5 | Bit 0: ditch light on bell; bit 1: ditch light on horn |
| **#117** | Flasher duty cycle | Tens digit = off time; ones digit = on time (0 = 100 ms … 9 = 1 s). E.g. 55 = 1 s equal on/off. |
| **#118** | Flasher mask | Bits select which outputs flash. Bit 6: FO2 inverse; bit 7: FO4 inverse (wig-wag). |

**Ditch lights:** Only active when headlights (F0) **and** F2 are on (American prototype). CV #33/#34 bits must also enable the FO — effect CV alone is insufficient.

### 3.10 SUSI pins as additional outputs (CV #124 bit 7, #201, #203)

| CV | Setting | Application |
|----|---------|-------------|
| **#124** bit 7 = 1 | 128 | Logic level instead of SUSI (legacy; prefer CV #201) |
| **#201** | 11 | SUSI pins as logic level outputs (FO9, FO10, …) |
| **#201** | 22 | Reed inputs |
| **#201** | 33 | Servo control lines |
| **#201** | 44 | SUSI burst mode |
| **#201** | 55 | I2C bus (not yet implemented) |
| **#201** | 66 | SUSI compatibility mode |

First non-zero CV in order (#201, #202, #203, #204, #181–#184) determines the application.

---

## 4. Digital coupler (uncoupler)

### 4.1 Effect assignment

Assign effect code **001100xx** (48, 49, 50) to the function output driving the coupler solenoid:

| Output | Effect CV | Example (bidirectional) |
|--------|-----------|-------------------------|
| FO1 | #127 | 48 |
| FO2 | #128 | 49 (forward only) |
| … | … | … |
| FO7 | #159 | |
| FO8 | #160 | |

Direction bits: 00 = both directions; 01 = forward; 10 = reverse.

### 4.2 Control CVs

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#115** | Uncoupler activation time / dimming | 0–255 | 0 | **Uncoupler mode** (effect 48 in #127–#132/#159/#160): Tens digit = pull-in time at full voltage (see table below). Hundreds digit = wait before driving away. **Dimming mode:** tens = 0; ones digit = PWM reduction 0–90%. |
| **#116** | Automatic disengagement | 0, 1–99, 0–199 | 0 | Tens digit = disengagement duration (same coding as #115). Ones digit × 4 = internal speed step for disengagement. Ones = 0: standstill. Hundreds: 0 = no unloading; 1 = coupler unloading before uncouple. |

**CV #115 pull-in time (tens digit):**

| Digit | Duration |
|-------|----------|
| 0 | 0 s |
| 1 | 0.1 s |
| 2 | 0.2 s |
| 3 | 0.4 s |
| 4 | 0.8 s |
| 5 | 1 s |
| 6 | 2 s |
| 7 | 3 s |
| 8 | 4 s |
| 9 | 5 s |

**CV #115 hundreds digit — wait before driving away:**

| Digit | Wait time |
|-------|-----------|
| 0 | 0.3 s |
| 1 | 2.5 s |
| 2 | 1.0 s |

### 4.3 Recommended settings

**Krois system:** CV #115 = **60**, **70**, or **80** (2, 3, or 4 s pull-in at full track power).

**Roco system:** CV #115 can define hold-in voltage via ones digit (partial voltage during pull-in).

### 4.4 Automated uncoupling sequence

**Automatic train disengagement** is active when the tens digit of CV #116 ≠ 0.

**Coupler unloading** (optional): CV #116 > 100 — engine moves toward train to relieve coupler tension before uncoupling.

**Procedure:**

1. Uncoupling starts when the coupling function is activated **and** the train is at speed 0. If moving, the sequence waits for a full stop.
2. Coupler unloading and/or disengagement run for the times set in CV #115 (coupler) and CV #116 (disengagement).
3. Sequence ends when the function key is released (or pressed again in latched mode), or when times expire.
4. **Aborted immediately** if the throttle is moved during the sequence.
5. Disengagement direction follows the set direction; directional bits in the uncoupler effect CV are **not** applied.

**Example:**
```
CV #115 = 60   (2 s disengagement drive-away after uncouple)
CV #116 = 155  (active push to disengage, speed step 20, 1 s)
```

### 4.5 Wiring

Connect the uncoupler solenoid to any function output FO1–FO8 (or logic-level FO9+ via SUSI pin reconfiguration). The negative return connects to the decoder ground; positive to the FO pin (or via appropriate driver circuit per coupler manufacturer specifications).

For Krois/Roco digital couplers, follow the manufacturer's voltage and current ratings. The decoder FO outputs switch track voltage (or stabilised low voltage when connected to LV pin).

### 4.6 Worked example: PIKO SM31 PKP (MS450P22)

Field configuration for a **PIKO SM31** with ZIMO **MS450P22** (PluX22). The factory uses **FO1** / **FO2** (AUX1 / AUX2) for directional white and red lighting — do not assign the uncoupler there. On this model **FO8** (`cv41=128`) drives shunting-step lighting; the digital coupler is wired to **FO6** (AUX6, PluX pin 21).

**F7** triggers uncoupling with:

- 2 s coupler energisation (`CV #115`)
- coupler unloading (push into the consist), then uncouple and drive away (`CV #116` hundreds digit = 1)
- 5 s / speed step 28 for the automated move (`CV #116` = 197)

| CV | Value | Role |
|----|-------|------|
| **#41** | 32 | F7 → **FO6** (not 128 — that is FO8) |
| **#132** | 48 | Uncoupler effect on FO6 |
| **#115** | 60 | 2 s pull-in at full voltage |
| **#116** | 197 | Unloading + 5 s disengage at speed step 28 |
| **#415** | 0 | F7 not remapped to F15 (cab/inspection lighting) |
| **#308** | 0 | No cornering-squeal sound on F7 |

Leave **CV #127** / **#128** at the PIKO project defaults (e.g. 89 / 90) so F0 and F4 lighting on FO1 / FO2 are unchanged.

```bash
loco cv set -l 31 "cv41=32, cv132=48, cv115=60, cv116=197, cv415=0, cv308=0"
```

**CV #116 = 197** decodes as: hundreds **1** (push into consist first) · tens **9** (5 s) · ones **7** (7 × 4 = speed step 28). For a shorter or faster shove-away, try **167** (2 s, step 28) or **157** (1 s, step 28); see [§4.2](#42-control-cvs).

**Sequence (loco at speed 0, direction set before F7):**

1. Push into the consist (coupler unloading).
2. Uncouple (up to 2 s on FO6 per `CV #115`).
3. Drive away from the consist for the time set in `CV #116`.

The sequence aborts if the throttle is moved during the manoeuvre.

---

## 5. Smoke generator

### 5.1 Connection overview

#### Small / medium decoders (FO1–FO8)

**Without fan (e.g. Seuthe 18 V):**

- Heating element → any FO1–FO8.
- Configure effect code **72** (steam) or **80** (diesel) in the corresponding effect CV.

**With fan (synchronised steam chuffs / diesel exhaust):**

- Heating element → FO1–FO8 (with effect 72 or 80).
- Fan → **FO4** (or FO2 on smaller decoders such as MS491). Set **CV #133 = 1**.
- Fan second pole: low voltage (5 V output if suitable) or external regulator.

#### Large-scale decoders (MS950, MS990, MN950)

**Dedicated fan outputs V1 and V2** (5 V, ground-referenced) — strongly preferred over FO4 for fans:

| Component | Connection |
|-----------|------------|
| 1st heating element | FO3 (+) |
| 2nd heating element (dual) | FO7 (+) |
| Fan 1 | V1 (ground) |
| Fan 2 (dual) | V2 (ground) |

**ZIMO smoke generators:**

| Model | Size | Gauges |
|-------|------|--------|
| **RAUSI1** | 49×29×27 mm | Single, 0/1/G |
| **RAUSI2** | 45×24×25 mm | Dual (smaller), 0 |
| **RAUDU1** | 49×29×31 mm | Dual, 0/1/G |

Heating element voltage: **20–24 V**. RAUSI/RAUDU units include overtemperature protection electronics; the decoder drives heaters and fans directly.

**Wiring (dual RAUDU):**
```
FO7 — common positive — FO3 — Ventilator V1 — GROUND — Ventilator V2
```

Use LOKPL950K / LOKPL990 adapter boards for plug-in connection.

### 5.2 Effect codes for smoke

| Code | Application |
|------|-------------|
| **72** | Speed/load-dependent smoke — steam (CV #137–#139) |
| **80** | Driving-state-dependent smoke — diesel (CV #137–#139, fan on FO4) |
| **76** | SUSI smoke generator |
| **116** | Steam heating element (planned, SW 6) |
| **124** | Fan for steam bursts |
| **132** | Fan for cylinder steam / Mallet |
| **120** | Diesel heating element (planned) |
| **128** | Diesel fan (planned) |
| **136** | Diesel fan, reduced speed when braking (planned) |

Assign to effect CV for the FO carrying the heater. **CV #137–#139 must be programmed** or no smoke is produced.

### 5.3 Smoke characteristic (CV #137–#139)

Valid when effect 72 or 80 is assigned in CV #127–#132:

| CV | Name | Range | Description |
|----|------|-------|-------------|
| **#137** | PWM at standstill | 0–255 | Heating PWM when stationary |
| **#138** | PWM at steady speed | 0–255 | Heating PWM during cruise |
| **#139** | PWM at acceleration | 0–255 | Heating PWM during acceleration |

**Example (Seuthe 18 V, ~20 V track):**
```
CV #137 = 70–90    (little smoke at standstill)
CV #138 = 200      (~80% capacity from step 1)
CV #139 = 255      (maximum under acceleration → thick smoke)
```

### 5.4 Fan control

#### CV #133 — FO4 as fan output

| Value | Meaning |
|-------|---------|
| **0** | FO4 = normal function output (default) |
| **1** | FO4 = smoke fan (cam sensor or virtual cam) |

Also configures reed input polarity (bits 2–6) and MS440: IN4 → FO9.

#### Fan speed CVs (V1)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#351** | Fan speed — diesel, constant | 1–255 | 128 | 128 = half voltage during normal driving |
| **#352** | Fan speed — diesel, acceleration/start | 1–255 | 255 | 255 = maximum at start-up |
| **#355** | Fan speed — standstill | 1–255 | 0 | 0 = no smoke at standstill; >0 = visible idle smoke with sound on |

#### Fan speed CVs (steam)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#352** | Fan on/off ratio during steam burst | 25–170 | 255 | 255 = continuous max speed; lower values = pulsed fan synced to chuffs |
| **#355** | Fan speed at standstill | 1–255 | 0 | Idle smoke emission at standstill |

#### Dual smoke generators

When two heaters on different FOs both have effect 72/80:
- Lower FO number → fan V1 automatically.
- Higher FO number → fan V2 automatically.
- V2 settings: CV page 145/0 (CV #31 = 145, CV #32 = 0), CV #500–#511 (planned SW 6).

**Large-scale example (dual RAUDU on FO3 + FO7):**
```
CV #159 = 72, CV #160 = 72   (steam, both heaters)
#430 = 6 (F6), #432 = 3 (FO3)
#436 = 7 (F7), #438 = 7 (FO7)
```

### 5.5 Overheat protection (CV #353)

| CV #353 | Auto turn-off time |
|---------|-------------------|
| 0 | Disabled |
| 1–255 | 25 seconds × value (½ min to ~2 h) |

After auto turn-off, function key must be pressed to reactivate smoke.

### 5.6 SUSI smoke generator (effect 76)

Connect SUSI smoke module via SUSI pins (default). Configure effect code 76 on the assigned FO.

### 5.7 SW 6.00 planned additions

- CV page 145/0 for fan V2 (#351, #352, #355 equivalents)
- Separate effect codes 116, 120, 124, 128, 132, 136 for split heater/fan control
- CV #394 bit 3: deactivate 2nd smoke fan and heating with solo-drive key (diesel dual-engine)

---

## 6. Volume regulation

*This section applies to **MS and FS sound decoders** only.*

Default volume values are defined by the **loaded sound project**, not the decoder firmware. A hard reset (CV #8 = 8) restores project defaults.

### 6.1 Master volume

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#266** | Total volume (multiplier) | 0–255 (= 0–400%) | 65 | 65 = highest distortion-free level for LS8×12 speakers. Larger speakers: up to ~85. 100% = value 65. |

### 6.2 Volume keys (CV #395–#397)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#395** | Maximum volume for increase key | 0–255 | 64 | Upper limit when using louder key (can exceed CV #266 base) |
| **#396** | Volume decrease key | 0–29 | 0 | 0 = none; 1–28 = F1–F28; 29 = F0 |
| **#397** | Volume increase key | 0–29 | 0 | 0 = none; 1–28 = F1–F28; 29 = F0 |

### 6.3 Driving sound on/off and fade (CV #310–#314)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#310** | ON/OFF key for driving sound | 0–28, 255 | 1 | Toggles chuffs/motor/thyristor/random sounds. 255 = always on. Default F8 in ZIMO projects; OEM often F1. |
| **#311** | ON/OFF key for function sounds | 0–28 | 0 | Separate mute for whistle, bell, etc. Same value as #310 = one key mutes all. |
| **#312** | Blow-off key | 0–28 | 13 | Function key for blow-off (shunting with open valves) |
| **#313** | Mute key (fade in/out) | 0–28, 101–128 | 114 | Fades all sounds. 101–128 = inverted action. Often same as #310 in projects. |
| **#314** | Mute fade time | 0–255 | 0 | Tenths of a second (0–10 = min 1 s; 11–255 = up to 25 s) |

### 6.4 Driving sound volume (CV #376)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#376** | Driving sound volume (multiplier) | 0–255 (= 0–100%) | 255 | Reduces driving sounds (motor, chuffs, turbo) relative to function sounds. 255 = 100%. |

### 6.5 Per-class background / driving sound volumes

| CV | Sound class | Range | Default |
|----|-------------|-------|---------|
| **#574** | Simmering | 0–255 | 0 |
| **#576** | Direction change (Johnson bar) | 0–255 | 0 |
| **#578** | Brake squeal | 0–255 | 0 |
| **#580** | Thyristor sound (electric) | 0–255 | 0 |
| **#582** | Starting whistle | 0–255 | 0 |
| **#584** | Blow-off (steam) | 0–255 | 0 |
| **#586** | Electric motor | 0–255 | 0 |
| **#590** | Electric switch gear | 0–255 | 0 |
| **#592** | Second thyristor (planned SW 6) | 0–255 | 0 |
| **#600** | Turbocharger (diesel) | 0–255 | 0 |
| **#602** | Dynamic brakes | 0–255 | 0 |
| **#604** | Cornering squeal | 0–255 | 0 |

Volume scale for all: 0 = full (same as 255); 1–254 = reduced 1–99.5%; 255 = full.

### 6.6 Per-function-key sound volumes

| CV | Function key | CV | Function key |
|----|--------------|-----|--------------|
| **#571** | F0 | **#523** | F4 |
| **#514** | F1 | … | … |
| **#517** | F2 | **#565** | F18 |
| **#520** | F3 | **#568** | F19 |
| | | **#674** | F20 |
| | | **#698** | F28 |

Each CV: 0–255. 0 or 255 = full volume; 1–254 = 1–99.5% of sample volume.

Adjacent CVs (e.g. #570, #572, #513, #515) store sound sample numbers and loop parameters — modifiable via CV #300 procedure.

### 6.7 Load-dependent chuff volume *(steam)*

Based on EMF measurements. Calibrate "basic load" first:

| Procedure | CV #302 value | Result stored in |
|-----------|---------------|------------------|
| Forward calibration run | 75 | CV #777, #778 |
| Reverse calibration run | 76 | CV #779, #780 |

**Requirements:** Clear straight track (20 cm – 2 m depending on gauge), no grades/curves. Do not use roller test bench.

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#275** | Chuff volume at low speed / basic load | 0–255 | 220 | Volume at ~⅒ full speed during calibration |
| **#276** | Chuff volume at high speed / no load | 0–255 | 220 | Set at maximum speed during calibration |
| **#277** | Load dependency of chuff volume | 0–255 | 10 | 0 = no reaction; higher = stronger response to load changes |
| **#278** | Load change threshold | 0–255 | 10 | Suppresses volume changes on small load variations (curves) |
| **#279** | Load change delay | 0–255 | 1 | Reaction speed to load changes |
| **#281** | Acceleration threshold for full-load sound | 0–255 (internal steps) | 1 | Speed steps increase before full acceleration sound |
| **#282** | Duration of acceleration sound | 0–255 (= 0–25 s) | 30 | Tenths of a second |
| **#283** | Chuff volume at full acceleration | 0–255 | 255 | |
| **#284** | Deceleration threshold | 0–255 (internal steps) | 1 | |
| **#285** | Duration of reduced volume after decel | 0–255 (= 0–25 s) | 30 | Tenths of a second |
| **#286** | Chuff volume during deceleration | 0–255 | 20 | |

### 6.8 Brake squeal volume (CV #287, #288)

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#287** | Brake squeal threshold | 0–255 | 50 | Speed step below which squeal starts; stops at speed 0 |
| **#288** | Minimum drive time | 0–25 s | 50 | Suppresses squeal on short runs (shunting without cars) |

Brake squeal can also be assigned to a function key via CV #300 procedure (manual start/stop from SW 6.00).

### 6.9 Random and switch-input sound volumes

| CV | Name | Description |
|----|------|-------------|
| **#745** | Random sound Z1 volume | |
| **#748** | Random sound Z2 volume | |
| **#751–#760** | Z3–Z8 | |
| **#739** | Switch input S1 volume | Planned SW 6 |
| **#741** | Switch input S2 volume | |
| **#743** | Switch input S3 volume | |

### 6.10 Sound equalising

ZIMO provides a sound equalising feature (see manual chapter 5) for adjusting frequency response. Refer to the factory manual for equaliser CV details and the CV #300 pseudo-programming workflow for sound sample selection.

---

## Appendix A — MS decoder types (overview)

| Type | Sound | mfx | Notes |
|------|-------|-----|-------|
| MS420, MS440C/D, MS450, MS481, MS491, MS500, MS540E24, MS560, MS581, MS591, MS950, MS970, MS990 | Yes | Most yes* | *MS491, MS501, MS560, MS591N18, MS920LE: NOT mfx-capable |
| MN140–MN340 | No | Most yes | Same motor/function behaviour as MS |
| FS840, FS850, FS890 | Function sound | FS890N18: no mfx | Function decoder variants |

For pin-outs, current ratings, and wiring diagrams, see chapter 2 (Technical Data) and chapter 7 (Installation) in the factory PDF.

---

## Appendix B — Key differences from MX decoders

| Topic | MX | MS/MN |
|-------|-----|-------|
| CV #5 / #57 | Top speed was in CV #57 | CV #5 = top speed; CV #57 = voltage reference |
| CV #3+#49 / #4+#50 | Values added | Higher value wins |
| Acceleration curve | 255 equidistant steps | Follows speed table |
| CV #121/#122 | Needed for exponential curve | Not needed |
| CV #56 vs #147–#149 | CV #56 primary | #147–#149 for fine PID; #56 legacy |
| CV #144 | Programming lock | Dropped (not needed) |
| CV #190/#191 | Narrower time range | 0–320 s |

---

## Appendix C — Full CV table

Source: manual CV overview (chapter 8, pp. 70–87), SW 5.27.14. Defaults for CV #3/#4 may be overridden by the loaded sound project. FO effect codes: CV #125–#132 / #159–#160 — see [§3](#3-output-mapping-brightness-and-lighting-effects).

### CV #1–#31 — Address, consist, and configuration

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#1** | Locomotive address | DCC: 1 - 127 MM: 1 - 255 | 3 | The “short” vehicle address (DCC, MM) In case of DCC operation: Primary address as per CV #1 is only valid, if CV #29 (basic configuration), Bit 5 = 0. |
| **#2** | Start Voltage Vstart 3-point speed table, if CV #29, bit 4 = 0 | 1 - 255 | 1 | Internal speed step (1 - 255) applied as lowest external speed step (= speed step 1) (applies to 14, 28, or 128 speed step modes) = 1: lowest possible speed |
| **#3** | Acceleration time | 0 - 255 | 2 | The value multiplied by 0.9 equals acceleration time in seconds from stop to full speed. |
| **#4** | Braking time (Deceleration) | 0 - 255 | 1 | This CV, multiplied by 0.9, provides the time in sec for the deceleration from full speed to stop. The actual default value: see above! |
| **#5** | Top Speed Vhigh 3-point speed table, if CV #29, bit 4 = 0 | 0 - 255 | 0, 1 equals 255 | Internal speed step (1 - 255) applied as highest external speed step (i.e. for the external speed step 14, 28 or 128, depending on the speed step mode accord… |
| **#6** | Medium Speed Vmid 1, ¼ to ½ of the Value in CV | — | — | — |
| **#7** | SW version number Also see CV #65 Sub-Version number | Read only | - | This CV holds the firmware version number currently in the decoder. CV #7 = number of the “main” version CV #65 = Sub-version number |
| **#8** | Manufacturer ID and HARD RESET by CV #8 = „8“ or CV #8 = 0 | Read only | — | always shows “145” for ZIMO ID Pseudoprogramming see descr. on the right 145 (= ZIMO) Reading out this CV always result in “145” (”10010001”), the number iss… |
| **#9** | Motor controlperiod or frequency and EMF-sampling Rate (sampling rate, Sampling time) Total PWM period | 0, 11 - 99 | — | High frequency with modified sampling rate 100 – 199 From SW V. 4.215 55 High frequency, medium Sampling rate = 55: Default motor control with high frequency… |
| **#10** | Motorola Subsequent addresses | 0-3 | 0 | Decimal: 0= No Subsequent address 1= One Subsequent address for F5-F8 2= Two Subsequent addresses for F5-F12 3= Three Subsequent addresses for F5-F16 3.1 |
| **#12** | Possible operating modes - 117 | Bit 0 - DC analog | — | 0 = disabled 1 = enabled Bit 2 – DCC NOT deactivatable 1 = enabled Bit 4 - AC analog 0 = disabled 1 = enabled Bit 5 - MM 0 = disabled 1 = enabled Bit 6 - mfx… |
| **#13** | Functions F1–F8 in analogue operation | 0–255 | 0 (MN) / 67 (MS) | Function bitmask for F1–F8 in analogue mode. See also CV #14. |
| **#14** | Functions F1 - F8 Functions F0, F9 - F12 in analog operation and Acceleration/ Deceleration, control in analog operation | 0 - 255 | — | (CV #14) 0 - 255 (CV #13) 0 (MN) 128 (MS) (CV #14) 67 therefore Bit 6 = 1: Bit 0 = 0: F1 is OFF in analog mode = 1: … ON … Bit 1 = 0: F2 is OFF in analog mod… |
| **#15** | Decoder lock (key) | 0–255 | 0 | Must match CV #16 of target decoder to unlock when multiple decoders share an address. |
| **#16** | Decoder Lock | 0 - 255 0 - 255 | 0 0 | The decoder lock is used to access the CVs of several decoders with identical address separately. |
| **#17** | Extended (long) address high byte | 0–255 | 192 | Long DCC address with CV #18; active when CV #29 bit 5 = 1. |
| **#18** | Extended (long) address low byte | 0–255 | 128 | Long DCC address low byte with CV #17. |
| **#19** | Consist address 0, | 1 – 127 129 - 255 | — | ( = 1 - 127 with inverted Direction) 0 Alternate loco address for consist function: If CV #19 > 0: Speed and direction is governed by this consist address (n… |
| **#20** | Extended consist address AND (regardless of whether extended consist address is used) | Bit 7: Activating the | — | RailCom feedback for consist address 0 – 102 128 - 130 0 “Extended” consist address: the value defined in CV #20 is multiplied by 100 and added to the value … |
| **#21** | Functions F1 - F8 in consist operation | 0 - 255 | 0 | Functions defined here will be controlled by the consist address. Bit 0 = 0: F1 controlled by individual address = 1: …. |
| **#22** | Functions F0 forw. rev. in consist function and Activating Auto-Consist | 0 - 255 | 0 | Select whether the headlights are controlled by the consist address or individual address. |
| **#23** | Acceleration variation | 0 - 255 | 0 | For a temporary elevation/decrease (Bit 7 = 0/1) of the acceleration time defined in CV #3. |
| **#24** | Deceleration variation | 0 - 255 | 0 | For a temporary elevation/decrease (Bit 7 = 0/1) of the deceleration time defined in CV #4. 3.1 3.9 0 |
| **#27** | BRAKING MODES: Position-dependent Stopping (“before a red signal”) or driving slowly by “asymmetrical DCC signal“ (“Le | Bit 0 and Bit 1 = 0: ABC not activated; no stopping Bit 0 = 1: Stops are initiat | — | direction of travel) is higher than in the left rail. This (CV #27 = 1) is the usual ABC application) Bit 1 = 1: ABC stops are initiated if the voltage in th… |
| **#28** | RailCom Configuration | 0, 1, 2, 3, | 65, 66, 67 129, 130, 131 131 (with Bit 7 | DCC-A) Bit 0 - RailCom Channel 1 (Broadcast) Bit 1 - RailCom Channel 2 (Data) Bit 6 - High voltage RailCom (large scale decoders only) for all Bits: 0 = OFF … |
| **#29** | Basic Configuration | 0 - 63 | 14 = 0000 1110 | Bit 3 = 1 (RailCom is switched on), and Bits 1,2 = 1 (28 or 128 speed steps and automatic analog operation enabled) Bit 0 - Train direction: 0 = normal, 1 = … |
| **#30** | Decoder self-test | 0 – 255 | 1 | CV #30 = 255: Decoder self-test 1 = 254: extended self-test (only with exactly 18 V) CV #30: Read out Error code(s), see chapter 11 CV #30 = 0: Delete CV30 (… |

### CV #32–#66 — Function mapping, speed curve, and motor

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#33** | NMRA Function mapping F0 | 0 - 255 | 1 | Function mapping for F0 forward |
| **#34** | NMRA Function mapping F0 | 0 - 255 | 2 | Function mapping for F0 reverse |
| **#35–#46** | Function mapping F1 - F12 | 0 - 255 | 4, 8, 2, 4, 8, … | Function mapping for F1 - F12 |
| **#49** | Signal controlled (HLU, ABC) Acceleration | 0 - 255 | 0 | ZIMO signal-controlled speed influence method (HLU) using MX9 or StEin: or when using the “asymmetrical DCC signal” stopping method: The value multiplied by … |
| **#50** | Signal controlled (HLU, ABC) braking distance | 0 - 255 | 0 | ZIMO signal-controlled speed influence (HLU) with ZIMO MX9 track section module or StEin or when using the “asymmetrical DCC signal” stopping method: The val… |
| **#55** | Signal controlled (HLU) speed limits | — | — | — |
| **#56** | P and I value for PID motor regulation (= EMF-load balance control) The value of this CV will be automatically transferr | — | — | — |
| **#57** | Voltage reference 0, | 100 - 255 | 0 | Absolute voltage in tenth of a volt applied to the motor at full speed (max. throttle setting). A useful (and well functioning) range is 10 to 24 V (i.e. |
| **#58** | BEMF intensity SW version 6.00 and higher | 0 - 255 | 255 | Intensity of back-EMF control at the lowest speed step. EXAMPLES: CV #58 = 0: no back-EMF (like unregulated decoders), CV #58 = 150: medium compensation, CV … |
| **#59** | Signal controlled (HLU, ABC) delay | 0 - 255 | 5 | ZIMO signal controlled speed influence (HLU) with ZIMO MX9 track section module or future module or when using the “asymmetrical DCC signal” stopping method … |
| **#60** | Dimming the function outputs = voltage reduction of the function outputs by PWM Generally, this affects all function o | 0 - 255 | 0 | Reduction of function output voltage with PWM (pulse width modulation), to reduce the light’s brightness, for example. |
| **#61** | Extended Mapping 0, 97 0 = 97: NMRA mapping “without left-shift” | — | — | — |
| **#62** | afterglow brake light | 0 - 255 | 0 | Brake light (code 001110xx in CV #125ff): Afterglow in tenths of a second (i.e. range 0 to 25 sec) at standstill after stopping |
| **#63** | Modifications of lighting effects | 0 - 99 | 51 | Tens digit: Changing cycle time for various effects (0 - 9, default 5), or brighting up at soft start at 001101 (0 - 09 sec) Ones digit: Extends off-time |
| **#64** | Ditchlight modification | 0 - 255 | 0 | Bit 7 - 4: define a ditch light key (function key+1)*16 consequent: 0=F2, 1=F0, 2=F1,.. 15=F14 Bit 3 - 0: Ditch light OFF time modification [s] |
| **#65** | SW Sub-version number Also see CV #7 for Version number | Read only | - | If there are subversions to the SW version in CV #7, it is read out in CV #65. The entire SW version number is thus composed of CVs #7 and #65 (i.e. 28.15). |
| **#66** | Directional speed trimming | 0 - 127 0 - 127 | 0 0 | Speed step multiplication by “n/128” (n is the trim value in this CV): #66: for forward direction; #95: for reverse direction |

### CV #67–#124 — Speed table, Back-EMF, and braking

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#67–#94** | Free (28-point) speed table | 0–255 each | — | Internal speed step (1–255) per external step when CV #29 bit 4 = 1. Default curve emphasises lower speeds. |
| **#95** | Directional speed trimming | 0 - 127 0 - 127 | 0 0 | Speed step multiplication by “n/128” (n is the trim value in this CV): #66: for forward direction; #95: for reverse direction |
| **#97** | Change between individual and consist address by function key | 0 - 28 | 0 | With this key, you can switch between the main address of the decoder (on CV #1 or CVs #17, #18) or the consist address by pressing this key (only on the mai… |
| **#99** | Deactivating RailCom ID 7 Transmissions | Bit 0 = deactivates Km/h Bit 1 = deactivates O/W Bit 2 = deactivates Temperature | — | — |
| **#100** | Current asymmetry Voltage from SW version 4.227 | 0 – 255 | — | The CV #100 delivers when read out via PoM (=Prog On the Main, =OP Prog Mode) the asymmetry voltage measured AT THE TIME in tenths of a volt. |
| **#101** | Correction factor for CV #100 from SW version 5.15 | 0 – 255 | 0 | CV #101 can be used to define a correction factor in one of the two directions (only necessary for models with 6pol NEM 651 interface, where the consumers lo… |
| **#102** | „SUSI“ Slave 3 Bit0 = 0 (binary): CV#980 to CV#1019 are available for scripts. Bit0 = 1 (binary): CV#980 to CV#1019 are | — | — | — |
| **#106** | User Data | 0 – 255 | 0 | Available for free use as storage space (without effect!) |
| **#107** | Turn off lights (i.e. front headlights AND the - according to CV #107 - additionally defined function output) | 0 - 220 | 0 | The value of this CV is calculated as follows: Number of a function output (FO1... FO6) x 32 + number of a function key (F1, F2, ...F28) →… value of CV #107. |
| **#108** | at driver's cab 2 (back) | 0 - 255 | 0 | As CV #107, but for other side of the loco. 3.4 |
| **#109** | Automatic unilateral light suppression Add. Fu-output at side 1 | Bit 7 = 0.1: Bit 7 = 0.1: 1 - 6 | — | If CV #109, bit 7=1 and CV #110, bit 7=1, the light suppression on the driver’s cab side in consist operation is activated automatically. |
| **#110** | Automatic unilateral light suppression Add. Fu-output at side 2 | Bit 7 = 0.1: Bit 7 = 0.1: 1 - 6 | — | If CV #109, bit 7=1 and CV #110, bit 7=1, the light suppression on the driver’s cab side in consist operation is activated automatically. |
| **#111** | Emergency Delay time | 0 - 255 | 0 | This CV value is valid for emergency stop instead of CV #4, i.e. for single stop and collective stop emerg. |
| **#112** | Special ZIMO Configuration bits from SW Vers. 5.00 | 0 - 255 | 4 = 00000100 | Bit 2 = 0: ZIMO loco number recognition OFF = 1: ZIMO loco number recognition ON |
| **#114** | Dim Mask 1 = Excludes specific function outputs from dimming as per CV #60 Also see Addition to CV #152 Bits | 0 - 7 | 0 | Enter function outputs that are not to be dimmed by CV #60. These outputs will receive the full voltage from the pin they are connected to. |
| **#115** | Uncoupler control Activation time or CV #115 can be used as alternative “second dimming value.” 0 The uncoupler functi | — | — | — |
| **#116** | Automatic Disengagement during uncoupling = “Automatic uncoupling“ | 0, 1 - 99, 0, 1 - 199 | 0 | Tens digit (0 - 9): Length of time the loco should move away (disengage) from the train; coding as in CV #115. |
| **#117** | Flasher functions Outputs are assigned in CV #118. Flashing mask | 0 - 99 | 0 | Duty cycle for flasher function: Tens digit: Off / Ones digit: On (0 = 100msec, 1 = 200msec…..9 = 1 sec) Example: CV #117 = 55: Flashes evenly at 1 a second … |
| **#118** | Flashing mask = Allocation of Function outputs to the flashing rhythm CV #117 Bits | 0 - 7 | 0 | Selected function outputs will flash when turned ON. Bit 0 - front headlights Bit 1 - rear headlights Bit 2 - for function output FO1, Bit 3 - ...FO2 Bit 4 -… |
| **#119** | Low beam mask for F6 = Allocation of Function outputs as (for example) low/high beam Bits | 0 - 7 | 0 | Selected outputs will dim, according to the dim value in CV |
| **#120** | Low beam mask for F7 Bits 0 - 7 Same as CV #119 but with F7 as low beam key. | — | — | — |
| **#123** | Adaptive Acceleration and deceleration momentum SW version 6.00 and higher | 0 - 99 | 0 | Raising or lowering the speed to the next internal step occurs only if the preceding step is almost reached. |
| **#124** | ATTENTION: Bits 2, 3, 4, 6 (i.e. selection for shunting key functions) are only valid if CVs #155 and #156 = 0 (These | Bit 7: | — | Switchover SUSI - Logic level outputs Bits 0 - 4, 6 Bit 7 0 0 Selection of a shunting key to activate the HALF SPEED: Bit 4 = 1 (and bit 3 = 0): F3 as half-s… |

### CV #125–#215 — Outputs, lighting effects, uncoupler, smoke

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#125–#132** | Special effect on FO1–FO6 | 0–255 | — | Effect code per output FO1–FO6 (lighting, uncoupler, smoke, servo, etc.). CV #125 = FO1 … #132 = FO8. |
| **#127–#132** | Effects on FO1, FO2, FO3, FO4, FO5, FO6 0 See CV #125 for details | — | — | — |
| **#133** | Using FO4 as Cam-sensor output for the module of your choice or FO4 as output for Steam fan of the Smoke generator of st | Bit 0 | 0 | = 0 (Default): FO4 is used as normal function output so it’s controllable by function key instead of a cam sensor. |
| **#134** | Asymmetry stopp (ABC) | 1 - 14 | 6 | threshold (tens digit, ones digit). |
| **#136** | Fine adjustment of the speed feedback or km/h - control no. calibration run RailCom Display factor 128 RailCom speed fee | — | — | — |
| **#139** | Definitions of smoke generator characteristic, connected to FO1 – 6. PWM at stand still PWM at steady speed PWM during | 0 - 255 0 - 255 0 - 255 | 0 0 0 | This is valid, if in one of the CVs #127 - #132 has set on of the function effects “smoke generation” (i.e. |
| **#140** | Distance controlled stopping - Constant Braking Distance Selection of the braking occasion and the braking behavior 0, | — | — | — |
| **#141** | Distance controlled stopping - Constant Braking Distance The braking distance | 0 - 255 | 20 | The value in this CV defines the "Constant Braking Distance". The value suitable for the existing braking distances must be determined by trial and error; as… |
| **#143** | Distance controlled stopping - Constant Braking Distance Compensation for HLU method | 0 - 255 | 0 | Since HLU is more error-resistant than ABC, no detection delay is usually necessary; therefore default 0. |
| **#144** | Confirmation jingle | Bit 4 = 1: activates confirmation jingle when programmed | — | From version v5.7.0 MN decoder: front and rear lights flash instead of jingle. |
| **#146** | Compensation for gear backlash during direction changes in order to prevent start-up jolt. SW version 6.00 and higher | 0 - 255 | 0 = 0: no effect | =1 to 255: in ase the driving direction was changed beforehand, the motor spins at minimum rpm for a defined timespan (according to CV #2) for a specific tim… |
| **#147–#149** | — | 0, 11 - 99 | — | modified Setting 55 medium PID Setting = 55: Default motor control using medium values in PID parameters P and I (Diff = 0). |
| **#151** | Reduction of motor control in consist operation or motor brake (if address NOT in consist) from SW-Version 6.0 | 0 - 99 | 0 | The tens digit reduces the motor compensation to 10 % – 90 % according to the value in CV #58. |
| **#152** | Dim Mask 2 Excludes specific function outputs from dimming Addition to CV #114 and FO3, FO4 as Direction outputs Bits | 0 - 5 | — | and Bit 6, Bit 7 0 0 ... Addition to CV #114. Bit 0 - function output FO7, Bit 1 - function output FO8, Bit 2 - function output FO9, Bit 3 - function output … |
| **#153** | Time limit for continued driving without rail signal | 0 - 255 | 100 0: Feature not used | 1 - 255: Time in tenths of a second after which the decoder starts a braking process if there is no more DCC reception via its two track contacts. |
| **#154** | Various special bits | Bit 1+2 | — | SW version 6.00 and higher 16 Bit 1 = 1: DIESEL, ELECTRO Drive off immediately even if playback of idle sound has not yet finished. |
| **#155** | to be preferred for new projects alternative to CV | — | — | — |
| **#156** | to be preferred for new projects alternative to CV | — | — | — |
| **#157** | Selection of a Function key for the MAN-function Only for non-ZIMO controllers that don’t have a dedicated MN key. 0, | 1 - 28, | 29 0 | The MAN function (or MAN key on ZIMO controllers) was originally designed for ZIMO applications only, in order to cancel stop and speed limit commands applie… |
| **#158** | Various special bits Bits 1, 3, 5, 6, 7 (only Diesel & Electro) - | Bit 1 = 1: Diesel mechanical: RPM is not raised | — | when braking (see CV #364). Bit 2 = 0: RailCom speed feedback (km/h) feedback in “old” format (for MX31ZL, RailCom ID 3) = 1: RailCom speed feedback (km/h) N… |
| **#159** | Special effect FO7 | 0–255 | 0 | Effect code for FO7. |
| **#160** | Special effect FO8 | 0–255 | 0 | Effect code for FO8. |
| **#161** | Servo outputs: Protocol and alternate Use of Servo outputs: 3 & 4 as SUSI pins | 0 - 3 | — | NOTE: For Smart Servo RC-1 set CV #161 = 2! 0 Bit 0 = 0: Servo protocol with positive pulses. = 1: Servo protocol with negative pulses. |
| **#162** | Servo 1 Left position | 0 - 255 | 49 = 1 ms | Servo pulse Defines the servo’s left stop position. “Left” may become the right stop, depending on values used. 0 |
| **#163** | Servo 1 - Right stop | 0 - 255 | 205 | Defines the servo’s left stop position. 0 |
| **#164** | Servo 1 Center position | 0 - 255 | 127 | Defines a center position, if three positions are used. 0 |
| **#165** | Servo 1 Rotating speed | 0 - 255 | 30 = 3 sec | Rotating speed; time between defined end stops in tenths of a second (total range of 25 sec, default 3 sec.). 0 |
| **#166** | - | — | — | — |
| **#170** | - | — | — | — |
| **#174** | - | — | — | — |
| **#177** | Same as input mapping above for other functions: servo 2 servo 3 servo 4 0 | — | — | — |
| **#178** | Panto Reverberation | 0 - 255 | 0 | Valid for each servo, which is "Panto..." under CVs #181 - |
| **#179** | Increased speed with rail tension | 0-255 | 0 = | CV-Val 128 Suitable for setting the maximum speed in analogue mode. Works in both controlled and uncontrolled analogue mode, from SW 5.15 onwards. 0 |
| **#184** | Servo 1 Servo 2 Servo 3 Servo 4 Function assignment NOTE: If a servo control line shares its connection with another fun | — | — | — |
| **#185** | Special assignment for live steam engines 0 = 1: Steam loco with one servo in operation; Speed and direction of travel | — | — | — |
| **#189** | "Panto1" "Panto2" "Panto3" "Panto4" 0 | Bit 7 = 0: Not sound-dependent | — | = 1: Sound-dependent Bit 6 - 5 = 00: direction independent, = 01: only if loco drives forward = 10: only if loco drives backwards = 11: only if F-key is turn… |
| **#190** | Brighting-up time for effects 88, 89 and 90 | 0 – 100 101-200 201-255 | 0 | The range 0 -100 corresponds to 0 - 1 sec (10ms/value) 101 – 200 1 – 100s (1s/value) 201 – 255 100 – 320s (4s/value) |
| **#191** | Dimming time for effects 88, 89 and 90 | 0 – 100 101-200 201-255 | 0 | The range 0 - 100 corresponds to 0 - 1 sec (10ms/value) 101 – 200 1 – 100s (1s/value) 201 – 255 100– 320s (4s/value) |
| **#192** | Value for effect dimming | 0 - 255 | 0 | Code 011010xx in CV #125ff: Decreases the brightness of the (light) function output by the set value (in percent). (e.g. value 127 = 50 %) 0 |
| **#193** | ABC - commuting with stopping times in reverse loops 0, | 1 - 255 | 0 | = 0: no commuting on ABC basis = 1 - 254: Commuting with stopovers (terminal loops by ABC slow-speed sections, stopovers defined by ABC stopping sections) St… |
| **#194** | ABC - commuting with additional stopovers 0, | 1 - 254, | 255 0 | Only as commuting if CV #193 = 1 - 255 = 0: Commuting without stopovers (see above) = 1 - 254: Commuting with stopovers (terminal loops by ABC slow-speed sec… |
| **#195–#199** | Effects on FA9, FA10, FA11, FA12, FA13 0 like CV #125 #195  FO9 #196  FO10 | — | — | — |
| **#201** | Alternative (clearer, preferred for new projects) "SUSI" usage | 0, 11, | 22, 33, 44, 55 0 | = 11: SUSI pins as logic level outputs (see above) = 22: SUSI pins as reed inputs = 33: SUSI pins as servo control lines = 44: SUSI "Burst Mode" all packets … |
| **#202** | If decoder (large scale decoder) has two "SUSI" connections | 0, 11, | 22, 33, 44, 55 0 | As above (CV #201), but for second SUSI connection; there, however, CV #202 is the only setting option, not just the alternative). |
| **#204** | Use of the inputs IN1 & IN2, or IN3 & IN4 Ones and tens 0, 1, 2, 4 0 = 11: both "IN"s as logic level outputs (see above) | — | — | — |

### CV #216–#299 — Sound control and programming assist

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#249** | Bootloader version and subversion | Read only | - | Reading out these CVs provides the version and subversion of the bootloader currently on the decoder (bootloader = program to load the actual software). |
| **#253** | Decoder-ID,, thereof CV #250 and CV #251 (Bits 74) = Decoder-Type (See chapter 2 Technical Data, schematics, opera | — | — | — |
| **#258** | Read out CV #8 CV #8 can be read out here (exception 3-6) 5 | — | — | — |
| **#259** | Read loco-set same | — | — | — |
| **#263** | “Load code” for “coded” sound projects - - New ZIMO sound decoders can be ordered for an additional small fee with a “ | — | — | — |
| **#264** | Variable low voltage (large scale and special decoders) | 10 - 158 | 15 | Variable low voltage (adjustable by CV, only large scale and some special decoders) = 10 - 158: Low voltage in tenths of a volt (1 - 15.8 V) 5 |
| **#265** | same | — | — | — |
| **#266** | Total volume (Multiplier) | 0 - 255 | = 0 - 400 % 65 = 100 % | The default value “65” results in the (mathematically) highest possible distortion-free playback volume. For LS8x12 speakers only values up to approx. |
| **#267** | Chuff beat frequency according to “virtual Cam sensor“ | 0 - 255 | 63 | CV #267 is only active if CV #268 = 0: Chuff beats follow the “virtual cam sensor”; an actual cam sensor is not needed in this case. |
| **#268** | Switch to real cam sensor and Number of spikes of the cam sensor for chuff beat and Special functions “simple articulate | 0 - 63 | — | and 128, 192 1 = 0: “Virtual“ cam sensor is active (to be adjusted with CV #267, see above). = 1: Real cam sensor (connected to „In2” resp. |
| **#269** | Lead-chuffaccentuation | 0 - 255 | 10 | A typical sound signature of a passing steam engine is that one chuff out of a group of 4 or 6 chuffs is louder in volume than the rest; this effect is alrea… |
| **#271** | Fast driving overlapping effect | 0 - 255 | — | (Useful up to @ 30) 1 The individual steam chuffs of a real engine overlap each other at high speed. |
| **#272** | Blow-off duration also see CV #312 in this table (Blow off Key) Opening the cylinder valves on a prototype steam engine | — | — | — |
| **#273** | Blow-off Start-up delay Opening the cylinder valves and with it the related blow-off sound on a real steam engine starts | — | — | — |
| **#274** | Blow-off Standstill and Starting whistle Standstill Constant opening and closing of the cylinder valves in real shunting | — | — | — |
| **#275** | Chuff volume at low speed without load | 0 - 255 | 220 | With this CV the chuff volume can be adjusted for low speed and “basic load” (same conditions as during the “automated recording run”). |
| **#276** | volume at high speed without load | 0 - 255 | 220 | Like CV #275 (see above) but for driving fast. Set the speed regulator to maximum during this set-up. 5.5 |
| **#277** | Dependency of chuff volume of current load from SW 5.15 | 0 - 255 | 10 | When deviating from the basic load (as determined by the “Automated recording of the motor’s “basic load” factor”, see above), the chuff beat volume should b… |
| **#278** | Load changing Threshold from SW Vers. 6.00 | 0 - 255 | 10 | With this CV, a change in volume in reaction to small load changes can be suppressed (i.e. in curves) in order to prevent chaotic sound impressions. |
| **#279** | Load changing Delay from SW Vers. 6.00 | 0 - 255 | 1 | This CV determines how quick the sound reacts to load changes, whereas the factor is not just time but rather “load-change dependent time” (= the bigger the … |
| **#280** | Diesel engine - Load dependency from SW Vers. 5.15 | 0 - 255 | 10 | With this setting, the diesel motor’s reaction to the load (defined by PWM and speed step). |
| **#281** | Chuff volume - | 0 – 255 | — | (internal speed steps) 1 More powerful and louder chuff sounds should be played back indicating increased power requirements during accelerations, compared t… |
| **#282** | Duration of acceleration sound | 0 - 255 | = 0 - 25 sec 30=3sec | The acceleration sound should remain for a certain length of time after the speed increased (otherwise each single speed step would be audible, which is unre… |
| **#283** | Driving noise- (Steam chuffs) Volume - for full acceleration sound | 0 - 255 | 255 | The volume of steam chuffs at maximum acceleration is set with CV #283 (default: 255 = full volume). |
| **#284** | Deceleration threshold for reduced volume during deceleration | 0 -255 | — | (Internal Speed steps) 1 Steam chuffs should be played back at less volume (or muted) signifying the reduced power requirement during deceleration. |
| **#285** | Time needed for the volume reduction at deceleration | 0 - 255 | = 0 - 25 sec 30 | After the speed has been reduced, the sound should remain quieter for a specific time (analog to the acceleration case). |
| **#286** | Volume - of reduced driving noiseat deceleration | 0 - 255 | 20 | CV #286 is used to define the chuff volume during deceleration (default: 20 = pretty quiet but not muted). |
| **#287** | Threshold for brake squeal | 0 - 255 | 50 | The brake squeal should start when the speed drops below a specific speed step. It will be automatically stopped at speed 0 (based on back-EMF results). 5.3 |
| **#288** | Brake squeal minimum drive time | 0 - 255 | = 0 - 25 sec 50 | The braking squeal is to be suppressed when an engine is driven for a short time only, which is usually a shunting run and often without any cars (it is most… |
| **#289** | Thyristor Step-effect | 0 - 255 | — | = 1 - 255: Effect of pitch 5.6 |
| **#290** | Thyristor sound pitch: “slow” pitch increase | 0 - 255 | 50 | Sound pitch for speed defined in CV #292. 5.6 |
| **#291** | Thyristor sound pitch: Maximum pitch | 0 - 255 | 255 | Sound pitch at top speed. 5.6 |
| **#292** | Thyristor sound pitch: slow speed | 0 - 255 | 128 | Speed for sound pitch per CV #290. 5.6 |
| **#293** | Thyristor sound pitch: Steady volume | 0 - 255 | 100 | Thyristor sound volume at steady speed (no acceleration or deceleration in progress). 5.6 |
| **#294** | Thyristor sound pitch: Vol. at acceleration | 0 - 255 | — | Volume during acceleration 5.6 |
| **#295** | Thyristor sound pitch: Vol. at deceleration | 0 - 255 | — | Volume during heavier decelerations (braking) 5.6 |
| **#296** | Electric motor Volume | 0 - 255 | 0 | Motor sound volume. 5.6 |
| **#297** | Electric motor Minimum load | 0 - 255 | 0 | Internal speed step at which the motor sound becomes audible; at this speed step is starts at a low volume and reaches maximum volume as per CV #296 at the s… |
| **#298** | Electric motor Volume - Speed dependency | 0 - 255 | 0 | Internal speed step at which the motor sound reaches the maximum volume defined in CV #296. See ZSP manual! 5.6 |
| **#299** | Electric motor Pitch (frequency) Speed dependency | 0 -100 | 0 | The motor sound will be played back faster, corresponding to this CV with rising speed. |

### CV #300–#399 — Sound samples, cam sensor, and smoke fan

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#307** | Cornering squealsorder Reed configuration Bit0 - switching input 1 activates cornering squeal Bit1 - switching input 2 | Bit 7 - 0 = key defined in CV #308 suppresses cornering | — | squeal of reed inputs if this key is ON 1 = key defined in CV #308 activates cornering squeal independent of Reed inputs 5.3 |
| **#308** | cornering squeal key | 0 - 28 | 25 | 0: No key defined. Reed inputs always active. 1-28 = key F1 to F28. |
| **#309** | Brake key | 0, 1 - 29 | 0 | The key defined here acts as a brake key according to the rate defined in CV #349 (the normal – higher – deceleration time in CV #4 is thereby ignored). |
| **#310** | ON/OFF key for Driving sound volume and Random Sounds | 0 - 28, | 255 1 | Function key to turn ON/OFF driving sounds (steam chuffs, boiling, blow-off, brake squeal, or diesel motor, thyristor sounds, etc.) as well as random sounds … |
| **#311** | ON/OFF key for Function sounds | 0 - 28 | 0 | Function key assigned as ON/OFF key of function sounds (i.e. F2 – whistle). = 0: does not mean that F0 is assigned for this task but rather that the function… |
| **#312** | Blow-off key | 0 - 28 | 13 | See chapter 5.4 SOUND: Steam engine → sound basic configuration; Defines function key which activates blow-off noise; e.g. for shunting with “open valves”. 5.3 |
| **#313** | “Mute key” fade in/out time Key | 0 - 28 101 - 128 | 114 | This CV assigns a function key with which the driving sounds can be faded in and out, i.e. when the train disappears behind scenery. |
| **#314** | Mute – fade in/out time | 0 - 255 | = 0 - 25 sec 0 | Time in tenths of a second for sound fading in/out when mute button is pressed. Total range is 25 seconds. |
| **#315** | Random generator Z1 Minimal interval | 0 - 255 | = 0 - 255 sec 1 | The random generator generates internal pulses in irregular intervals that are used to playback a sound file assigned to the random generator. |
| **#316** | Random generator Z1 Highest interval | 0 - 255 | = 0 - 255 sec 60 | CV #316 defines the maximum time interval between two consecutive pulses of the random generator Z1; the actually occurring pulses between the values in CV #… |
| **#317** | Random generator Z1 Duration of playback | 0 - 255 | = 0 - 255 sec 5 | The sound sample assigned to the random generator Z1 (most often the compressor) is played back for the timespan defined in CV #317. |
| **#320** | As above, however… Random generator Z2 | 0 - 255 0 - 255 0 - 255 | 20 80 5 | By default, Z2 is assigned for coal shoveling at stand-still. 5.8 |
| **#323** | As above, however… Random generator Z3 | 0 - 255 0 - 255 0 - 255 | 30 90 3 | By default, Z3 is assigned for the injector at stand-still. 5.8 |
| **#324** | - | — | — | — |
| **#338** | As above, however… random generator Z4 - Z8 | 0 - 255 0 - 255 0 - 255 | — | At delivery this random generator is not used. 5.7 |
| **#339** | Key for raising of diesel sound step | 0 - 28 | 0 | Function key that raises the diesel sound to the minimum speed defined with CV #340. See below if more keys for further speed raises are required. 5.7 |
| **#340** | Diesel sound step, to which is to be raised, and possibly more keys. | 0 - 10 | 0 | The minimum diesel step the sound is to be raised to with the function key defined with CV #339. |
| **#341** | Switch input 1 Duration of playback | 0 - 255 | = 0 - 255 sec 0 | The sound sample allocated to switch input 1 is played back for the duration defined with this CV. = 0: Play back sound sample for the first time 5.8 |
| **#342** | Switch input 2 Duration of playback | 0 - 255 | = 0 - 255 sec 0 | The sound sample allocated to switch input 2 is played back for the duration defined with this CV. = 0: Play back sound sample for the first time 5.8 |
| **#343** | Switch input 3 Duration of playback | 0 - 255 | = 0 - 255 sec 0 | The sound sample allocated to switch input 3 (as far as it is not used as cam sensor) is played back for the duration defined with this CV. |
| **#344** | Run time of Motor sounds (Cooling fan, etc.) after stops | 0 - 255 | = 0 - 25 sec | After the engine comes to a stop, some accessories are supposed to remain operating (e.g. |
| **#345** | — | 0 – 2 | 0 | Bit 0 = 1: Switches also at stand-still, Bit 1 = 1: Switches also while cruising (bits for standstill and cruising possible at the same time) Bit 2 = 1: Tran… |
| **#346** | Conditions for switching between collections, as per CV | — | — | — |
| **#347** | Key to switch key for driving and sound performance when driving solo | 0 - 28 | 0 0=: no key, no solo drive | = 1 - 28: One function key (F1 – F28) acts as the switchover key for driving a heavy train or a single locomotive. 3.7 5.3 5.6 |
| **#348** | If the key for solo drive (CV #347, see above) is activated, the measures defined here have to be met. | Bit 2 already in ver- | — | sion 4.10 0 - 31 When driving solo (function key as per CV #347 is ON): Bit 0 = 1: ... |
| **#349** | Braking time for brake key | 0 - 255 | 0 | To achieve the desired effect, the deceleration time in CV |
| **#350** | Electric switch gear sound, locked after starting | 0 - 255 | 0 | Time in tenth of seconds (0-25 sec), the switchgear sound shall not be played back after starting; this is useful if the first switching step is already in t… |
| **#351** | Speed of the smoke fan at constant speed for DIESEL locomotives | 1 - 255 | 128 | The speed of the fan is set by PWM; the value of CV #351 defines the behaviour during normal driving. = 128: Half voltage (PWM) when driving. |
| **#352** | Speed of the smoke fan during acceleration and engine starting for DIESEL locomotives | 1 - 255 | 255 | To create a cloud of smoke when starting the machines, the fan is set to higher (usually maximum) speed, as well as in case of strong acceleration during ope… |
| **#353** | Automatic turn-off of the smoke generator | 0 - 255 | = 0 -106min 0 | For effects “010010xx” or “010100xx” (smoke generator): overheating protection: Turn-off half a minute to 2 hours. |
| **#354** | Chuff beat frequency at speed step 1 also see CV #267 in this table | 1 - 255 | 11 | CV #354 works only if used together with CV #267! CV #354 compensates for the non-linear speed measurements of the “virtual cam sensor”: While the adjustment… |
| **#355** | Speed of the smoke fan at standstill for STEAM locomotives, and DIESEL locomotives | 1 - 255 | 0 | With CV #355 the speed of the fan at standstill - if sound is switched on - is set. |
| **#356** | Speedlock key | 0 - 28 | — | If this key is activated, the speed controller changes the driving sound, not the speed 5.6 |
| **#357** | Thyristor sound pitch: Lowering volume at higher speed | 0 - 255 | — | Internal speed step at which the thyristor sound volume should be reduced. The volume stays at this reduced level while braking. |
| **#358** | Thyristor sound pitch: Course of Lowering volume at higher speed | 0 - 255 | — | Defines a curve as to how the thyristor sound should be lowered at the speed step defined in CV #357. = 0: no reduction. |
| **#359** | Electric switch gear sound, Switch gear playback duration during speed changes | 0 - 255 | 30 | Time in tenth of a second the switch gear should be heard during speed changes (adjustable from 0 – 25 sec.). |
| **#360** | Electric switch gear sound, Duration of playback after stopping | 0 - 255 | 0 | Time in tenth of a second the switch gear should be heard after the engine comes to a full stop (adjustable from 0 – 25 sec.). = 0: no sound after stop. 5.6 |
| **#361** | Electric switchgear Time until the next playback | 0 - 255 | 20 | During rapid successions in speed changes the switch gear sound would be played back too often. |
| **#362** | Thyristor sound pitch: Switching threshold to second sound: | 0 - 255 | — | Defines a speed step at which a second thyristor sound for higher speeds is played back; this was introduced for the sound project “ICN” (Roco OEM sound) = 0… |
| **#363** | Electric switch gear sound, Distribution of speed steps on switching steps | 0 - 255 | 0 | Number of shift steps to cover the whole speed range; i.e. if 10 shift steps are programmed, the switch gear sound is played back at internal speed step 25, … |
| **#364** | from SW 6.00 Diesel engine with Switchgear Speed drop during upshifts 0 This special CV applies only to diesel-mechanica | — | — | — |
| **#365** | from SW 6.00 Diesel engine with Switchgear Upshift rpm 0 This special CV applies only to diesel-mechanical engines and d | — | — | — |
| **#366** | Turbocharger Maximum volume | 0 - 255 | 48 5.6 | — |
| **#367** | Minimum load for turbofor DIESEL engines Turbo rpm dependency on speed | 0 - 255 | 150 | Turbo playback frequency depending on engine speed. 5.6 |
| **#368** | Turbocharger Turbo rpm dependency on accelerationfor DIESEL engines acceleration | 0 - 255 | 100 | Playback frequency depends on the difference of set speed to actual speed (= acceleration). 5.6 |
| **#369** | Turbocharger Minimum load | 0 - 255 | 30 | Audibility threshold for turbochargers; the load is derived from CV #367 and #368. 5.6 |
| **#370** | Turbocharger Frequency increase | 0 - 255 | 25 | Speed of frequency-increase of the turbocharger. 5.6 |
| **#371** | Turbocharger frequency lowering | 0 - 255 | 15 | Speed of frequency-decrease of the turbocharger. 5.6 |
| **#372** | Electric motor Volume - Acceleration dependency | 0 - 255 | 0 = 0: No function | = 1 - 255: minimal to maximal effect 5.6 |
| **#373** | Electric motor Volume - Dependent on braking | 0 - 255 | 0 = 0: No function | = 1 - 255: minimal to maximal effect 5.7 |
| **#374** | Coasting-Key (or Notching) for diesel sound projects | 0 - 29 | 0 | Function key that activates “Coasting“, which forces the motor sound to a specified speed independent of the driving situation. |
| **#375** | Coasting step (or Notching) | 0 - 10 | 0 | Motor sound (speed) to be activated with the coasting key (as per CV #374), independent of the driving situation. |
| **#376** | Driving sound Volume - (Multiplier) | 0 - 255 | = 0 - 100 % 255 = 100 % | To reduce the driving sound volume (e.g. Diesel motor with related sounds such as turbo charger) compared to the function sounds. 5.6 |
| **#378** | Statistical probability of switchgear sparks during Acceleration from SW Vers. 6.00 | 0 - 255 | 0 | Likelihood for sparks (as per CV #158 Bit 7 for FO7 or #394 for FO6) when accelerating= 0: always = 1: very rarely = 255: very often (almost always) 5.6 |
| **#379** | Statistical probability of switchgear sparks during Deceleration from SW Vers. 6.00 | 0 - 255 | 0 | Likelihood for sparks when decelerating (as per CV #158 Bit 7 for FO7 or #394 for FO6) = 0: always = 1: very rarely = 255: very often (almost always) 5.6 |
| **#380** | Manual electric brake key | 1 – 28 | 0 | Defines a function key to manually control the sound of a “dynamic” or “electric” brake. 5.6 |
| **#381** | Electric brake minimal speed step | 0 – 255 | 0 | The electric brake shall only be heard between the value defend in CV #381… 5.6 |
| **#382** | Electric brake maximum speed step | 0 – 255 | 0 …and the value in CV #382 5.6 | — |
| **#383** | Electric brake Pitch | 0 – 255 | 0 | = 0: Pitch independent of speed = 1 - 255: …depends increasingly on speed. 5.6 |
| **#384** | Electric brake Deceleration threshold | 0 – 255 | 0 | The number of speed steps to be reduced during deceleration before the electric brake sound is played back. 5.6 |
| **#385** | Electric brake Driving on slopes | 0 – 255 | 0 = 0: no effect at “negative” load | = 1 - 255: Sound triggered at “negative” load. 5.6 |
| **#386** | Electric brake Loop | 0 – 15 | 0 | Bit 3 = 0: Sound fades out at the end of the sample = 1: Sound ends without fading at end Bit 2 - 0: Prolongation of the minimal runtime of the braking sound… |
| **#387** | Acceleration influence on diesel sound steps | 0 - 255 | 0 | In addition to the selected speed step (defined in the ZSP flow diagram), actual changes in speed (acceleration, deceleration) should also have an influence … |
| **#388** | Deceleration influence On diesel sound steps | 0 - 255 | 0 | Same as CV #387 but used during decelerations. = 0: No influence (sound depends on speed step only) = 64: experience has shown this to be a practical value =… |
| **#389** | Limited acceleration influence on diesel sound steps | 0 - 255 | 0 | This CV determines how far the sound step may deviate during acceleration from the simple speed step dependence (= difference between target speed from the c… |
| **#390** | Momentum reduction when driving solo (engine only) | 0 - 255 | 0 | When switching to solo driving with key defined in CV #347 the momentum reduction is activated (with CV #348, Bit 1): = 0 or 255: No reduction = 128: Reducti… |
| **#391** | Driving with idle sound, when driving solo | 0 - 255 | 0 | The diesel motor sound should remain at idle when driving solo (with function key defined in CV #347), until the speed step defined in CV #391 is reached. 5.8 |
| **#392** | Switch input 4 Playback duration | 0 - 255 | = 0 - 255 sec 0 | The sound sample allocated to switch input 4 is played back for the duration defined with this CV. = 0: Play back sound sample for the first time 3.20 5.6 |
| **#393** | ZIMO Config 5 0 | Bit 0 = 1: Activate ditch light if bell is played Bit 1 = 1: Activate ditch ligh | — | switchgear Bit 2 = 1: Use one sample after the other, if at the end, start again with 1st sample Bit 3 = 0: Play first and last part when shifting up (middle… |
| **#394** | ZIMO Config 4 From SW version 6.00 | 0 - 255 | - | Bit 0 = 1: Light flashes at switchgear sound. Bit 4 = 1: Faster acceleration and sound on high power when speed controller is set to full quickly Bit 5 = 1: … |
| **#395** | Maximum volume for volume increase key | 0 - 255 | 64 | Configuration range for volume with the help of the louder key according to CV #397; can also be higher than the basic configuration in CV #266. 5.3 |
| **#396** | volume decrease key | 0 - 29 | 0 0 = | No key defined. 1-28 = key F1 to F28 29 = F0-key 5.3 |
| **#397** | Volume increase key | 0 - 29 | 0 0 = No key defined. | 1-28 = key F1 to F28 29 = F0-key 5.7 |
| **#398** | Automatic Coasting | 0 - 255 | 0 | The number of speed steps the train’s speed has to be reduced within 0.5 seconds in order for the automatic coasting effect to set the motor sound to idle (w… |
| **#399** | Speed dependent high beam (Rule 17) From SW version 6.00 | 0 - 255 | 0 | In combination with the “Swiss Mapping” special high-beam setting, see CV #431 = 255; applies to all 17 CV-groups (CV |

### CV #400–#499 — Indexed outputs and extended mapping

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#400** | Input mapping for internal F0 that is, which function key switches the internal (decoder) function F0. 0, | 1 - 28, 29 30 - 187 | 254, 255 0 = 0: | Key F0 (i.e. F0 from the DCC packet) is sent to the internal F0 (1:1). = 1: Key F1 is sent to the internal F0.... = 28: Key F28 is sent to the internal F0. |
| **#401–#428** | Input mapping for internal F1 - F28 0, | 1 - 28, 29, 30 - 255 | 0 | Same as input mapping above for other functions: CV #403 = 1: Key F1 is forwarded to F3 = 9: Key F9 is forwarded to F3, etc. |
| **#430** | Swiss Mapping Group 1 “F-key” | 0 - 28, | — | 29 (for F0) 129 - 157 0 With the F-key defined here, the FOs defined in A1 (forw or Rev) and A2 (forw or rev) shall be turned on. |
| **#431** | Swiss Mapping Group 1 “M-key” or | Bit 0 - 6: 0 - 28, | — | 29 (for F0) and bit 7 0 The ”normal function mapping” of the M-key defined here shall be deactivated (i.e. |
| **#432** | Swiss Mapping Group 1 “A1” forward Bits 0 - 3: | 1 - 12 | — | 14 (FO0f) 15 (FO0r) Bits 5 - 7: 0 - 7 0 Bits 0 - 3: Function output to be switched ON in forward direction provided that both the “F” and “M”-keys are ON (if… |
| **#433** | Swiss Mapping Group 1 “A2” forward Bits 0 - 3: | 1 - 12 | — | 14 (FO0f) 15 (FO0r) Bits 5 - 7: 0 - 7 0 Bits 0 - 3: Additional function output to be switched ON in forward direction provided that both the “F” and “M”-keys… |
| **#434** | Swiss Mapping Group 1 “A1” reverse Bits 0 - 3: | 1 - 12 | — | 14 (FO0f) 15 (FO0r) Bits 5 - 7: 0 - 7 0 Bits 0 - 3: Function output to be switched ON in reverse direction provided that both, the “F” and “M”-keys are ON (i… |
| **#435** | Swiss Mapping Group 1 “A2” reverse Bits 0 - 3: | 1 - 12 | — | 14 (FO0f) 15 (FO0r) Bits 5 - 7: 0 - 7 0 Bits 0 - 3: Additional function output to be switched ON in reverse direction provided that both the “F” and “M”-keys… |
| **#436–#441** | . . . Group 2. . . . 0 All 6 CVs in group 2 are defined the same way as the 6 CVs in group 1. | — | — | — |
| **#442–#447** | . . . Group 3. . . . 0 All 6 CVs of the following groups are defined the same way is the 6 CVs in group 1. | — | — | — |
| **#448–#453** | . . . Group 4. . . . 0 . . . | — | — | — |
| **#454–#459** | . . . Group 5. . . . 0 . . . | — | — | — |
| **#460–#465** | . . . Group 6. . . . 0 . . . | — | — | — |
| **#466–#471** | . . . Group 7. . . . 0 . . . | — | — | — |
| **#472–#477** | . . . Group 8. . . . 0 . . . | — | — | — |
| **#478–#483** | . . . Group 9. . . . 0 . . . | — | — | — |
| **#484–#489** | . . . Group 10. . . . 0 . . . | — | — | — |
| **#490–#495** | . . . Group 11. . . . 0 . . . | — | — | — |
| **#496–#501** | . . . Group 12. . . . 0 . . . | — | — | — |

### CV #500–#699 — Per-function sound volumes and samples

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#502–#507** | . . . Group 13. . . . 0 . . . | — | — | — |
| **#512** | Dimming values for the “Swiss Mapping” Special configurations (031)*8 (Only bits | 3 - 7 are | — | used) Bits 0 - 2 248 Each group-CV (i.e. #432, #433, #434, #435) can be linked to one of these five dimming CVs. |
| **#513** | Sound number F1 Sample number of function sound on F1 5.3 | — | — | — |
| **#514** | Function sound F1 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#515** | Loop info F1 | Bit 0 to Bit2: Loop count 0-7 Bit 3 = 1: repeat sound when “loop” is on (active  | — | =sound looping Bit 4 = 1: play sound only when driving forwards Bit 5 = 1: Play sound only when driving backwards Bit 6 = 1: /shorten sound when off (deactiv… |
| **#516** | Sound number F2 Sample number of function sound on F2 5.3 | — | — | — |
| **#517** | Function sound F2 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#518** | Loop info F2 Same as CV #515 but for F2 5.3 | — | — | — |
| **#519** | Sound number F3 Sample number of function sound on F3 5.3 | — | — | — |
| **#520** | Function sound F3 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#521** | Loop info F3 Same as CV #515 but for F3 5.3 | — | — | — |
| **#522** | Sound number F4 Sample number of function sound on F4 5.3 | — | — | — |
| **#523** | Function sound F4 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#524** | Loop info F4 Same as CV #515 but for F4 5.3 | — | — | — |
| **#525** | Sound number F5 Sample number of function sound on F5 5.3 | — | — | — |
| **#526** | Function sound F5 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#527** | Loop info F5 Same as CV #515 but for F5 5.3 | — | — | — |
| **#528** | Sound number F6 Sample number of function sound on F6 5.3 | — | — | — |
| **#529** | Function sound F6 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#530** | Loop info F6 Same as CV #515 but for F6 5.3 | — | — | — |
| **#531** | Sound number F7 Sample number of function sound on F7 5.3 | — | — | — |
| **#532** | Function sound F7 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#533** | Loop info F7 Same as CV #515 but for F7 5.3 | — | — | — |
| **#534** | Sound number F8 Sample number of function sound on F8 5.3 | — | — | — |
| **#535** | Function sound F8 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#536** | Loop info F8 Same as CV #515 but for F8 5.3 | — | — | — |
| **#537** | Sound number F9 Sample number of function sound on F9 5.3 | — | — | — |
| **#538** | Function sound F9 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#539** | Loop info F9 Same as CV #515 but for F9 5.3 | — | — | — |
| **#540** | Sound number F10 Sample number of function sound on F10 5.3 | — | — | — |
| **#541** | Function sound F10 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#542** | Loop info F10 Same as CV #515 but for F10 5.3 | — | — | — |
| **#543** | Sound number F11 Sample number of function sound on F11 5.3 | — | — | — |
| **#544** | Function sound F11 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#545** | Loop info F11 Same as CV #515 but for F11 5.3 | — | — | — |
| **#546** | Sound number F12 Sample number of function sound on F12 5.3 | — | — | — |
| **#547** | Function sound F12 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#548** | Loop info F12 Same as CV #515 but for F12 5.3 | — | — | — |
| **#549** | Sound number F13 Sample number of function sound on F13 5.3 | — | — | — |
| **#550** | Function sound F13 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#551** | Loop info F13 Same as CV #515 but for F13 5.3 | — | — | — |
| **#552** | Sound number F14 Sample number of function sound on F14 5.3 | — | — | — |
| **#553** | Function sound F14 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#554** | Loop info F14 Same as CV #515 but for F14 5.3 | — | — | — |
| **#555** | Sound number F15 Sample number of function sound on F15 5.3 | — | — | — |
| **#556** | Function sound F15 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#557** | Loop info F15 Same as CV #515 but for F15 5.3 | — | — | — |
| **#558** | Sound number F16 Sample number of function sound on F16 5.3 | — | — | — |
| **#559** | Function sound F16 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#560** | Loop info F16 Same as CV #515 but for F16 5.3 | — | — | — |
| **#561** | Sound number F17 Sample number of function sound on F17 5.3 | — | — | — |
| **#562** | Function sound F17 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#563** | Loop info F17 Same as CV #515 but for F17 5.3 | — | — | — |
| **#564** | Sound number F18 Sample number of function sound on F18 5.3 | — | — | — |
| **#565** | Function sound F18 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#566** | Loop info F18 Same as CV #515 but for F18 5.3 | — | — | — |
| **#567** | Sound number F19 Sample number of function sound on F19 5.3 | — | — | — |
| **#568** | Function sound F19 | 0 - 255 | 0 | Volume adjustment 5.3 |
| **#569** | Loop info F19 Same as CV #515 but for F19 | — | — | — |
| **#570** | Sound number F0 Sample number of function sound on F0 5.3 | — | — | — |
| **#571** | Function sound F0 | 0 - 255 | = 100, 1- 100 % 0 | Sound volume operated with function key F0 = 0: full volume, original sound sample volume (same as 255) = 1 - 254: reduced volume 1 – 99.5 % = 255: full volume |
| **#572** | Loop info F0 Same as CV #515 but for F0 | — | — | — |
| **#573** | Sound number simmering Sample number 5.3 | — | — | — |
| **#574** | “Simmering” | 0 - 255 | 0 | Volume driving sound “simmering” |
| **#575** | Sound number change of direction Sample number 5.3 | — | — | — |
| **#576** | Sound “changing directions” | 0 - 255 | 0 | Volume driving sound for e.g. Johnson Bar |
| **#577** | Sound number brake squeal Sample number 5.3 | — | — | — |
| **#578** | “Brake squeal” | 0 - 255 | 0 | Brake squeal volume |
| **#579** | Sound number thyristor sound Sample number 5.3 | — | — | — |
| **#580** | Thyristor sound | 0 - 255 | 0 | Thyristor sound volume (ELECTRIC engine) |
| **#581** | Sound number starting whistle Sample number 5.3 | — | — | — |
| **#582** | “Starting whistle” | 0 - 255 | 0 | Volume Starting whistle (STEAM/DIESEL) |
| **#583** | Sound number blow-off Sample number 5.3 | — | — | — |
| **#584** | Blow-off | 0 - 255 | 0 | Blow-off volume (STEAM engine) |
| **#585** | Sound number electric motor Sample number 5.3 | — | — | — |
| **#586** | Electric motor | 0 - 255 | 0 | Electric motor volume (ELECTRIC engine) |
| **#587** | Sound number rolling sound Sample number | — | — | — |
| **#588** | “Rolling sound” | 0 - 255 | 0 | Driving sounds volume (rolling/wheels) |
| **#589** | Sound number switchgear Sample number 5.3 | — | — | — |
| **#590** | Electric switch gear sound | 0 - 255 | 0 | Switch gear volume (ELECTRIC engine) 5.3 |
| **#600** | Turbo | 0 - 255 | 0 | Turbocharger volume (DIESEL engine) 5.3 |
| **#602** | Dynamic brakes | 0 - 255 | 0 | Volume “dynamic brake” 5.3 |
| **#604** | “Brake squeal” | 0 - 255 | 0 | Volume “cornering squeal” 5.3 |
| **#671** | Switch input sound S4 | 0 - 255 | 0 | Number of sound sample for input S4 5.3 |
| **#672** | Switch input sound S4 | 0 - 255 | 0 | Volume setting for the sound activ. with switch input S4 5.3 |
| **#673** | Sound number F20 Sample number of function sound on F20 5.3 | — | — | — |
| **#674** | Function sound F20 | 0 - 255 | — | Volume adjustment 5.3 |
| **#675** | Loop info F20 Same as CV #515 but for F20 5.3 | — | — | — |
| **#676** | Sound number F21 Sample number of function sound on F21 5.3 | — | — | — |
| **#677** | Function sound F21 | 0 - 255 | — | Volume adjustment 5.3 |
| **#678** | Loop info F21 Same as CV #515 but for F21 5.3 | — | — | — |
| **#679** | Sound number F22 Sample number of function sound on F22 5.3 | — | — | — |
| **#680** | Function sound F22 | 0 - 255 | — | Volume adjustment 5.3 |
| **#681** | Loop info F22 Same as CV #515 but for F22 5.3 | — | — | — |
| **#682** | Sound number F23 Sample number of function sound on F23 5.3 | — | — | — |
| **#683** | Function sound F23 | 0 - 255 | — | Volume adjustment 5.3 |
| **#684** | Loop Info F23 Same as CV #515 but for F23 5.3 | — | — | — |
| **#685** | Sound number F24 Sample number of function sound on F24 5.3 | — | — | — |
| **#686** | Function sound F24 | 0 - 255 | — | Volume adjustment 5.3 |
| **#687** | Loop info F24 Same as CV #515 but for F24 5.3 | — | — | — |
| **#688** | Sound number F25 Sample number of function sound on F25 5.3 | — | — | — |
| **#689** | Function sound F25 | 0 - 255 | — | Volume adjustment 5.3 |
| **#690** | Loop info F25 Same as CV #515 but for F25 5.3 | — | — | — |
| **#691** | Sound number F26 Sample number of function sound on F26 5.3 | — | — | — |
| **#692** | Function sound F26 | 0 - 255 | — | Volume adjustment 5.3 |
| **#693** | Loop info F26 Same as CV #515 but for F26 5.3 | — | — | — |
| **#694** | Sound number F27 Sample number of function sound on F27 5.3 | — | — | — |
| **#695** | Function sound F27 | 0 - 255 | — | Volume adjustment 5.3 |
| **#696** | Loop info F27 Same as CV #515 but for F27 5.3 | — | — | — |
| **#697** | Sound number F28 Sample number of function sound on F28 5.3 | — | — | — |
| **#698** | Function sound F28 | 0 - 255 | — | Volume adjustment 5.3 |
| **#699** | Loop info F28 Same as CV #515 but for F28 5.3 | — | — | — |

### CV #700–#849 — Random sounds, RailCom, and version info

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#726** | Connection 1 sound 0 Sound number for connection 1 (usually defined by sound project and should not be changed if possib | — | — | — |
| **#727** | Connection 1 FO 0 Function output to connection 1 which shall be activated - the sound is played back. 1 = FO0f, 2 = FO0 | — | — | — |
| **#728** | Connection 2 sound 0 Sound number to connection 2 5.3 | — | — | — |
| **#729** | Connection 2 FO 0 Function output to connection 2: 1 = FO0f, 2 = FO0r, 3 = FO1, ... 5.3 | — | — | — |
| **#730** | … | — | — | — |
| **#735** | … 0 … 5.3 | — | — | — |
| **#736** | Conn. 6 sound 0 Soundnumber to connection 6. 5.3 | — | — | — |
| **#737** | Connection 6 FO 0 Function output to connection 6: 1 = FO0f, 2 = FO0r, 3=FO1, ... | — | — | — |
| **#738** | Reed 1 sound number Sample number according to sample info for switch input S1 5.3 | — | — | — |
| **#739** | Switch input sound S1 | 0 - 255 | = 100, 1- 100 % 0 | Volume setting for the sound activated with switch input S1 = 0: full volume, original sound sample volume (same as 255) = 1 - 254: reduced volume 1 – 99.5 %… |
| **#740** | Reed 2 sound number Sample number according to sample info for switch input S2 5.3 | — | — | — |
| **#741** | Switch input sound S2 | 0 - 255 | 0 | Volume setting for the sound activated with switch input S2 |
| **#742** | Reed 3 sound number Sample number according to sample info for switch input S3 5.3 | — | — | — |
| **#743** | Switch input sound S3 | 0 - 255 | 0 | Volume setting for the sound activated with switch input S3 |
| **#744** | Random Sound Z1 Sample number of function sound on Z1 5.3 | — | — | — |
| **#745** | Function sound Z1 Volume setting for sound activated by random generator Z1 | — | — | — |
| **#746** | Random sound Z1 - info Bit3=1: Random sound Z1 may come at standstill Bit6=1: Random sound Z1 may come when moving | — | — | — |
| **#747** | Random Sound Z2 Sample number of function sound on Z2 5.3 | — | — | — |
| **#748** | Function sound Z2 Volume setting for sound activated by random generator Z2 | — | — | — |
| **#749** | Random sound Z2 - Loop info Bit3=1: Random sound Z2 may come at standstill Bit6=1: Random sound Z2 may come when moving | — | — | — |
| **#750** | Random Sound Z3 Sample number of function sound on Z3 5.3 | — | — | — |
| **#751** | Function sound Z3 Volume setting for sound activated by random generator Z3 | — | — | — |
| **#752** | Random sound Z3 - Loop info Bit3=1: Random sound Z3 may come at standstill Bit6=1: Random sound Z3 may come when moving | — | — | — |
| **#753** | Random Sound Z4 Sample number of function sound on Z4 5.3 | — | — | — |
| **#754** | Function sound Z4 Volume setting for sound activated by random generator Z4 | — | — | — |
| **#755** | Random sound Z4 - Loop info Bit3=1: Random sound Z4 may come at standstill Bit6=1: Random sound Z4 may come when moving | — | — | — |
| **#756** | Random Sound Z5 Sample number of function sound on Z5 5.3 | — | — | — |
| **#757** | Function sound Z5 Volume setting for sound activated by random generator Z5 | — | — | — |
| **#758** | Random sound Z5 - Loop info Bit3=1: Random sound Z5 may come at standstill Bit6=1: Random sound Z5 may come when moving | — | — | — |
| **#759** | Random Sound Z6 Sample number of function sound on Z6 5.3 | — | — | — |
| **#760** | Function sound Z6 Volume setting for sound activated by random gen. Z6 | — | — | — |
| **#761** | Random sound Z6 - Loop info Bit3=1: Random sound Z6 may come at standstill Bit6=1: Random sound Z6 may come when moving | — | — | — |
| **#762** | Random Sound Z7 Sample number of function sound on Z7 5.3 | — | — | — |
| **#763** | Function sound Z7 Volume setting for sound activated by random generator Z7 | — | — | — |
| **#764** | Random sound Z7 - Loop info Bit3=1: Random sound Z7 may come at standstill Bit6=1: Random sound Z7 may come when moving | — | — | — |
| **#765** | Random Sound Z8 Sample number of function sound on Z8 5.3 | — | — | — |
| **#766** | Function sound Z8 Volume setting for sound activated by random generator Z8 | — | — | — |
| **#767** | Random sound Z8 - Loop info Bit3=1: Random sound Z8 may come at standstill Bit6=1: Random sound Z8 may come when moving | — | — | — |
| **#768** | Read CV #265 | 0 - 31 = steam set 1-32 (CV 265 =1 to =32) 32- 63 = diesel- oder E-loco-set 1-32 | — | to=132). CV #265 can be read with CV #259 1:1 |
| **#800** | - #805 Swiss Mapping Group 14 “A2” reverse Bits 0 - 3: | 1 - 12 | — | 14 (FO0f) 15 (FO0r) Bits 5 - 7: 0 - 7 0 Bits 0 - 3: Additional function output to be switched ON in reverse direction provided that both the “F” and “M”-keys… |
| **#806** | - #811 . . . - Group 15. . . . 0 . . . | — | — | — |
| **#812** | - #817 . . . - Group 16. . . . 0 . . . | — | — | — |
| **#818–#823** | . . . - Group 17. . . . 0 . . . | — | — | — |
| **#830** | Braking distance forward High Byte | 0 - 255 | 0 | Supplementary to CV #140: Extended definition of the Constant Braking Distance: With CV #830 - #833 a more precise and direction dependent braking distance c… |
| **#831** | Braking distance forward Low Byte | 0 - 255 | 0 | — |
| **#832** | Braking distance backward High Byte | 0 - 255 | 0 | — |
| **#833** | Braking distance backward Low Byte | 0 - 255 | 0 5.6 | — |
| **#835** | Further switching keys | 0-32 | — | Extension to CV #345. Here the number of consecutive keys can be defined, which then switch to auf Set2, Set3, Set4, …. .. |
| **#836** | Motor Start Sound in SW version 6.00 or higher | Bit 0 Bit 0 = 1: Bit 0 = 1: Loco shall not start driving before Start | — | sound is fully played. 8 |
| **#837** | Script processes | Bit 0-7 | 0 | Bit 0 - 7 = 1: Deactivate scripts 1 - 8 5.6 |
| **#838** | Thyristors Maximum speed | 0 – 255 | 255 | Definition of the "maximum" speed level (1 - 255) for which pitch applies according to CV #291. |
| **#839** | Software Patch Version 0 Software Patch Version | — | — | — |
| **#842** | Bootloader Patch Version Bootloader Patch Version 8 | — | — | — |
| **#843** | Script processes 0 | Bit 0 - 7 = 1: Deactivate scripts 9 - 16 | — | — |
| **#844** | Electric motor pitch (frequency) dependence on speed | 0 - 255 | 0 | Extension of the frequency increase to CV 299 0: no further frequency increase to CV #299 1: value of CV #299 x 1.01 2 … 254: value of CV #299 x 1.02 … 3.54 … |

### CV #850–#1024 — Extended / read-only

| CV | Name | Range | Default | Description |
|----|------|-------|---------|-------------|
| **#980** | - | — | — | — |
| **#1019** | Script CVs, see changelog ZSP 0 Values of these CVs are read by scripts. This allows to change values in scripts when th | — | — | — |

---

*Document compiled from ZIMO instruction manual MS-MN-Decoders_EN.pdf, SW 5.27.14. Features marked SW 6.00 are preview/planned and may not be available in current firmware.*
