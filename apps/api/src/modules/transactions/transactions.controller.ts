import {
  type CreateTransactionInput,
  createTransactionSchema,
  type ExportTransactionsQuery,
  exportTransactionsQuerySchema,
  type ListTransactionsQuery,
  listTransactionsQuerySchema,
  type Paginated,
  type Transaction,
  type TransferInput,
  transferSchema,
  type UpdateTransactionInput,
  updateTransactionSchema,
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
  Res,
} from "@nestjs/common";
import { ApiOperation, ApiQuery, ApiTags } from "@nestjs/swagger";
import type { Response } from "express";
import { z } from "zod";
import { ZodValidationPipe } from "../../common/zod-validation.pipe";
import { ExportService } from "./export.service";
import { TransactionsService } from "./transactions.service";

const deleteScopeSchema = z.enum(["single", "group"]).default("single");

@ApiTags("transactions")
@Controller("transactions")
export class TransactionsController {
  constructor(
    private readonly service: TransactionsService,
    private readonly exporter: ExportService,
  ) {}

  @Get()
  @ApiOperation({ summary: "Extrato paginado com filtros por período, conta, categoria e busca" })
  @ApiQuery({ name: "month", required: false })
  @ApiQuery({ name: "year", required: false })
  @ApiQuery({ name: "accountId", required: false })
  @ApiQuery({ name: "cardId", required: false })
  @ApiQuery({ name: "categoryId", required: false })
  @ApiQuery({ name: "type", required: false })
  @ApiQuery({ name: "status", required: false })
  @ApiQuery({ name: "search", required: false })
  list(
    @Query(new ZodValidationPipe(listTransactionsQuerySchema)) query: ListTransactionsQuery,
  ): Promise<Paginated<Transaction>> {
    return this.service.list(query);
  }

  @Get("export")
  @ApiOperation({ summary: "Exporta o extrato filtrado em CSV ou OFX" })
  @ApiQuery({ name: "format", required: false, enum: ["csv", "ofx"] })
  async export(
    @Query(new ZodValidationPipe(exportTransactionsQuerySchema)) query: ExportTransactionsQuery,
    @Res() res: Response,
  ): Promise<void> {
    const { content, contentType, filename } = await this.exporter.export(query);
    res
      .header("Content-Type", contentType)
      .header("Content-Disposition", `attachment; filename="${filename}"`)
      .send(content);
  }

  @Get(":id")
  @ApiOperation({ summary: "Detalha uma transação" })
  findById(@Param("id", ParseUUIDPipe) id: string): Promise<Transaction> {
    return this.service.findById(id);
  }

  @Post()
  @ApiOperation({ summary: "Lança receita ou despesa em uma conta" })
  create(
    @Body(new ZodValidationPipe(createTransactionSchema)) input: CreateTransactionInput,
  ): Promise<Transaction> {
    return this.service.create(input);
  }

  @Post("transfer")
  @ApiOperation({ summary: "Transfere entre contas próprias (débito + crédito atômicos)" })
  transfer(
    @Body(new ZodValidationPipe(transferSchema)) input: TransferInput,
  ): Promise<{ transferGroupId: string }> {
    return this.service.transfer(input);
  }

  @Patch(":id")
  @ApiOperation({ summary: "Atualiza uma transação" })
  update(
    @Param("id", ParseUUIDPipe) id: string,
    @Body(new ZodValidationPipe(updateTransactionSchema)) input: UpdateTransactionInput,
  ): Promise<Transaction> {
    return this.service.update(id, input);
  }

  @Patch(":id/confirm")
  @ApiOperation({ summary: "Marca transação pendente como paga/recebida" })
  confirm(@Param("id", ParseUUIDPipe) id: string): Promise<Transaction> {
    return this.service.confirm(id);
  }

  @Delete(":id")
  @HttpCode(204)
  @ApiOperation({
    summary: "Exclui transação (transferências caem em par; scope=group exclui todas as parcelas)",
  })
  @ApiQuery({ name: "scope", required: false, enum: ["single", "group"] })
  async delete(
    @Param("id", ParseUUIDPipe) id: string,
    @Query("scope", new ZodValidationPipe(deleteScopeSchema)) scope: "single" | "group",
  ): Promise<void> {
    await this.service.delete(id, scope);
  }
}
