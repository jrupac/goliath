import Greader from '../greader';
import { LoginInfo } from '../interface';
import type { Mock, MockInstance } from 'vitest';
import { beforeEach, describe, expect, it, vi } from 'vitest';

describe('Greader', () => {
  let greader: Greader;
  let mockFetch: Mock;
  let mockSetCookie: MockInstance;
  const loginInfo: LoginInfo = { username: 'test_user', password: 'password' };

  beforeEach(() => {
    greader = new Greader();
    mockFetch = vi.fn();
    global.fetch = mockFetch;
    mockSetCookie = vi.spyOn(document, 'cookie', 'set');
  });

  it('initializes', () => {
    new Greader();
  });

  it('HandleAuth returns true on successful login', async () => {
    mockFetch.mockResolvedValueOnce({ ok: true });

    const result = await greader.HandleAuth(loginInfo);

    expect(result).toBe(true);
    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      '/auth',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
        body: JSON.stringify({ username: 'test_user', password: 'password' }),
      })
    );
  });

  it('HandleAuth returns false on failed login', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, statusText: 'Unauthorized' });

    const result = await greader.HandleAuth(loginInfo);

    expect(result).toBe(false);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  // The session is held in a cookie the server sets and marks HttpOnly, so a
  // script that gets into the page has nothing durable to steal. Logging in
  // must never put a credential where JavaScript can reach it.
  it('HandleAuth never writes a credential into document.cookie', async () => {
    mockFetch.mockResolvedValueOnce({ ok: true });

    await greader.HandleAuth(loginInfo);

    expect(mockSetCookie).not.toHaveBeenCalled();
  });
});
