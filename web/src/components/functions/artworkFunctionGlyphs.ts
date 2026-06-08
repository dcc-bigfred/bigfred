import hornHighSvg from "../../icons/horn_high.svg?raw";
import hornLowSvg from "../../icons/horn_low.svg?raw";

export type ArtworkPath = {
  fill: string;
  d: string;
  transform?: string;
};

const HORN_VIEW_BOX = "0 0 928 1152";

function parseArtworkPaths(svg: string): ArtworkPath[] {
  const paths: ArtworkPath[] = [];
  const re = /<path\b([^>]*?)\/>/gs;
  let match: RegExpExecArray | null;
  while ((match = re.exec(svg)) !== null) {
    const attrs = match[1];
    const fill = attrs.match(/\bfill="([^"]+)"/)?.[1];
    const d = attrs.match(/\bd="([^"]+)"/)?.[1];
    if (!fill || !d) continue;
    const transform = attrs.match(/\btransform="([^"]+)"/)?.[1];
    paths.push({ fill, d, transform });
  }
  return paths;
}

const ARTWORK_BY_SLUG: Record<string, { viewBox: string; paths: ArtworkPath[] }> =
  {
    horn_low: {
      viewBox: HORN_VIEW_BOX,
      paths: parseArtworkPaths(hornLowSvg),
    },
    horn_high: {
      viewBox: HORN_VIEW_BOX,
      paths: parseArtworkPaths(hornHighSvg),
    },
  };

export function hasArtworkFunctionGlyph(slug: string): boolean {
  return slug in ARTWORK_BY_SLUG;
}

export function getArtworkFunctionGlyph(slug: string) {
  return ARTWORK_BY_SLUG[slug];
}
