import { AddedFeed, FetchAPI, FolderSummary, LoginInfo } from './interface';
import {
  ArticleSelection,
  FeedSelection,
  FolderSelection,
  KeyUnread,
  KeySaved,
  MarkState,
  SelectionKey,
  Status,
} from '../utils/types';
import { parseJson } from '../utils/helpers';
import { ContentTreeCls } from '../models/contentTree';
import { FolderCls, FolderId } from '../models/folder';
import { FaviconCls, FeedCls, FeedId } from '../models/feed';
import { ArticleCls, ReadStatus, SavedStatus } from '../models/article';
import {
  GReaderItemContent,
  GReaderItemRef,
  GReaderQuickAddResponse,
  GReaderStream,
  GReaderStreamContents,
  GReaderStreamIds,
  GReaderSubscription,
  GReaderSubscriptionList,
  GReaderTag,
  GReaderTagList,
  GReaderURI,
  GoliathURI,
} from './greaderTypes';

interface GReaderFetch {
  uri: string;
  init?: RequestInit;
  formData?: FormData;
  omitPostToken?: boolean;
}

export default class GReader implements FetchAPI {
  // Set by the server when it refuses a post token, as opposed to refusing the
  // session. It is what separates "ask for another token" from "sign in again".
  private static badPostTokenHeader: string = 'X-Reader-Google-Bad-Token';

  private folderFeeds: Map<FolderId, FeedCls[]>;
  private folderMap: Map<FolderId, FolderCls>;
  private feedToArticles: Map<FeedId, ArticleCls[]>;
  private postToken: string;

  constructor() {
    this.folderFeeds = new Map<FolderId, FeedCls[]>();
    this.folderMap = new Map<FolderId, FolderCls>();
    this.feedToArticles = new Map<FeedId, ArticleCls[]>();
    this.postToken = '';
  }

