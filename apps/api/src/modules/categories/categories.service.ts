import type {
  Category,
  CategoryKind,
  CreateCategoryInput,
  UpdateCategoryInput,
} from "@finflow/contracts";
import {
  BadRequestException,
  ConflictException,
  Injectable,
  NotFoundException,
} from "@nestjs/common";
import type { Category as DbCategory } from "../../generated/prisma/client";
import { CategoriesRepository } from "./categories.repository";

function toContract(db: DbCategory): Category {
  return {
    id: db.id,
    name: db.name,
    kind: db.kind,
    icon: db.icon,
    color: db.color,
    parentId: db.parentId,
  };
}

@Injectable()
export class CategoriesService {
  constructor(private readonly repository: CategoriesRepository) {}

  async list(kind?: CategoryKind): Promise<Category[]> {
    const categories = await this.repository.findAll(kind);
    return categories.map(toContract);
  }

  async findByNameOrNull(name: string, kind: CategoryKind): Promise<Category | null> {
    const found = await this.repository.findByName(name, kind);
    return found ? toContract(found) : null;
  }

  async create(input: CreateCategoryInput): Promise<Category> {
    const duplicate = await this.repository.findByName(input.name, input.kind);
    if (duplicate) {
      throw new ConflictException(`categoria "${input.name}" (${input.kind}) já existe`);
    }
    if (input.parentId) {
      await this.validateParent(input.parentId, input.kind);
    }
    const created = await this.repository.create({
      name: input.name,
      kind: input.kind,
      icon: input.icon,
      color: input.color,
      parentId: input.parentId ?? null,
    });
    return toContract(created);
  }

  async update(id: string, input: UpdateCategoryInput): Promise<Category> {
    const existing = await this.ensureExists(id);
    if (input.parentId === id) {
      throw new BadRequestException("uma categoria não pode ser subcategoria dela mesma");
    }
    const targetKind = input.kind ?? existing.kind;
    if (input.parentId) {
      await this.validateParent(input.parentId, targetKind);
    }
    const updated = await this.repository.update(id, {
      name: input.name,
      kind: input.kind,
      icon: input.icon,
      color: input.color,
      parentId: input.parentId,
    });
    return toContract(updated);
  }

  async delete(id: string): Promise<void> {
    await this.ensureExists(id);
    await this.repository.delete(id);
  }

  private async ensureExists(id: string): Promise<DbCategory> {
    const found = await this.repository.findById(id);
    if (!found) throw new NotFoundException("categoria não encontrada");
    return found;
  }

  private async validateParent(parentId: string, kind: CategoryKind): Promise<void> {
    const parent = await this.repository.findById(parentId);
    if (!parent) throw new NotFoundException("categoria pai não encontrada");
    if (parent.kind !== kind) {
      throw new BadRequestException(
        "subcategoria deve ter o mesmo tipo (receita/despesa) da categoria pai",
      );
    }
  }
}
