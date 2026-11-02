import "./env";
import "reflect-metadata";
import { NestFactory } from "@nestjs/core";
import { DocumentBuilder, SwaggerModule } from "@nestjs/swagger";
import { AppModule } from "./app.module";

async function bootstrap(): Promise<void> {
  const app = await NestFactory.create(AppModule);

  app.setGlobalPrefix("api");
  app.enableCors({
    origin: process.env.WEB_ORIGIN?.split(",") ?? true,
    credentials: true,
  });
  app.enableShutdownHooks();

  const swaggerConfig = new DocumentBuilder()
    .setTitle("FinFlow API")
    .setDescription("API de controle financeiro pessoal — contas, cartões, orçamentos e metas")
    .setVersion("0.1.0")
    .addBearerAuth()
    .build();
  SwaggerModule.setup("docs", app, SwaggerModule.createDocument(app, swaggerConfig));

  const port = Number(process.env.API_PORT ?? 3333);
  await app.listen(port);
  console.log(
    `FinFlow API em http://localhost:${port}/api — Swagger em http://localhost:${port}/docs`,
  );
}

void bootstrap();
