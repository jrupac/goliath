import {Feed, FeedCls, FeedId} from "./feed";
import {ArticleListEntry, FeedEntry, MarkState} from "../utils/types";
import {ArticleId} from "./article";

/** FolderId is the ID of a Folder object. */
export type FolderId = string;
/** Folder is a logical grouping of zero or more Feeds. */
export type Folder = {
  title: string;
  unread_count: number;
  feeds: Map<FeedId, Feed>;
}

export class FolderCls {
  private readonly id: FolderId;
  private readonly title: string;
  private unread_count: number;
  private feeds: Map<FeedId, FeedCls>;

  constructor(id: FolderId, title: string) {
    this.id = id;
    this.title = title;

    this.unread_count = 0;
    this.feeds = new Map<FeedId, FeedCls>();
  }

  public AddFeed(feed: FeedCls): void {
    const existing = this.feeds.get(feed.Id());
    if (existing !== undefined) {
      console.log(`WARNING: Replacing feed: ${existing.Title()} in folder: ${this.title}`);
      this.unread_count -= feed.UnreadCount();
    }

    this.feeds.set(feed.Id(), feed);
    this.sort();
    this.unread_count += feed.UnreadCount();
  }

  public MarkArticle(articleId: ArticleId, feedId: FeedId, markState: MarkState): number {
    const feed = this.getFeedOrThrow(feedId);

    this.unread_count -= feed.UnreadCount();
    feed.MarkArticle(articleId, markState);
    this.unread_count += feed.UnreadCount();

    return this.unread_count;
  }

  public MarkFeed(feedId: FeedId, markState: MarkState): number {
    const feed = this.getFeedOrThrow(feedId);

    this.unread_count -= feed.UnreadCount();
    feed.MarkFeed(markState);
    this.unread_count += feed.UnreadCount();

    return this.unread_count;
  }

  public MarkFolder(markState: MarkState): number {
    let unread: number = 0;

    this.feeds.forEach((f: FeedCls): void => {
      unread += f.MarkFeed(markState);
    });

    this.unread_count = unread;
    return this.unread_count;
  }

  public GetArticleEntry(articleId: ArticleId, feedId: FeedId): ArticleListEntry[] {
    const feed: FeedCls = this.getFeedOrThrow(feedId);
    return [
      [...feed.GetArticleEntry(articleId), this.id],
    ];
  }

  public GetFeedEntry(feedId: FeedId): ArticleListEntry[] {
    const entries: ArticleListEntry[] = [];
    const feed: FeedCls = this.getFeedOrThrow(feedId);

    feed.GetFeedEntry().forEach((e: FeedEntry): void => {
      entries.push([...e, this.id]);
    })

    return entries;
  }

  public GetFolderEntry(): ArticleListEntry[] {
    const entries: ArticleListEntry[] = [];

    this.feeds.forEach((f: FeedCls): void => {
      f.GetFeedEntry().forEach((e: FeedEntry): void => {
        entries.push([...e, this.id]);
      })
    });

    return entries;
  }

  public UnreadCount(): number {
    return this.unread_count;
  }

  public Id(): FolderId {
    return this.id;
  }

  public Title(): string {
    return this.title;
  }

  public static Comparator(a: FolderCls, b: FolderCls): number {
    return a.Title().localeCompare(b.Title());
  }

  private getFeedOrThrow(feedId: FeedId): FeedCls {
    const feed = this.feeds.get(feedId);
    if (feed === undefined) {
      throw new Error(`No feed by ID: ${feedId} in folder ${this.id}`);
    }
    return feed;
  }

  private sort(): void {
    const comparator = (a: [FeedId, FeedCls], b: [FeedId, FeedCls]) => FeedCls.Comparator(a[1], b[1]);
    this.feeds = new Map([...this.feeds.entries()].sort(comparator));
  }
}