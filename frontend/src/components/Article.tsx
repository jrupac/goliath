import Card from 'antd/lib/card';
import moment from 'moment';
import React from "react";
import Tooltip from 'antd/lib/tooltip';
import {ArticleType, FeedType} from "../utils/types";

export interface ArticleProps {
  article: ArticleType;
  feed: FeedType;
  isSelected: boolean;
}

export default class Article extends React.Component<ArticleProps, any> {
  ref: any = null;

  setRef = (ref: Card | null) => {
    this.ref = ref;
  };

  render() {
    const date = new Date(this.props.article.created_on_time * 1000);
    const cardClass = this.props.isSelected ? 'card-selected' : 'card-normal';
    const feedTitle = this.props.feed.title;

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
                  <div
                    dangerouslySetInnerHTML={
                      {__html: this.props.article.title}}/>
                </a>
                <div className="article-feed">
                  {this.renderFavicon()}
                  <p className="article-feed-title">{feedTitle}</p>
                </div>
              </div>
              <div className="article-date">
                <Tooltip title={formatFullDate(date)} overlay="">
                  {formatDate(date)}
                </Tooltip>
                {this.renderReadIcon()}
              </div>
            </div>
          }>
          <div className="article-content">
            <div dangerouslySetInnerHTML={{__html: this.props.article.html}}/>
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
    const favicon = this.props.feed.favicon;
    if (!favicon) {
      return <i className="fas fa-rss-square"/>
    } else {
      return <img src={`data:${favicon}`} height={16} width={16} alt=''/>
    }
  }


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