/** Global types for Goliath RSS */

/** Well-known paths for the frontend. */
export enum GoliathPath {
  /** Default URI path for the frontend. */
  Default = "/",
  /** URI path for the login page on the frontend. */
  Login = "/login",
}

/** Theme is a list of possible theme values for the application. */
export enum Theme {
  Default = 0,
  Dark = 1,
}

/** VersionData describes metadata about the backend version. */
export type VersionData = {
  build_timestamp: string,
  build_hash: string
}

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

/** ArticleListView lists the UI options for the article list. */
export enum ArticleListView {
  Combined = 0,
  Split = 1,
}

/** ArticleId is the ID associated with an Article object. */
export type ArticleId = string;

/** FaviconId is the ID of the favicon associated with a feed. */
export type FaviconId = string;

// Favicon is described by a MIME type followed by a base64 encoding.
export type Favicon = string;

/** Feed is a content source which contains zero or more articles. */
export type FeedId = string;

/** FeedTitle is the textual title of a feed. */
export type FeedTitle = string;

/** FolderId is the ID of a Folder object. */
export type FolderId = string;

/** Article is a single unit of content. */
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

/** Folder is a logical grouping of zero or more Feeds. */
export type Folder = {
  title: string;
  unread_count: number;
  feeds: Map<FeedId, Feed>;
}

/** ContentTree is a map of FolderID -> Folder with all associated content. */
export type ContentTree = Map<FolderId, Folder>;

/** Create a new, empty ContentTree object. */
export const initContentTree = () => new Map<FolderId, Folder>();

/** ArticleListEntry also holds metadata associated with a displayed Article. */
export type ArticleListEntry = [Article, FeedTitle, Favicon, FeedId, FolderId];

/** ArticleImagePreview holds cropping information for image previews. */
export type ArticleImagePreview = {
  src: string;
  x: number;
  y: number;
  origWidth: number;
  width: number;
  height: number;
}