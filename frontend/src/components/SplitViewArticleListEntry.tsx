import React from "react";
import {ArticleListEntry} from "../utils/types";
import {Divider, Grid, Paper, Typography} from "@mui/material";
import {extractText} from "../utils/helpers";

export interface SplitViewArticleListEntryProps {
  article: ArticleListEntry,
  preview: string | undefined,
  selected: boolean
}

export interface SplitViewArticleListEntryState {
}

export default class SplitViewArticleListEntry
  extends React.Component<SplitViewArticleListEntryProps, SplitViewArticleListEntryState> {
  constructor(props: SplitViewArticleListEntryProps) {
    super(props);
  }

  render() {
    const [article, title, favicon] = this.props.article;
    const extractedTitle = extractText(title) || "";

    let faviconImg = <i className="fas fa-rss-square" title={extractedTitle}/>;
    if (favicon) {
      faviconImg = (
        <img
          src={`data:${favicon}`}
          className="GoliathFeedIcon"
          alt={extractedTitle}
          title={extractedTitle}/>);
    }

    let previewImg = null;
    if (this.props.preview) {
      previewImg = (
        <img
          className="GoliathArticleListImagePreview"
          src={this.props.preview}/>);
    }

    let elevation = 3, extraClasses = [];
    if (this.props.selected) {
      elevation = 10;
      extraClasses.push("GoliathArticleListSelected");
    } else if (article.is_read === 1) {
      elevation = 0;
      extraClasses.push("GoliathArticleListRead");
    }

    return (
      <Paper elevation={elevation} square className={extraClasses.join(" ")}>
        <Grid container direction="column" className="GoliathArticleListEntry">
          <Grid zeroMinWidth item className="GoliathArticleListTitle">
            <Typography noWrap className="GoliathArticleListTitle">
              {extractText(article.title)}
            </Typography>
          </Grid>
          <Grid zeroMinWidth item xs>
            <Divider
              textAlign="left"
              className="GoliathSplitViewArticleListTitleDivider">
              {faviconImg}
            </Divider>
          </Grid>
          <Grid container item wrap="nowrap">
            <Grid item xs='auto'>
              {previewImg}
            </Grid>
            <Grid
              item zeroMinWidth xs
              className="GoliathArticleContentPreviewGrid">
              <Typography className="GoliathArticleContentPreview">
                {extractText(article.html)}
              </Typography>
            </Grid>
          </Grid>
        </Grid>
        <Divider/>
      </Paper>
    );
  }
}
