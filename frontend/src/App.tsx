import 'antd/dist/antd.css';
import './App.css';
import ArticleList from './components/ArticleList';
import {Decimal} from 'decimal.js-light';
import FolderFeedList from './components/FolderFeedList';
import Layout from 'antd/lib/layout';
import Loading from './components/Loading';
import Menu from 'antd/lib/menu';
import React from 'react';
import {
  ArticleId,
  ArticleListEntry,
  ArticleSelection,
  ArticleType,
  FaviconId,
  FeedId,
  FeedSelection,
  FeedType,
  FolderId,
  FolderSelection,
  KeyAll,
  maxArticleId,
  SelectionKey,
  SelectionType,
  Status
} from "./utils/types";

// LosslessJSON needs require-style import.
const LosslessJSON = require('lossless-json');

const {Content, Footer, Sider} = Layout;

export interface AppProps {
}

interface Version {
  build_timestamp: string;
  build_hash: string;
}

export type FolderData = {
  // feeds: FeedType[];
  title: string;
  unread_count: number;
  feedMap: Map<FeedId, FeedType>
}

export interface AppState {
  buildTimestamp: string;
  buildHash: string;
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  status: Status;
  structure: Map<FolderId, FolderData>;
  unreadCount: number;
  // Fever response types
  feverFetchGroupsResponse: FeverFetchGroupsType;
  feverFetchFeedsResponse: FeverFetchFeedsType;
  feverFetchFaviconsResponse: FeverFetchFaviconsType;
  feverFetchItemsResponse: FeverFetchItemsType;
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
      buildTimestamp: "",
      buildHash: "",
      selectionKey: KeyAll,
      selectionType: SelectionType.All,
      status: Status.Start,
      structure: new Map<FolderId, FolderData>(),
      unreadCount: 0,
      // Fever response types
      feverFetchGroupsResponse: {groups: [], feeds_groups: []},
      feverFetchFeedsResponse: {feeds: [], feeds_groups: []},
      feverFetchFaviconsResponse: {favicons: []},
      feverFetchItemsResponse: {items: [], total_items: 0}

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
      const structure = new Map<FolderId, FolderData>();

      // Map of (Folder ID) -> (Feed ID).
      const folderToFeeds = new Map<FolderId, FeedId[]>();
      prevState.feverFetchGroupsResponse.feeds_groups.forEach(
        (feedGroup: FeverFeedGroupType) => {
          folderToFeeds.set(feedGroup.group_id, feedGroup.feed_ids.split(','));
        });

      // Map of all (Feed ID) -> (Feed Data).
      const globalFeedMap = new Map<FeedId, FeedType>();
      prevState.feverFetchFeedsResponse.feeds.forEach(
        (feed: FeedType) => globalFeedMap.set(feed.id, feed));

      // Map of (Feed ID) -> (list of Article Data).
      const feedToArticles = new Map<FeedId, ArticleType[]>();
      prevState.feverFetchItemsResponse.items.forEach(
        (article: ArticleType) => {
          const entry = feedToArticles.get(article.feed_id) || [];
          feedToArticles.set(article.feed_id, [...entry, article]);
        });

      // Map of (Favicon ID) -> (Favicon Data).
      const globalFaviconMap = new Map<FaviconId, string>();
      prevState.feverFetchFaviconsResponse.favicons.forEach(
        (favicon: FeverFaviconType) => {
          globalFaviconMap.set(favicon.id, favicon.data)
        });

      prevState.feverFetchGroupsResponse.groups.forEach(
        (group: FeverGroupType) => {
          const folderId = group.id as FolderId;

          const feedIdList = folderToFeeds.get(folderId);
          // Not all folders have feeds, so nothing more to be done here.
          if (feedIdList === undefined) {
            return;
          }

          // Populate feeds in this folder.
          const feeds = new Map<FeedId, FeedType>();
          feedIdList.forEach(
            (feedId: FeedId) => {
              const feed = globalFeedMap.get(feedId);
              if (feed === undefined) {
                throw new Error("Unknown feed ID: " + feedId);
              }

              // Populate articles in this feed.
              const articles = feedToArticles.get(feedId) || [];
              feed.articleMap = new Map<ArticleId, ArticleType>();
              articles.forEach((article: ArticleType) => {
                feed.articleMap.set(article.id, article);
              });

              // Compute other metadata about this feed.
              feed.unread_count = articles.reduce(
                (acc: number, a: ArticleType) => acc + (1 - a.is_read), 0);
              feed.favicon = globalFaviconMap.get(feed.favicon_id) || "";

              feeds.set(feedId, feed);
            }
          );

          // Compute other metadata about this folder.
          const unread_count = Array.from(feeds.values()).reduce(
            (acc: number, f: FeedType) => acc + f.unread_count, 0);

          const folderData: FolderData = {
            feedMap: feeds,
            title: group.title,
            unread_count: unread_count
          };

          structure.set(folderId, folderData);
        }
      );

