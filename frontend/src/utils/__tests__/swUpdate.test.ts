import { beforeEach, describe, expect, it, vi } from 'vitest';

// The module keeps the pending state for the lifetime of the page, so each
// test needs its own copy of it.
async function freshModule() {
  vi.resetModules();
  return await import('../swUpdate');
}

describe('swUpdate', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it('starts with no update pending', async () => {
    const sw = await freshModule();
    expect(sw.isUpdatePending()).toBe(false);
  });

  it('tells subscribers when a build is waiting', async () => {
    const sw = await freshModule();
    const listener = vi.fn();
    sw.subscribeUpdatePending(listener);

    expect(listener).not.toHaveBeenCalled();
    sw.markUpdatePending();

    expect(listener).toHaveBeenCalledWith(true);
    expect(sw.isUpdatePending()).toBe(true);
  });

  it('reports a build that was already waiting on subscribing', async () => {
    const sw = await freshModule();
    sw.markUpdatePending();

    const listener = vi.fn();
    sw.subscribeUpdatePending(listener);

    expect(listener).toHaveBeenCalledWith(true);
  });

  it('announces a waiting build once, however often it is reported', async () => {
    const sw = await freshModule();
    const listener = vi.fn();
    sw.subscribeUpdatePending(listener);

    sw.markUpdatePending();
    sw.markUpdatePending();

    expect(listener).toHaveBeenCalledTimes(1);
  });

  it('stops telling a subscriber that has unsubscribed', async () => {
    const sw = await freshModule();
    const listener = vi.fn();
    sw.subscribeUpdatePending(listener)();

    sw.markUpdatePending();

    expect(listener).not.toHaveBeenCalled();
  });

  it('applies the update through the applier the entry point installed', async () => {
    const sw = await freshModule();
    const applier = vi.fn();
    sw.setUpdateApplier(applier);

    sw.applyPendingUpdate();

    expect(applier).toHaveBeenCalledTimes(1);
  });
});
