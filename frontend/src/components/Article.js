import Card from 'antd/lib/card';
import defaultFavicon from '../../public/favicon.ico';
import moment from 'moment';
import React from 'react';
import Tooltip from 'antd/lib/tooltip';

export default class Article extends React.Component {
  render() {
    const date = new Date(this.props.article.created_on_time * 1000);
    const cardClass = this.props.isSelected ? 'card-selected' : 'card-normal';
    const headerClass = (
        this.props.article.is_read ? 'article-header-read' : 'article-header');
    const feedTitle = this.props.feed.title;
    return (
        <div className="ant-card-outer">
          <Card
              className={cardClass}
              ref={(ref) => {this.ref = ref}}
              title={
                <div className={headerClass}>
                  <div className="article-title">
                    <a target="_blank" href={this.props.article.url}>
                      {this.props.article.title}
                    </a>
                    <br />
                    <div className="article-feed">
                      {this.renderFavicon()}
                      <p className="article-feed-title">{feedTitle}</p>
                    </div>
                  </div>
                  <div className="article-date">
                    <Tooltip title={this.formatFullDate(date)} overlay="">
                      {this.formatDate(date)}
                    </Tooltip>
                  </div>
                </div>
              } >
            <div className="article-content">
              <div dangerouslySetInnerHTML={{__html: this.props.article.html}} />
            </div>
          </Card>
        </div>
    )
  }

  renderFavicon() {
    const favicon = this.props.feed.favicon;
    if (!favicon) {
      return <img src={defaultFavicon} height={16} width={16} alt=''/>
    } else {
      return <img src={`data:${favicon}`} height={16} width={16} alt=''/>
    }
  }

  formatFullDate(date) {
    return moment(date).format('dddd, MMMM Do YYYY, h:mm:ss A');
  }

  formatDate(date) {
    var now = moment();
    var before = moment(now).subtract(1, 'days');
    var then = moment(date);
    if (then.isBetween(before, now)) {
      return then.fromNow();
    } else {
      return then.format("ddd, MMM D, h:mm A");
    }
  }
}