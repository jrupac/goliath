import { MarkState, SelectionKey, Status } from '../utils/types';
import { ContentTreeCls } from '../models/contentTree';
import { FeedId } from '../models/feed';
import { FolderId } from '../models/folder';
import GReader from './greader';

export type LoginInfo = {
  username: string;
  password: string;
};

// AddedFeed describes a subscription the server reports after an add. The
// same shape comes back whether the add created the feed or found it already
// subscribed, so the caller has to compare the ID against what it knew of to
// tell the two apart.
export type AddedFeed = {
  id: FeedId;
  title: string;
};

// FolderSummary is one of the user's folders as the server lists them. The
// list includes folders with no feeds in them, which the content tree, being
// built from subscriptions, does not know about.
export type FolderSummary = {
  id: FolderId;
  title: string;
};

export interface FetchAPI {
  // HandleLogin will attempt to authenticate the user.
  // This method returns a Promise that when resolved will return a boolean
  // indicating success or failure of the login attempt.
  HandleAuth(loginInfo: LoginInfo): Promise<boolean>;

  // Logout ends the session the browser holds. The server revokes it rather
  // than only clearing the cookie, so the credential is withdrawn instead of
  // being left behind for whoever recovers it.
  Logout(): Promise<void>;

  // ResumeSession picks up a session the browser already holds and prepares
  // the API for use, returning false if there is no usable session.
  //
  // The credential itself is a cookie this code cannot read, so whether one
  // exists is only answerable by asking the server. Doing so also yields the
  // short-lived token that writes must carry, which is why resuming is a step
  // rather than a question.
  ResumeSession(): Promise<boolean>;

  // InitializeContent will return a promise that when resolved returns the
  // number of unread items and a fully populated map of folder IDs to folders,
  // each of which has nested feeds and articles.
  //
  // The given `cb` callback will be invoked to update status as the fetching is
  // progressing.
  InitializeContent(cb: (s: Status) => void): Promise<ContentTreeCls>;

  // MarkArticle will mark the specified article with the specified mark status.
  MarkArticle(mark: MarkState, entity: SelectionKey): Promise<Response>;

  // MarkFolder will mark the specified folder with the specified mark status.
  MarkFolder(mark: MarkState, entity: SelectionKey): Promise<Response>;

  // MarkFeed will mark the specified feed with the specified mark status.
  MarkFeed(mark: MarkState, entity: SelectionKey): Promise<Response>;

  // MarkAll will mark all items with the specified mark status.
  MarkAll(mark: MarkState, entity: SelectionKey): Promise<Response>;

  // ParseFullArticle fetches and extracts the full text of the article.
  ParseFullArticle(articleId: string): Promise<string>;

  // AddFeed subscribes to the feed at the given URL. The server fetches the
  // URL before anything is stored, so a rejection means the address is not a
  // feed or could not be reached; the returned error says which in terms a
  // user can act on. A new subscription lands unfiled.
  AddFeed(url: string): Promise<AddedFeed>;

  // RenameFeed gives the feed a title of the user's choosing, which fetching
  // then leaves alone.
  RenameFeed(feedId: FeedId, title: string): Promise<void>;

  // MoveFeed files the feed under another folder. The folder it is leaving is
  // optional: it is only ever what the client believes, and the server does
  // not need it.
  MoveFeed(
    feedId: FeedId,
    toFolderId: FolderId,
    fromFolderId?: FolderId
  ): Promise<void>;

  // UnsubscribeFeed removes the feed and every article fetched into it.
  UnsubscribeFeed(feedId: FeedId): Promise<void>;

  // ListFolders returns every folder the user has, empty ones included.
  ListFolders(): Promise<FolderSummary[]>;

  // MoveFeedToNewFolder files the feed under a folder named rather than
  // identified, which the server creates if the user has no folder by that
  // name. There is no request that only creates a folder: one is made by
  // filing something in it.
  MoveFeedToNewFolder(
    feedId: FeedId,
    folderName: string,
    fromFolderId?: FolderId
  ): Promise<void>;

  // RenameFolder gives the folder a new name. Its ID, and so everything filed
  // under it, is unchanged.
  RenameFolder(folderId: FolderId, name: string): Promise<void>;

  // DeleteFolder removes the folder. The feeds in it are not unsubscribed;
  // they move to the unfiled folder, taking their articles with them.
  DeleteFolder(folderId: FolderId): Promise<void>;
}

export class FetchAPIFactory {
  // Create returns a concrete implementation of a FetchAPI.
  static Create(): FetchAPI {
    return new GReader();
  }
}
