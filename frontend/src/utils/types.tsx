// Special-case ID for root folder.
import {Decimal} from "decimal.js-light";

export enum Status {
  Start = 0,
  Folder = 1 << 0,
  Feed = 1 << 1,
  Article = 1 << 2,
  Favicon = 1 << 3,
  Ready = 1 << 4,
}

export enum SelectionType {
  All = 0,
  Folder = 1,
  Feed = 2,
  Article = 3,
}

export interface FeedId extends String {
}

export interface FolderId extends String {
}

export interface FaviconId extends String {
}

export interface ArticleId extends String {
}

export type SelectionKeyAll = string;
export const KeyAll: SelectionKeyAll = "";
export type SelectionKey = ArticleId | FeedId | FolderId | SelectionKeyAll;

export interface FeedType {
  id: FeedId;
  favicon_id: FaviconId;
  favicon: string;
  title: string;
  url: string;
  site_url: string;
  is_spark: 0 | 1,
  last_updated_on_time: number,
  unread_count: number;
}

export interface ArticleType {
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

export function maxArticleId(a: Decimal | ArticleId, b: Decimal | ArticleId): Decimal {
  a = new Decimal(a.toString());
  b = new Decimal(b.toString());
  return a > b ? a : b;
}

export function isUnread(article: ArticleType) {
  return !(article.is_read === 1);
}

export function sortArticles(articles: ArticleType[]) {
  // Sort by descending time.
  return articles.sort(
    (a: ArticleType, b: ArticleType) => b.created_on_time - a.created_on_time);
}