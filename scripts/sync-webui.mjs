import { cpSync, existsSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";

const src = path.resolve("apps/web/dist");
const dest = path.resolve("internal/webui/dist");

if (!existsSync(path.join(src, "index.html"))) {
  console.error(
    "apps/web/dist/index.html não encontrado — rode `pnpm --filter @finflow/web build` primeiro.",
  );
  process.exit(1);
}

rmSync(dest, { recursive: true, force: true });
mkdirSync(dest, { recursive: true });
cpSync(src, dest, { recursive: true });
writeFileSync(
  path.join(dest, ".gitkeep"),
  "# Placeholder so `go build` succeeds before `apps/web/dist` is copied here.\n",
);
console.log("✔ apps/web/dist sincronizado em internal/webui/dist");
