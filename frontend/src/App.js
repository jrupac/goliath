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
};

class App extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      folders: new Map(),
      feeds: new Map(),
      feed_groups: new Map(),
      articles: [],
      structure: new Map(),
      done: 0,
    };
  }

  ready() {
    return (
      this.state.done === (
        DoneFlags.FolderFetch | DoneFlags.FeedFetch | DoneFlags.ArticleFetch));
  }

  restructure() {
    this.setState((prevState) => {
      var structure = new Map();
      prevState.folders.forEach((title, group_id) => {
        if (!prevState.feed_groups.has(group_id)) {
          return;
        }
        if (!prevState.feeds) {
          return;
        }

        structure.set(group_id, {
          'title': title,
          'feeds': prevState.feed_groups.get(group_id).map(
              feedId => Object.assign({}, prevState.feeds.get(feedId)))
        });
      });
      return {
        structure: structure
      }
    });
  }

  componentDidMount() {
    Promise.all(
        [this.fetchFolders(), this.fetchFeeds(), this.fetchItems()])
        .then(() => { console.log("Completed all requests to server."); });
  }

  fetchFolders() {
    fetch('/fever/?api&groups')
        .then(result => result.json())
        .then(body => {
            this.setState(prevState => {
              var feed_groups = new Map();
              body.feeds_groups.forEach(e => {
                feed_groups.set(e.group_id, e.feed_ids.split(',').map(Number));
              });
              var groups = new Map();
              body.groups.forEach(group => {
                groups.set(group.id, group.title);
              });
              return {
                folders: groups,
                feed_groups: feed_groups,
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

  render() {
    return (
      <Layout className="App">
        <Sider width={250}>
          <div className="logo" >
            Goliath RSS
          </div>
          <Menu mode="inline" theme="dark">
            {this.ready() ? <FolderFeedList tree={this.state.structure}/> : null}
          </Menu>
        </Sider>
        <Layout>
          <Content>
            <ArticleList articles={this.state.articles}/>
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
