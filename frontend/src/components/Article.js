import React from 'react';
import { Card } from 'antd';

class Article extends React.Component {
  render() {
    var date = new Date(this.props.article.created_on_time * 1000);
    return (
        <Card
            title={
              <div>
                <a target="_blank" href={this.props.article.url}>
                  {this.props.article.title}
                </a>
              </div>
            } extra={date.toLocaleString()}>
          <div className="article-content">
            <div dangerouslySetInnerHTML={{__html: this.props.article.html}}></div>
          </div>
        </Card>
    )
  }
}

export default Article;