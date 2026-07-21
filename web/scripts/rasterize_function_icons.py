#!/usr/bin/env python3
"""Rasterize individual function-icon SVGs to PNG for the Throttle UI.

Default output size is 70x70. Sources stay committed under src/icons/*.svg;
generated PNGs go to src/icons/png/ (gitignored).

Requires `rsvg-convert` (librsvg), e.g. pacman -S librsvg / apt install librsvg2-bin.
"""

from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
from pathlib import Path

DEFAULT_SIZE = 70


def main() -> int:
    script_dir = Path(__file__).resolve().parent
    web_root = script_dir.parent
    default_src = web_root / "src" / "icons"
    default_out = default_src / "png"

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--src",
        type=Path,
        default=default_src,
        help=f"Directory with source SVGs (default: {default_src})",
    )
    parser.add_argument(
        "--out",
        type=Path,
        default=default_out,
        help=f"Output directory for PNGs (default: {default_out})",
    )
    parser.add_argument(
        "--size",
        type=int,
        default=DEFAULT_SIZE,
        help=f"Output width and height in pixels (default: {DEFAULT_SIZE})",
    )
    args = parser.parse_args()

    if args.size < 1:
        print("error: --size must be >= 1", file=sys.stderr)
        return 1

    rsvg = shutil.which("rsvg-convert")
    if rsvg is None:
        print(
            "error: rsvg-convert not found. Install librsvg, e.g.:\n"
            "  pacman -S librsvg\n"
            "  apt install librsvg2-bin",
            file=sys.stderr,
        )
        return 1

    src_dir: Path = args.src.resolve()
    out_dir: Path = args.out.resolve()

    if not src_dir.is_dir():
        print(f"error: source directory not found: {src_dir}", file=sys.stderr)
        return 1

    svgs = sorted(src_dir.glob("*.svg"))
    if not svgs:
        print(f"error: no SVG files in {src_dir}", file=sys.stderr)
        return 1

    # Always wipe the output dir so removed SVGs do not leave orphan PNGs.
    if out_dir.exists():
        shutil.rmtree(out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    errors = 0
    for svg_path in svgs:
        png_path = out_dir / f"{svg_path.stem}.png"
        try:
            subprocess.run(
                [
                    rsvg,
                    f"--width={args.size}",
                    f"--height={args.size}",
                    "--keep-aspect-ratio",
                    "-f",
                    "png",
                    "-o",
                    str(png_path),
                    str(svg_path),
                ],
                check=True,
                capture_output=True,
                text=True,
            )
            print(f"ok  {svg_path.name} -> {png_path.relative_to(web_root)}")
        except subprocess.CalledProcessError as exc:
            errors += 1
            detail = (exc.stderr or exc.stdout or str(exc)).strip()
            print(f"fail {svg_path.name}: {detail}", file=sys.stderr)

    if errors:
        print(f"error: {errors} of {len(svgs)} icons failed", file=sys.stderr)
        return 1

    print(f"rasterized {len(svgs)} icons at {args.size}x{args.size} -> {out_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
