/** Global types for Goliath RSS */

import {ArticleId} from "../models/article";
import {FolderId} from "../models/folder";
import {FeedId} from "../models/feed";
import {Theme} from "@mui/material";

/** Well-known paths for the frontend. */
export enum GoliathPath {
  /** Default URI path for the frontend. */
  Default = "/",
  /** URI path for the login page on the frontend. */
  Login = "/login",
}

/** Theme is a list of possible theme values for the application. */
export enum GoliathTheme {
  Default = 0,
  Dark = 1,
}

export type ThemeInfo = {
  themeClasses: string,
  theme: Theme
}

/** Status describes which items have been fetched so far via the Fever API. */
export enum Status {
  Start = 0,
  LoginVerification = 1 << 0,
  Folder = 1 << 1,
  Feed = 1 << 2,
  Article = 1 << 3,
  Favicon = 1 << 4,
  Ready = 1 << 5,
}

/** SelectionType is an indicator for the subset of entries being processed. */
export enum SelectionType {
  All = 0,
  Folder = 1,
  Feed = 2,
  Article = 3,
  Saved = 4,
}

/** SelectionKey is a unique descriptor for a corresponding SelectionType. */
export type ArticleSelection = [ArticleId, FeedId, FolderId];
export type FeedSelection = [FeedId, FolderId];
export type FolderSelection = FolderId;
export type AllSelection = string;
export const KeyAll: AllSelection = "ALL";
export type SavedSelection = string;
export const KeySaved: SavedSelection = "SAVED";

export type SelectionKey =
  ArticleSelection
  | FeedSelection
  | FolderSelection
  | AllSelection
  | SavedSelection;

/** MarkState describes the desired state of a mark operation. */
export type MarkState = "read";

/** ArticleListView lists the UI options for the article list. */
export enum ArticleListView {
  Combined = 0,
  Split = 1,
}

/** ArticleImagePreview holds cropping information for image previews. */
export type ArticleImagePreview = {
  src: string;
  x: number;
  y: number;
  origWidth: number;
  width: number;
  height: number;
}