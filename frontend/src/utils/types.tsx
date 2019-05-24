/** Global types for Goliath RSS */

/** Status describes which items have been fetched so far via the Fever API. */
export enum Status {
  Start = 0,
  Folder = 1 << 0,
  Feed = 1 << 1,
  Article = 1 << 2,
  Favicon = 1 << 3,
  Ready = 1 << 4,
}

/** SelectionType is an indicator for the subset of entries being processed. */
export enum SelectionType {
  All = 0,
  Folder = 1,
  Feed = 2,
  Article = 3,
}

/** SelectionKey is a unique descriptor for a corresponding SelectionType. */
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

/** MarkState describes the desired state of a mark operation. */
export type MarkState = "read";

/** Article is a single unit of content. */
export interface ArticleId extends String {
}

export interface Article {
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

/** Favicon describes the image associated with a feed. */
export interface FaviconId extends String {
}

// Favicon is described by a MIME type followed by a base64 encoding.
export type Favicon = string;

/** Feed is a content source which contains zero or more articles. */
export interface FeedId extends String {
}

export type FeedTitle = string;

export interface Feed {
  id: FeedId;
  favicon_id: FaviconId;
  favicon: Favicon;
  title: FeedTitle;
  url: string;
  site_url: string;
  is_spark: 0 | 1,
  last_updated_on_time: number,
  unread_count: number;
  articles: Map<ArticleId, Article>
}

/** Folder is a logical grouping of zero or more Feeds. */
export interface FolderId extends String {
}

export type Folder = {
  title: string;
  unread_count: number;
  feeds: Map<FeedId, Feed>
}

/** ArticleListEntry also holds metadata associated with a displayed Article. */
export type ArticleListEntry = [Article, FeedTitle, Favicon, FeedId, FolderId];

