import * as React from "react";

import {Status} from "../utils/types";
import {Box, LinearProgress} from "@mui/material";

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
      <Box className="GoliathLoadingPageContainer">
        <LinearProgress
          value={100 * progress}
          className="GoliathLoadingProgress"
          variant="determinate"
          data-testid="loading-progress"/>
      </Box>
    )
  }
}