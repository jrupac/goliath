import React, {memo, useMemo} from "react";
import {ArticleImagePreview} from "../utils/types";
import {Avatar, Chip, Grid, Paper, Typography} from "@mui/material";
import {extractText, formatFriendly} from "../utils/helpers";
import RssFeedIcon from "@mui/icons-material/RssFeed";
import {ArticleView} from "../models/article";
import {FaviconCls} from "../models/feed";

export interface ArticleListEntryProps {
  articleView: ArticleView;
  favicon: FaviconCls | undefined;
  preview: ArticleImagePreview | undefined;
  selected: boolean;
}

const ArticleListEntry: React.FC<ArticleListEntryProps> = memo(
  function ArticleListEntry(
    {articleView, favicon, preview, selected,}: ArticleListEntryProps) {
    const extractedTitle: string = useMemo(() => {
      return extractText(articleView.title) || "";
    }, [articleView.title]);
    const extractedContent: string = useMemo(() => {
      return extractText(articleView.html) || "";
    }, [articleView.html]);

    const renderMeta = () => {
      const date = new Date(articleView.creationTime * 1000);
      if (favicon && favicon.GetFavicon()) {
        return (
          <Chip
            size="small"
            className="GoliathArticleListMetaChip"
            avatar={
              <Avatar
                src={`data:${favicon.GetFavicon()}`}
                alt={extractedTitle}/>
            }
            label={formatFriendly(date)}
          />
        );
      } else {
        return (
          <Chip
            size="small"
            className="GoliathArticleListMetaChip"
            icon={<RssFeedIcon/>}
            label={formatFriendly(date)}
          />
        );
      }
    };

    const renderImagePreview = () => {
      const crop = preview;

      if (crop) {
        const scale = 100 / crop.width;
        const style = {
          left: -(crop.x * scale) + "px",
          top: -(crop.y * scale) + "px",
          width: crop.origWidth * scale,
        };

        return (
          <figure className="GoliathArticleListImagePreviewFigure">
            <img
              className="GoliathArticleListImagePreview"
              src={crop.src}
              style={style}
            />
          </figure>
        );
      }

      return null;
    };

    let elevation = 3;
    const extraClasses = ["GoliathArticleListBase"];
    if (selected) {
      elevation = 10;
      extraClasses.push("GoliathArticleListSelected");
    } else if (articleView.isRead) {
      elevation = 0;
      extraClasses.push("GoliathArticleListRead");
    }

    return (
      <Paper elevation={elevation} square className={extraClasses.join(" ")}>
        <Grid container direction="column" className="GoliathArticleListGrid">
          <Grid zeroMinWidth item className="GoliathArticleListTitleGrid">
            <Typography noWrap className="GoliathArticleListTitleType">
              {extractedTitle}
            </Typography>
          </Grid>
          <Grid zeroMinWidth item xs>
            <div className="GoliathArticleListMeta">{renderMeta()}</div>
          </Grid>
          <Grid
            container item wrap="nowrap"
            className="GoliathArticleListContent">
            <Grid item xs="auto">{renderImagePreview()}</Grid>
            <Grid
              item zeroMinWidth xs
              className="GoliathArticleContentPreviewGrid">
              <Typography className="GoliathArticleContentPreview">
                {extractedContent}
              </Typography>
            </Grid>
          </Grid>
        </Grid>
      </Paper>
    );
  }
);

export default ArticleListEntry;