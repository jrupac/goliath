import {Article, ArticleCls, ArticleId, ReadStatus} from "./article";
import {FeedEntry, MarkState} from "../utils/types";

/** Feed is a content source which contains zero or more articles. */
export type FeedId = string;
/** FeedTitle is the textual title of a feed. */
export type FeedTitle = string;

/** FaviconId is the ID of the favicon associated with a feed. */
export type FaviconId = string;
// Favicon is described by a MIME type followed by a base64 encoding.
export type Favicon = string;

export interface Feed {
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

export class FaviconCls {
  private readonly data: Favicon;

  public constructor(favicon: Favicon) {
    this.data = favicon;
  }

  public GetFavicon(): Favicon {
    return this.data;
  }
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
    this.unread_count += (article.ReadStatus() === ReadStatus.Unread) ? 1 : 0;
  }

  public MarkArticle(articleId: ArticleId, markState: MarkState): number {
    const article = this.articles?.get(articleId);
    if (article === undefined) {
      throw new Error(`No article by ID: ${articleId} in feed ${this.id}`);
    }

    this.unread_count -= article.ReadStatus();
    this.unread_count += article.MarkArticle(markState);
    return this.unread_count;
  }

  public MarkFeed(markState: MarkState): number {
    let unread: number = 0;

    this.articles.forEach((a: ArticleCls): void => {
      unread += a.MarkArticle(markState);
    });

    this.unread_count = unread;
    return this.unread_count;
  }

  public GetArticleEntry(articleId: ArticleId): FeedEntry {
    const article: ArticleCls = this.getArticleOrThrow(articleId);
    const favicon: Favicon = (this.favicon ===
      null) ? "" : this.favicon.GetFavicon();
    return [article.GetArticleEntry(), this.title, favicon, this.id];
  }

  public GetFeedEntry(): FeedEntry[] {
    const entries: FeedEntry[] = [];
    const favicon: Favicon = (this.favicon ===
      null) ? "" : this.favicon.GetFavicon();

    this.articles.forEach((a: ArticleCls): void => {
      entries.push([a.GetArticleEntry(), this.title, favicon, this.id]);
    });

    return entries;
  }

  public static Comparator(a: FeedCls, b: FeedCls): number {
    return a.Title().localeCompare(b.Title());
  }

  private getArticleOrThrow(articleId: ArticleId): ArticleCls {
    const article = this.articles.get(articleId);
    if (article === undefined) {
      throw new Error(`No article by ID: ${articleId} in feed ${this.id}`);
    }
    return article;
  }

  private sort(): void {
    const comparator = (a: [ArticleId, ArticleCls], b: [ArticleId, ArticleCls]) => ArticleCls.Comparator(a[1], b[1]);
    this.articles = new Map([...this.articles.entries()].sort(comparator));
  }
}