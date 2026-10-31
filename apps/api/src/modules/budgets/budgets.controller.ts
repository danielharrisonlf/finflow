import {
  type Budget,
  type ListBudgetsQuery,
  listBudgetsQuerySchema,
  type UpsertBudgetInput,
  upsertBudgetSchema,
} from "@finflow/contracts";
import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  ParseUUIDPipe,
  Post,
  Query,
} from "@nestjs/common";
import { ApiOperation, ApiQuery, ApiTags } from "@nestjs/swagger";
import { ZodValidationPipe } from "../../common/zod-validation.pipe";
import { BudgetsService } from "./budgets.service";

@ApiTags("budgets")
@Controller("budgets")
export class BudgetsController {
  constructor(private readonly service: BudgetsService) {}

  @Get()
  @ApiOperation({ summary: "Lista orçamentos com gasto do mês e status (verde/amarelo/vermelho)" })
  @ApiQuery({ name: "month", required: false })
  @ApiQuery({ name: "year", required: false })
  list(
    @Query(new ZodValidationPipe(listBudgetsQuerySchema)) query: ListBudgetsQuery,
  ): Promise<Budget[]> {
    return this.service.list(query);
  }

  @Post()
  @ApiOperation({ summary: "Define (ou atualiza) o teto mensal de uma categoria" })
  upsert(
    @Body(new ZodValidationPipe(upsertBudgetSchema)) input: UpsertBudgetInput,
  ): Promise<Budget> {
    return this.service.upsert(input);
  }

  @Delete(":id")
  @HttpCode(204)
  @ApiOperation({ summary: "Remove um orçamento" })
  async delete(@Param("id", ParseUUIDPipe) id: string): Promise<void> {
    await this.service.delete(id);
  }
}
