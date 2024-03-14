import {Article, ArticleCls, ArticleId} from "./article";
import {MarkState} from "../src/utils/types";

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
  private readonly id: FaviconId;
  private readonly favicon: Favicon;

  public constructor(id: FaviconId, favicon: Favicon) {
    this.id = id;
    this.favicon = favicon;
  }

  public GetId(): FaviconId {
    return this.id;
  }

  public GetFavicon(): Favicon {
    return this.favicon;
  }
}

export class FeedCls {
  private readonly id: FeedId;
  private readonly favicon: FaviconCls;
  private readonly title: FeedTitle;
  private readonly url: string;
  private readonly site_url: string;
  private readonly is_spark: 0 | 1;
  private readonly last_updated_on_time: number;
  private unread_count: number;
  private articles: Map<ArticleId, ArticleCls>;

  private constructor(id: FeedId, favicon: FaviconCls, title: FeedTitle, url: string,
                      site_url: string, is_spark: 0 | 1, last_updated_on_time: number,
                      unread_count: number, articles: Map<ArticleId, ArticleCls>) {
    this.id = id;
    this.favicon = favicon;
    this.title = title;
    this.url = url;
    this.site_url = site_url;
    this.is_spark = is_spark;
    this.last_updated_on_time = last_updated_on_time;
    this.unread_count = unread_count;
    this.articles = articles;
    this.sort();
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

  public AddArticles(articles: ArticleCls[]): void {
    for (const article of articles) {
      this.articles.set(article.Id(), article);
    }
    this.sort();
  }

  public AddArticle(article: ArticleCls): void {
    this.articles.set(article.Id(), article);
    this.sort();
  }

  public MarkArticle(articleId: ArticleId, markState: MarkState): number {
    const article = this.articles.get(articleId);
    if (article === undefined) {
      console.log(`Unknown article in feed ${this.id}: ${articleId}`);
      return this.unread_count;
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

  public static Comparator(a: FeedCls, b: FeedCls): number {
    return a.Title().localeCompare(b.Title());
  }

  private sort(): void {
    const comparator = (a: [ArticleId, ArticleCls], b: [ArticleId, ArticleCls]) => ArticleCls.Comparator(a[1], b[1]);
    this.articles = new Map([...this.articles.entries()].sort(comparator));
  }
}