import {
  type CashflowPoint,
  type CategorySlice,
  type DashboardQuery,
  type DashboardSummary,
  dashboardQuerySchema,
  type UpcomingItem,
} from "@finflow/contracts";
import { Controller, Get, Query } from "@nestjs/common";
import { ApiOperation, ApiQuery, ApiTags } from "@nestjs/swagger";
import { ZodValidationPipe } from "../../common/zod-validation.pipe";
import { DashboardService } from "./dashboard.service";

@ApiTags("dashboard")
@Controller("dashboard")
export class DashboardController {
  constructor(private readonly service: DashboardService) {}

  @Get("summary")
  @ApiOperation({ summary: "Métricas principais: saldo, receitas, despesas, balanço e faturas" })
  @ApiQuery({ name: "month", required: false })
  @ApiQuery({ name: "year", required: false })
  summary(
    @Query(new ZodValidationPipe(dashboardQuerySchema)) query: DashboardQuery,
  ): Promise<DashboardSummary> {
    return this.service.summary(query);
  }

  @Get("cashflow")
  @ApiOperation({ summary: "Fluxo de caixa diário do mês (entradas vs. saídas)" })
  @ApiQuery({ name: "month", required: false })
  @ApiQuery({ name: "year", required: false })
  cashflow(
    @Query(new ZodValidationPipe(dashboardQuerySchema)) query: DashboardQuery,
  ): Promise<CashflowPoint[]> {
    return this.service.cashflow(query);
  }

  @Get("categories")
  @ApiOperation({ summary: "Distribuição de despesas por categoria (donut)" })
  @ApiQuery({ name: "month", required: false })
  @ApiQuery({ name: "year", required: false })
  categories(
    @Query(new ZodValidationPipe(dashboardQuerySchema)) query: DashboardQuery,
  ): Promise<CategorySlice[]> {
    return this.service.categories(query);
  }

  @Get("upcoming")
  @ApiOperation({ summary: "Vencimentos dos próximos 7 dias (pendências e faturas)" })
  upcoming(): Promise<UpcomingItem[]> {
    return this.service.upcoming();
  }
}
