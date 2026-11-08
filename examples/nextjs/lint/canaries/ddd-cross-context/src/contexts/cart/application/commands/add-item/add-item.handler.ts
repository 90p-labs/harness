import type { Product } from "../../../../catalog/domain/product.aggregate";

export const addItem = (product: Product): string => product.id;
