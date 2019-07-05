import Card from 'antd/lib/card';
import moment from 'moment';
import React, {ReactNode} from "react";
import Tooltip from 'antd/lib/tooltip';
import {Article} from "../utils/types";
import Mercury from '@postlight/mercury-parser';
import {Skeleton} from "antd";

export interface ArticleProps {
  article: Article;
  title: string;
  favicon: string;
  isSelected: boolean;
}

export interface ArticleState {
  parsed: string | null;
  showParsed: boolean;
  loading: boolean;
}

// Copied from @types/postlight__mercury-parser because this type is not
// exported there.
interface ParseResult {
  title: string | null;
  content: string | null;
  author: string | null;
  date_published: string | null;
  lead_image_url: string | null;
  dek: string | null;
  next_page_url: string | null;
  url: string;
  domain: string;
  excerpt: string | null;
  word_count: number;
  direction: 'ltr' | 'rtl';
  total_pages: number;
  rendered_pages: number;
}

export default class ArticleCard extends React.Component<ArticleProps, ArticleState> {
  ref: any = null;

  constructor(props: ArticleProps) {
    super(props);
    this.state = {
      parsed: null,
      showParsed: false,
      loading: false,
    }
  }

  setRef = (ref: Card | null) => {
    this.ref = ref;
  };

  componentWillMount() {
    window.addEventListener('keydown', this.handleKeyDown);
  };

  componentWillUnmount() {
    window.removeEventListener('keydown', this.handleKeyDown);
  };

  render() {
    const date = new Date(this.props.article.created_on_time * 1000);
    const cardClass = this.props.isSelected ? 'card-selected' : 'card-normal';
    const feedTitle = this.props.title;

    let headerClass: string;
    if (this.props.isSelected) {
      headerClass = 'article-header';
    } else if (this.props.article.is_read === 1) {
      headerClass = 'article-header-read';
    } else {
      headerClass = 'article-header';
    }

    return (
      <div className="ant-card-outer">
        <Card
          className={cardClass}
          ref={this.setRef}
          title={
            <div className={headerClass}>
              <div className="article-title">
                <a
                  target="_blank"
                  rel="noopener noreferrer"
                  href={this.props.article.url}>
                  <div>
                    <div
                      dangerouslySetInnerHTML={{__html: this.props.article.title}}/>
                  </div>
                </a>
              </div>
              <div className="article-metadata">
                <div className="article-feed">
                  {this.renderFavicon()}
                  <p className="article-feed-title">{feedTitle}</p>
                </div>
                <div className="article-date">
                  <Tooltip
                    title={formatFullDate(date)}
                    overlayClassName="article-date-tooltip">
                    {formatDate(date)}
                  </Tooltip>
                  {this.renderReadIcon()}
                </div>
              </div>
            </div>
          }>
          <div className="article-content">
            {this.renderContent()}
          </div>
        </Card>
      </div>
    )
  }

  renderReadIcon() {
    if (this.props.article.is_read === 1) {
      return <i
        className="fa fa-circle-thin article-status-read"
        aria-hidden="true"/>
    } else {
      return <i
        className="fa fa-circle article-status-unread"
        aria-hidden="true"/>
    }
  }

  renderFavicon() {
    const favicon = this.props.favicon;
    if (!favicon) {
      return <i className="fas fa-rss-square"/>
    } else {
      return <img src={`data:${favicon}`} height={16} width={16} alt=''/>
    }
  }

  renderContent(): ReactNode {
    if (this.state.loading) {
      return <Skeleton active/>;
    } else {
      return <div
        dangerouslySetInnerHTML={{__html: this.getArticleContent()}}/>;
    }
  }

  getArticleContent(): string {
    if (this.state.showParsed) {
      // This field is checked for non-nullity before being set.
      return this.state.parsed!;
    }
    return this.props.article.html;
  }

  toggleParseContent() {
    // If already showing parsed content, disable showing it.
    if (this.state.showParsed) {
      this.setState({
        showParsed: false
      });
      return;
    }

    // If already parsed before, just enabling showing it.
    if (this.state.parsed !== null) {
      this.setState({
        showParsed: true
      });
      return;
    }

    // It's fine for this to be async. If parsing completes before this is set,
    // it'll just get reset to false below.
    this.setState({
      loading: true
    });

    const url = makeAbsolute("/cache?url=" + encodeURI(this.props.article.url));

    Mercury.parse(url)
      .then((result: ParseResult) => {
        if (result === null || result.content === null) {
          return;
        }
        this.setState({
          parsed: result.content,
          showParsed: true
        })
      })
      .catch((reason: any) => {
        console.log("Mercury.parse failed: " + reason);
      })
      .finally(() => {
        this.setState({
          loading: false
        });
      })
  }

  handleKeyDown = (event: KeyboardEvent) => {
    // Ignore all key events unless this is the selected article.
    if (!this.props.isSelected) {
      return;
    }

    // Ignore keypress events when some modifiers are also enabled to avoid
    // triggering on (e.g.) browser shortcuts.
    if (event.altKey || event.metaKey || event.ctrlKey || event.shiftKey) {
      return;
    }

    if (event.key === 'm') {
      this.toggleParseContent();
    }
  }
}

function makeAbsolute(url: string): string {
  const a = document.createElement('a');
  a.href = url;
  return a.href;
}

function formatFullDate(date: Date) {
  return moment(date).format('dddd, MMMM Do YYYY, h:mm:ss A');
}

function formatDate(date: Date) {
  const now = moment();
  const before = moment(now).subtract(1, 'days');
  const then = moment(date);
  if (then.isBetween(before, now)) {
    return then.fromNow();
  } else {
    return then.format("ddd, MMM D, h:mm A");
  }
}