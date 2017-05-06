import Article from './Article.js';
import React from 'react';
import ReactList from 'react-list';

const goToAllSequence = ['g', 'a'];

export default class ArticleList extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      articles: Array.from(this.props.articles),
      scrollIndex: -1,
      keypressBuffer: new Array(goToAllSequence.length)
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
    if (this.props.articles === nextProps.articles) {
      return;
    }
    if (this.list) {
      this.list.scrollTo(0);
    }
    this.setState({
      articles: Array.from(nextProps.articles),
      scrollIndex: -1
    });
  }

  render() {
    if (this.state.articles.length === 0) {
      return (
          <div className="article-list-empty">
            No unread articles.
          </div>
      )
    } else {
      const articles = this.state.articles;
      const feeds = this.props.feeds;
      const renderArticle = (index) => (
          <Article
              key={articles[index].id}
              article={articles[index]}
              feed={feeds.get(articles[index].feed_id)}
              isSelected={index === this.state.scrollIndex} />);
      return (
          <ReactList
              ref={this.handleMounted}
              itemRenderer={(e) => renderArticle(e)}
              length={articles.length}
              type='variable'/>
      )
    }
  }

  handleKeyDown = (event) => {
    this.setState((prevState) => {
      var keypressBuffer = [...prevState.keypressBuffer.slice(1), event.key];
      var scrollIndex = prevState.scrollIndex;
      if (goToAllSequence.every((e, i) => e === keypressBuffer[i])) {
        this.props.handleSelect('all', null);
        keypressBuffer = new Array(goToAllSequence.length);
      } else {
        switch (event.key) {
        case 'ArrowDown':
          event.preventDefault(); // fallthrough
        case 'j':
          scrollIndex = Math.min(
              prevState.scrollIndex + 1, this.state.articles.length - 1);
          break;
        case 'ArrowUp':
          event.preventDefault(); // fallthrough
        case 'k':
          scrollIndex = Math.max(prevState.scrollIndex - 1, 0);
          break;
        case 'v':
          const article = this.state.articles[prevState.scrollIndex];
          window.open(article.url, '_blank');
          break;
        }
      }
      return {
        keypressBuffer: keypressBuffer,
        scrollIndex: scrollIndex
      };
    }, this.handleScroll);
  };

  handleMounted = (list) => {
    this.list = list;
  };

  handleScroll() {
    // If this feed is empty, there's no list, so nothing to scroll.
    if (this.list) {
      // A scroll index of less than 0 is meaningless.
      if (this.state.scrollIndex < 0) {
        return;
      }
      this.list.scrollTo(this.state.scrollIndex);
      this.setState((prevState) => {
        const articles = Array.from(prevState.articles);
        const article = articles[prevState.scrollIndex];
        if (!article) {
          console.log('why');
        }
        if (!article.is_read) {
          this.props.handleMark('read', article);
          article.is_read = true;
        }
        return {
          articles: articles
        };
      });
    }
  }
}