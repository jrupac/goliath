/*
 * Whether the page is running a build the server has since replaced, and how
 * to move it onto the new one.
 *
 * The service worker registration belongs to the entry point, which is the
 * only place that can talk to the worker at all. This module is the seam
 * between that and the UI, so components -- and their tests -- do not depend
 * on a worker being registered.
 */

type Listener = (pending: boolean) => void;

const listeners: Set<Listener> = new Set();
let updatePending = false;
let applyUpdate: () => void = () => window.location.reload();

// Records that a newer build is installed and waiting. Idempotent: a worker
// can report itself waiting more than once.
export function markUpdatePending(): void {
  if (updatePending) {
    return;
  }
  updatePending = true;
  listeners.forEach((listener: Listener): void => listener(true));
}

// Hands this module the means of switching the page onto the waiting build.
// Until it is called, asking for the update falls back to a plain reload,
// which is the right answer where no worker is in play.
export function setUpdateApplier(applier: () => void): void {
  applyUpdate = applier;
}

export function applyPendingUpdate(): void {
  applyUpdate();
}

export function isUpdatePending(): boolean {
  return updatePending;
}

// Subscribes to the pending state, returning the unsubscribe. A worker that
// is already waiting by the time the UI mounts is reported on subscribing,
// since there is no later edge to catch.
export function subscribeUpdatePending(listener: Listener): () => void {
  listeners.add(listener);
  if (updatePending) {
    listener(true);
  }
  return (): void => {
    listeners.delete(listener);
  };
}
