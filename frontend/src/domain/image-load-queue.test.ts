import { ImageLoadQueue } from "./image-load-queue.js";

const queue = new ImageLoadQueue(2);
let active = 0;
let maxActive = 0;
const starts: number[] = [];
const releases: Array<() => void> = [];

function enqueueTask(id: number) {
  return queue.enqueue(() => {
    active += 1;
    maxActive = Math.max(maxActive, active);
    starts.push(id);
    return new Promise<number>((resolve) => {
      releases.push(() => {
        active -= 1;
        resolve(id);
      });
    });
  });
}

const task1 = enqueueTask(1);
const task2 = enqueueTask(2);
const task3 = enqueueTask(3);
await tick();
assertEqual(maxActive, 2);
assertEqual(starts.join(","), "1,2");

releases.shift()?.();
assertEqual(await task1, 1);
await tick();
assertEqual(starts.join(","), "1,2,3");
assertEqual(maxActive, 2);

releases.shift()?.();
releases.shift()?.();
assertEqual(await task2, 2);
assertEqual(await task3, 3);

const holdingQueue = new ImageLoadQueue(1);
let releaseFirstTask: (() => void) | undefined;
const heldTask = holdingQueue.enqueue(() => new Promise<void>((resolve) => {
  releaseFirstTask = resolve;
}));
let secondTaskStarted = false;
const waitingTask = holdingQueue.enqueue(() => {
  secondTaskStarted = true;
  return Promise.resolve();
});
await tick();
assertEqual(holdingQueue.getActiveCount(), 1);
assertEqual(secondTaskStarted, false);
releaseFirstTask?.();
await heldTask;
await waitingTask;
assertEqual(secondTaskStarted, true);
await tick();
assertEqual(holdingQueue.getActiveCount(), 0);

const cancelQueue = new ImageLoadQueue(1);
const blocker = cancelQueue.enqueue(() => new Promise<number>(() => {}));
const cancellable = cancelQueue.enqueue(() => Promise.resolve(4));
cancellable.cancel();
try {
  await cancellable;
  throw new Error("expected cancelled task to reject");
} catch (error) {
  assertEqual(error instanceof DOMException ? error.name : String(error), "AbortError");
}
blocker.cancel();

const cachedQueue = new ImageLoadQueue(2);
assertEqual(cachedQueue.hasLoaded("/api/store-space/channel-snapshots/one.jpg"), false);
cachedQueue.rememberLoaded("/api/store-space/channel-snapshots/one.jpg");
assertEqual(cachedQueue.hasLoaded("/api/store-space/channel-snapshots/one.jpg"), true);
assertEqual(cachedQueue.hasLoaded("/api/store-space/channel-snapshots/two.jpg"), false);

console.log("image-load-queue tests passed");

function tick() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function assertEqual(actual: unknown, expected: unknown) {
  if (actual !== expected) {
    throw new Error(`expected ${expected}, got ${actual}`);
  }
}
