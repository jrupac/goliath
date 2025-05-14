import {MarkState, SelectionKey, Status} from "../utils/types";
import {ContentTreeCls} from "../models/contentTree";
import GReader from "./greader";

export type LoginInfo = {
  username: string,
  password: string
}

export interface FetchAPI {
  // HandleLogin will attempt to authenticate the user.
  // This method returns a Promise that when resolved will return a boolean
  // indicating success or failure of the login attempt.
  HandleAuth(loginInfo: LoginInfo): Promise<boolean>;

  // VerifyAuth returns true if a previous login attempt has been successful
  // based on the presence of some side effect (e.g., a cookie being present).
  VerifyAuth(): Promise<boolean>;

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
}

export class FetchAPIFactory {
  // Create returns a concrete implementation of a FetchAPI.
  static Create(): FetchAPI {
    return new GReader();
  }
}