import path from "node:path";
import { config } from "dotenv";

// Carrega o .env da raiz do monorepo (dist/ e src/ têm a mesma profundidade).
config({ path: path.resolve(__dirname, "../../../.env") });
config();
