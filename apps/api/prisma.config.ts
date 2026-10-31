import path from "node:path";
import { config } from "dotenv";
import { defineConfig } from "prisma/config";

// O Prisma 7 não carrega .env automaticamente; o arquivo fica na raiz do monorepo.
config({ path: path.resolve(process.cwd(), "../../.env") });
config();

export default defineConfig({
  schema: path.join("prisma", "schema.prisma"),
  datasource: {
    // URL usada pelo CLI (migrate/db push); o runtime usa o adapter no PrismaClient.
    url: process.env.DATABASE_URL ?? "",
  },
  migrations: {
    path: path.join("prisma", "migrations"),
    seed: "tsx prisma/seed.ts",
  },
});
