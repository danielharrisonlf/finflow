import {
  type Account,
  type CreateAccountInput,
  createAccountSchema,
  type UpdateAccountInput,
  updateAccountSchema,
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
import { AccountsService } from "./accounts.service";

@ApiTags("accounts")
@Controller("accounts")
export class AccountsController {
  constructor(private readonly service: AccountsService) {}

  @Get()
  @ApiOperation({ summary: "Lista contas com saldo atual" })
  @ApiQuery({ name: "includeArchived", required: false, type: Boolean })
  list(
    @Query("includeArchived", new ParseBoolPipe({ optional: true }))
    includeArchived?: boolean,
  ): Promise<Account[]> {
    return this.service.list(includeArchived ?? false);
  }

  @Get(":id")
  @ApiOperation({ summary: "Detalha uma conta" })
  findById(@Param("id", ParseUUIDPipe) id: string): Promise<Account> {
    return this.service.findById(id);
  }

  @Post()
  @ApiOperation({ summary: "Cria uma conta" })
  create(
    @Body(new ZodValidationPipe(createAccountSchema)) input: CreateAccountInput,
  ): Promise<Account> {
    return this.service.create(input);
  }

  @Patch(":id")
  @ApiOperation({ summary: "Atualiza ou arquiva uma conta" })
  update(
    @Param("id", ParseUUIDPipe) id: string,
    @Body(new ZodValidationPipe(updateAccountSchema)) input: UpdateAccountInput,
  ): Promise<Account> {
    return this.service.update(id, input);
  }

  @Delete(":id")
  @HttpCode(204)
  @ApiOperation({ summary: "Exclui conta sem movimentações" })
  async delete(@Param("id", ParseUUIDPipe) id: string): Promise<void> {
    await this.service.delete(id);
  }
}
