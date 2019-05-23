import 'antd/dist/antd.css';
import './App.css';
import ArticleList from './components/ArticleList';
import {Decimal} from 'decimal.js-light';
import FolderFeedList from './components/FolderFeedList';
import Layout from 'antd/lib/layout';
import Loading from './components/Loading';
import Menu from 'antd/lib/menu';
import React from 'react';

// LosslessJSON needs require-style import.
const LosslessJSON = require('lossless-json');

const {Content, Footer, Sider} = Layout;

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

export interface AppProps {
}

interface Version {
  build_timestamp: string;
  build_hash: string;
}

export interface FeedId extends String {
}

export interface FolderId extends String {
}

export interface FaviconId extends String {
}

export interface ArticleId extends String {
}

// Special-case ID for root folder.
type SelectionKeyAll = string;
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

export type StructureValue = {
  feeds: FeedType[];
  title: string;
  unread_count: number;
}

export interface AppState {
  articles: Map<ArticleId, ArticleType>;
  buildTimestamp: string;
  buildHash: string;
  selectionKey: SelectionKeyAll;
  selectionType: SelectionType;
  favicons: Map<FaviconId, string>;
  feeds: Map<FeedId, FeedType>;
  folderToFeeds: Map<FolderId, FeedId[]>;
  folders: Map<FolderId, string>;
  readBuffer: ArticleId[];
  shownArticles: ArticleType[];
  status: Status;
  structure: Map<FolderId, StructureValue>;
  unreadCount: number;
  unreadCountMap: Map<FeedId, number>;
}

// The following several interfaces conform to the Fever API.
interface FeverFeedGroupType {
  group_id: string;
  feed_ids: string;
}

interface FeverGroupType {
  id: string;
  title: string;
}

interface FeverFaviconType {
  id: FaviconId;
  data: string;
}

interface FeverFetchGroupsType {
  groups: Array<FeverGroupType>;
  feeds_groups: Array<FeverFeedGroupType>;
}

interface FeverFetchFeedsType {
  feeds: Array<FeedType>;
  feeds_groups: Array<FeverFeedGroupType>;
}

interface FeverFetchFaviconsType {
  favicons: Array<FeverFaviconType>;
}

interface FeverFetchItemsType {
  items: Array<ArticleType>;
  total_items: number;
}

export default class App extends React.Component<AppProps, AppState> {
  constructor(props: AppProps) {
    super(props);
    this.state = {
      articles: new Map<ArticleId, ArticleType>(),
      buildTimestamp: "",
      buildHash: "",
      selectionKey: KeyAll,
      selectionType: SelectionType.All,
      favicons: new Map<FaviconId, string>(),
      feeds: new Map<FeedId, FeedType>(),
      folderToFeeds: new Map<FolderId, FeedId[]>(),
      folders: new Map<FolderId, string>(),
      readBuffer: [] as ArticleId[],
      shownArticles: [] as ArticleType[],
      status: Status.Start,
      structure: new Map<FolderId, StructureValue>(),
      unreadCount: 0,
      unreadCountMap: new Map<FeedId, number>(),
    };
  }

  componentDidMount() {
    Promise.all(
      [this.fetchFolders(), this.fetchFeeds(), this.fetchItems(),
        this.fetchFavicons(), this.fetchVersion()])
      .then(() => {
        console.log('Completed all requests to server.');
      });
  }

