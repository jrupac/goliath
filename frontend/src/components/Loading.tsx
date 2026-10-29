import { Status } from '../utils/types';
import { LinearProgress } from '@mui/material';

export interface LoadingProps {
  status: Status;
}

const Loading = ({ status }: LoadingProps) => {
  const progress =
    status / (Status.Folder | Status.Feed | Status.Article | Status.Favicon);
  // Rounded here because the value is also what gets announced, and a long
  // fraction is read out in full. The width it sets is visually unchanged.
  const percent = Math.round(100 * progress);
  return (
    <div className="GoliathLoadingPageContainer">
      <LinearProgress
        value={percent}
        className="GoliathLoadingProgress"
        variant="determinate"
        data-testid="loading-progress"
      />
    </div>
  );
};

export default Loading;
