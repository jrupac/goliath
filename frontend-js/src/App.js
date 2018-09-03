import 'antd/dist/antd.css';
import './App.css';
import ArticleList from './components/ArticleList.js';
import Decimal from 'decimal.js-light';
import FolderFeedList from './components/FolderFeedList';
import Layout from 'antd/lib/layout';
import Loading from './components/Loading';
import Menu from 'antd/lib/menu';
import React from 'react';

// LosslessJSON needs require-style import.
const LosslessJSON = require('lossless-json');

const {Content, Footer, Sider} = Layout;

export const Status = {
  Start: 0,
  Folder: 1 << 0,
  Feed: 1 << 1,
  Article: 1 << 2,
  Favicon: 1 << 3,
  Ready: 1 << 4,
};

export const EnclosingType = {
  All: 0,
  Folder: 1,
  Feed: 2,
  Article: 3,
};

// Special-case ID for root folder.
export const KeyAll = 0;

export default class App extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      articles: new Map(),
      buildTimestamp: "",
      buildHash: "",
      enclosingKey: KeyAll,
      enclosingType: EnclosingType.All,
      favicons: new Map(),
      feeds: new Map(),
      folderToFeeds: new Map(),
      folders: new Map(),
      readBuffer: [],
      shownArticles: [],
      status: Status.Start,
      structure: new Map(),
      unreadCount: 0,
      unreadCountMap: new Map(),
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
    // Only build the structure object once all other requests are done.
    if ((this.state.status !== Status.Ready) &&
        (this.state.status !== (
         Status.Folder | Status.Feed |
         Status.Article | Status.Favicon))) {
      return;
    }

    this.setState((prevState) => {
      const structure = new Map();

      prevState.folders.forEach((title, folderId) => {
        // Some folders may not have any feeds.
        if (!prevState.folderToFeeds.has(folderId)) {
          return;
        }
        const g = {
          feeds: prevState.folderToFeeds.get(folderId).map(
              (feedId) => {
                const f = prevState.feeds.get(feedId);
                f.favicon = prevState.favicons.get(f.favicon_id) || '';
                f.unread_count = prevState.unreadCountMap.get(feedId) || 0;
                return f;
              }),
          title: title,
        };
        g.unread_count = g.feeds.reduce((a, b) => a + b.unread_count, 0);
        structure.set(folderId, g);
      });
      const unreadCount = Array.from(structure.values()).reduce(
          (a, b) => a + b.unread_count, 0);
      return {
        status: Status.Ready,
        structure,
        unreadCount,
      };
    });
  }

  parseJson(t) {
    // Parse as Lossless numbers since values from the server are 64-bit
    // Integer, but then convert back to String for use going forward.
    return LosslessJSON.parse(t, (k, v) => {
      if (v && v.isLosslessNumber) {
        return String(v);
      }

      return v;
    });
  }

  max(a, b) {
    a = new Decimal(a);
    b = new Decimal(b);

    return a > b ? a : b;
  }

  fetchFolders() {
    fetch('/fever/?api&groups', {
      credentials: 'include'
    }).then((result) => result.text())
      .then((result) => this.parseJson(result))
      .then((body) => {
        this.setState((prevState) => {
          const folderToFeeds = prevState.folderToFeeds;
          const folders = prevState.folders;

          body.feeds_groups.forEach((e) => {
            folderToFeeds.set(e.group_id, e.feed_ids.split(','));
          });
          body.groups.forEach((group) => {
            folders.set(group.id, group.title);
          });
          return {
            folders: folders,
            folderToFeeds,
            status: prevState.status | Status.Folder,
          };
        }, this.buildStructure);
      }).catch((e) => console.log(e));
  }

  fetchFeeds() {
    fetch('/fever/?api&feeds', {
      credentials: 'include'
    }).then((result) => result.text())
    .then((result) => this.parseJson(result))
    .then((body) => {
      this.setState((prevState) => {
        const feeds = prevState.feeds;
        body.feeds.forEach((feed) => {
          feeds.set(feed.id, {
            id: feed.id,
            favicon_id: feed.favicon_id,
            favicon: '',
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
        };
      }, this.buildStructure);
    }).catch((e) => console.log(e));
  }

  fetchItems(sinceId) {
    var since, itemUri;
    if (sinceId && sinceId instanceof Decimal) {
      since = sinceId;
      itemUri = '/fever/?api&items&since_id=' + since.toString();
    } else {
      since = new Decimal(0);
      itemUri = '/fever/?api&items';
    }
    fetch(itemUri, {
      credentials: 'include'
    }).then((result) => result.text())
    .then((result) => this.parseJson(result))
    .then((body) => {
      const itemCount = body.items.length;
      this.setState((prevState) => {
        const articles = prevState.articles;
        const unreadCountMap = prevState.unreadCountMap;

        body.items.forEach((item) => {
          since = this.max(since, item.id);
          const feed_id = item.feed_id;

          unreadCountMap.set(
                feed_id, (unreadCountMap.get(feed_id) || 0) + 1);
          articles.set(item.id, {
            id: item.id,
            feed_id: feed_id,
            title: item.title,
            url: item.url,
            html: item.html,
            is_read: item.is_read,
            created_on_time: item.created_on_time
          });
        });
        // Keep fetching until we see less than the max items returned.
        // Don't update the status field until we're done.
        if (itemCount === 50) {
          this.fetchItems(since);
          return {
            articles,
            unreadCountMap,
          };
        }
        return {
          articles,
          unreadCountMap,
          shownArticles: Array.from(articles.values()),
          status: prevState.status | Status.Article,
        };
      }, this.buildStructure);
    }).catch((e) => console.log(e));
  }

  fetchFavicons() {
    fetch('/fever/?api&favicons', {
      credentials: 'include'
    }).then((result) => result.text())
    .then((result) => this.parseJson(result))
    .then((body) => {
      this.setState((prevState) => {
        const favicons = prevState.favicons;
        body.favicons.forEach((favicon) => {
          favicons.set(favicon.id, favicon.data);
        });
        return {
          favicons,
          status: prevState.status | Status.Favicon,
        };
      }, this.buildStructure);
    }).catch((e) => console.log(e));
  }

  fetchVersion() {
    fetch('/version', {
      credentials: 'include'
    }).then((result) => result.text())
    .then((result) => this.parseJson(result))
    .then((body) => {
      this.setState({
        buildTimestamp: body.build_timestamp,
        buildHash: body.build_hash
      })
    }).catch((e) => console.log(e));
  }

  handleMark = (mark, entity, type) => {
    switch(type) {
    case EnclosingType.Article:
      fetch('/fever/?api&mark=item&as=' + mark + '&id=' + entity, {
        credentials: 'include'
      }).then(() => {
        // Update the read buffer and unread counts.
        this.setState((prevState) => {
          let feedId = prevState.articles.get(entity).feed_id;
          let readBuffer = [...prevState.readBuffer, entity];
          const unreadCountMap = new Map(prevState.unreadCountMap);
          unreadCountMap.set(feedId, unreadCountMap.get(feedId) - 1);
          return {
            readBuffer: readBuffer,
            unreadCountMap: unreadCountMap
          }
        }, this.buildStructure);
      }).catch((e) => console.log(e));
      break;
    case EnclosingType.Feed:
      fetch('/fever/?api&mark=feed&as=' + mark + '&id=' + entity, {
        credentials: 'include'
      }).then(() => {
        // Update the read buffer and unread counts.
        this.setState((prevState) => {
          let articles = new Map(prevState.articles);
          let ids = [];
          articles.forEach((v, k) => {
            if (!v.is_read && v.feed_id === entity) {
              ids.push(k);
            }
          });

          let readBuffer = [...prevState.readBuffer, ...ids];
          let unreadCountMap = new Map(prevState.unreadCountMap);
          unreadCountMap.set(entity, 0);
          return {
            readBuffer: readBuffer,
            unreadCountMap: unreadCountMap
          }
        }, this.buildStructure);
      }).catch((e) => console.log(e));
      break;
    case EnclosingType.Folder:
      fetch('/fever/?api&mark=group&as=' + mark + '&id=' + entity, {
        credentials: 'include'
      }).then(() => {
        // Update the read buffer and unread counts.
        this.setState((prevState) => {
          const feeds = prevState.folderToFeeds.get(entity) || [];
          let articles = new Map(prevState.articles);
          let ids = new Map();
          articles.forEach((v, k) => {
            if (!v.is_read && feeds.indexOf(v.feed_id) >= 0) {
              ids.set(k, (ids.get(k) || 0) + 1);
            }
          });

          let readBuffer = [...prevState.readBuffer, ...Array.from(ids.keys())];
          let unreadCountMap = new Map(prevState.unreadCountMap);
          ids.forEach((v, k) => {
            unreadCountMap.set(k, unreadCountMap.get(k) - v);
          });
          return {
            readBuffer: readBuffer,
            unreadCountMap: unreadCountMap
          }
        }, this.buildStructure);
      }).catch((e) => console.log(e));
      break;
    case EnclosingType.All:
      fetch('/fever/?api&mark=group&as=' + mark + '&id=' + entity, {
        credentials: 'include'
      }).then(() => {
        // Update the read buffer and unread counts.
        this.setState((prevState) => {
          let articles = new Map(prevState.articles);
          let ids = [];
          articles.forEach((v, k) => { if (!v.is_read) { ids.push(k); } });

          let readBuffer = [...prevState.readBuffer, ...ids];
          let unreadCountMap = new Map(prevState.unreadCountMap);
          unreadCountMap.forEach((v, k) => {
            unreadCountMap.set(k, 0);
          });
          return {
            readBuffer: readBuffer,
            unreadCountMap: unreadCountMap
          }
        }, this.buildStructure);
      }).catch((e) => console.log(e));
      break;
    default:
      console.log("Unexpected enclosing type: ", type)
    }
  };

  handleSelect = (type, key) => {
    this.setState((prevState) => {
      // Apply read buffer to articles in state.
      var articles = new Map(prevState.articles);
      prevState.readBuffer.forEach((e) => articles.get(e)['is_read'] = true);
      var shownArticles = Array.from(articles.values());

      // TODO: Consider having a "read" list too.
      switch (type) {
      case EnclosingType.All:
        shownArticles = shownArticles.filter(this.checkUnread);
        break;
      case EnclosingType.Feed:
        shownArticles = shownArticles.filter(
            (e) => e.feed_id === key && this.checkUnread(e));
        break;
      case EnclosingType.Folder:
        // Some folder may not have feeds.
        const feeds = prevState.folderToFeeds.get(key) || [];
        // Consider using a Set() polyfill to speed this up.
        shownArticles = shownArticles.filter(
            (e) => feeds.indexOf(e.feed_id) > -1 && this.checkUnread(e));
        break;
      default:
        console.log("Unexpected enclosing type: ", type)
      }

      return {
        articles: articles,
        enclosingKey: key,
        enclosingType: type,
        readBuffer: [],
        shownArticles: shownArticles,
      }
    });
  };

  checkUnread(article) {
    return !article.is_read;
  }

  sortArticles(articles) {
    // Sort by descending time.
    return articles.sort((a, b) => b.created_on_time - a.created_on_time);
  }

  render() {
    if (this.state.status !== Status.Ready) {
      return <Loading status={this.state.status} />;
    }

    if (this.state.unreadCount === 0) {
      document.title = 'Goliath RSS';
    } else {
      document.title = `(${this.state.unreadCount})  Goliath RSS`;
    }
    return (
      <Layout className="App">
        <Sider width={300}>
          <div className="logo" >
            Goliath
          </div>
          <Menu mode="inline" theme="dark">
            <FolderFeedList
              tree={this.state.structure}
              unreadCount={this.state.unreadCount}
              selectedKey={this.state.enclosingKey}
              handleSelect={this.handleSelect} />
          </Menu>
        </Sider>
        <Layout>
          <Content>
            <ArticleList
                articles={this.sortArticles(this.state.shownArticles)}
                enclosingKey={this.state.enclosingKey}
                enclosingType={this.state.enclosingType}
                feeds={this.state.feeds}
                handleMark={this.handleMark}
                handleSelect={this.handleSelect} />
            <Footer>
              Goliath RSS
              <br />
              Built at: {this.state.buildTimestamp}
              <br />
              {this.state.buildHash}
            </Footer>
          </Content>
        </Layout>
      </Layout>
    );
  }
}
