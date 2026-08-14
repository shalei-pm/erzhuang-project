type QueueTask<T> = {
  run: () => Promise<T>;
  resolve: (value: T) => void;
  reject: (reason?: unknown) => void;
  cancelled: boolean;
};

export type QueuedImageLoad<T> = Promise<T> & {
  cancel: () => void;
};

export class ImageLoadQueue {
  private active = 0;
  private readonly pending: Array<QueueTask<unknown>> = [];
  private readonly loadedKeys = new Set<string>();

  constructor(private readonly concurrency: number) {}

  enqueue<T>(run: () => Promise<T>): QueuedImageLoad<T> {
    const task = {
      run,
      resolve: (_value: T) => {},
      reject: (_reason?: unknown) => {},
      cancelled: false,
    };
    const promise = new Promise<T>((resolve, reject) => {
      task.resolve = resolve;
      task.reject = reject;
    }) as QueuedImageLoad<T>;

    promise.cancel = () => {
      if (task.cancelled) return;
      task.cancelled = true;
      const index = this.pending.indexOf(task as QueueTask<unknown>);
      if (index >= 0) {
        this.pending.splice(index, 1);
        task.reject(new DOMException("Image load cancelled", "AbortError"));
      }
    };

    this.pending.push(task as QueueTask<unknown>);
    this.drain();
    return promise;
  }

  getActiveCount() {
    return this.active;
  }

  hasLoaded(key: string) {
    return this.loadedKeys.has(key);
  }

  rememberLoaded(key: string) {
    this.loadedKeys.add(key);
  }

  private drain() {
    while (this.active < Math.max(1, this.concurrency) && this.pending.length > 0) {
      const task = this.pending.shift();
      if (!task || task.cancelled) continue;
      this.active += 1;
      void task.run()
        .then(task.resolve)
        .catch(task.reject)
        .finally(() => {
          this.active -= 1;
          this.drain();
        });
    }
  }
}
