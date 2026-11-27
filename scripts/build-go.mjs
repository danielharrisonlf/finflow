import { execFileSync } from "node:child_process";

const binary = process.platform === "win32" ? "finflow.exe" : "finflow";
execFileSync("go", ["build", "-ldflags=-s -w", "-o", binary, "./cmd/finflow"], {
  stdio: "inherit",
});
console.log(`✔ Binário compilado: ./${binary}`);
