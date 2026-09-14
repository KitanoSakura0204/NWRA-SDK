/**
 * Bounded set of recently seen event ids (LRU eviction) used for idempotency.
 *
 * `Map` keeps insertion order, so deleting and re-inserting a key moves it to the
 * "most recent" end and the first key is always the least recently used one. The
 * user center retries a failed delivery for roughly nine hours, so a few thousand
 * ids are enough for one endpoint. Use a shared store (Redis SET NX EX, a unique
 * DB column, ...) when several receiver instances share one URL.
 */
export class LruSet {
  private readonly map = new Map<string, number>();

  constructor(private readonly capacity: number) {
    if (!Number.isFinite(capacity) || capacity < 1) this.capacity = 1;
  }

  /** True when `id` is already known; refreshes its position. */
  has(id: string): boolean {
    if (!this.map.has(id)) return false;
    this.map.delete(id);
    this.map.set(id, Date.now());
    return true;
  }

  add(id: string): void {
    this.map.delete(id);
    this.map.set(id, Date.now());
    while (this.map.size > this.capacity) {
      const oldest = this.map.keys().next();
      if (oldest.done) break;
      this.map.delete(oldest.value);
    }
  }

  get size(): number {
    return this.map.size;
  }
}
