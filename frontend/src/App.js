import React from 'react';
import Layout from 'antd/lib/layout';
import Menu from 'antd/lib/menu';
import ArticleList from './components/ArticleList.js';
import FolderFeedList from './components/FolderFeedList';
import Loading from "./components/Loading";

import 'antd/dist/antd.css';
import './App.css';

const { Content, Footer, Sider } = Layout;

export const Status = {
  Start: 0,
  Folder: 1 << 0,
  Feed: 1 << 1,
  Article: 1 << 2,
  Favicon: 1 << 3,
  Ready: 1 << 4
};

export default class App extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      folders: new Map(),
      feeds: new Map(),
      favicons: new Map(),
      folderToFeeds: new Map(),
      unreadCountMap: new Map(),
      unreadCount: 0,
      articles: [],
      shownArticles: [],
      structure: new Map(),
      selectedKey: null,
      status: Status.Start,
    };
  }

  buildStructure() {
    // Only build the structure object once all other requests are done.
    if (this.state.status !== (
        Status.Folder | Status.Feed |
        Status.Article | Status.Favicon)) {
      return;
    }

    this.setState((prevState) => {
      var structure = new Map();
      prevState.folders.forEach((title, group_id) => {
        // Some folders may not have any feeds.
        if (!prevState.folderToFeeds.has(group_id)) {
          return;
        }
        var g = {
          'title': title,
          'feeds': prevState.folderToFeeds.get(group_id).map(
              feedId => {
                var f = prevState.feeds.get(feedId);
                f.favicon = prevState.favicons.get(f.favicon_id) || '';
                f.unread_count = prevState.unreadCountMap.get(feedId) || 0;
                return f;
              })
        };
        g.unread_count = g.feeds.reduce((a, b) => a + b.unread_count, 0);
        structure.set(group_id, g);
      });
      const unreadCount = Array.from(structure.values()).reduce(
          (a, b) => a + b.unread_count, 0);
      return {
        structure: structure,
        shownArticles: prevState.articles,
        unreadCount: unreadCount,
        status: Status.Ready,
      }
    });
  }

  componentDidMount() {
    Promise.all(
        [this.fetchFolders(), this.fetchFeeds(), this.fetchItems(),
          this.fetchFavicons()])
        .then(() => { console.log("Completed all requests to server."); });
  }

  fetchFolders() {
    fetch('/fever/?api&groups')
        .then(result => result.json())
        .then(body => {
            this.setState(prevState => {
              var folderToFeeds = new Map();
              body.feeds_groups.forEach(e => {
                folderToFeeds.set(
                    String(e.group_id),
                    e.feed_ids.split(',').map(Number).map(String));
              });
              var groups = new Map();
              body.groups.forEach(group => {
                groups.set(String(group.id), group.title);
              });
              return {
                folders: groups,
                folderToFeeds: folderToFeeds,
                status: prevState.status | Status.Folder,
              };
            }, this.buildStructure);
          }
        );
  }

  fetchFeeds() {
    fetch('/fever/?api&feeds')
        .then(result => result.json())
        .then(body => {
            this.setState(prevState => {
              var feeds = new Map();
              body.feeds.forEach(feed => {
                feeds.set(String(feed.id), {
                  'id': String(feed.id),
                  'favicon_id': String(feed.favicon_id),
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
                feeds: feeds,
                status: prevState.status | Status.Feed,
              };
            }, this.buildStructure);
          }
        );
  }

  fetchItems() {
    fetch('/fever/?api&items')
        .then(result => result.json())
        .then(body => {
            this.setState(prevState => {
              var articles = [];
              var unreadCounts = new Map();
              body.items.forEach(item => {
                var feed_id = String(item.feed_id);
                unreadCounts.set(
                    feed_id, (unreadCounts.get(feed_id) || 0) + 1);
                articles.push({
                  'id': String(item.id),
                  'feed_id': feed_id,
                  'title': item.title,
                  'url': item.url,
                  'html': item.html,
                  'is_read': item.is_read,
                  'created_on_time': item.created_on_time,
                });
              });
              return {
                articles: articles,
                unreadCountMap: unreadCounts,
                status: prevState.status | Status.Article,
              }}, this.buildStructure);
          }
        );
  }

  fetchFavicons() {
    fetch('/fever/?api&favicons')
        .then(result => result.json())
        .then(body => {
              this.setState(prevState => {
                var favicons = new Map();
                body.favicons.forEach(favicon => {
                  favicons.set(String(favicon.id), favicon.data);
                });
                return {
                  favicons: favicons,
                  status: prevState.status | Status.Favicon,
                }}, this.buildStructure);
            }
        );
  }

  sortArticles(articles) {
    return articles.sort((a, b) => {
      return  b.created_on_time - a.created_on_time;
    });
  }

  handleSelect = (type, key) => {
    if (type === "all") {
      this.setState((prevState) => {
        return {
          shownArticles: prevState.articles,
          selectedKey: key,
        }
      });
    } else if (type === "feed") {
      this.setState((prevState) => {
        return {
          shownArticles: prevState.articles.filter(e => e.feed_id === key),
          selectedKey: key,
        };
      });
    } else if (type === "folder") {
      this.setState((prevState) => {
        var feeds = prevState.folderToFeeds.get(key) || [];
        return {
          shownArticles: prevState.articles.filter(
              e => feeds.indexOf(e.feed_id) !== -1),
          selectedKey: key,
        };
      });
    }
  };

  render() {
    if (this.state.status !== Status.Ready) {
      return <Loading status={this.state.status} />
    }

    if (this.state.unreadCount === 0) {
      document.title = 'Goliath RSS';
    } else {
      document.title = '(' + this.state.unreadCount + ')  Goliath RSS';
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
