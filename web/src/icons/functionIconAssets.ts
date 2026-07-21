/** PNG files in `./png/` (generated from sibling SVGs); basename = catalogue slug. */
const modules = import.meta.glob<string>("./png/*.png", {
  eager: true,
  import: "default",
  query: "?url",
});

const urlByBasename = new Map<string, string>();

for (const [path, url] of Object.entries(modules)) {
  const match = path.match(/\/([^/]+)\.png$/);
  if (match) {
    urlByBasename.set(match[1], url);
  }
}

const UNSPECIFIED_BASENAME = "unknown-func";

/** Resolved URL for a function-icon slug, if `web/src/icons/png/<slug>.png` exists. */
export function functionIconAssetUrl(slug: string): string | undefined {
  if (slug === "unspecified") {
    return urlByBasename.get(UNSPECIFIED_BASENAME);
  }
  return urlByBasename.get(slug);
}

export function hasFunctionIconAsset(slug: string): boolean {
  return functionIconAssetUrl(slug) !== undefined;
}
