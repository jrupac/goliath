import Progress from 'antd/lib/progress';
import * as React from "react";

import {Status} from "../utils/types";

export interface LoadingProps {
  // TODO: Make "status" a proper type.
  status: number;
}

export default class Loading extends React.Component<LoadingProps, never> {
  render() {
    const progress = this.props.status / (
      Status.Folder | Status.Feed |
      Status.Article | Status.Favicon);
    return (
      <div className="loading-page-container">
        <Progress
          percent={100 * progress}
          className="loading-page"
          showInfo={false}
          status="active"
          strokeColor={{
            from: "#ececec",
            to: "#222",
          }}
          strokeWidth={5}/>
      </div>
    )
  }
}