  buildStructure() {
    // Only build the structure once everything else is fetched.
    if ((this.state.status !== Status.Ready) &&
      (this.state.status !==
        (Status.Folder | Status.Feed | Status.Article | Status.Favicon))) {
      return;
    }

    this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
      const structure = new Map<FolderId, StructureValue>();

      prevState.folders.forEach((title: string, folderId: FolderId) => {
        const feedIds = prevState.folderToFeeds.get(folderId);

        if (feedIds === undefined) {
          // Some folders may not have any feeds, which is not an error.
          return;
        } else {
          const feeds = feedIds.map((feedId: FeedId): FeedType => {
            const feed = prevState.feeds.get(feedId);

            if (feed === undefined) {
              throw new Error("BUG: Feed: " + feedId +
                " not found in feed list.");
            } else {
              feed.favicon = prevState.favicons.get(feed.favicon_id) || '';
              feed.unread_count = prevState.unreadCountMap.get(feedId) || 0;
              return feed;
            }
          });
          const unread_count = feeds.reduce(
            (a: number, b: FeedType): number => a + b.unread_count, 0);

          structure.set(folderId, {
            feeds: feeds,
            title: title,
            unread_count: unread_count
          });
        }
      });
      const unreadCount = Array.from(structure.values()).reduce(
        (a, b) => a + b.unread_count, 0);
      return {
        status: Status.Ready,
        structure,
        unreadCount,
      } as AppState;
    });
  }

  parseJson(text: string): any {
    // Parse as Lossless numbers since values from the server are 64-bit
    // Integer, but then convert back to String for use going forward.
    return LosslessJSON.parse(text, (k: string, v: any) => {
      if (v && v.isLosslessNumber) {
        return String(v);
      }
      return v;
    });
  }

  fetchFolders() {
    return fetch('/fever/?api&groups', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: FeverFetchGroupsType) => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const folderToFeeds = prevState.folderToFeeds;
          const folders = prevState.folders;

          body.feeds_groups.forEach((e: FeverFeedGroupType) => {
            folderToFeeds.set(e.group_id, e.feed_ids.split(','));
          });
          body.groups.forEach((group: FeverGroupType) => {
            folders.set(group.id, group.title);
          });

          return {
            folders,
            folderToFeeds,
            status: prevState.status | Status.Folder,
          } as AppState;
        }, this.buildStructure);
      }).catch((e) => console.log(e));
  }

  fetchFeeds() {
    return fetch('/fever/?api&feeds', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: FeverFetchFeedsType) => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const feeds = prevState.feeds;

          body.feeds.forEach((feed: FeedType) => {
            feeds.set(feed.id, {
              id: feed.id,
              favicon_id: feed.favicon_id,
              favicon: "",
              title: feed.title,
              url: feed.url,
              site_url: feed.site_url,
              is_spark: feed.is_spark,
              last_updated_on_time: feed.last_updated_on_time,
              unread_count: 0
            });
          });

          return {
            feeds,
            status: prevState.status | Status.Feed,
          } as AppState;
        }, this.buildStructure);
      }).catch((e) => console.log(e));
  }

  fetchFavicons() {
    return fetch('/fever/?api&favicons', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: FeverFetchFaviconsType) => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const favicons = prevState.favicons;

          body.favicons.forEach((favicon: FeverFaviconType) => {
            favicons.set(favicon.id, favicon.data);
          });

          return {
            favicons,
            status: prevState.status | Status.Favicon,
          } as AppState;
        }, this.buildStructure);
      }).catch((e) => console.log(e));
  }

  fetchItems(sinceId?: Decimal) {
    let since: Decimal, itemUri: string;

    if (sinceId !== undefined) {
      since = sinceId;
      itemUri = '/fever/?api&items&since_id=' + since.toString();
    } else {
      since = new Decimal(0);
      itemUri = '/fever/?api&items';
    }

    return fetch(itemUri, {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: FeverFetchItemsType) => {
        this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
          const itemCount = body.items.length;
          const articles = prevState.articles;
          const unreadCountMap = prevState.unreadCountMap;

          body.items.forEach((item: ArticleType) => {
            const feed_id = item.feed_id;

            unreadCountMap.set(
              feed_id, (unreadCountMap.get(feed_id) || 0) + 1);
            articles.set(item.id, item);

            // Update latest seen article ID.
            since = maxArticleId(since, item.id);
          });

          // Keep fetching until we see less than the max items returned.
          // Don't update the status field until we're done.
          if (itemCount === 50) {
            // Kick off a recursive call asynchronously. Errors will be thrown
            // within the call, so ignore the result here.
            this.fetchItems(since).finally();
            return {
              articles,
              unreadCountMap,
            } as AppState;
          }

          // If we get here, then we are at the last fetch call.
          return {
            articles,
            unreadCountMap,
            shownArticles: Array.from(articles.values()),
            status: prevState.status | Status.Article,
          } as AppState;
        }, this.buildStructure);
      }).catch((e) => console.log(e));
  }

  fetchVersion() {
    return fetch('/version', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body: Version) => {
        this.setState({
          buildTimestamp: body.build_timestamp,
          buildHash: body.build_hash
        })
      }).catch((e) => console.log(e));
  }

  handleMark = (mark: "read", entity: SelectionKey, type: SelectionType) => {
    switch (type) {
      case SelectionType.Article:
        fetch('/fever/?api&mark=item&as=' + mark + '&id=' + entity, {
          credentials: 'include'
        }).then(() => {
          this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
            const readBuffer = [...prevState.readBuffer, entity];
            const unreadCountMap = new Map(prevState.unreadCountMap);

            const article = prevState.articles.get(entity);
            if (article === undefined) {
              throw new Error("Could not find article with ID: " + entity);
            }

            const feedId = article.feed_id;

            const prevFeedUnreadCount = unreadCountMap.get(feedId);
            if (prevFeedUnreadCount === undefined) {
              throw new Error("Could not find unread count entry for feed: " +
                feedId);
            }

            unreadCountMap.set(feedId, prevFeedUnreadCount - 1);

            return {
              readBuffer,
              unreadCountMap
            } as AppState
          }, this.buildStructure);
        }).catch((e) => console.log(e));
        break;
      case SelectionType.Feed:
        fetch('/fever/?api&mark=feed&as=' + mark + '&id=' + entity, {
          credentials: 'include'
        }).then(() => {
          this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
            const articles = prevState.articles;
            const newReadIds: ArticleId[] = [];

            // Add all articles in the given feed that are not already marked
            // as read to the read buffer.
            articles.forEach((v: ArticleType, k: ArticleId) => {
              if (v.feed_id === entity && isUnread(v)) {
                newReadIds.push(k);
              }
            });
            const readBuffer = [...prevState.readBuffer, ...newReadIds];

            // Directly mark the given feed as having no unread articles.
            const unreadCountMap = new Map(prevState.unreadCountMap);
            unreadCountMap.set(entity, 0);

            return {
              readBuffer,
              unreadCountMap
            } as AppState
          }, this.buildStructure);
        }).catch((e) => console.log(e));
        break;
      case SelectionType.Folder:
        fetch('/fever/?api&mark=group&as=' + mark + '&id=' + entity, {
          credentials: 'include'
        }).then(() => {
          this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
            // Not all folders have feeds.
            const feeds = prevState.folderToFeeds.get(entity) || [];
            const articles = prevState.articles;
            const newReadIds: ArticleId[] = [];

            // Add all articles in the given folder that are not already marked
            // as read to the read buffer.
            articles.forEach((v: ArticleType, k: ArticleId) => {
              if (feeds.indexOf(v.feed_id) > -1 && isUnread(v)) {
                newReadIds.push(k)
              }
            });
            const readBuffer = [...prevState.readBuffer, ...newReadIds];

            // Directly mark all feeds in the given folder as having no unread
            // articles.
            const unreadCountMap = new Map(prevState.unreadCountMap);
            feeds.forEach((e) => {
              unreadCountMap.set(e, 0);
            });

            return {
              readBuffer,
              unreadCountMap
            } as AppState
          }, this.buildStructure);
        }).catch((e) => console.log(e));
        break;
      case SelectionType.All:
        fetch('/fever/?api&mark=group&as=' + mark + '&id=' + entity, {
          credentials: 'include'
        }).then(() => {
          // Update the read buffer and unread counts.
          this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
            const articles = prevState.articles;
            const newReadIds: ArticleId[] = [];

            // All all unread articles to the read buffer.
            articles.forEach((v: ArticleType, k: ArticleId) => {
              if (isUnread(v)) {
                newReadIds.push(k);
              }
            });
            const readBuffer = [...prevState.readBuffer, ...newReadIds];

            // Directly mark all feeds as having no unread articles.
            const unreadCountMap = new Map(prevState.unreadCountMap);
            unreadCountMap.forEach((v, k) => {
              unreadCountMap.set(k, 0);
            });

            return {
              readBuffer,
              unreadCountMap
            } as AppState
          }, this.buildStructure);
        }).catch((e) => console.log(e));
        break;
      default:
        console.log("Unexpected enclosing type: ", type)
    }
  };

  handleSelect = (type: SelectionType, key: SelectionKey) => {
    this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
      // Apply read buffer to articles in state.
      const articles = new Map(prevState.articles);
      prevState.readBuffer.forEach((e: ArticleId) => {
        const article = articles.get(e);
        if (article !== undefined) {
          article.is_read = 1;
        }
      });

      // Apply the given selector to the list of articles.
      // TODO: Consider having a "read" list too.
      let shownArticles = Array.from(articles.values());
      switch (type) {
        case SelectionType.Feed:
          shownArticles = shownArticles.filter(
            (e: ArticleType) => e.feed_id === key && isUnread(e));
          break;
        case SelectionType.Folder:
          // Some folder may not have feeds.
          const feeds = prevState.folderToFeeds.get(key) || [];
          // TODO: Consider using a Set() polyfill to speed this up.
          shownArticles = shownArticles.filter(
            (e: ArticleType) => feeds.indexOf(e.feed_id) > -1 && isUnread(e));
          break;
        case SelectionType.All:
          shownArticles = shownArticles.filter(isUnread);
          break;
        default:
          console.log("Unexpected enclosing type: ", type)
      }

      return {
        articles,
        selectionKey: key,
        selectionType: type,
        // Empty out readBuffer as we've marked everything there as read.
        readBuffer: [] as ArticleId[],
        shownArticles,
      } as AppState
    });
  };

  render() {
    if (this.state.status !== Status.Ready) {
      return <Loading status={this.state.status}/>;
    }

    if (this.state.unreadCount === 0) {
      document.title = 'Goliath RSS';
    } else {
      document.title = `(${this.state.unreadCount})  Goliath RSS`;
    }
    return (
      <Layout className="App">
        <Sider width={300}>
          <div className="logo">
            Goliath
          </div>
          <Menu mode="inline" theme="dark">
            <FolderFeedList
              tree={this.state.structure}
              unreadCount={this.state.unreadCount}
              selectedKey={this.state.selectionKey}
              handleSelect={this.handleSelect}/>
          </Menu>
        </Sider>
        <Layout>
          <Content>
            <ArticleList
              articles={sortArticles(this.state.shownArticles)}
              enclosingKey={this.state.selectionKey}
              enclosingType={this.state.selectionType}
              feeds={this.state.feeds}
              handleMark={this.handleMark}
              selectAllCallback={() => this.handleSelect(SelectionType.All, KeyAll)}/>
            <Footer>
              Goliath RSS
              <br/>
              Built at: {this.state.buildTimestamp}
              <br/>
              {this.state.buildHash}
            </Footer>
          </Content>
        </Layout>
      </Layout>
    );
  }
}

function maxArticleId(a: Decimal | ArticleId, b: Decimal | ArticleId): Decimal {
  a = new Decimal(a.toString());
  b = new Decimal(b.toString());
  return a > b ? a : b;
}

function isUnread(article: ArticleType) {
  return !(article.is_read === 1);
}

function sortArticles(articles: ArticleType[]) {
  // Sort by descending time.
  return articles.sort(
    (a: ArticleType, b: ArticleType) => b.created_on_time - a.created_on_time);
}