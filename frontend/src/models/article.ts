import {MarkState} from "../utils/types";
import {Favicon, FeedId, FeedTitle} from "./feed";
import {FolderId} from "./folder";

/** ArticleId is the ID associated with an Article object. */
export type ArticleId = string;

/** ArticleView holds metadata associated with a displayed Article. */
export interface ArticleView {
  // Folder metadata
  folder_id: FolderId;
  // Feed metadata
  feed_id: FeedId;
  feed_title: FeedTitle;
  favicon: Favicon;
  // Article metadata and content
  id: ArticleId;
  title: string;
  author: string;
  html: string;
  url: string;
  isRead: boolean;
  isSaved: boolean;
  created_on_time: number;
}

export enum ReadStatus {
  Read = 0,
  Unread = 1
}

export enum SavedStatus {
  Saved = 0,
  Unsaved = 1
}

export class ArticleCls {
  private readonly id: ArticleId;
  private readonly title: string;
  private readonly author: string;
  private readonly html: string;
  private readonly url: string;
  private readonly created_on_time: number;
  private savedStatus: SavedStatus;
  private readStatus: ReadStatus;

  constructor(
    id: ArticleId, title: string, author: string, html: string,
    url: string, saved_status: SavedStatus, created_on_time: number,
    read_status: ReadStatus) {
    this.id = id;
    this.title = title;
    this.author = author;
    this.html = html;
    this.url = url;
    this.created_on_time = created_on_time;
    this.readStatus = read_status;
    this.savedStatus = saved_status;
  }

  public MarkArticle(markState: MarkState): void {
    if (markState === MarkState.Read) {
      this.readStatus = ReadStatus.Read;
    }
  }

  public GetArticleView(
    feedTitle: FeedTitle, favicon: Favicon, feedId: FeedId,
    folderId: FolderId): ArticleView {
    return {
      folder_id: folderId,
      feed_id: feedId,
      feed_title: feedTitle,
      favicon: favicon,
      id: this.id,
      title: this.title,
      author: this.author,
      html: this.html,
      url: this.url,
      isRead: (this.readStatus === ReadStatus.Read),
      isSaved: (this.savedStatus === SavedStatus.Saved),
      created_on_time: this.created_on_time,
    };
  }

  public IsRead(): boolean {
    return this.readStatus === ReadStatus.Read;
  }

  public IsSaved(): boolean {
    return this.savedStatus === SavedStatus.Saved;
  }

  public Id(): ArticleId {
    return this.id;
  }

  public static Comparator(a: ArticleCls, b: ArticleCls): number {
    // Sort by article creation time in descending order
    return b.created_on_time - a.created_on_time;
  }

  public static SortAndFilterViews(articleViews: ArticleView[]): ArticleView[] {
    const filterUnread = (a: ArticleView) => !a.isRead;
    const comparator = (a: ArticleView, b: ArticleView) =>
      b.created_on_time - a.created_on_time;
    return articleViews.filter(filterUnread).sort(comparator);
  }
}