import React from "react";
import {ArticleImagePreview, ArticleListEntry} from "../utils/types";
import {Grid, Paper, Typography} from "@mui/material";

export interface SplitViewArticleListEntryProps {
  article: ArticleListEntry,
  selected: boolean
}

export interface SplitViewArticleListEntryState {
  preview: ArticleImagePreview
}

export default class SplitViewArticleListEntry
  extends React.Component<SplitViewArticleListEntryProps, SplitViewArticleListEntryState> {
  constructor(props: SplitViewArticleListEntryProps) {
    super(props);
    this.state = {
      preview: null
    }
  }

  async generateImagePreview() {
    // Already generated preview for this article, so nothing to do.
    if (this.state.preview !== null) {
      return;
    }

    const minPixelSize = 100, imgFetchLimit = 5;
    const p: Promise<any>[] = [];
    const [article] = this.props.article;
    const images = new DOMParser()
      .parseFromString(article.html, "text/html").images;

    if (images === undefined) {
      return;
    }

    let limit = Math.min(imgFetchLimit, images.length)
    for (let j = 0; j < limit; j++) {
      const img = new Image();
      img.src = images[j].src;

      p.push(new Promise((resolve, reject) => {
        img.decode().then(() => {
          if (img.height >= minPixelSize && img.width >= minPixelSize) {
            resolve([img.height, img.src]);
          }
          reject();
        }).catch(() => {
          /* Ignore errors */
        });
      }));
    }

    const results = await Promise.allSettled(p);
    let height = 0;
    let src: string;

    results.forEach((result) => {
      if (result.status === 'fulfilled') {
        const [imgHeight, imgSrc] = result.value;
        if (imgHeight > height) {
          height = imgHeight;
          src = imgSrc;
        }
      }
    })

    if (height > 0) {
      this.setState(() => {
        return {
          preview: (
            <img className="GoliathArticleListImagePreview" src={src}/>)
        }
      });
    }
  }

  render() {
    // We want this to be async, but add the .then() to quiet the warning.
    this.generateImagePreview().then();

    const [article, title, favicon] = this.props.article;
    let faviconImg;

    if (!favicon) {
      faviconImg = <i className="fas fa-rss-square"/>
    } else {
      faviconImg = <img src={`data:${favicon}`} height={16} width={16} alt=''/>
    }

    let elevation = 3, readClass = "";
    if (this.props.selected) {
      elevation = 10;
    } else if (article.is_read === 1) {
      elevation = 0;
      readClass = "GoliathArticleListRead"
    }

    return (
      <Paper elevation={elevation} className={readClass}>
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
              {this.state.preview}
            </Grid>
            <Grid item zeroMinWidth xs style={{height: '100px'}}>
              <Typography className="GoliathArticleContentPreview">
                {extractContent(article.html)}
              </Typography>
            </Grid>
          </Grid>
        </Grid>
      </Paper>
    );
  }
}

function extractContent(html: string): string | null {
  return new DOMParser()
    .parseFromString(html, "text/html")
    .documentElement.textContent;
}