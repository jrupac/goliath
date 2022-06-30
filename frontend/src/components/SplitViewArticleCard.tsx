import React, {ReactNode} from "react";
import {Article} from "../utils/types";
import {
  Box,
  Card,
  CardHeader,
  Skeleton,
  Tooltip,
  Typography
} from "@mui/material";
import {
  fetchReadability,
  formatFriendly,
  formatFull,
  makeAbsolute
} from "../utils/helpers";

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

export default class SplitViewArticleCard extends React.Component<ArticleProps, ArticleState> {
  constructor(props: ArticleProps) {
    super(props);
    this.state = {
      parsed: null,
      showParsed: false,
      loading: false,
    }
  }

  componentDidMount() {
    window.addEventListener('keydown', this.handleKeyDown);
  }

  componentWillUnmount() {
    window.removeEventListener('keydown', this.handleKeyDown);
  }

  render() {
    const date = new Date(this.props.article.created_on_time * 1000);
    const feedTitle = this.props.title;

    let headerClass = '';
    let elevation;

    if (this.props.isSelected) {
      elevation = 1;
    } else if (this.props.article.is_read === 1) {
      headerClass = 'GoliathArticleHeaderRead';
      elevation = 0;
    } else {
      elevation = 2;
    }

    return (
      <Box className="GoliathArticleOuter">
        <Card elevation={elevation} className="GoliathHeaderContainer">
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
                  title={formatFull(date)}>
                  <Box className="GoliathArticleDate">
                    {formatFriendly(date)}
                  </Box>
                </Tooltip>
              </Box>
            }/>
        </Card>
        <Typography
          className="GoliathSplitViewArticleContent GoliathArticleContentStyling">
          {this.renderContent()}
        </Typography>
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
      this.setState({showParsed: false});
      return;
    }

    // If already parsed before, just enabling showing it.
    if (this.state.parsed !== null) {
      this.setState({showParsed: true});
      return;
    }

    // It's okay if this state change doesn't happen fast enough, it'll just get
    // reset lower down anyway.
    this.setState({loading: true});

    const url = makeAbsolute("/cache?url=" + encodeURI(this.props.article.url));
    fetchReadability(url).then((content) => {
      this.setState({
        parsed: content,
        showParsed: true,
        loading: false
      });
    }).catch((e) => {
      console.log("Could not parse URL %s: %s", url, e);
      this.setState({
        showParsed: false,
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