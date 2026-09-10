import Greader from '../greader';
import { LoginInfo } from '../interface';
import { MarkState, SelectionKey } from '../../utils/types';
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

describe('post token refresh', () => {
  let greader: Greader;
  let mockFetch: Mock;

  const badTokenResponse = () => ({
    ok: false,
    status: 401,
    statusText: 'Unauthorized',
    headers: new Headers({ 'X-Reader-Google-Bad-Token': 'true' }),
  });
  const okResponse = () => ({
    ok: true,
    status: 200,
    headers: new Headers(),
    text: vi.fn().mockResolvedValue('{}'),
  });

  beforeEach(() => {
    greader = new Greader();
    mockFetch = vi.fn();
    global.fetch = mockFetch;
  });

  // A post token obtained when the page loaded expires while the page is still
  // open, and nothing prompts the page to notice except a request being
  // refused. Before this, everything the reader did after that point failed.
  it('fetches a new post token and repeats a rejected request', async () => {
    mockFetch
      .mockResolvedValueOnce(badTokenResponse())
      .mockResolvedValueOnce({
        ...okResponse(),
        text: vi.fn().mockResolvedValue('fresh-token'),
      })
      .mockResolvedValueOnce(okResponse());

    const res = await greader.MarkArticle(MarkState.Read, [
      '1',
    ] as unknown as SelectionKey);

    expect(res.ok).toBe(true);
    expect(mockFetch).toHaveBeenCalledTimes(3);
    expect(mockFetch.mock.calls[1][0]).toBe('/greader/reader/api/0/token');

    // The repeated request must carry the token just obtained, not the stale one.
    const retried = mockFetch.mock.calls[2][1].body as FormData;
    expect(retried.get('T')).toBe('fresh-token');
  });

  // If the session itself has ended, a new token cannot be had, and retrying
  // forever would only hide that.
  it('gives up when the session can no longer produce a token', async () => {
    mockFetch.mockResolvedValueOnce(badTokenResponse()).mockResolvedValueOnce({
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      headers: new Headers(),
    });

    const res = await greader.MarkArticle(MarkState.Read, [
      '1',
    ] as unknown as SelectionKey);

    expect(res.status).toBe(401);
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  // A failure that is not about the post token must not trigger a refresh.
  it('does not refresh on an unrelated failure', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      headers: new Headers(),
    });

    await greader.MarkArticle(MarkState.Read, ['1'] as unknown as SelectionKey);

    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});

describe('feed management', () => {
  let greader: Greader;
  let mockFetch: Mock;

  const okText = (body: string) => ({
    ok: true,
    status: 200,
    headers: new Headers(),
    text: vi.fn().mockResolvedValue(body),
  });

  beforeEach(() => {
    greader = new Greader();
    mockFetch = vi.fn();
    global.fetch = mockFetch;
  });

  it('AddFeed sends the address and reads back the new feed', async () => {
    mockFetch.mockResolvedValueOnce(
      okText(
        '{"query":"https://example.com/feed","numResults":1,' +
          '"streamId":"feed/385100064196591617","streamName":"Example"}'
      )
    );

    const added = await greader.AddFeed('https://example.com/feed');

    expect(added).toEqual({ id: '385100064196591617', title: 'Example' });
    expect(mockFetch.mock.calls[0][0]).toBe(
      '/greader/reader/api/0/subscription/quickadd'
    );
    const form = mockFetch.mock.calls[0][1].body as FormData;
    expect(form.get('quickadd')).toBe('https://example.com/feed');
  });

  // The server answers a bad address with a client error. That is the user's
  // mistake to fix, so it is reported as such rather than as a server fault.
  it('AddFeed explains a rejected address', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      headers: new Headers(),
    });

    await expect(greader.AddFeed('https://example.com/')).rejects.toThrow(
      /not a feed/
    );
  });

  it('RenameFeed sends an edit with the new title', async () => {
    mockFetch.mockResolvedValueOnce(okText('OK'));

    await greader.RenameFeed('42', 'New name');

    expect(mockFetch.mock.calls[0][0]).toBe(
      '/greader/reader/api/0/subscription/edit'
    );
    const form = mockFetch.mock.calls[0][1].body as FormData;
    expect(form.get('ac')).toBe('edit');
    expect(form.get('s')).toBe('feed/42');
    expect(form.get('t')).toBe('New name');
  });

  it('MoveFeed names the destination and, when known, the source', async () => {
    mockFetch.mockResolvedValueOnce(okText('OK'));
    mockFetch.mockResolvedValueOnce(okText('OK'));

    await greader.MoveFeed('42', '7', '3');
    await greader.MoveFeed('42', '7');

    const withSource = mockFetch.mock.calls[0][1].body as FormData;
    expect(withSource.get('a')).toBe('user/-/label/7');
    expect(withSource.get('r')).toBe('user/-/label/3');

    const withoutSource = mockFetch.mock.calls[1][1].body as FormData;
    expect(withoutSource.get('a')).toBe('user/-/label/7');
    expect(withoutSource.has('r')).toBe(false);
  });

  it('UnsubscribeFeed sends the unsubscribe action', async () => {
    mockFetch.mockResolvedValueOnce(okText('OK'));

    await greader.UnsubscribeFeed('42');

    const form = mockFetch.mock.calls[0][1].body as FormData;
    expect(form.get('ac')).toBe('unsubscribe');
    expect(form.get('s')).toBe('feed/42');
  });

  it('edit failures are reported as errors', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
      statusText: 'Not Found',
      headers: new Headers(),
    });

    await expect(greader.RenameFeed('42', 'x')).rejects.toThrow(/Not Found/);
  });
});
