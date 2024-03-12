import {ContentTree, SelectionKey, Status} from "../utils/types";

export type LoginInfo = {
  username: string,
  password: string
}

export interface FetchAPI {
  // HandleLogin will attempt to authenticating the user.
  // This method returns a Promise that when resolved will return a boolean
  // indicating success or failure of the login attempt.
  HandleLogin(loginInfo: LoginInfo): Promise<boolean>;

  // InitializeContent will return a promise that when resolved returns the
  // number of unread items and a fully populated map of folder IDs to folders,
  // each of which has nested feeds and articles.
  //
  // The given `cb` callback will be invoked to update status as the fetching is
  // progressing.
  InitializeContent(cb: (s: Status) => void): Promise<[number, ContentTree]>;

  // MarkArticle will mark the specified article with the specified mark status.
  MarkArticle(mark: string, entity: SelectionKey): Promise<Response>;

  // MarkFolder will mark the specified folder with the specified mark status.
  MarkFolder(mark: string, entity: SelectionKey): Promise<Response>;

  // MarkFeed will mark the specified feed with the specified mark status.
  MarkFeed(mark: string, entity: SelectionKey): Promise<Response>;

  // MarkAll will mark all items with the specified mark status.
  MarkAll(mark: string, entity: SelectionKey): Promise<Response>;
}