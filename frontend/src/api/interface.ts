import { MarkState, SelectionKey, Status } from '../utils/types';
import { ContentTreeCls } from '../models/contentTree';
import GReader from './greader';

export type LoginInfo = {
  username: string;
  password: string;
};

export interface FetchAPI {
  // HandleLogin will attempt to authenticate the user.
  // This method returns a Promise that when resolved will return a boolean
  // indicating success or failure of the login attempt.
  HandleAuth(loginInfo: LoginInfo): Promise<boolean>;

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
}

export class FetchAPIFactory {
  // Create returns a concrete implementation of a FetchAPI.
  static Create(): FetchAPI {
    return new GReader();
  }
}
