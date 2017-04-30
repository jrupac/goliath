import React from 'react';
import Article from './Article.js';

class ArticleList extends React.Component {

  render() {
    if (!this.props.ready) {
      return (
          <div className="article-list-empty">
            Loading feeds...
          </div>
      )
    } else if (this.props.articles.length === 0) {
      return (
          <div className="article-list-empty">
            Nothing to see here...
          </div>
      )
    } else {
      return (
          <div className="article-list">
            {this.getFeedList()}
          </div>
      )
    }
  }

  getFeedList() {
    return this.props.articles.map((f) => <Article key={f.id} article={f}/>)
  }
}

export default ArticleList;