import {FetchAPI, LoginInfo} from "./interface";
import {
  ArticleSelection,
  FeedSelection,
  FolderSelection,
  SelectionKey,
  Status
} from "../utils/types";
import {parseJson} from "../utils/helpers";
import {ContentTreeCls} from "../models/contentTree";
import {FolderCls, FolderId} from "../models/folder";
import {FaviconCls, FeedCls, FeedId} from "../models/feed";
import {ArticleCls} from "../models/article";

interface GReaderFetch {
  uri: string,
  init?: RequestInit,
  formData?: FormData,
  omitSessionToken?: boolean,
  omitPostToken?: boolean
}

/**
 * The following several interfaces conform to the GReader API.
 */
interface GReaderHandleLogin {
  SID: string;
  LSID: string;
  Auth: string;
}

interface GReaderCategory {
  id: string;
  label: string;
}

interface GReaderSubscription {
  title: string;
  firstItemMsec: string;
  htmlUrl: string;
  iconUrl: string;
  sortId: string;
  id: string;
  categories: GReaderCategory[];
}

interface GReaderSubscriptionList {
  subscriptions: GReaderSubscription[];
}

interface GReaderCanonical {
  href: string;
}

interface GReaderContent {
  direction: string;
  content: string;
}

interface GReaderOrigin {
  streamId: string;
  title: string;
  htmlUrl: string;
}

interface GReaderItemContent {
  crawlTimeMsec: string;
  timestampUsec: string;
  id: string;
  categories: string[];
  title: string;
  published: number;
  canonical: GReaderCanonical[];
  alternate: GReaderCanonical[];
  summary: GReaderContent;
  origin: GReaderOrigin;
}

interface GReaderItemRef {
  id: string;
  directStreamIds: string[];
  timestampUsec: string;
}

interface GReaderStream {
  items: GReaderItemContent[];
  itemRefs: GReaderItemRef[];
}

export default class GReader implements FetchAPI {
  private static golaithSessionCookie: string = "goliath_token";

  private folderFeeds: Map<FolderId, FeedCls[]>;
  private folderMap: Map<FolderId, FolderCls>;
  private feedToArticles: Map<FeedId, ArticleCls[]>;
  private sessionToken: string;
  private postToken: string;

  constructor() {
    this.folderFeeds = new Map<FolderId, FeedCls[]>();
    this.folderMap = new Map<FolderId, FolderCls>();
    this.feedToArticles = new Map<FeedId, ArticleCls[]>();
    this.sessionToken = "";
    this.postToken = "";
  }

  public async HandleAuth(loginInfo: LoginInfo): Promise<boolean> {
    const formData = new FormData();
    formData.append("Email", loginInfo.username);
    formData.append("Passwd", loginInfo.password);

    const res: Response = await this.doFetch({
      uri: '/greader/accounts/ClientLogin',
      formData: formData,
      omitSessionToken: true,
      omitPostToken: true
    });

    if (!res.ok) {
      console.log("Login failed: " + res);
      return false;
    }

    const result: string = await res.text();
    const loginResult: GReaderHandleLogin = await parseJson(result);
    this.sessionToken = loginResult.Auth;

    // Set the session ID as a cookie to support session persistence. The
    // session token also needs to be parsed out and set as a header on calls.
    document.cookie = GReader.golaithSessionCookie + "=" + this.sessionToken;

    return true;
  }

  public async VerifyAuth(): Promise<boolean> {
    // First, look for session token in cookies. If not set, return false to
    // force re-authentication.
    const cookies = document.cookie.split(';');
    for (let i = 0; i < cookies.length; i++) {
      const cookie = cookies[i].trim();
      if (cookie.startsWith(GReader.golaithSessionCookie + '=')) {
        this.sessionToken = cookie.substring(
          GReader.golaithSessionCookie.length + 1);
        break;
      }
    }

    if (this.sessionToken === "") {
      console.log("Could not find session token in cookies.");
      return false;
    }

    // Second, if a session token exists, generate a post token. It is both
    // needed for future operations and serves as verification of the session
    // token.
    const res: Response = await this.doFetch({
      uri: '/greader/reader/api/0/token',
      omitPostToken: true
    });

    if (!res.ok) {
      console.log("Could not create post token: " + res.statusText);
      return false;
    } else {
      this.postToken = await res.text();
      return true;
    }
  }

