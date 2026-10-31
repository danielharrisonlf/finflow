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

// Toda a suíte de integração roda contra o banco de teste.
process.env.DATABASE_URL = testUrl;
