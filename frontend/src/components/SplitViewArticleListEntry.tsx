import React from "react";
import {ArticleListEntry} from "../utils/types";
import {Divider, Grid, Paper, Typography} from "@mui/material";

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

    let faviconImg = <i className="fas fa-rss-square"/>;
    if (favicon) {
      faviconImg = <img src={`data:${favicon}`} height={16} width={16} alt=''/>
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
              {extractContent(article.title)}
            </Typography>
          </Grid>
          <Grid zeroMinWidth container item xs>
            <Grid item sx={{paddingRight: "10px"}} xs="auto">
              {faviconImg}
            </Grid>
            <Grid item zeroMinWidth xs>
              <Typography noWrap className="GoliathArticleFeedTitle">
                {extractContent(title)}
              </Typography>
            </Grid>
          </Grid>
          <Grid container item wrap="nowrap">
            <Grid item xs='auto'>
              {previewImg}
            </Grid>
            <Grid item zeroMinWidth xs style={{height: '100px'}}>
              <Typography className="GoliathArticleContentPreview">
                {extractContent(article.html)}
              </Typography>
            </Grid>
          </Grid>
        </Grid>
        <Divider/>
      </Paper>
    );
  }
}

function extractContent(html: string): string | null {
  return new DOMParser()
    .parseFromString(html, "text/html")
    .documentElement.textContent;
}
