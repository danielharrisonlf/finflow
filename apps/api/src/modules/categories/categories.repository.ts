import { Injectable } from "@nestjs/common";
import { PrismaService } from "../../common/prisma.service";
import type { Category, CategoryKind, Prisma } from "../../generated/prisma/client";

@Injectable()
export class CategoriesRepository {
  constructor(private readonly prisma: PrismaService) {}

  findAll(kind?: CategoryKind): Promise<Category[]> {
    return this.prisma.category.findMany({
      where: kind ? { kind } : undefined,
      orderBy: [{ kind: "asc" }, { name: "asc" }],
    });
  }

  findById(id: string): Promise<Category | null> {
    return this.prisma.category.findUnique({ where: { id } });
  }

  findByName(name: string, kind: CategoryKind): Promise<Category | null> {
    return this.prisma.category.findFirst({
      where: { kind, name: { equals: name, mode: "insensitive" } },
    });
  }

  create(data: Prisma.CategoryUncheckedCreateInput): Promise<Category> {
    return this.prisma.category.create({ data });
  }

  update(id: string, data: Prisma.CategoryUncheckedUpdateInput): Promise<Category> {
    return this.prisma.category.update({ where: { id }, data });
  }

  delete(id: string): Promise<Category> {
    return this.prisma.category.delete({ where: { id } });
  }
}
