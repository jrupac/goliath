import React from 'react';
import Article from './Article.js';

class ArticleList extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
        articles: props.articles
    }
  }

  render() {
    return (
      <div className="feed-list">
        {this.getFeedList()}
      </div>
    )
  }

  componentWillReceiveProps(nextProps) {
    this.setState({
      articles: nextProps.articles
    })
  }

  getFeedList() {
    return this.state.articles.map((f) => <Article key={f.id} article={f}/>)
  }
}

export default ArticleList;