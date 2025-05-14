import {MarkState} from "../utils/types";
import {Favicon, FeedId, FeedTitle} from "./feed";
import {FolderId} from "./folder";

/** ArticleId is the ID associated with an Article object. */
export type ArticleId = string;

/** ArticleView holds metadata associated with a displayed Article. */
export type ArticleView = Readonly<{
  // Folder metadata
  folderId: FolderId;
  // Feed metadata
  feedId: FeedId;
  feedTitle: FeedTitle;
  favicon: Favicon;
  // Article metadata
  id: ArticleId;
  title: string;
  author: string;
  html: string;
  url: string;
  creationTime: number;
  isRead: boolean;
  isSaved: boolean;
}>

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
  private readonly creationTime: number;
  private savedStatus: SavedStatus;
  private readStatus: ReadStatus;

  constructor(
    id: ArticleId, title: string, author: string, html: string,
    url: string, savedStatus: SavedStatus, creationTime: number,
    readStatus: ReadStatus) {
    this.id = id;
    this.title = title;
    this.author = author;
    this.html = html;
    this.url = url;
    this.creationTime = creationTime;
    this.readStatus = readStatus;
    this.savedStatus = savedStatus;
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
      folderId: folderId,
      feedId: feedId,
      feedTitle: feedTitle,
      favicon: favicon,
      id: this.id,
      title: this.title,
      author: this.author,
      html: this.html,
      url: this.url,
      isRead: (this.readStatus === ReadStatus.Read),
      isSaved: (this.savedStatus === SavedStatus.Saved),
      creationTime: this.creationTime,
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
    return b.creationTime - a.creationTime;
  }

  public static SortAndFilterViews(articleViews: ArticleView[]): ArticleView[] {
    const filterUnread = (a: ArticleView) => !a.isRead;
    const comparator = (a: ArticleView, b: ArticleView) =>
      b.creationTime - a.creationTime;
    return articleViews.filter(filterUnread).sort(comparator);
  }
}