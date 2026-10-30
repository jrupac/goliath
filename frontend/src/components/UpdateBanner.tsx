import { Button, IconButton } from '@mui/material';
import CloseTwoToneIcon from '@mui/icons-material/CloseTwoTone';
import RefreshTwoToneIcon from '@mui/icons-material/RefreshTwoTone';

export interface UpdateBannerProps {
  onReload: () => void;
  onDismiss: () => void;
}

/**
 * Offers a newly deployed build rather than taking it.
 *
 * Replacing the page under its reader costs them their place in what they were
 * reading, and a tab left on an older build is often being kept there on
 * purpose -- to compare two releases side by side, say. So the new build waits
 * until it is asked for, in the tab that asks and no other.
 *
 * Dismissing only hides the banner. The offer stays open in the chrome menu,
 * which is where a reader who waved it away goes looking for it later.
 */
const UpdateBanner = ({ onReload, onDismiss }: UpdateBannerProps) => (
  <div className="GoliathUpdateBanner" role="status">
    <span className="GoliathUpdateBannerText">A new version is ready</span>
    <Button
      className="GoliathUpdateBannerAction"
      size="small"
      startIcon={<RefreshTwoToneIcon />}
      onClick={onReload}
    >
      Reload
    </Button>
    <IconButton
      aria-label="Dismiss update notice"
      className="GoliathButton GoliathUpdateBannerDismiss"
      size="small"
      onClick={onDismiss}
    >
      <CloseTwoToneIcon fontSize="small" />
    </IconButton>
  </div>
);

export default UpdateBanner;
