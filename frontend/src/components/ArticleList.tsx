import Article from './Article';
import React from "react";
import ReactList from 'react-list';
import {EnclosingType, KeyAll} from '../App';
import {animateScroll as scroll} from 'react-scroll';

const goToAllSequence = ['g', 'a'];
const markAllRead = ['Shift', 'I'];
const keyBufLength = 2;

// TODO: Add proper types for these.
export type ArticleType = any;
export type FeedType = any;

export interface ArticleListProps {
  articles: Array<ArticleType>;
  feeds: Map<string, FeedType>;
  is_read: any;

  handleSelect: any;
  handleMark: any;

  enclosingKey: any;
  enclosingType: any;
}

export interface ArticleListState {
  articles: Array<ArticleType>;

  scrollIndex: number;
  keypressBuffer: Array<string>;
}

export default class ArticleList extends React.Component<ArticleListProps, ArticleListState> {
  list : ReactList | null = null;

  constructor(props: ArticleListProps) {
    super(props);
    this.state = {
      articles: Array.from(this.props.articles),
      scrollIndex: -1,
      keypressBuffer: new Array(keyBufLength)
    };
  }

  componentWillMount() {
    window.addEventListener('keydown', this.handleKeyDown);
  };

  componentWillUnmount() {
    window.removeEventListener('keydown', this.handleKeyDown);
  };

  componentWillReceiveProps(nextProps: ArticleListProps) {
    // Reset scroll position when articles change.
    if (this.props.articles === nextProps.articles) {
      return;
    }
    if (this.list) {
      this.list.scrollTo(0);
    }
    this.setState({
      articles: Array.from(nextProps.articles),
      scrollIndex: -1,
      keypressBuffer: new Array(keyBufLength)
    });
  }

  render() {
    if (this.state.articles.length === 0) {
      return (
        <div className="article-list-empty">
          <i className="fas fa-check article-list-empty-icon"/>
          <p className="article-list-empty-text">No unread articles.</p>
        </div>
      )
    } else {
      const articles = this.state.articles;
      const feeds = this.props.feeds;
      const renderArticle = (index: number) => (
        <Article
          key={articles[index].id}
          article={articles[index]}
          feed={feeds.get(articles[index].feed_id)}
          isSelected={index === this.state.scrollIndex}/>);
      return (
        <ReactList
          ref={this.handleMounted}
          itemRenderer={(e) => renderArticle(e)}
          length={articles.length}
          minSize={5}
          threshold={1000}
          type='variable'/>
      )
    }
  }

  handleKeyDown = (event: KeyboardEvent) => {
    // Ignore keypress events when some modifiers are also enabled to avoid
    // triggering on (e.g.) browser shortcuts. Shift is the exception here since
    // we do care about Shift+I.
    if (event.altKey || event.metaKey || event.ctrlKey) {
      return;
    }

    this.setState((prevState) => {
      let scrollIndex = prevState.scrollIndex;
      let articles = Array.from(prevState.articles);

      // Add new keypress to buffer, dropping the oldest entry.
      let keypressBuffer = [...prevState.keypressBuffer.slice(1), event.key];

      // If this sequence is fulfilled, reset the buffer and handle it.
      if (goToAllSequence.every((e, i) => e === keypressBuffer[i])) {
        this.props.handleSelect(EnclosingType.All, KeyAll);
        keypressBuffer = new Array(keyBufLength);
      } else if (markAllRead.every((e, i) => e === keypressBuffer[i])) {
        this.props.handleMark(
          'read', this.props.enclosingKey, this.props.enclosingType);
        articles.forEach((e: any) => {
          e.is_read = 1
        });
        keypressBuffer = new Array(keyBufLength);
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
        default:
          // No known key pressed, just ignore.
          break;
        }
      }
      return {
        articles: articles,
        keypressBuffer: keypressBuffer,
        scrollIndex: scrollIndex
      };
    }, this.handleScroll);
  };

  handleMounted = (list: ReactList) => {
    this.list = list;
  };

  handleScroll() {
    // If this feed is empty, there's no list, so nothing to scroll.
    if (this.list) {
      // A scroll index of less than 0 is meaningless.
      if (this.state.scrollIndex < 0) {
        return;
      }

      // Animate scrolling using technique described by react-list author here:
      // https://github.com/coderiety/react-list/issues/79
      // @ts-ignore
      const scrollPos = this.list.getSpaceBefore(this.state.scrollIndex);

      scroll.scrollTo(scrollPos, {
        // The scrolling container is not trivial to figure out, but react-list
        // has already done the work to figure it out, so use it directly.
        // @ts-ignore
        container: this.list.scrollParent,
        duration: 400,
        smooth: "defaultEasing",
      });

      this.setState((prevState: ArticleListState) => {
        const articles = Array.from(prevState.articles);
        const article = articles[prevState.scrollIndex];
        if (!(article.is_read === 1)) {
          this.props.handleMark('read', article.id, EnclosingType.Article);
          article.is_read = 1;
        }
        return {
          articles: articles
        };
      });
    }
  }
}