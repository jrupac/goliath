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

export type ArticleSelection = [ArticleId, FeedId, FolderId];
export type FeedSelection = [FeedId, FolderId];
export type FolderSelection = FolderId;
export type AllSelection = string;
export const KeyAll: AllSelection = "ALL";

export type SelectionKey =
  ArticleSelection
  | FeedSelection
  | FolderSelection
  | AllSelection;

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
  articleMap: Map<ArticleId, ArticleType>
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

// Article data, feed title, feed favicon, feed ID, folder ID
export type ArticleListEntry = [ArticleType, string, string, FeedId, FolderId];

export function maxArticleId(a: Decimal | ArticleId, b: Decimal | ArticleId): Decimal {
  a = new Decimal(a.toString());
  b = new Decimal(b.toString());
  return a > b ? a : b;
}