import {
  type Card,
  type CreateCardExpenseInput,
  type CreateCardInput,
  createCardExpenseSchema,
  createCardSchema,
  type Invoice,
  type PayInvoiceInput,
  payInvoiceSchema,
  type Transaction,
  type UpdateCardInput,
  updateCardSchema,
} from "@finflow/contracts";
import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  ParseBoolPipe,
  ParseUUIDPipe,
  Patch,
  Post,
  Query,
} from "@nestjs/common";
import { ApiOperation, ApiQuery, ApiTags } from "@nestjs/swagger";
import { ZodValidationPipe } from "../../common/zod-validation.pipe";
import { CardsService } from "./cards.service";

@ApiTags("cards")
@Controller("cards")
export class CardsController {
  constructor(private readonly service: CardsService) {}

  @Get()
  @ApiOperation({ summary: "Lista cartões com limite usado e fatura atual" })
  @ApiQuery({ name: "includeArchived", required: false, type: Boolean })
  list(
    @Query("includeArchived", new ParseBoolPipe({ optional: true }))
    includeArchived?: boolean,
  ): Promise<Card[]> {
    return this.service.list(includeArchived ?? false);
  }

  @Get(":id")
  @ApiOperation({ summary: "Detalha um cartão" })
  findById(@Param("id", ParseUUIDPipe) id: string): Promise<Card> {
    return this.service.findById(id);
  }

  @Post()
  @ApiOperation({ summary: "Cadastra um cartão de crédito" })
  create(@Body(new ZodValidationPipe(createCardSchema)) input: CreateCardInput): Promise<Card> {
    return this.service.create(input);
  }

  @Patch(":id")
  @ApiOperation({ summary: "Atualiza ou arquiva um cartão" })
  update(
    @Param("id", ParseUUIDPipe) id: string,
    @Body(new ZodValidationPipe(updateCardSchema)) input: UpdateCardInput,
  ): Promise<Card> {
    return this.service.update(id, input);
  }

  @Delete(":id")
  @HttpCode(204)
  @ApiOperation({ summary: "Exclui cartão sem lançamentos" })
  async delete(@Param("id", ParseUUIDPipe) id: string): Promise<void> {
    await this.service.delete(id);
  }

  @Post(":id/expenses")
  @ApiOperation({ summary: "Lança compra no cartão (à vista ou parcelada em N vezes)" })
  addExpense(
    @Param("id", ParseUUIDPipe) id: string,
    @Body(new ZodValidationPipe(createCardExpenseSchema)) input: CreateCardExpenseInput,
  ): Promise<{ created: number }> {
    return this.service.addExpense(id, input);
  }

  @Get(":id/invoices")
  @ApiOperation({ summary: "Lista faturas do cartão com status e totais" })
  listInvoices(@Param("id", ParseUUIDPipe) id: string): Promise<Invoice[]> {
    return this.service.listInvoices(id);
  }

  @Get(":id/invoices/:invoiceId/items")
  @ApiOperation({ summary: "Lista os lançamentos de uma fatura" })
  listInvoiceItems(
    @Param("id", ParseUUIDPipe) id: string,
    @Param("invoiceId", ParseUUIDPipe) invoiceId: string,
  ): Promise<Transaction[]> {
    return this.service.listInvoiceItems(id, invoiceId);
  }

  @Post(":id/invoices/:invoiceId/pay")
  @ApiOperation({ summary: "Paga a fatura debitando de uma conta (operação atômica)" })
  payInvoice(
    @Param("id", ParseUUIDPipe) id: string,
    @Param("invoiceId", ParseUUIDPipe) invoiceId: string,
    @Body(new ZodValidationPipe(payInvoiceSchema)) input: PayInvoiceInput,
  ): Promise<Invoice> {
    return this.service.payInvoice(id, invoiceId, input);
  }
}
