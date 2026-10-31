/**
 * Prepara o banco de testes: valida que o nome termina em _test e aplica as
 * migrations versionadas com `prisma migrate deploy` (não-destrutivo).
 * Nunca toca no banco de desenvolvimento.
 */
import { execSync } from "node:child_process";
import path from "node:path";
import { config } from "dotenv";

config({ path: path.resolve(__dirname, "../../../.env") });
config();

const testUrl = process.env.DATABASE_URL_TEST;
if (!testUrl) {
  throw new Error("DATABASE_URL_TEST não definido — configure o .env e suba o db_test");
}
const dbName = new URL(testUrl).pathname.replace("/", "");
if (!dbName.endsWith("_test")) {
  throw new Error(`banco de integração deve terminar em _test (recebi "${dbName}")`);
}

execSync("pnpm prisma migrate deploy", {
  cwd: path.resolve(__dirname, ".."),
  stdio: "inherit",
  env: { ...process.env, DATABASE_URL: testUrl },
});
