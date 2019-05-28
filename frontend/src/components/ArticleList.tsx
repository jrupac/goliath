import Article from './ArticleCard';
import React from "react";
import ReactList from 'react-list';
import {animateScroll as scroll} from 'react-scroll';
import {
  ArticleListEntry,
  MarkState,
  SelectionKey,
  SelectionType
} from "../utils/types";

const goToAllSequence = ['g', 'a'];
const markAllRead = ['Shift', 'I'];
const keyBufLength = 2;

export interface ArticleListProps {
  articleEntries: ArticleListEntry[];
  selectionKey: SelectionKey;
  selectionType: SelectionType;
  selectAllCallback: () => void;
  handleMark: (mark: MarkState, entity: SelectionKey, type: SelectionType) => void;
}

export interface ArticleListState {
  articleEntries: ArticleListEntry[];

  scrollIndex: number;
  keypressBuffer: Array<string>;
}

export default class ArticleList extends React.Component<ArticleListProps, ArticleListState> {
  list: ReactList | null = null;

  constructor(props: ArticleListProps) {
    super(props);
    this.state = {
      articleEntries: props.articleEntries,
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
    // Reset scroll position when the enclosing key changes.
    if (this.props.selectionKey === nextProps.selectionKey) {
      return;
    }
    if (this.list) {
      this.list.scrollTo(0);
    }
    this.setState({
      articleEntries: Array.from(nextProps.articleEntries),
      scrollIndex: -1,
      keypressBuffer: new Array(keyBufLength)
    });
  }

  render() {
    if (this.state.articleEntries.length === 0) {
      return (
        <div className="article-list-empty">
          <i className="fas fa-check article-list-empty-icon"/>
          <p className="article-list-empty-text">No unread articles.</p>
        </div>
      )
    } else {
      const articles = this.state.articleEntries;
      return (
        <ReactList
          ref={this.handleMounted}
          itemRenderer={(e) => this.renderArticle(articles, e)}
          length={articles.length}
          minSize={5}
          threshold={1000}
          type='variable'/>
      )
    }
  }

  renderArticle(articles: ArticleListEntry[], index: number) {
    const [article, title, favicon] = articles[index];

    return <Article
      key={article.id.toString()}
      article={article}
      title={title}
      favicon={favicon}
      isSelected={index === this.state.scrollIndex}/>
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
      let articleEntries = Array.from(prevState.articleEntries);

      // Add new keypress to buffer, dropping the oldest entry.
      let keypressBuffer = [...prevState.keypressBuffer.slice(1), event.key];

      // If this sequence is fulfilled, reset the buffer and handle it.
      if (goToAllSequence.every((e, i) => e === keypressBuffer[i])) {
        this.props.selectAllCallback();
        keypressBuffer = new Array(keyBufLength);
      } else if (markAllRead.every((e, i) => e === keypressBuffer[i])) {
        this.props.handleMark(
          'read', this.props.selectionKey, this.props.selectionType);
        articleEntries.forEach((e: ArticleListEntry) => {
          const [article] = e;
          article.is_read = 1
        });
        keypressBuffer = new Array(keyBufLength);
      } else {
        switch (event.key) {
        case 'ArrowDown':
          event.preventDefault(); // fallthrough
        case 'j':
          scrollIndex = Math.min(
            prevState.scrollIndex + 1, this.state.articleEntries.length - 1);
          break;
        case 'ArrowUp':
          event.preventDefault(); // fallthrough
        case 'k':
          scrollIndex = Math.max(prevState.scrollIndex - 1, 0);
          break;
        case 'v':
          // If trying to open an article before any articles are selected,
          // treat it like a scroll to the first article.
          if (scrollIndex === -1) {
            scrollIndex = 0;
          }

          const [article] = this.state.articleEntries[scrollIndex];
          window.open(article.url, '_blank');
          break;
        default:
          // No known key pressed, just ignore.
          break;
        }
      }
      return {
        articleEntries,
        keypressBuffer,
        scrollIndex
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
        isDynamic: true,
        duration: 400,
        smooth: "easeInOutQuad",
      });

      this.setState((prevState: ArticleListState) => {
        const articleEntries = Array.from(prevState.articleEntries);
        const entry = articleEntries[prevState.scrollIndex];
        const [article, , , feedId, folderId] = entry;

        if (!(article.is_read === 1)) {
          this.props.handleMark(
            'read', [article.id, feedId, folderId], SelectionType.Article);
          article.is_read = 1;
        }
        return {
          articleEntries
        };
      });
    }
  }
}