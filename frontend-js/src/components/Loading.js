import Progress from 'antd/lib/progress';
import React from 'react';

import {Status} from '../App.js';

export default class Article extends React.Component {
  render() {
    const progress = this.props.status / (
      Status.Folder | Status.Feed |
      Status.Article | Status.Favicon);
    return (
      <div className="article-list-empty">
        <Progress
          percent={100 * progress}
          showInfo={false}
          status="active"
          strokeWidth={5}/>
      </div>
    )
  }
}