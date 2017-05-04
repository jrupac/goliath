import 'antd/dist/antd.css';
import './App.css';
import ArticleList from './components/ArticleList.js';
import Decimal from 'decimal.js-light';
import FolderFeedList from './components/FolderFeedList';
import Layout from 'antd/lib/layout';
import Loading from './components/Loading';
import LosslessJSON from 'lossless-json';
import Menu from 'antd/lib/menu';
import React from 'react';

const {Content, Footer, Sider} = Layout;

export const Status = {
  Start: 0,
  Folder: 1 << 0,
  Feed: 1 << 1,
  Article: 1 << 2,
  Favicon: 1 << 3,
  Ready: 1 << 4,
};

export default class App extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      articles: [],
      favicons: new Map(),
      feeds: new Map(),
      folderToFeeds: new Map(),
      folders: new Map(),
      selectedKey: null,
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
        this.fetchFavicons()])
        .then(() => {
          console.log('Completed all requests to server.');
        });
  }

  buildStructure() {
    // Only build the structure object once all other requests are done.
    if (this.state.status !== (
        Status.Folder | Status.Feed |
        Status.Article | Status.Favicon)) {
      return;
    }

    this.setState((prevState) => {
      const structure = new Map();

      prevState.folders.forEach((title, groupId) => {
        // Some folders may not have any feeds.
        if (!prevState.folderToFeeds.has(groupId)) {
          return;
        }
        const g = {
          'feeds': prevState.folderToFeeds.get(groupId).map(
              (feedId) => {
                const f = prevState.feeds.get(feedId);
                f.favicon = prevState.favicons.get(f.favicon_id) || '';
                f.unread_count = prevState.unreadCountMap.get(feedId) || 0;
                return f;
              }),
          title,
        };
        g.unread_count = g.feeds.reduce((a, b) => a + b.unread_count, 0);
        structure.set(groupId, g);
      });
      const unreadCount = Array.from(structure.values()).reduce(
          (a, b) => a + b.unread_count, 0);
      return {
        shownArticles: prevState.articles,
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
    fetch('/fever/?api&groups')
        .then((result) => result.text())
        .then((result) => this.parseJson(result))
        .then((body) => {
          this.setState((prevState) => {
            const folderToFeeds = prevState.folderToFeeds;
            const groups = prevState.groups;

            body.feeds_groups.forEach((e) => {
              folderToFeeds.set(e.group_id, e.feed_ids.split(','));
            });
            body.groups.forEach((group) => {
              groups.set(group.id, group.title);
            });
            return {
              folders: groups,
              folderToFeeds,
              status: prevState.status | Status.Folder,
            };
          }, this.buildStructure);
        }
        );
  }

  fetchFeeds() {
    fetch('/fever/?api&feeds')
        .then((result) => result.text())
        .then((result) => this.parseJson(result))
        .then((body) => {
          this.setState((prevState) => {
            const feeds = prevState.feeds;
            body.feeds.forEach((feed) => {
              feeds.set(feed.id, {
                'id': feed.id,
                'favicon_id': feed.favicon_id,
                'favicon': '',
                'title': feed.title,
                'url': feed.url,
                'site_url': feed.site_url,
                'is_spark': feed.is_spark,
                'last_updated_on_time': feed.last_updated_on_time,
                'unread_count': 0,
              });
            });
            return {
              feeds,
              status: prevState.status | Status.Feed,
            };
          }, this.buildStructure);
        }
        );
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
    fetch(itemUri)
        .then((result) => result.text())
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
              articles.push({
                'id': item.id,
                feed_id,
                'title': item.title,
                'url': item.url,
                'html': item.html,
                'is_read': item.is_read,
                'created_on_time': item.created_on_time,
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
              status: prevState.status | Status.Article,
            };
          }, this.buildStructure);
        }
        );
  }

  fetchFavicons() {
    fetch('/fever/?api&favicons')
        .then((result) => result.text())
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
        }
        );
  }

  sortArticles(articles) {
    return articles.sort((a, b) => b.created_on_time - a.created_on_time);
  }

  handleSelect = (type, key) => {
    if (type === 'all') {
      this.setState((prevState) => ({
        shownArticles: prevState.articles,
        selectedKey: key,
      }));
    } else if (type === 'feed') {
      this.setState((prevState) => ({
        shownArticles: prevState.articles.filter((e) => e.feed_id === key),
        selectedKey: key,
      }));
    } else if (type === 'folder') {
      this.setState((prevState) => {
        const feeds = prevState.folderToFeeds.get(key) || [];


        return {
          shownArticles: prevState.articles.filter(
              (e) => feeds.indexOf(e.feed_id) !== -1),
          selectedKey: key,
        };
      });
    }
  };

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
        <Sider width={250}>
          <div className="logo" >
            Goliath
          </div>
          <Menu mode="inline" theme="dark">
            <FolderFeedList
              tree={this.state.structure}
              unreadCount={this.state.unreadCount}
              selectedKey={this.state.selectedKey}
              handleSelect={this.handleSelect} />
          </Menu>
        </Sider>
        <Layout>
          <Content>
            <ArticleList
                articles={this.sortArticles(this.state.shownArticles)} />
            <Footer>
              Goliath RSS
            </Footer>
          </Content>
        </Layout>
      </Layout>
    );
  }
}
