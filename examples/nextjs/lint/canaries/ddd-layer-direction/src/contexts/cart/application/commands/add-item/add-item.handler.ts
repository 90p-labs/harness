import { CartMemoryRepository } from "../../../infrastructure/cart.memory-repository";

export const addItem = (repository: CartMemoryRepository, item: string): void => {
  repository.add(item);
};
