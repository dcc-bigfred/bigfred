import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { Plugin } from "vite";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const scriptPath = path.join(webRoot, "scripts", "rasterize_function_icons.py");

function rasterizeFunctionIcons(): void {
  const result = spawnSync("python3", [scriptPath], {
    cwd: webRoot,
    encoding: "utf8",
    stdio: ["ignore", "inherit", "inherit"],
  });

  if (result.error) {
    throw new Error(
      `Failed to run rasterize_function_icons.py: ${result.error.message}`,
    );
  }
  if (result.status !== 0) {
    throw new Error(
      `rasterize_function_icons.py exited with code ${result.status ?? "null"} ` +
        "(requires rsvg-convert / librsvg)",
    );
  }
}

/**
 * Ensures `src/icons/png/*.png` exist before Vite resolves `import.meta.glob`.
 * Runs on both `vite` (dev) and `vite build`.
 */
export function rasterizeFunctionIconsPlugin(): Plugin {
  let ran = false;

  return {
    name: "bigfred-rasterize-function-icons",
    buildStart() {
      // Vite may call buildStart more than once in one process; rasterize once.
      if (ran) return;
      ran = true;
      rasterizeFunctionIcons();
    },
  };
}
