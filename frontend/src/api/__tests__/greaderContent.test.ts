import Greader from '../greader';
import { ContentTreeCls } from '../../models/contentTree';
import { ArticleView } from '../../models/article';
import { KeyAllItems, MarkState, SelectionType } from '../../utils/types';
import type { Mock } from 'vitest';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// A stand-in for the server's side of loading: one folder, one feed, and
// articles whose state a test changes between loads, as another client or the
// fetcher would.
type ServerArticle = { read: boolean; saved: boolean };

const feedId = '7';
const folderId = '3';

class FakeServer {
  articles = new Map<bigint, ServerArticle>();
  // The hex IDs each contents request asked for, in order.
  contentRequests: string[][] = [];
  // When set, the unread ID list is held back until this resolves.
  holdUnreadIds: Promise<void> | null = null;

  add(id: bigint, read = false, saved = false): void {
    this.articles.set(id, { read, saved });
  }

  respond = async (uri: string, init?: RequestInit): Promise<Response> => {
    const form = init?.body as FormData | undefined;
    switch (uri) {
      case '/greader/reader/api/0/subscription/list':
        return this.json({
          subscriptions: [
            {
              id: `feed/${feedId}`,
              title: 'Feed',
              htmlUrl: 'https://example.com/',
              iconUrl: '',
              categories: [{ id: `user/-/label/${folderId}`, label: 'Folder' }],
            },
          ],
        });
      case '/greader/reader/api/0/stream/items/ids': {
        const starred = form?.get('s') === 'user/-/state/com.google/starred';
        if (!starred && this.holdUnreadIds) {
          await this.holdUnreadIds;
        }
        const refs = [...this.articles]
          .filter(([, a]) => (starred ? a.saved : !a.read))
          .map(([id]) => ({
            id: id.toString(10),
            directStreamIds: [`feed/${feedId}`, `user/-/label/${folderId}`],
            timestampUsec: '0',
          }));
        return this.json(refs.length ? { itemRefs: refs } : {});
      }
      case '/greader/reader/api/0/stream/items/contents': {
        const ids = (form?.getAll('i') ?? []) as string[];
        this.contentRequests.push(ids);
        return this.json({
          id: 'user/-/state/com.google/reading-list',
          updated: 0,
          items: ids
            .filter((hex) => this.articles.has(BigInt('0x' + hex)))
            .map((hex) => ({
              id: `tag:google.com,2005:reader/item/${hex}`,
              title: `Article ${hex}`,
              published: Number(BigInt('0x' + hex)),
              categories: [
                'user/-/state/com.google/reading-list',
                `feed/${feedId}`,
                `user/-/label/${folderId}`,
              ],
              canonical: [{ href: `https://example.com/${hex}` }],
              summary: { content: `<p>${hex}</p>` },
            })),
        });
      }
      default:
        throw new Error(`unexpected request to ${uri}`);
    }
  };

  private json(body: unknown): Response {
    return {
      ok: true,
      status: 200,
      headers: new Headers(),
      text: async () => JSON.stringify(body),
    } as unknown as Response;
  }
}

// Every article a tree holds, by hex ID, with its state.
function state(tree: ContentTreeCls): Map<string, [boolean, boolean]> {
  return new Map(
    tree
      .GetArticleView(KeyAllItems, SelectionType.All)
      .map((v: ArticleView) => [v.id, [v.isRead, v.isSaved]])
  );
}

const noop = (): void => {};

describe('loading content', () => {
  let greader: Greader;
  let server: FakeServer;
  let mockFetch: Mock;

  beforeEach(() => {
    greader = new Greader();
    server = new FakeServer();
    mockFetch = vi.fn(server.respond);
    global.fetch = mockFetch;
  });

  it('fetches the contents of every unread and saved article at first', async () => {
    server.add(0x10n);
    server.add(0x11n, true, true);
    server.add(0x12n, true);

    const tree = await greader.InitializeContent(noop);

    expect(server.contentRequests.flat().sort()).toEqual(['10', '11']);
    expect(state(tree)).toEqual(
      new Map([
        ['10', [false, false]],
        ['11', [true, true]],
      ])
    );
  });

  // Contents are nearly all of what a load transfers and never change, so a
  // reload that finds nothing new should transfer none.
  it('fetches no contents again when nothing has changed', async () => {
    server.add(0x10n);
    server.add(0x11n, false, true);
    await greader.InitializeContent(noop);
    server.contentRequests = [];

    const tree = await greader.InitializeContent(noop);

    expect(server.contentRequests).toEqual([]);
    expect(state(tree)).toEqual(
      new Map([
        ['10', [false, false]],
        ['11', [false, true]],
      ])
    );
  });

  it('fetches only what is new, and takes state changed elsewhere', async () => {
    server.add(0x10n);
    server.add(0x11n);
    server.add(0x12n);
    await greader.InitializeContent(noop);
    server.contentRequests = [];

    server.articles.get(0x10n)!.read = true; // read elsewhere
    server.articles.get(0x11n)!.saved = true; // saved elsewhere
    server.add(0x13n); // fetched since

    const tree = await greader.InitializeContent(noop);

    expect(server.contentRequests).toEqual([['13']]);
    expect(state(tree)).toEqual(
      new Map([
        ['11', [false, true]],
        ['12', [false, false]],
        ['13', [false, false]],
      ])
    );
  });

  // The reader keeps using the tree on screen while a reload is under way. A
  // mark made then is newer than the server's answer, which was read before
  // the mark reached it.
  it('keeps a mark made while the load was in progress', async () => {
    server.add(0x10n);
    server.add(0x11n);
    const before = await greader.InitializeContent(noop);

    let release!: () => void;
    server.holdUnreadIds = new Promise((r) => (release = r));
    const reloading = greader.InitializeContent(noop);
    await vi.waitFor(() =>
      expect(
        mockFetch.mock.calls.some(
          (c) => c[0] === '/greader/reader/api/0/stream/items/ids'
        )
      ).toBe(true)
    );

    before.Mark(
      MarkState.Read,
      ['10', feedId, folderId],
      SelectionType.Article
    );
    release();
    const after = await reloading;

    expect(state(after).get('10')).toEqual([true, false]);
    expect(state(after).get('11')).toEqual([false, false]);
  });

  // The tree a reload replaces is still the one on screen until the new one
  // is handed over, and its counts are kept per feed and folder.
  it('leaves the previous tree as it was', async () => {
    server.add(0x10n);
    server.add(0x11n);
    const before = await greader.InitializeContent(noop);

    server.articles.get(0x10n)!.read = true;
    await greader.InitializeContent(noop);

    expect(state(before).get('10')).toEqual([false, false]);
    expect(before.UnreadCount()).toBe(2);
  });

  it('keeps what it held when a load fails partway', async () => {
    server.add(0x10n);
    await greader.InitializeContent(noop);

    server.add(0x11n);
    mockFetch.mockImplementation(async (uri: string, init?: RequestInit) =>
      uri === '/greader/reader/api/0/stream/items/contents'
        ? ({
            ok: false,
            status: 500,
            statusText: 'boom',
            headers: new Headers(),
          } as Response)
        : server.respond(uri, init)
    );
    await expect(greader.InitializeContent(noop)).rejects.toBe('boom');

    mockFetch.mockImplementation(server.respond);
    server.contentRequests = [];
    await greader.InitializeContent(noop);

    expect(server.contentRequests).toEqual([['11']]);
  });
});
