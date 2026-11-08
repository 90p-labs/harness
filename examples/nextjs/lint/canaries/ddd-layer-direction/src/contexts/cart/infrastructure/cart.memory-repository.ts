export class CartMemoryRepository {
  private readonly items: string[] = [];

  add(item: string): void {
    this.items.push(item);
  }
}
