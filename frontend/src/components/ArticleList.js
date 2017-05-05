import Article from './Article.js';
import React from 'react';
import ReactList from 'react-list';

const goToAllSequence = ['g', 'a'];

export default class ArticleList extends React.Component {
  constructor(props) {
    super(props);
    this.handleKeyDown = this.handleKeyDown.bind(this);
    this.handleMounted = this.handleMounted.bind(this);
    this.state = {
      scrollIndex: 0,
      keypressBuffer: new Array(2)
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
    if (this.props.articles.length === 0) {
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
              type='variable'/>
      )
    }
  }

  handleMounted(list) {
    this.list = list;
  }

  handleScroll() {
    // If this feed is empty, there's no list, so nothing to scroll.
    if (this.list) {
      this.list.scrollTo(this.state.scrollIndex);
    }
  }

  handleKeyDown(event) {
    this.setState((prevState) => {
      var keypressBuffer = [...prevState.keypressBuffer.slice(1), event.key];
      var scrollIndex = prevState.scrollIndex;
      if (goToAllSequence.every((e, i) => e === keypressBuffer[i])) {
        this.props.handleSelect('all', null);
        keypressBuffer = new Array(2);
      } else {
        switch (event.key) {
        case 'ArrowDown':
          event.preventDefault(); // fallthrough
        case 'j':
          scrollIndex = Math.min(
              prevState.scrollIndex + 1, this.props.articles.length - 1);
          break;
        case 'ArrowUp':
          event.preventDefault(); // fallthrough
        case 'k':
          scrollIndex = Math.max(prevState.scrollIndex - 1, 0);
          break;
        case 'v':
          const article = this.props.articles[prevState.scrollIndex];
          window.open(article.url, '_blank');
          break;
        }
      }
      return {
        keypressBuffer: keypressBuffer,
        scrollIndex: scrollIndex
      };
    }, this.handleScroll);

  }
}