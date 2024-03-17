import {
  ArticleSelection,
  FeedSelection,
  FolderSelection,
  SelectionKey,
  Status
} from "../utils/types";
import {Decimal} from "decimal.js-light";
import {cookieExists, maxDecimal, parseJson} from "../utils/helpers";
import {FetchAPI, LoginInfo} from "./interface";
import {ContentTreeCls} from "../models/contentTree";
import {ArticleCls, ArticleId} from "../models/article";
import {FolderCls, FolderId} from "../models/folder";
import {Favicon, FaviconCls, FeedCls, FeedId, FeedTitle} from "../models/feed";

// The server sets a cookie with this name on successful login.
const feverAuthCookie: string = "goliath";

/**
 * The following several interfaces conform to the Fever API.
 */

type FaviconId = string;

interface Article {
  id: ArticleId;
  feed_id: FeedId;
  title: string;
  author: string;
  html: string;
  url: string;
  is_saved: 0 | 1;
  is_read: 0 | 1;
  created_on_time: number;
}

interface Feed {
  id: FeedId;
  favicon_id: FaviconId;
  favicon: Favicon;
  title: FeedTitle;
  url: string;
  site_url: string;
  is_spark: 0 | 1;
  last_updated_on_time: number;
  unread_count: number;
  articles: Map<ArticleId, Article>;
}

interface FeverGroupType {
  id: string;
  title: string;
}

interface FeverFeedGroupType {
  group_id: string;
  feed_ids: string;
}

interface FeverFaviconType {
  id: FaviconId;
  data: string;
}

interface FeverFetchGroupsType {
  groups: FeverGroupType[];
  feeds_groups: FeverFeedGroupType[];
}

interface FeverFetchFeedsType {
  feeds: Feed[];
  feeds_groups: FeverFeedGroupType[];
}

interface FeverFetchFaviconsType {
  favicons: FeverFaviconType[];
}

interface FeverFetchItemsType {
  items: Article[];
  total_items: number;
}

export default class Fever implements FetchAPI {
  private feverFetchGroupsResponse: FeverFetchGroupsType;
  private feverFetchFeedsResponse: FeverFetchFeedsType;
  private feverFetchFaviconsResponse: FeverFetchFaviconsType;
  private feverFetchItemsResponse: FeverFetchItemsType;

  constructor() {
    this.feverFetchGroupsResponse = {groups: [], feeds_groups: []};
    this.feverFetchFeedsResponse = {feeds: [], feeds_groups: []};
    this.feverFetchFaviconsResponse = {favicons: []};
    this.feverFetchItemsResponse = {items: [], total_items: 0};
  }

  public async HandleAuth(loginInfo: LoginInfo): Promise<boolean> {
    const res: Response = await fetch('/auth', {
      method: 'POST',
      body: JSON.stringify(loginInfo),
      credentials: 'include'
    });
    if (!res.ok) {
      console.log(res);
      return false;
    } else {
      if (!await this.VerifyAuth()) {
        console.log("Server did not set auth cookie.");
        return false;
      }
      return true;
    }
  }

  public VerifyAuth(): Promise<boolean> {
    return Promise.resolve(cookieExists(feverAuthCookie));
  }

  public async InitializeContent(cb: (status: Status) => void): Promise<ContentTreeCls> {
    await Promise.all([
      this.fetchFeeds(cb),
      this.fetchFolders(cb),
      this.fetchFavicons(cb),
      this.fetchItems(cb)
    ]);

    return this.buildTree();
  }

  public async MarkArticle(mark: string, entity: SelectionKey): Promise<Response> {
    const feverId: string = (entity as ArticleSelection)[0] as string;
    return await fetch('/fever/?api&mark=item&as=' + mark + '&id=' + feverId, {
      credentials: 'include'
    });
  }

  public async MarkFeed(mark: string, entity: SelectionKey): Promise<Response> {
    const feverId: string = (entity as FeedSelection)[0];
    return await fetch('/fever/?api&mark=feed&as=' + mark + '&id=' + feverId, {
      credentials: 'include'
    });
  }

  public async MarkFolder(mark: string, entity: SelectionKey): Promise<Response> {
    const feverId: string = (entity as FolderSelection);
    return await fetch('/fever/?api&mark=group&as=' + mark + '&id=' + feverId, {
      credentials: 'include'
    });
  }

  public async MarkAll(mark: string, entity: SelectionKey): Promise<Response> {
    return await fetch('/fever/?api&mark=group&as=' + mark + '&id=' + entity, {
      credentials: 'include'
    });
  }

