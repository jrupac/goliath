import moment from 'moment';
import React, {ReactNode} from "react";
import {Article} from "../utils/types";
import Mercury from '@postlight/mercury-parser';
import {
  Box,
  Card,
  CardContent,
  CardHeader,
  Skeleton,
  Tooltip
} from "@mui/material";

export interface ArticleProps {
  article: Article;
  title: string;
  favicon: string;
  isSelected: boolean;
  shouldRerender: () => void;
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
  constructor(props: ArticleProps) {
    super(props);
    this.state = {
      parsed: null,
      showParsed: false,
      loading: false,
    }
  }

  componentWillMount() {
    window.addEventListener('keydown', this.handleKeyDown);
  };

  componentWillUnmount() {
    window.removeEventListener('keydown', this.handleKeyDown);
  };

  render() {
    const date = new Date(this.props.article.created_on_time * 1000);
    const feedTitle = this.props.title;

    let headerClass = '';
    let elevation;

    if (this.props.isSelected) {
      elevation = 8;
    } else if (this.props.article.is_read === 1) {
      headerClass = 'GoliathArticleHeaderRead';
      elevation = 1;
    } else {
      elevation = 2;
    }

    return (
      <Box className="GoliathArticleOuter">
        <Card elevation={elevation}>
          <CardHeader
            className={`GoliathArticleHeader ${headerClass}`}
            title={
              <Box className="GoliathArticleTitle">
                <a
                  target="_blank"
                  rel="noopener noreferrer"
                  href={this.props.article.url}>
                  <div>
                    <div
                      dangerouslySetInnerHTML={{__html: this.props.article.title}}/>
                  </div>
                </a>
              </Box>
            }
            subheader={
              <Box className="GoliathArticleMeta">
                <Box className="GoliathArticleFeed">
                  {this.renderFavicon()}
                  <p className="GoliathArticleFeedTitle">{feedTitle}</p>
                </Box>
                <Tooltip
                  title={formatFullDate(date)}>
                  <Box className="GoliathArticleDate">
                    {formatDate(date)}
                  </Box>
                </Tooltip>
              </Box>
            }/>
          <CardContent className="GoliathArticleContent">
            {this.renderContent()}
          </CardContent>
        </Card>
      </Box>
    )
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
      return <Box>
        <Skeleton variant="text" animation="wave"/>
        <Skeleton variant="text" animation="wave"/>
        <Skeleton variant="text" animation="wave"/>
      </Box>;
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
      }, () => {
        this.props.shouldRerender()
      });
      return;
    }

    // If already parsed before, just enabling showing it.
    if (this.state.parsed !== null) {
      this.setState({
        showParsed: true
      }, () => {
        this.props.shouldRerender()
      });
      return;
    }

    // It's fine for this to be async. If parsing completes before this is set,
    // it'll just get reset to false below.
    this.setState({
      loading: true
    }, () => {
      this.props.shouldRerender()
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
        }, () => {
          this.props.shouldRerender()
        })
      })
      .catch((reason: any) => {
        console.log("Mercury.parse failed: " + reason);
      })
      .finally(() => {
        this.setState({
          loading: false
        }, () => {
          this.props.shouldRerender()
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