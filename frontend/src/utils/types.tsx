/** Global types for Goliath RSS */

import {Article, ArticleId} from "../models/article";
import {FolderId} from "../models/folder";
import {Favicon, FeedId, FeedTitle} from "../models/feed";

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

export type ArticleEntry = Article;
export type FeedEntry = [ArticleEntry, FeedTitle, Favicon, FeedId];
/** ArticleListEntry also holds metadata associated with a displayed Article. */
export type ArticleListEntry = [...FeedEntry, FolderId];

/** ArticleImagePreview holds cropping information for image previews. */
export type ArticleImagePreview = {
  src: string;
  x: number;
  y: number;
  origWidth: number;
  width: number;
  height: number;
}