  public async InitializeContent(cb: (s: Status) => void): Promise<ContentTreeCls> {
    await Promise.all([
      this.fetchSubscriptions(cb),
      this.fetchArticles(cb)
    ]);

    return this.buildTree();
  }

  public async MarkArticle(mark: string, entity: SelectionKey): Promise<Response> {
    const greaderId: string = (entity as ArticleSelection)[0] as string;
    const formData = new FormData();
    formData.set("a", mark);
    formData.set("i", greaderId);

    return this.doFetch({
      uri: '/greader/reader/api/0/edit-tag',
      formData: formData,
    });
  }

  public async MarkFeed(_: string, entity: SelectionKey): Promise<Response> {
    const greaderId: string = (entity as FeedSelection)[0];
    const formData = new FormData();
    formData.set("s", greaderId);

    return this.doFetch({
      uri: '/greader/reader/api/0/mark-all-as-read',
      formData: formData,
    });
  }

  public async MarkFolder(_: string, entity: SelectionKey): Promise<Response> {
    const greaderId: string = (entity as FolderSelection);
    const formData = new FormData();
    formData.set("t", greaderId);

    return this.doFetch({
      uri: '/greader/reader/api/0/mark-all-as-read',
      formData: formData,
    });
  }

  public async MarkAll(_: string, entity: SelectionKey): Promise<Response> {
    return await fetch(
      '/greader/reader/api/0/mark-all-as-read' +
      '?T=' + this.postToken +
      '&t=' + entity,
      {credentials: 'include'});
  }

  private async fetchSubscriptions(cb: (status: Status) => void): Promise<void> {
    const res: Response = await this.doFetch({
      uri: '/greader/reader/api/0/subscription/list',
    });

    if (!res.ok) {
      console.log("Fetching subscription list failed: %s" + res.statusText);
      return Promise.reject(res.statusText);
    }

    const result: string = await res.text();
    const subscriptionList: GReaderSubscriptionList = await parseJson(result);

    this.populateFolderFeeds(subscriptionList.subscriptions);

    cb(Status.Folder);
    cb(Status.Favicon);
    cb(Status.Feed);
  }

  private async fetchArticles(cb: (status: Status) => void): Promise<void> {
    const articleRefLimit = 10000;
    const articleContentLimit = 100;

    // TODO: Implement continuation tokens.
    let keepFetching = false;

    const formData = new FormData();
    formData.set("s", "user/-/state/com.google/reading-list");
    formData.set("xt", "user/-/state/com.google/read")
    formData.set("n", articleRefLimit.toString());

    const articleIdStrs: string[] = [];

    for (; ;) {
      const res: Response = await this.doFetch({
        uri: '/greader/reader/api/0/stream/items/ids',
        formData: formData
      });

      if (!res.ok) {
        console.log("Fetching item ids failed: %s" + res.statusText);
        return Promise.reject(res.statusText);
      }

      const result: string = await res.text();
      const stream: GReaderStream = await parseJson(result);

      stream.itemRefs.forEach(
        (greaderStreamRef: GReaderItemRef) => {
          // The ID is given as a 64-bit base-10 number as a string. This needs
          // a BigInt type to hold. But when requesting contents, send it as a
          // hex string. This seems to match other real-world client behavior.
          articleIdStrs.push(BigInt(greaderStreamRef.id).toString(16));
        }
      )

      // Keep fetching until we see less than the max items returned.
      if (stream.itemRefs.length === articleRefLimit) {
        if (!keepFetching) {
          console.log("WARNING: %s items returned, more may exist.", articleRefLimit);
          break;
        }
      } else {
        break;
      }
    }

    for (let i = 0; i < articleIdStrs.length; i += articleContentLimit) {
      const articleContentsForm = new FormData();
      articleIdStrs.slice(i, i + articleContentLimit).forEach(
        (id) => articleContentsForm.append("i", id));

      const res: Response = await this.doFetch({
        uri: '/greader/reader/api/0/stream/items/contents',
        formData: articleContentsForm
      });

      if (!res.ok) {
        console.log("Fetching item contents failed: %s" + res.statusText);
        return Promise.reject(res.statusText);
      }

      const result: string = await res.text();
      const streamItemContents: GReaderStream = await parseJson(result);

      streamItemContents.items.forEach(
        (item: GReaderItemContent) => {
          const article = new ArticleCls(
            this.parseArticleID(item.id), item.title, "", item.summary.content,
            item.canonical[0].href, 0, item.published, 0);

          const feedId = this.parseFeedID(item.categories[1]);
          const articles = this.feedToArticles.get(feedId);
          if (articles === undefined) {
            this.feedToArticles.set(feedId, [article]);
          } else {
            articles.push(article);
          }
        });
    }

    cb(Status.Article);
  }

