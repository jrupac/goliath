import React from "react";
import {ArticleImagePreview, ArticleListEntry} from "../utils/types";
import {Avatar, Chip, Divider, Grid, Paper, Typography} from "@mui/material";
import {extractText, formatFriendly} from "../utils/helpers";
import RssFeedIcon from "@mui/icons-material/RssFeed";

export interface SplitViewArticleListEntryProps {
  article: ArticleListEntry,
  preview: ArticleImagePreview | undefined,
  selected: boolean
}

export interface SplitViewArticleListEntryState {
  extractedTitle: string,
  extractedContent: string
}

export default class SplitViewArticleListEntry
  extends React.PureComponent<SplitViewArticleListEntryProps, SplitViewArticleListEntryState> {
  constructor(props: SplitViewArticleListEntryProps) {
    super(props);
    const [article] = props.article;
    this.state = {
      extractedTitle: extractText(article.title) || "",
      extractedContent: extractText(article.html) || ""
    }
  }

  componentDidUpdate(prevProps: SplitViewArticleListEntryProps) {
    const [article] = this.props.article;
    const [prevArticle] = prevProps.article;

    if (article.id === prevArticle.id) {
      return;
    }

    this.setState({
      extractedTitle: extractText(article.title) || "",
      extractedContent: extractText(article.html) || ""
    });
  }

  render() {
    const [article] = this.props.article;

    let elevation = 3, extraClasses = ["GoliathArticleListBase"];
    if (this.props.selected) {
      elevation = 10;
      extraClasses.push("GoliathArticleListSelected");
    } else if (article.is_read === 1) {
      elevation = 0;
      extraClasses.push("GoliathArticleListRead");
    }

    return (
      <Paper elevation={elevation} square className={extraClasses.join(" ")}>
        <Grid container direction="column" className="GoliathArticleListGrid">
          <Grid zeroMinWidth item className="GoliathArticleListTitleGrid">
            <Typography noWrap className="GoliathArticleListTitleType">
              {this.state.extractedTitle}
            </Typography>
          </Grid>
          <Grid zeroMinWidth item xs>
            <div className="GoliathArticleListMeta">
              {this.renderMeta()}
            </div>
          </Grid>
          <Grid
            container item
            wrap="nowrap"
            className="GoliathArticleListContent">
            <Grid item xs='auto'>
              {this.renderImagePreview()}
            </Grid>
            <Grid
              item zeroMinWidth xs
              className="GoliathArticleContentPreviewGrid">
              <Typography className="GoliathArticleContentPreview">
                {this.state.extractedContent}
              </Typography>
            </Grid>
          </Grid>
        </Grid>
        <Divider/>
      </Paper>
    );
  }

  renderMeta() {
    const [article, , favicon] = this.props.article;
    const extractedTitle = this.state.extractedTitle;
    const date = new Date(article.created_on_time * 1000);
    if (favicon) {
      return (
        <Chip
          size="small"
          className="GoliathArticleListMetaChip"
          avatar={<Avatar src={`data:${favicon}`} alt={extractedTitle}/>}
          label={formatFriendly(date)}/>);
    } else {
      return (
        <Chip
          size="small"
          className="GoliathArticleListMetaChip"
          icon={<RssFeedIcon/>}
          label={formatFriendly(date)}/>);
    }
  }

  renderImagePreview() {
    const crop = this.props.preview;

    if (crop) {
      const scale = 100 / crop.width;
      const style = {
        left: -(crop.x * scale) + "px",
        top: -(crop.y * scale) + "px",
        width: crop.origWidth * scale
      }

      return (
        <figure className="GoliathArticleListImagePreviewFigure">
          <img
            className="GoliathArticleListImagePreview"
            src={crop.src}
            style={style}/>
        </figure>);
    }

    return null;
  }
}
