import {ArticleCls, ArticleId, ArticleView} from "./article";
import {MarkState} from "../utils/types";
import {FolderId} from "./folder";
//import {Favicon} from "../api/fever";

/** Feed is a content source which contains zero or more articles. */
export type FeedId = string;
/** FeedTitle is the textual title of a feed. */
export type FeedTitle = string;
/** Favicon is described by a MIME type followed by a base64 encoding. */
export type Favicon = string;

export class FaviconCls {
  private readonly data: Favicon;

  public constructor(favicon: Favicon | undefined) {
    this.data = favicon === undefined ? "" : favicon;
  }

  public GetFavicon(): Favicon {
    return this.data;
  }
}

export interface FeedView {
  id: FeedId;
  folder_id: FolderId;
  favicon: FaviconCls | null;
  title: FeedTitle;
  unread_count: number;
}

export class FeedCls {
  private readonly id: FeedId;
  private readonly title: FeedTitle;
  private readonly url: string;
  private readonly site_url: string;
  private readonly is_spark: 0 | 1;
  private readonly last_updated_on_time: number;
  private unread_count: number;
  private favicon: FaviconCls | null;
  private articles: Map<ArticleId, ArticleCls>;

  constructor(id: FeedId, title: FeedTitle, url: string,
    site_url: string, is_spark: 0 | 1, last_updated_on_time: number) {
    this.id = id;
    this.favicon = null;
    this.title = title;
    this.url = url;
    this.site_url = site_url;
    this.is_spark = is_spark;
    this.last_updated_on_time = last_updated_on_time;
    this.unread_count = 0;
    this.articles = new Map<ArticleId, ArticleCls>();
  }

  public Title(): FeedTitle {
    return this.title;
  }

  public Id(): FeedId {
    return this.id;
  }

  public UnreadCount(): number {
    return this.unread_count;
  }

  public SetFavicon(favicon: FaviconCls): void {
    this.favicon = favicon;
  }

  public AddArticle(article: ArticleCls): void {
    this.articles.set(article.Id(), article);
    this.sort();
    this.unread_count += (article.IsRead() ? 0 : 1);
  }

  public MarkArticle(articleId: ArticleId, markState: MarkState): number {
    const article: ArticleCls = this.getArticleOrThrow(articleId);
    this.unread_count -= (article.IsRead() ? 0 : 1);
    article.MarkArticle(markState);
    this.unread_count += (article.IsRead() ? 0 : 1);
    return this.unread_count;
  }

  public MarkFeed(markState: MarkState): number {
    let unread: number = 0;

    this.articles.forEach((a: ArticleCls): void => {
      a.MarkArticle(markState);
      unread += (a.IsRead() ? 0 : 1);
    });

    this.unread_count = unread;
    return this.unread_count;
  }

  public GetArticleView(folderId: FolderId, articleId?: ArticleId): ArticleView[] {
    const favicon: Favicon = (this.favicon ===
      null) ? "" : this.favicon.GetFavicon();

    if (typeof articleId !== 'undefined') {
      const article: ArticleCls = this.getArticleOrThrow(articleId);
      return [article.GetArticleView(this.title, favicon, this.id, folderId)];
    }

    const entries: ArticleView[] = [];
    this.articles.forEach((a: ArticleCls): void => {
      entries.push(a.GetArticleView(this.title, favicon, this.id, folderId));
    });
    return entries;
  }

  public GetFolderFeedView(folderId: FolderId): FeedView {
    return {
      id: this.id,
      folder_id: folderId,
      favicon: this.favicon,
      title: this.title,
      unread_count: this.unread_count,
    };
  }

  public static Comparator(a: FeedCls, b: FeedCls): number {
    // Sort by title of feed
    return a.Title().localeCompare(b.Title());
  }

  private getArticleOrThrow(articleId: ArticleId): ArticleCls {
    const article: ArticleCls | undefined = this.articles.get(articleId);
    if (article === undefined) {
      throw new Error(`No article by ID: ${articleId} in feed ${this.id}`);
    }
    return article;
  }

  private sort(): void {
    const comparator = (a: [ArticleId, ArticleCls], b: [ArticleId, ArticleCls]) =>
      ArticleCls.Comparator(a[1], b[1]);
    this.articles = new Map([...this.articles.entries()].sort(comparator));
  }
}