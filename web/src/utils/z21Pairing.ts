/** Maps Z21 pairing CV3/CV4 to the F-key digit sequence (F0–F9 per digit). */
export function formatZ21PairingFunctionKeys(cv3: number, cv4: number): string {
  return `${cv3}${cv4}`
    .split("")
    .map((digit) => `F${digit}`)
    .join(" ");
}

/** Two-line pairing code: CV3/CV4 row and an F-key row separated by orLabel. */
export function formatZ21PairingCodeLines(
  cv3: number,
  cv4: number,
  orLabel: string,
): string {
  return `CV3=${cv3} + CV4=${cv4}\n${orLabel} ${formatZ21PairingFunctionKeys(cv3, cv4)}`;
}
