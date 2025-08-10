import React from 'react';
import { Status } from '../utils/types';
import { Box, LinearProgress } from '@mui/material';

export interface LoadingProps {
  status: Status;
}

const Loading: React.FC<LoadingProps> = ({ status }) => {
  const progress =
    status / (Status.Folder | Status.Feed | Status.Article | Status.Favicon);
  return (
    <Box className="GoliathLoadingPageContainer">
      <LinearProgress
        value={100 * progress}
        className="GoliathLoadingProgress"
        variant="determinate"
        data-testid="loading-progress"
      />
    </Box>
  );
};

export default Loading;
