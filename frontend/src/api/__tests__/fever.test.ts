import Fever from '../fever';
import {LoginInfo} from '../interface';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import * as helpers from '../../utils/helpers';

describe('Fever', () => {
  it('initializes', () => {
    // Minimal test to check if the class can be initialized
    new Fever();
  });
});

describe('Fever', () => {
  let fever: Fever;
  let mockFetch: vi.Mock;
  let mockCookieExists: vi.SpyOn;
  const loginInfo: LoginInfo = {username: 'test_user', password: 'password'};

  beforeEach(() => {
    fever = new Fever();
    mockFetch = vi.fn();
    global.fetch = mockFetch;
    mockCookieExists = vi.spyOn(helpers, 'cookieExists');
  });

  it('HandleAuth returns true on successful login', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({})
    });
    mockCookieExists.mockReturnValue(true);

    const result = await fever.HandleAuth(loginInfo);

    expect(result).toBe(true);
    expect(mockFetch).toHaveBeenCalledWith('/auth', {
      method: 'POST',
      body: JSON.stringify(loginInfo),
      credentials: 'include',
    });
  });

  it('HandleAuth returns false on failed login', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      json: vi.fn().mockResolvedValue({})
    });

    const result = await fever.HandleAuth(loginInfo);

    expect(result).toBe(false);
  });

  it('HandleAuth returns false when the auth cookie is missing', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({})
    });
    mockCookieExists.mockReturnValue(false);

    const result = await fever.HandleAuth(loginInfo);

    expect(result).toBe(false);
  });
});