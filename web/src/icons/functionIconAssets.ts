/** SVG files in this folder; basename (without `.svg`) = catalogue slug. */
const modules = import.meta.glob<string>("./*.svg", {
  eager: true,
  import: "default",
  query: "?url",
});

const urlByBasename = new Map<string, string>();

/** Rendered as inline SVG paths in `ArtworkFunctionGlyph` (not `<img>`). */
const INLINE_ARTWORK_SLUGS = new Set(["horn_low", "horn_high"]);

for (const [path, url] of Object.entries(modules)) {
  const match = path.match(/\/([^/]+)\.svg$/);
  if (match && !INLINE_ARTWORK_SLUGS.has(match[1])) {
    urlByBasename.set(match[1], url);
  }
}

const UNSPECIFIED_BASENAME = "unknown-func";

/** Resolved URL for a function-icon slug, if `web/src/icons/<slug>.svg` exists. */
export function functionIconAssetUrl(slug: string): string | undefined {
  if (slug === "unspecified") {
    return urlByBasename.get(UNSPECIFIED_BASENAME);
  }
  return urlByBasename.get(slug);
}

export function hasFunctionIconAsset(slug: string): boolean {
  return functionIconAssetUrl(slug) !== undefined;
}
