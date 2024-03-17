import {FeedCls, FeedId, FeedTitle, FeedView} from "./feed";
import {MarkState} from "../utils/types";
import {ArticleId, ArticleView} from "./article";

/** FolderId is the ID of a Folder object. */
export type FolderId = string;

export interface FolderView {
  id: FolderId;
  title: FeedTitle;
  unread_count: number;
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

  public GetArticleEntry(articleId: ArticleId, feedId: FeedId): ArticleView[] {
    const feed: FeedCls = this.getFeedOrThrow(feedId);
    return [feed.GetArticleEntry(articleId, this.id)];
  }

  public GetFeedEntry(feedId: FeedId): ArticleView[] {
    const feed: FeedCls = this.getFeedOrThrow(feedId);
    return feed.GetArticleEntries(this.id);
  }

  public GetFolderEntry(): ArticleView[] {
    let entries: ArticleView[] = [];

    this.feeds.forEach((f: FeedCls): void => {
      entries = entries.concat(f.GetArticleEntries(this.id));
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

  public GetViewEntry(): [FolderView, FeedView[]] {
    const folderView: FolderView = {
      id: this.id,
      title: this.title,
      unread_count: this.unread_count
    };

    const feedViews: FeedView[] = [];
    this.feeds.forEach((f: FeedCls): void => {
      feedViews.push(f.GetView(this.id));
    });

    return [folderView, feedViews];
  }

  public static Comparator(a: FolderCls, b: FolderCls): number {
    // Sort by title of folder
    return a.Title().localeCompare(b.Title());
  }

  private getFeedOrThrow(feedId: FeedId): FeedCls {
    const feed: FeedCls | undefined = this.feeds.get(feedId);
    if (feed === undefined) {
      throw new Error(`No feed by ID: ${feedId} in folder ${this.id}`);
    }
    return feed;
  }

  private sort(): void {
    const comparator = (a: [FeedId, FeedCls], b: [FeedId, FeedCls]) =>
      FeedCls.Comparator(a[1], b[1]);
    this.feeds = new Map([...this.feeds.entries()].sort(comparator));
  }
}