import {Feed, FeedCls, FeedId} from "./feed";
import {MarkState} from "../utils/types";

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

  constructor(id: FolderId, title: string, feeds: Map<FeedId, FeedCls>) {
    this.id = id;
    this.title = title;
    this.feeds = feeds;

    this.unread_count = 0;
    this.feeds.forEach((f: FeedCls): void => {
      this.unread_count += f.UnreadCount();
    });
    this.sort();
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

  public MarkFolder(markState: MarkState) {
    let unread: number = 0;

    this.feeds.forEach((f: FeedCls): void => {
      unread += f.MarkFeed(markState);
    });

    this.unread_count = unread;
    return this.unread_count;
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

  private sort(): void {
    const comparator = (a: [FeedId, FeedCls], b: [FeedId, FeedCls]) => FeedCls.Comparator(a[1], b[1]);
    this.feeds = new Map([...this.feeds.entries()].sort(comparator));
  }
}