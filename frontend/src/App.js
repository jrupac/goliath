import React from 'react';
import { Layout, Menu } from 'antd';
import ArticleList from './components/ArticleList.js';
import FolderFeedList from "./components/FolderFeedList";

import 'antd/dist/antd.css';
import './App.css';

const { Content, Footer, Sider } = Layout;

const DoneFlags = {
  FolderFetch: 1,
  FeedFetch: 2,
  ArticleFetch: 4,
  FaviconFetch: 8,
};

class App extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      folders: new Map(),
      feeds: new Map(),
      favicons: new Map(),
      folderToFeeds: new Map(),
      articles: [],
      shownArticles: [],
      structure: new Map(),
      done: 0,
    };
  }

  ready() {
    return (
      this.state.done === (
        DoneFlags.FolderFetch | DoneFlags.FeedFetch |
        DoneFlags.ArticleFetch | DoneFlags.FaviconFetch));
  }

  restructure() {
    this.setState((prevState) => {
      var structure = new Map();
      prevState.folders.forEach((title, group_id) => {
        if (!prevState.folderToFeeds.has(group_id)) {
          return;
        }
        if (!prevState.feeds) {
          return;
        }

        structure.set(group_id, {
          'title': title,
          'feeds': prevState.folderToFeeds.get(group_id).map(
              feedId => {
                var f = prevState.feeds.get(feedId);
                if (f === undefined) {
                  return null;
                }
                f.favicon = prevState.favicons.get(f.favicon_id) || '';
                return f;
              })
        });
      });
      return {
        structure: structure,
        shownArticles: prevState.articles,
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
                folderToFeeds.set(e.group_id, e.feed_ids.split(',').map(Number));
              });
              var groups = new Map();
              body.groups.forEach(group => {
                groups.set(group.id, group.title);
              });
              return {
                folders: groups,
                folderToFeeds: folderToFeeds,
                done: prevState.done | DoneFlags.FolderFetch,
              };
            }, this.restructure);
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
                feeds.set(feed.id, {
                  'id': feed.id,
                  'favicon_id': feed.favicon_id,
                  'favicon': '',
                  'title': feed.title,
                  'url': feed.url,
                  'site_url': feed.site_url,
                  'is_spark': feed.is_spark,
                  'last_updated_on_time': feed.last_updated_on_time,
                });
              });
              return {
                feeds: feeds,
                done: prevState.done | DoneFlags.FeedFetch,
              };
            }, this.restructure);
          }
        );
  }

  fetchItems() {
    fetch('/fever/?api&items')
        .then(result => result.json())
        .then(body => {
            this.setState(prevState => {
              return {
                articles: body.items,
                done: prevState.done | DoneFlags.ArticleFetch,
              }}, this.restructure);
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
                  favicons.set(favicon.id, favicon.data);
                });
                return {
                  favicons: favicons,
                  done: prevState.done | DoneFlags.FaviconFetch,
                }}, this.restructure);
            }
        );
  }

  sortArticles(articles) {
    return articles.sort((a, b) => {
      return a.created_on_time - b.created_on_time;
    });
  }

  handleSelect = (type, key) => {
    if (type === "feed") {
      this.setState((prevState) => {
        return {
          shownArticles: prevState.articles.filter(e => e.feed_id === key),
        };
      });
    } else if (type === "folder") {
      this.setState((prevState) => {
        var feeds = prevState.folderToFeeds.get(key) || [];
        return {
          shownArticles: prevState.articles.filter(
              e => feeds.indexOf(e.feed_id) !== -1),
        };
      });
    }
  };

  render() {
    return (
      <Layout className="App">
        <Sider width={250}>
          <div className="logo" >
            Goliath
          </div>
          <Menu mode="inline" theme="dark">
            {this.ready()
                ? <FolderFeedList
                    tree={this.state.structure}
                    handleSelect={this.handleSelect} />
                : null}
          </Menu>
        </Sider>
        <Layout>
          <Content>
            <ArticleList
                ready={this.ready()}
                articles={this.sortArticles(this.state.shownArticles)} />
          </Content>
          <Footer>
            Goliath RSS
          </Footer>
        </Layout>
      </Layout>
    );
  }
}

export default App;