  private async fetchFolders(cb: (status: Status) => void): Promise<void> {
    const res: Response = await fetch('/fever/?api&groups', {
      credentials: 'include'
    });
    const result: string = await res.text();
    this.feverFetchGroupsResponse = await parseJson(result);
    cb(Status.Folder);
  }

  private async fetchFeeds(cb: (status: Status) => void): Promise<void> {
    const res: Response = await fetch('/fever/?api&feeds', {
      credentials: 'include'
    });
    const result: string = await res.text();
    this.feverFetchFeedsResponse = await parseJson(result);
    cb(Status.Feed);
  }

  private async fetchFavicons(cb: (status: Status) => void): Promise<void> {
    const res: Response = await fetch('/fever/?api&favicons', {
      credentials: 'include'
    });
    const result: string = await res.text();
    this.feverFetchFaviconsResponse = await parseJson(result);
    cb(Status.Favicon);
  }

  private async fetchItems(cb: (status: Status) => void): Promise<void> {
    let since = new Decimal(0);
    let itemUri = '/fever/?api&items';
    let itemCount = -1;

    for (; ;) {
      const res: Response = await fetch(itemUri, {
        credentials: 'include'
      });
      const result: string = await res.text();
      const items = await parseJson(result).items;

      if (!items) {
        return;
      }

      itemCount = items.length;
      this.feverFetchItemsResponse = {
        total_items: this.feverFetchItemsResponse.total_items + itemCount,
        items: [
          ...this.feverFetchItemsResponse.items,
          ...items
        ]
      }

      // Keep fetching until we see less than the max items returned.
      // Don't update the status field until we're done.
      if (itemCount === 50) {
        items.forEach((item: Article) => {
          // Update latest seen article ID.
          since = maxDecimal(since, item.id);
        });
        itemUri = '/fever/?api&items&since_id=' + since.toString();
      } else {
        break;
      }
    }

    cb(Status.Article);
  }

  private buildTree(): ContentTreeCls {
    let treeCls: ContentTreeCls = ContentTreeCls.new();

    // Map of (Folder ID) -> (Feed ID).
    const folderToFeeds = new Map<FolderId, FeedId[]>();
    this.feverFetchGroupsResponse.feeds_groups.forEach(
      (feedGroup: FeverFeedGroupType) => {
        folderToFeeds.set(feedGroup.group_id, feedGroup.feed_ids.split(','));
      });

    // Map of all (Feed ID) -> (Feed Data).
    const globalFeedMap = new Map<FeedId, Feed>();
    this.feverFetchFeedsResponse.feeds.forEach(
      (feed: Feed) => globalFeedMap.set(feed.id, feed));

    // Map of (Feed ID) -> (list of Article Data).
    const feedToArticles = new Map<FeedId, Article[]>();
    this.feverFetchItemsResponse.items.forEach(
      (article: Article) => {
        const entry = feedToArticles.get(article.feed_id) || [];
        feedToArticles.set(article.feed_id, [...entry, article]);
      });

    // Map of (Favicon ID) -> (Favicon Data).
    const globalFaviconMap = new Map<FaviconId, string>();
    this.feverFetchFaviconsResponse.favicons.forEach(
      (favicon: FeverFaviconType) => {
        globalFaviconMap.set(favicon.id, favicon.data)
      });

    this.feverFetchGroupsResponse.groups.forEach(
      (group: FeverGroupType) => {
        const feedIdList = folderToFeeds.get(group.id);
        // Not all folders have feeds, so nothing more to be done here.
        if (feedIdList === undefined) {
          return;
        }

        let folderCls: FolderCls = new FolderCls(group.id, group.title);
        // Populate feeds in this folder.
        feedIdList.forEach(
          (feedId: FeedId) => {
            const feed = globalFeedMap.get(feedId);
            if (feed === undefined) {
              throw new Error("Unknown feed ID: " + feedId);
            }

            const feedCls: FeedCls = new FeedCls(
              feed.id, feed.title, feed.url, feed.site_url, feed.is_spark,
              feed.last_updated_on_time);

            // Populate articles in this feed.
            const articles = feedToArticles.get(feedId) || [];
            articles.forEach((article: Article) => {
              feedCls.AddArticle(new ArticleCls(
                article.id, article.title, article.author, article.html,
                article.url, article.is_saved, article.created_on_time,
                article.is_read));
            });

            feedCls.SetFavicon(
              new FaviconCls(globalFaviconMap.get(feed.favicon_id)));
            folderCls.AddFeed(feedCls);
          }
        );
        treeCls.AddFolder(folderCls);
      }
    );
    return treeCls;
  }
}
