import Card from 'antd/lib/card';
import moment from 'moment';
import React from 'react';

export default class Article extends React.Component {
  render() {
    const date = new Date(this.props.article.created_on_time * 1000);
    const cardClass = this.props.isSelected ? 'card-selected' : 'card-normal';
    return (
        <div className="ant-card-outer">
          <Card
              className={cardClass}
              ref={(ref) => {this.ref = ref}}
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
              <div dangerouslySetInnerHTML={{__html: this.props.article.html}} />
            </div>
          </Card>
        </div>
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