      // Compute other global metadata.
      const unreadCount = Array.from(structure.values()).reduce(
        (acc: number, f: FolderData) => acc + f.unread_count, 0);

      return {
        structure,
        unreadCount,
        status: Status.Ready,
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
          return {
            feverFetchGroupsResponse: body,
            status: prevState.status | Status.Folder,
          } as AppState
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
          return {
            feverFetchFeedsResponse: body,
            status: prevState.status | Status.Feed,
          } as AppState
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
          return {
            feverFetchFaviconsResponse: body,
            status: prevState.status | Status.Favicon,
          } as AppState
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
          const feverFetchItemsResponse: FeverFetchItemsType = {
            total_items: itemCount,
            items: [
              ...prevState.feverFetchItemsResponse.items,
              ...body.items
            ]
          };

          // Keep fetching until we see less than the max items returned.
          // Don't update the status field until we're done.
          if (itemCount === 50) {
            body.items.forEach((item: ArticleType) => {
              // Update latest seen article ID.
              since = maxArticleId(since, item.id);
            });
            // Kick off a recursive call asynchronously. Errors will be thrown
            // within the call, so ignore the result here.
            this.fetchItems(since).finally();
            return {
              feverFetchItemsResponse
            } as AppState;
          }

          // If we're here, then this is the last fetch call so update status.
          return {
            feverFetchItemsResponse,
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
    let feverId: string;

    switch (type) {
      case SelectionType.Article:
        feverId = (entity as ArticleSelection)[0] as string;
        fetch('/fever/?api&mark=item&as=' + mark + '&id=' + feverId, {
          credentials: 'include'
        }).then(() => {
          console.log("COMPLETED ARTICLE MARK!");

          this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
            const structure = new Map(prevState.structure);

            const article = getArticleOrThrow(
              structure, entity as ArticleSelection);
            article.is_read = 1;

            const unreadCount = updateUnreadCount(structure);

            return {
              structure,
              unreadCount,
            } as AppState
          });
        }).catch((e) => console.log(e));
        break;
      case SelectionType.Feed:
        feverId = (entity as FeedSelection)[0] as string;
        fetch('/fever/?api&mark=feed&as=' + mark + '&id=' + feverId, {
          credentials: 'include'
        }).then(() => {
          this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
            const structure = new Map(prevState.structure);

            const feed = getFeedOrThrow(structure, entity as FeedSelection);
            feed.articleMap.forEach(
              (article: ArticleType) => article.is_read = 1);

            const unreadCount = updateUnreadCount(structure);

            return {
              structure,
              unreadCount,
            } as AppState
          });
        }).catch((e) => console.log(e));
        break;
      case SelectionType.Folder:
        feverId = (entity as FolderSelection) as string;
        fetch('/fever/?api&mark=group&as=' + mark + '&id=' + feverId, {
          credentials: 'include'
        }).then(() => {
          this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
            const structure = new Map(prevState.structure);

            const folder = getFolderOrThrow(
              structure, entity as FolderSelection);

            folder.feedMap.forEach(
              (feed: FeedType) => {
                feed.articleMap.forEach(
                  (article: ArticleType) => {
                    article.is_read = 1;
                  });
              });

            const unreadCount = updateUnreadCount(structure);

            return {
              structure,
              unreadCount,
            } as AppState
          });
        }).catch((e) => console.log(e));
        break;
      case SelectionType.All:
        fetch('/fever/?api&mark=group&as=' + mark + '&id=' + entity, {
          credentials: 'include'
        }).then(() => {
          // Update the read buffer and unread counts.
          this.setState((prevState: AppState): Pick<AppState, keyof AppState> => {
            const structure = new Map(prevState.structure);

            structure.forEach(
              (folder: FolderData) => {
                folder.feedMap.forEach(
                  (feed: FeedType) => {
                    feed.articleMap.forEach(
                      (article: ArticleType) => {
                        article.is_read = 1;
                      });
                  });
              });

            const unreadCount = updateUnreadCount(structure);

            return {
              structure,
              unreadCount,
            } as AppState
          });
        }).catch((e) => console.log(e));
        break;
      default:
        console.log("Unexpected enclosing type: ", type)
    }
  };

  handleSelect = (type: SelectionType, key: SelectionKey) => {
    this.setState({
        selectionKey: key,
        selectionType: type,
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
              selectionType={this.state.selectionType}
              handleSelect={this.handleSelect}/>
          </Menu>
        </Sider>
        <Layout>
          <Content>
            <ArticleList
              articleEntries={this.populateArticleListEntries()}
              enclosingKey={this.state.selectionKey}
              enclosingType={this.state.selectionType}
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

  populateArticleListEntries(): ArticleListEntry[] {
    let key: SelectionKey;
    let feedId: FeedId, folderId: FolderId;
    let folderData: FolderData, feed: FeedType, favicon: string, title: string,
      article: ArticleType;
    const entries = [] as ArticleListEntry[];

    switch (this.state.selectionType) {
      case SelectionType.Article:
        key = this.state.selectionKey as ArticleSelection;
        feedId = key[1];
        folderId = key[2];

        feed = getFeedOrThrow(this.state.structure, [feedId, folderId]);
        title = feed.title;
        favicon = feed.favicon;
        article = getArticleOrThrow(this.state.structure, key);

        entries.push([article, title, favicon, feedId, folderId]);
        break;
      case SelectionType.Feed:
        [feedId, folderId] = this.state.selectionKey as FeedSelection;

        feed = getFeedOrThrow(this.state.structure, [feedId, folderId]);
        title = feed.title;
        favicon = feed.favicon;

        feed.articleMap.forEach((article: ArticleType) => {
          entries.push([article, title, favicon, feedId, folderId]);
        });
        break;
      case SelectionType.Folder:
        folderId = this.state.selectionKey as FolderSelection;

        folderData = getFolderOrThrow(this.state.structure, folderId);

        folderData.feedMap.forEach(
          (feed: FeedType) => {
            feed.articleMap.forEach(
              (article: ArticleType) => {
                entries.push([article, feed.title, feed.favicon, feed.id, folderId])
              });
          });
        break;
      case SelectionType.All:
        this.state.structure.forEach(
          (folder: FolderData, folderId: FolderId) => {
            folder.feedMap.forEach(
              (feed: FeedType) => {
                feed.articleMap.forEach(
                  (article: ArticleType) => {
                    entries.push([article, feed.title, feed.favicon, feed.id, folderId])
                  });
              });
          });
        break
    }

    return sortArticles(entries.filter(articleIsUnread));
  }
}

function getFolderOrThrow(structure: Map<FolderId, FolderData>, folderId: FolderSelection): FolderData {
  const folderData = structure.get(folderId);
  if (folderData === undefined) {
    throw new Error("Unknown group: " + folderId);
  }
  return folderData;
}

function getFeedOrThrow(structure: Map<FolderId, FolderData>, feedSelection: FeedSelection): FeedType {
  const [feedId, folderId] = feedSelection;
  const folder = getFolderOrThrow(structure, folderId);
  const feed = folder.feedMap.get(feedId);
  if (feed === undefined) {
    throw new Error("Unknown feed: " + feedSelection);
  }
  return feed;
}

function getArticleOrThrow(structure: Map<FolderId, FolderData>, articleSelection: ArticleSelection): ArticleType {
  const [articleId, feedId, folderId] = articleSelection;
  const feed = getFeedOrThrow(structure, [feedId, folderId]);
  const article = feed.articleMap.get(articleId);
  if (article === undefined) {
    throw new Error("Unknown feed: " + articleId + " in feed " + feed.id);
  }
  return article;
}

function articleIsUnread(articleEntry: ArticleListEntry): boolean {
  const [article] = articleEntry;
  return !(article.is_read === 1) as boolean;
}

function sortArticles(articles: ArticleListEntry[]) {
  // Sort by descending time.
  return articles.sort(
    (a: ArticleListEntry, b: ArticleListEntry) =>
      b[0].created_on_time - a[0].created_on_time);
}

function updateUnreadCount(structure: Map<FolderId, FolderData>): number {
  structure.forEach((folder: FolderData) => {
    folder.feedMap.forEach((feed: FeedType) => {
      const articles = Array.from(feed.articleMap.values());
      feed.unread_count = articles.reduce(
        (acc: number, a: ArticleType) => acc + (1 - a.is_read), 0);
    });

    const feeds = Array.from(folder.feedMap.values());
    folder.unread_count = Array.from(feeds.values()).reduce(
      (acc: number, f: FeedType) => acc + f.unread_count, 0);
  });

  return Array.from(structure.values()).reduce(
    (acc: number, f: FolderData) => acc + f.unread_count, 0);
}