  public async HandleAuth(loginInfo: LoginInfo): Promise<boolean> {
    // The session is established server-side and returned as a cookie the
    // page cannot read. Nothing long-lived is held in JavaScript, so a script
    // that gets into the page cannot walk away with a durable credential.
    const res: Response = await fetch(GoliathURI.Login, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: loginInfo.username,
        password: loginInfo.password,
      }),
    });

    if (!res.ok) {
      console.log('Login failed: ' + res.statusText);
      return false;
    }

    return true;
  }

  public async Logout(): Promise<void> {
    // No post token: this is Goliath's own route rather than a GReader one,
    // and it authorizes itself from the session cookie.
    const res: Response = await fetch(GoliathURI.Logout, {
      method: 'POST',
      credentials: 'include',
    });

    // Dropped either way, since a caller signing out is done with it whatever
    // the server made of the request.
    this.postToken = '';

    if (!res.ok) {
      throw new Error('Logout failed: ' + res.statusText);
    }
  }

  public async ResumeSession(): Promise<boolean> {
    // Asking for a post token is the check: it requires the session cookie, so
    // succeeding means the browser still has a usable session. The token it
    // returns is short-lived and scoped to that session, and is the one thing
    // this code does hold, since writes have to send it as a parameter.
    return this.refreshPostToken();
  }

  public async InitializeContent(
    cb: (s: Status) => void
  ): Promise<ContentTreeCls> {
    // Reset so that this can be called again to pick up a changed feed list.
    // Otherwise a second call would layer the new subscriptions and articles
    // over the old ones, and every feed would appear twice.
    this.folderFeeds = new Map<FolderId, FeedCls[]>();
    this.folderMap = new Map<FolderId, FolderCls>();
    this.feedToArticles = new Map<FeedId, ArticleCls[]>();

    await Promise.all([this.fetchSubscriptions(cb), this.fetchArticles(cb)]);

    return this.buildTree();
  }

  public async MarkArticle(
    mark: MarkState,
    entity: SelectionKey
  ): Promise<Response> {
    const greaderId: string = (entity as ArticleSelection)[0] as string;
    const formData = new FormData();

    if (mark === MarkState.Read) {
      // Add the "read" tag to mark as read.
      formData.set('a', GReaderTag.Read);
    } else if (mark === MarkState.Unread) {
      // Remove the "read" tag to mark as unread.
      formData.set('r', GReaderTag.Read);
    } else if (mark === MarkState.Saved) {
      // Add the "starred" tag to mark as saved.
      formData.set('a', GReaderTag.Starred);
    } else if (mark === MarkState.Unsaved) {
      // Remove the "starred" tag to mark as unsaved.
      formData.set('r', GReaderTag.Starred);
    } else {
      console.log('Unexpected mark state: ' + mark);
      return Promise.reject(new Error('Unexpected mark state: ' + mark));
    }
    formData.set('i', greaderId);

    return this.doFetch({
      uri: GReaderURI.EditTag,
      formData: formData,
    });
  }

  public async MarkFeed(_: MarkState, entity: SelectionKey): Promise<Response> {
    const greaderId: string = (entity as FeedSelection)[0];
    const formData = new FormData();
    formData.set('s', greaderId);

    return this.doFetch({
      uri: GReaderURI.MarkAllAsRead,
      formData: formData,
    });
  }

  public async MarkFolder(
    _: MarkState,
    entity: SelectionKey
  ): Promise<Response> {
    const greaderId: string = entity as FolderSelection;
    const formData = new FormData();
    formData.set('t', greaderId);

    return this.doFetch({
      uri: GReaderURI.MarkAllAsRead,
      formData: formData,
    });
  }

  public async MarkAll(_: MarkState, entity: SelectionKey): Promise<Response> {
    const formData = new FormData();
    if (entity === KeySaved) {
      // TODO: Support saved articles.
      return Promise.reject('Marking saved articles not yet supported!');
    } else if (entity === KeyUnread) {
      // Value 0 means "all folders" when marking.
      formData.set('t', '0' as string);
    } else {
      formData.set('t', entity as FolderSelection);
    }

    return this.doFetch({
      uri: GReaderURI.MarkAllAsRead,
      formData: formData,
    });
  }

  public async ParseFullArticle(articleId: string): Promise<string> {
    const formData = new FormData();
    formData.set('i', articleId);

    const res: Response = await this.doFetch({
      uri: GReaderURI.ParseFullArticle,
      formData: formData,
    });

    if (!res.ok) {
      console.log('Parsing full article failed: ' + res.statusText);
      return Promise.reject(new Error(res.statusText));
    }

    const result = await res.text();
    const responseJson = parseJson(result);
    return responseJson.content;
  }

  public async AddFeed(url: string): Promise<AddedFeed> {
    const formData = new FormData();
    formData.set('quickadd', url);

    const res: Response = await this.doFetch({
      uri: GReaderURI.SubscriptionQuickAdd,
      formData: formData,
    });

    if (!res.ok) {
      // The server refuses with a client error when the address itself is
      // the problem: it is not a feed, or nothing answered there. Anything
      // else is the server's failure, not the user's.
      if (res.status === 400) {
        throw new Error(
          'That address is not a feed, or it could not be fetched.'
        );
      }
      throw new Error('Adding the feed failed: ' + res.statusText);
    }

    const result: string = await res.text();
    const response: GReaderQuickAddResponse = parseJson(result);
    if (response.numResults < 1 || !response.streamId) {
      throw new Error('That address is not a feed.');
    }

    const id = this.parseFeedID(response.streamId);
    return {
      id,
      title: response.streamName,
      folderId: await this.folderOfFeed(id),
    };
  }

  // folderOfFeed finds which folder a feed is filed in, from the subscription
  // list, since an add does not say. Not finding out is no reason to report
  // the add as failed, so it is reported as not knowing.
  private async folderOfFeed(feedId: FeedId): Promise<FolderId | undefined> {
    try {
      const res: Response = await this.doFetch({
        uri: GReaderURI.SubscriptionList,
      });
      if (!res.ok) {
        return undefined;
      }
      const list: GReaderSubscriptionList = parseJson(await res.text());
      const sub = list.subscriptions?.find(
        (s) => s.id === this.feedStreamId(feedId)
      );
      const category = sub?.categories?.[0];
      return category ? this.parseFolderID(category.id) : undefined;
    } catch (err) {
      console.log(`Finding the folder of feed ${feedId} failed: ${err}`);
      return undefined;
    }
  }

  public async RenameFeed(feedId: FeedId, title: string): Promise<void> {
    const formData = new FormData();
    formData.set('ac', 'edit');
    formData.set('s', this.feedStreamId(feedId));
    formData.set('t', title);

    await this.editSubscription(formData, 'Renaming the feed');
  }

  public async MoveFeed(
    feedId: FeedId,
    toFolderId: FolderId,
    fromFolderId?: FolderId
  ): Promise<void> {
    const formData = new FormData();
    formData.set('ac', 'edit');
    formData.set('s', this.feedStreamId(feedId));
    formData.set('a', this.folderStreamId(toFolderId));
    if (fromFolderId !== undefined) {
      formData.set('r', this.folderStreamId(fromFolderId));
    }

    await this.editSubscription(formData, 'Moving the feed');
  }

  public async UnsubscribeFeed(feedId: FeedId): Promise<void> {
    const formData = new FormData();
    formData.set('ac', 'unsubscribe');
    formData.set('s', this.feedStreamId(feedId));

    await this.editSubscription(formData, 'Unsubscribing');
  }

  public async MoveFeedToNewFolder(
    feedId: FeedId,
    folderName: string,
    fromFolderId?: FolderId
  ): Promise<void> {
    const formData = new FormData();
    formData.set('ac', 'edit');
    formData.set('s', this.feedStreamId(feedId));
    // A name where a move would give an ID: the server files the feed under
    // the folder of that name, and makes the folder if there is none.
    formData.set('a', this.folderLabel(folderName));
    if (fromFolderId !== undefined) {
      formData.set('r', this.folderStreamId(fromFolderId));
    }

    await this.editSubscription(formData, 'Creating the folder');
  }

  public async ListFolders(): Promise<FolderSummary[]> {
    const res: Response = await this.doFetch({ uri: GReaderURI.TagList });

    if (!res.ok) {
      throw new Error('Listing folders failed: ' + res.statusText);
    }

    const result: string = await res.text();
    const tagList: GReaderTagList = parseJson(result);
    return (tagList.tags ?? [])
      .filter((tag) => tag.type === 'folder')
      .map((tag) => ({
        id: this.parseFolderID(tag.id),
        title: tag.label ?? '',
      }));
  }

  public async RenameFolder(folderId: FolderId, name: string): Promise<void> {
    const formData = new FormData();
    formData.set('s', this.folderStreamId(folderId));
    formData.set('dest', this.folderLabel(name));

    const res: Response = await this.doFetch({
      uri: GReaderURI.RenameTag,
      formData: formData,
    });

    if (!res.ok) {
      // Folder names are unique, and a clash is the one refusal the user can
      // act on without knowing anything more.
      if (res.status === 409) {
        throw new Error('Another folder already has that name.');
      }
      console.log('Renaming the folder failed: ' + res.statusText);
      throw new Error('Renaming the folder failed: ' + res.statusText);
    }
  }

  public async DeleteFolder(folderId: FolderId): Promise<void> {
    const formData = new FormData();
    formData.set('s', this.folderStreamId(folderId));

    const res: Response = await this.doFetch({
      uri: GReaderURI.DisableTag,
      formData: formData,
    });

    if (!res.ok) {
      console.log('Removing the folder failed: ' + res.statusText);
      throw new Error('Removing the folder failed: ' + res.statusText);
    }
  }

  private async editSubscription(
    formData: FormData,
    what: string
  ): Promise<void> {
    const res: Response = await this.doFetch({
      uri: GReaderURI.SubscriptionEdit,
      formData: formData,
    });

    if (!res.ok) {
      console.log(what + ' failed: ' + res.statusText);
      throw new Error(what + ' failed: ' + res.statusText);
    }
  }

  private async fetchSubscriptions(
    cb: (status: Status) => void
  ): Promise<void> {
    const res: Response = await this.doFetch({
      uri: GReaderURI.SubscriptionList,
    });

    if (!res.ok) {
      console.log('Fetching subscription list failed: %s' + res.statusText);
      return Promise.reject(res.statusText);
    }

    const result: string = await res.text();
    const subscriptionList: GReaderSubscriptionList = await parseJson(result);

    // Only populate if there are any subscriptions returned.
    if (subscriptionList.subscriptions) {
      this.populateFolderFeeds(subscriptionList.subscriptions);
    }

    cb(Status.Folder);
    cb(Status.Favicon);
    cb(Status.Feed);
  }

  private async fetchArticles(cb: (status: Status) => void): Promise<void> {
    const articleRefLimit = 1000;
    const articleContentLimit = 100;

    // Fetch unread items
    const unreadFormData = new FormData();
    unreadFormData.set('s', GReaderStream.ReadingList);
    unreadFormData.set('xt', GReaderTag.Read);
    unreadFormData.set('n', articleRefLimit.toString());

    const unreadArticleIdStrs = await this.fetchStreamIds(
      unreadFormData,
      articleRefLimit
    );

    // Fetch saved items
    const savedFormData = new FormData();
    savedFormData.set('s', GReaderStream.Starred);
    savedFormData.set('n', articleRefLimit.toString());

    const savedArticleIdStrs = await this.fetchStreamIds(
      savedFormData,
      articleRefLimit
    );

    const unreadSet = new Set(unreadArticleIdStrs.map(([id]) => id));
    const savedSet = new Set(savedArticleIdStrs.map(([id]) => id));

    // Combine into a single list of unique article IDs (maintaining metadata)
    const uniqueArticleMap = new Map<string, [string, string]>();
    unreadArticleIdStrs.forEach(([id, feedId, folderId]) => {
      uniqueArticleMap.set(id, [feedId, folderId]);
    });
    savedArticleIdStrs.forEach(([id, feedId, folderId]) => {
      uniqueArticleMap.set(id, [feedId, folderId]);
    });

    const articleIdStrs = Array.from(uniqueArticleMap.entries()).map(
      ([id, [feedId, folderId]]) => [id, feedId, folderId]
    );

    for (let i = 0; i < articleIdStrs.length; i += articleContentLimit) {
      const articleContentsForm = new FormData();
      articleIdStrs
        .slice(i, i + articleContentLimit)
        .forEach(([id, _feedId, _folderId]) =>
          articleContentsForm.append('i', id)
        );

      const res: Response = await this.doFetch({
        uri: GReaderURI.StreamItemContents,
        formData: articleContentsForm,
      });

      if (!res.ok) {
        console.log('Fetching item contents failed: %s' + res.statusText);
        return Promise.reject(res.statusText);
      }

      const result: string = await res.text();
      const streamItemContents: GReaderStreamContents = await parseJson(result);

      if (!streamItemContents.items) {
        return;
      }

      streamItemContents.items.forEach((item: GReaderItemContent) => {
        const hexId = this.parseArticleID(item.id);
        const isRead = unreadSet.has(hexId)
          ? ReadStatus.Unread
          : ReadStatus.Read;
        const isSaved = savedSet.has(hexId)
          ? SavedStatus.Saved
          : SavedStatus.Unsaved;

        const article = new ArticleCls(
          hexId,
          item.title,
          '',
          item.summary.content,
          item.canonical[0].href,
          isSaved,
          item.published,
          isRead
        );

        const feedId = this.parseFeedID(item.categories[1]);
        const articles = this.feedToArticles.get(feedId);
        if (!articles) {
          this.feedToArticles.set(feedId, [article]);
        } else {
          articles.push(article);
        }
      });
    }

    cb(Status.Article);
  }

  private async fetchStreamIds(
    formData: FormData,
    limit: number
  ): Promise<string[][]> {
    const articleIdStrs: string[][] = [];

    for (;;) {
      const res: Response = await this.doFetch({
        uri: GReaderURI.StreamItemIds,
        formData: formData,
      });

      if (!res.ok) {
        console.log('Fetching item ids failed: %s' + res.statusText);
        return Promise.reject(res.statusText);
      }

      const result: string = await res.text();
      const stream: GReaderStreamIds = await parseJson(result);

      // If no articles are returned, this list might be null.
      if (!stream.itemRefs) {
        break;
      }

      stream.itemRefs.forEach((greaderStreamRef: GReaderItemRef) => {
        // The ID is a 64-bit base-10 number as a string, so parse as BigInt.
        const id: bigint = BigInt(greaderStreamRef.id);
        const feedId: bigint = BigInt(
          this.parseFeedID(greaderStreamRef.directStreamIds[0])
        );
        const folderId: bigint = BigInt(
          this.parseFolderID(greaderStreamRef.directStreamIds[1])
        );

        // When requesting article IDs, pass a hex string. This seems to match
        // other real-world client behavior.
        articleIdStrs.push([
          id.toString(16),
          feedId.toString(16),
          folderId.toString(16),
        ]);
      });

      // Keep fetching until we see less than the max items returned. This can
      // also be determined by the existence of the continuation token, but
      // this is a safer approach.
      if (stream.itemRefs.length === limit) {
        formData.set('c', stream.continuation);
      } else {
        break;
      }
    }

    return articleIdStrs;
  }

  private populateFolderFeeds(subscriptions: GReaderSubscription[]) {
    subscriptions.forEach((sub: GReaderSubscription) => {
      const feed = new FeedCls(
        this.parseFeedID(sub.id),
        sub.title,
        sub.htmlUrl,
        sub.htmlUrl,
        0
      );
      feed.SetFavicon(new FaviconCls(sub.iconUrl));

      const folderId = this.parseFolderID(sub.categories[0].id);
      const folderTitle = sub.categories[0].label;

      let folder = this.folderMap.get(folderId);
      if (!folder) {
        folder = new FolderCls(folderId, folderTitle);
        this.folderMap.set(folderId, folder);
      }

      let feeds = this.folderFeeds.get(folderId);
      if (!feeds) {
        feeds = [];
      }
      feeds.push(feed);
      this.folderFeeds.set(folderId, feeds);
    });
  }

  private buildTree(): ContentTreeCls {
    let treeCls: ContentTreeCls = ContentTreeCls.new();

    this.folderMap.forEach((folder: FolderCls, folderId: FolderId) => {
      const feeds = this.folderFeeds.get(folderId);
      // Not all folders have feeds, so nothing more to be done here.
      if (!feeds) {
        return;
      }

      feeds.forEach((feed: FeedCls) => {
        const articles = this.feedToArticles.get(feed.Id());
        // If this is undefined, it just means that there are no *unread*
        // articles for this feed, which is fine. Just add the empty folder
        // to the feed.
        if (articles !== undefined) {
          articles.forEach((article: ArticleCls) => feed.AddArticle(article));
        }
        folder.AddFeed(feed);
        feed.SetFolderId(folderId);
      });
      treeCls.AddFolder(folder);
    });
    return treeCls;
  }

  private async doFetch(fetchParams: GReaderFetch): Promise<Response> {
    const res = await this.sendOnce(fetchParams);

    // Post tokens expire, so one obtained when the page loaded stops working
    // while the page is still open. The server says so with this header
    // specifically to distinguish it from the session itself having ended, and
    // the fix is to ask for another and repeat the request. Only once: if the
    // second attempt is refused too, the session is gone and retrying further
    // would just be a loop.
    if (!this.postTokenWasRejected(res, fetchParams)) {
      return res;
    }
    if (!(await this.refreshPostToken())) {
      return res;
    }
    return this.sendOnce(fetchParams);
  }

  private postTokenWasRejected(
    res: Response,
    fetchParams: GReaderFetch
  ): boolean {
    return (
      res.status === 401 &&
      res.headers.get(GReader.badPostTokenHeader) === 'true' &&
      !fetchParams.omitPostToken
    );
  }

  // refreshPostToken obtains a new post token, returning false if the session
  // can no longer produce one.
  private async refreshPostToken(): Promise<boolean> {
    const res: Response = await this.sendOnce({
      uri: GReaderURI.Token,
      omitPostToken: true,
    });

    if (!res.ok) {
      console.log('Could not refresh post token: ' + res.statusText);
      return false;
    }

    this.postToken = await res.text();
    return true;
  }

  private sendOnce(fetchParams: GReaderFetch): Promise<Response> {
    const init: RequestInit = { ...fetchParams.init };

    // The session travels as a cookie, so every request has to carry
    // credentials; there is no token here to put in a header.
    init.credentials = 'include';

    // Unless explicitly specified otherwise, set the "T" value to the post
    // token in each request. Set on every attempt rather than once, so that a
    // repeated request carries the token obtained in between.
    if (!fetchParams.omitPostToken) {
      if (!fetchParams.formData) {
        fetchParams.formData = new FormData();
      }
      fetchParams.formData.set('T', this.postToken);
    }

    // If the request has form data, override the method to 'POST' since it
    // will be encoded as a multipart form.
    if (fetchParams.formData) {
      init.method = 'POST';
      init.body = fetchParams.formData;
    }

    return fetch(fetchParams.uri, init);
  }

  private parseArticleID(uri: string): string {
    // Note: This is parsing the article ID as hex.
    const regex = /^tag:google.com,2005:reader\/item\/(\p{Hex_Digit}+)$/u;
    const match = uri.match(regex);
    if (match && match[1]) {
      return match[1];
    } else {
      throw new Error('Invalid article ID: ' + uri);
    }
  }

  private feedStreamId(feedId: FeedId): string {
    return 'feed/' + feedId;
  }

  private folderStreamId(folderId: FolderId): string {
    return 'user/-/label/' + folderId;
  }

  // The same stream form carrying a name instead of an ID, which the server
  // tells apart by whether it reads as a number.
  private folderLabel(name: string): string {
    return 'user/-/label/' + name;
  }

  private parseFeedID(uri: string): string {
    const regex = /^feed\/(\d+)$/;
    const match = uri.match(regex);
    if (match && match[1]) {
      return match[1];
    } else {
      throw new Error('Invalid feed ID: ' + uri);
    }
  }

  private parseFolderID(uri: string): string {
    const regex = /^user\/-\/label\/(\d+)$/;
    const match = uri.match(regex);
    if (match && match[1]) {
      return match[1];
    } else {
      throw new Error('Invalid folder ID: ' + uri);
    }
  }
}
