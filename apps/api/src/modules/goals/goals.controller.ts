import {
  type ContributeGoalInput,
  type CreateGoalInput,
  contributeGoalSchema,
  createGoalSchema,
  type Goal,
  type UpdateGoalInput,
  updateGoalSchema,
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
} from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";
import { ZodValidationPipe } from "../../common/zod-validation.pipe";
import { GoalsService } from "./goals.service";

@ApiTags("goals")
@Controller("goals")
export class GoalsController {
  constructor(private readonly service: GoalsService) {}

  @Get()
  @ApiOperation({ summary: "Lista metas com progresso e aporte mensal necessário" })
  list(): Promise<Goal[]> {
    return this.service.list();
  }

  @Get(":id")
  @ApiOperation({ summary: "Detalha uma meta" })
  findById(@Param("id", ParseUUIDPipe) id: string): Promise<Goal> {
    return this.service.findById(id);
  }

  @Post()
  @ApiOperation({ summary: "Cria uma meta financeira (caixinha)" })
  create(@Body(new ZodValidationPipe(createGoalSchema)) input: CreateGoalInput): Promise<Goal> {
    return this.service.create(input);
  }

  @Patch(":id")
  @ApiOperation({ summary: "Atualiza uma meta" })
  update(
    @Param("id", ParseUUIDPipe) id: string,
    @Body(new ZodValidationPipe(updateGoalSchema)) input: UpdateGoalInput,
  ): Promise<Goal> {
    return this.service.update(id, input);
  }

  @Post(":id/contributions")
  @ApiOperation({ summary: "Registra aporte (positivo) ou resgate (negativo) na meta" })
  contribute(
    @Param("id", ParseUUIDPipe) id: string,
    @Body(new ZodValidationPipe(contributeGoalSchema)) input: ContributeGoalInput,
  ): Promise<Goal> {
    return this.service.contribute(id, input);
  }

  @Delete(":id")
  @HttpCode(204)
  @ApiOperation({ summary: "Exclui uma meta e seus aportes" })
  async delete(@Param("id", ParseUUIDPipe) id: string): Promise<void> {
    await this.service.delete(id);
  }
}
