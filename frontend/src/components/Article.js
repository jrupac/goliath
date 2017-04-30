import React from 'react';
import moment from 'moment';
import { Card } from 'antd';

class Article extends React.Component {
  render() {
    var date = new Date(this.props.article.created_on_time * 1000);
    return (
        <Card
            title={
              <div className="article-header">
                <div className="article-title">
                  <a target="_blank" href={this.props.article.url}>
                    {this.props.article.title}
                  </a>
                </div>
                <div className="article-date">
                  {this.formatDate(date)}
                </div>
              </div>
            } >
          <div className="article-content">
            <div dangerouslySetInnerHTML={{__html: this.props.article.html}}></div>
          </div>
        </Card>
    )
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

export default Article;