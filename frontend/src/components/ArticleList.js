import React from 'react';
import { Spin } from 'antd';
import ReactList from 'react-list';
import Article from './Article.js';

class ArticleList extends React.Component {
  constructor(props) {
    super(props);
    this.handleKeyDown = this.handleKeyDown.bind(this);
    this.handleMounted = this.handleMounted.bind(this);
    this.state = {
      scrollIndex: 0
    };
  }

  componentWillMount() {
    window.addEventListener('keydown', this.handleKeyDown);
  };

  componentWillUnmount() {
    window.removeEventListener('keydown', this.handleKeyDown);
  };

  componentWillReceiveProps(nextProps) {
    // Reset scroll position when articles change.
    if (this.props === nextProps) {
      return;
    }
    this.setState({
      scrollIndex: 0
    });
  }

  render() {
    if (!this.props.ready) {
      return (
          <div className="article-list-empty">
            <Spin size="large" />
          </div>
      )
    } else if (this.props.articles.length === 0) {
      return (
          <div className="article-list-empty">
            No unread articles.
          </div>
      )
    } else {
      const articles = this.props.articles;
      const renderArticle = (index) => (
          <Article
              key={articles[index].id}
              article={articles[index]}
              isSelected={index === this.state.scrollIndex} />);
      return (
          <ReactList
              ref={this.handleMounted}
              itemRenderer={(e) => renderArticle(e)}
              length={articles.length}
              type='simple'/>
      )
    }
  }

  handleMounted(list) {
    this.list = list;
  }

  handleScroll() {
    this.list.scrollTo(this.state.scrollIndex);
  }

  handleKeyDown(event) {
    this.setState((prevState) => {
      var scrollIndex;
      if (event.key === 'j') {
        scrollIndex = Math.min(
            prevState.scrollIndex + 1, this.props.articles.length - 1);
      } else if (event.key === 'k') {
        scrollIndex = Math.max(
            prevState.scrollIndex - 1, 0);
      }
      return { scrollIndex: scrollIndex };
    }, this.handleScroll);
  }
}



export default ArticleList;