  private populateFolderFeeds(subscriptions: GReaderSubscription[]) {
    subscriptions.forEach(
      (sub: GReaderSubscription) => {
        const feed = new FeedCls(
          this.parseFeedID(sub.id), sub.title, sub.htmlUrl, sub.htmlUrl, 0, 0);
        feed.SetFavicon(new FaviconCls(sub.iconUrl));

        const folderId = this.parseFolderID(sub.categories[0].id);
        const folderTitle = sub.categories[0].label;

        let folder = this.folderMap.get(folderId);
        if (folder === undefined) {
          folder = new FolderCls(folderId, folderTitle);
          this.folderMap.set(folderId, folder);
        }

        let feeds = this.folderFeeds.get(folderId);
        if (feeds === undefined) {
          feeds = [];
        }
        feeds.push(feed);
        this.folderFeeds.set(folderId, feeds);
      });
  }

  private buildTree(): ContentTreeCls {
    let treeCls: ContentTreeCls = ContentTreeCls.new();

    this.folderMap.forEach(
      (folder: FolderCls, folderId: FolderId) => {
        const feeds = this.folderFeeds.get(folderId);
        // Not all folders have feeds, so nothing more to be done here.
        if (feeds === undefined) {
          return;
        }

        feeds.forEach(
          (feed: FeedCls) => {
            const articles = this.feedToArticles.get(feed.Id());
            // If this is undefined, it just means that there are no *unread*
            // articles for this feed, which is fine. Just add the empty folder
            // to the feed.
            if (articles !== undefined) {
              articles.forEach(
                (article: ArticleCls) => feed.AddArticle(article));
            }
            folder.AddFeed(feed);
          }
        );
        treeCls.AddFolder(folder);
      }
    );
    return treeCls;
  }

  private doFetch(fetchParams: GReaderFetch): Promise<Response> {
    if (fetchParams.init === undefined) {
      fetchParams.init = {};
    }

    // Unless explicitly specified otherwise, add the authorization header
    if (!fetchParams.omitSessionToken) {
      const headers = new Headers(fetchParams.init.headers);
      headers.append("Authorization", "GoogleLogin auth=" + this.sessionToken);
      fetchParams.init.headers = headers;

      // Send credentials whenever we have a session token defined
      fetchParams.init.credentials = 'include';
    }

    // Unless explicitly specified otherwise, set the "T" value to the post
    // token in each request.
    if (!fetchParams.omitPostToken) {
      if (fetchParams.formData === undefined) {
        fetchParams.formData = new FormData();
      }
      fetchParams.formData.set("T", this.postToken);
    }

    // If the request has form data, override the method to 'POST' since it
    // will be encoded as a multipart form.
    if (fetchParams.formData !== undefined) {
      fetchParams.init.method = 'POST';
      fetchParams.init.body = fetchParams.formData;
    }

    return fetch(fetchParams.uri, fetchParams.init);
  }

  private parseArticleID(uri: string): string {
    // Note: This is parsing the article ID as hex.
    const regex = /^tag:google.com,2005:reader\/item\/(\p{Hex_Digit}+)$/u;
    const match = uri.match(regex);
    if (match && match[1]) {
      return match[1];
    } else {
      throw new Error("Invalid article ID: " + uri);
    }
  }

  private parseFeedID(uri: string): string {
    const regex = /^feed\/(\d+)$/;
    const match = uri.match(regex);
    if (match && match[1]) {
      return match[1];
    } else {
      throw new Error("Invalid feed ID: " + uri);
    }
  }

  private parseFolderID(uri: string): string {
    const regex = /^user\/-\/label\/(\d+)$/;
    const match = uri.match(regex);
    if (match && match[1]) {
      return match[1];
    } else {
      throw new Error("Invalid folder ID: " + uri);
    }
  }
}