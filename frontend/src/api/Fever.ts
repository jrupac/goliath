import {
  Article,
  ArticleId,
  ArticleSelection,
  FaviconId,
  Feed,
  FeedId,
  FeedSelection,
  Folder,
  FolderId,
  FolderSelection,
  SelectionKey,
  Status
} from "../utils/types";
import {Decimal} from "decimal.js-light";
import {maxDecimal, parseJson} from "../utils/helpers";

// The following several interfaces conform to the Fever API.
interface FeverFeedGroupType {
  group_id: string;
  feed_ids: string;
}

interface FeverGroupType {
  id: string;
  title: string;
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

export default class Fever {
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

  public async initialize(cb: (status: Status) => void): Promise<[number, Map<FolderId, Folder>]> {
    await Promise.all([
      this.fetchFeeds(cb),
      this.fetchFolders(cb),
      this.fetchFavicons(cb),
      this.fetchItems(cb)
    ]);

    return this.buildTree();
  }

  public markArticle(mark: string, entity: SelectionKey): Promise<Response> {
    const feverId = (entity as ArticleSelection)[0] as string;
    return fetch('/fever/?api&mark=item&as=' + mark + '&id=' + feverId, {
      credentials: 'include'
    });
  }

  public markFeed(mark: string, entity: SelectionKey): Promise<Response> {
    const feverId = (entity as FeedSelection)[0] as string;
    return fetch('/fever/?api&mark=feed&as=' + mark + '&id=' + feverId, {
      credentials: 'include'
    });
  }

  public markFolder(mark: string, entity: SelectionKey): Promise<Response> {
    const feverId = (entity as FolderSelection) as string;
    return fetch('/fever/?api&mark=group&as=' + mark + '&id=' + feverId, {
      credentials: 'include'
    });
  }

  public markAll(mark: string, entity: SelectionKey): Promise<Response> {
    return fetch('/fever/?api&mark=group&as=' + mark + '&id=' + entity, {
      credentials: 'include'
    });
  }

  private fetchFolders(cb: (status: Status) => void) {
    return fetch('/fever/?api&groups', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => parseJson(result))
      .then((body: FeverFetchGroupsType) => {
        this.feverFetchGroupsResponse = body;
        cb(Status.Folder);
      }).catch((e) => console.log(e));
  }

  private fetchFeeds(cb: (status: Status) => void) {
    return fetch('/fever/?api&feeds', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => parseJson(result))
      .then((body: FeverFetchFeedsType) => {
        this.feverFetchFeedsResponse = body;
        cb(Status.Feed);
      }).catch((e) => console.log(e));
  }

  private fetchFavicons(cb: (status: Status) => void) {
    return fetch('/fever/?api&favicons', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => parseJson(result))
      .then((body: FeverFetchFaviconsType) => {
        this.feverFetchFaviconsResponse = body;
        cb(Status.Favicon);
      }).catch((e) => console.log(e));
  }

  private async fetchItems(cb: (status: Status) => void) {
    let since = new Decimal(0);
    let itemUri = '/fever/?api&items';
    let itemCount = -1;

    for (; ;) {
      let items = await fetch(itemUri, {
        credentials: 'include'
      }).then((result) => result.text())
        .then((result) => parseJson(result))
        .then((body: FeverFetchItemsType) => body.items)
        .catch((e) => console.log(e));

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

  private buildTree(): [number, Map<FolderId, Folder>] {
    let tree = new Map<FolderId, Folder>();

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
        const folderId = group.id as FolderId;
        const feedIdList = folderToFeeds.get(folderId);

        // Not all folders have feeds, so nothing more to be done here.
        if (feedIdList === undefined) {
          return;
        }

        // Populate feeds in this folder.
        let feeds = new Map<FeedId, Feed>();
        feedIdList.forEach(
          (feedId: FeedId) => {
            const feed = globalFeedMap.get(feedId);
            if (feed === undefined) {
              throw new Error("Unknown feed ID: " + feedId);
            }

            // Populate articles in this feed.
            const articles = feedToArticles.get(feedId) || [];
            feed.articles = new Map<ArticleId, Article>();
            articles.forEach((article: Article) => {
              feed.articles.set(article.id, article);
            });

            // Compute other metadata about this feed.
            feed.unread_count = articles.reduce(
              (acc: number, a: Article) => acc + (1 - a.is_read), 0);
            feed.favicon = globalFaviconMap.get(feed.favicon_id) || "";

            feeds.set(feedId, feed);
          }
        );

        // Compute other metadata about this folder.
        const unreadCount = Array.from(feeds.values()).reduce(
          (acc: number, f: Feed) => acc + f.unread_count, 0);

        // Sort feeds by title
        feeds = new Map([...feeds].sort(
          (a, b) => a[1].title.localeCompare(b[1].title)))

        const folderData: Folder = {
          feeds: feeds,
          title: group.title,
          unread_count: unreadCount
        };
        tree.set(folderId, folderData);
      }
    );

    // Compute other global metadata.
    const unreadCount = Array.from(tree.values()).reduce(
      (acc: number, f: Folder) => acc + f.unread_count, 0);

    // Sort folders by title
    tree = new Map([...tree].sort(
      (a, b) => a[1].title.localeCompare(b[1].title)))

    return [unreadCount, tree];
  }
}