import {
  type Category,
  type CreateCategoryInput,
  categoryKindSchema,
  createCategorySchema,
  type UpdateCategoryInput,
  updateCategorySchema,
} from "@finflow/contracts";
import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  ParseUUIDPipe,
  Patch,
  Post,
  Query,
} from "@nestjs/common";
import { ApiOperation, ApiQuery, ApiTags } from "@nestjs/swagger";
import { ZodValidationPipe } from "../../common/zod-validation.pipe";
import { CategoriesService } from "./categories.service";

@ApiTags("categories")
@Controller("categories")
export class CategoriesController {
  constructor(private readonly service: CategoriesService) {}

  @Get()
  @ApiOperation({ summary: "Lista categorias, opcionalmente filtradas por tipo" })
  @ApiQuery({ name: "kind", required: false, enum: categoryKindSchema.options })
  list(
    @Query("kind", new ZodValidationPipe(categoryKindSchema.optional()))
    kind?: Category["kind"],
  ): Promise<Category[]> {
    return this.service.list(kind);
  }

  @Post()
  @ApiOperation({ summary: "Cria uma categoria" })
  create(
    @Body(new ZodValidationPipe(createCategorySchema)) input: CreateCategoryInput,
  ): Promise<Category> {
    return this.service.create(input);
  }

  @Patch(":id")
  @ApiOperation({ summary: "Atualiza uma categoria" })
  update(
    @Param("id", ParseUUIDPipe) id: string,
    @Body(new ZodValidationPipe(updateCategorySchema)) input: UpdateCategoryInput,
  ): Promise<Category> {
    return this.service.update(id, input);
  }

  @Delete(":id")
  @HttpCode(204)
  @ApiOperation({ summary: "Remove uma categoria (transações ficam sem categoria)" })
  async delete(@Param("id", ParseUUIDPipe) id: string): Promise<void> {
    await this.service.delete(id);
  }
}
