import Greader from '../greader';
import {LoginInfo} from '../interface';
import {beforeEach, describe, expect, it, vi} from 'vitest';

describe('Greader', () => {
  let greader: Greader;
  let mockFetch: vi.Mock;
  let mockSetCookie: vi.SpyOn;
  const loginInfo: LoginInfo = {username: 'test_user', password: 'password'};

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
    // Set the auth token that should be set in the cookie.
    const authSuccess = {
      SID: '',
      LSID: '',
      Auth: 'token',
    };

    mockFetch.mockResolvedValueOnce({
      ok: true,
      text: vi.fn().mockResolvedValue(JSON.stringify(authSuccess)),
    });

    const result = await greader.HandleAuth(loginInfo);

    expect(result).toBe(true);
    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      '/greader/accounts/ClientLogin',
      expect.objectContaining({
        method: 'POST',
        body: expect.any(FormData),
      }),
    );

    const formData = mockFetch.mock.calls[0][1].body as FormData;
    expect(formData.get('Email')).toBe('test_user');
    expect(formData.get('Passwd')).toBe('password');

    expect(mockSetCookie).toHaveBeenCalledWith('goliath_token=token');
  });

  it('HandleAuth returns false on failed login to client login', async () => {
    // Set the response from /greader/accounts/ClientLogin
    mockFetch.mockResolvedValueOnce({
      ok: false,
      text: vi.fn().mockResolvedValue(''),
    });

    const result = await greader.HandleAuth(loginInfo);

    expect(result).toBe(false);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch).toHaveBeenCalledWith(
      '/greader/accounts/ClientLogin',
      expect.objectContaining({
        method: 'POST',
        body: expect.any(FormData),
      }),
    );

    const formData = mockFetch.mock.calls[0][1].body as FormData;
    expect(formData.get('Email')).toBe('test_user');
    expect(formData.get('Passwd')).toBe('password');

    expect(mockSetCookie).not.toHaveBeenCalled();
  });

  it('HandleAuth returns false on failed parsing of response', async () => {
    // Set the response from /greader/accounts/ClientLogin
    mockFetch.mockResolvedValueOnce({
      ok: true,
      text: vi.fn().mockResolvedValue('invalid_json'),
    });

    const result = await greader.HandleAuth(loginInfo);

    expect(result).toBe(false);
    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      '/greader/accounts/ClientLogin',
      expect.objectContaining({
        method: 'POST',
        body: expect.any(FormData),
      }),
    );

    const formData = mockFetch.mock.calls[0][1].body as FormData;
    expect(formData.get('Email')).toBe('test_user');
    expect(formData.get('Passwd')).toBe('password');

    expect(mockSetCookie).not.toHaveBeenCalled();
  });
});