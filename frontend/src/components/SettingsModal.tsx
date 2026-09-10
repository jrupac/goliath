import React, { FormEvent, ReactNode, useState } from 'react';
import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  IconButton,
  LinearProgress,
  MenuItem,
  Select,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import AddTwoToneIcon from '@mui/icons-material/AddTwoTone';
import CheckTwoToneIcon from '@mui/icons-material/CheckTwoTone';
import ChevronRightTwoToneIcon from '@mui/icons-material/ChevronRightTwoTone';
import CloseTwoToneIcon from '@mui/icons-material/CloseTwoTone';
import DeleteTwoToneIcon from '@mui/icons-material/DeleteTwoTone';
import DriveFileMoveTwoToneIcon from '@mui/icons-material/DriveFileMoveTwoTone';
import EditTwoToneIcon from '@mui/icons-material/EditTwoTone';
import RssFeedTwoToneIcon from '@mui/icons-material/RssFeedTwoTone';
import TuneTwoToneIcon from '@mui/icons-material/TuneTwoTone';
import { GoliathTheme } from '../utils/types';
import { FolderId, FolderView } from '../models/folder';
import { FeedId, FeedView } from '../models/feed';
import { extractText } from '../utils/helpers';
import FeedIcon from './FeedIcon';

type SettingsSection = 'general' | 'feeds';

export interface SettingsModalProps {
  open: boolean;
  onClose: () => void;
  isMobile: boolean;
  // Whether the subscription list is being re-read after a change. The
  // list on show is the old one until that finishes.
  reloading: boolean;

  theme: GoliathTheme;
  onToggleTheme: () => void;
  hideEmpty: boolean;
  onToggleHideEmpty: () => void;
  onShowKeybindings: () => void;
  buildTimestamp: string;
  buildHash: string;

  folderFeedView: Map<FolderView, FeedView[]>;
  onAddFeed: () => void;
  renameFeed: (feedId: FeedId, title: string) => Promise<void>;
  moveFeed: (
    feedId: FeedId,
    toFolderId: FolderId,
    fromFolderId: FolderId
  ) => Promise<void>;
  unsubscribeFeed: (feedId: FeedId) => Promise<void>;
  // Called after a feed changed on the server, so the caller can re-read the
  // subscription list.
  onFeedsChanged: () => void;
}

const sections: { id: SettingsSection; label: string; icon: ReactNode }[] = [
  { id: 'general', label: 'General', icon: <TuneTwoToneIcon /> },
  { id: 'feeds', label: 'Feeds', icon: <RssFeedTwoToneIcon /> },
];

const SettingsModal: React.FC<SettingsModalProps> = ({
  open,
  onClose,
  isMobile,
  reloading,
  theme,
  onToggleTheme,
  hideEmpty,
  onToggleHideEmpty,
  onShowKeybindings,
  buildTimestamp,
  buildHash,
  folderFeedView,
  onAddFeed,
  renameFeed,
  moveFeed,
  unsubscribeFeed,
  onFeedsChanged,
}) => {
  const [section, setSection] = useState<SettingsSection>('general');

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="md"
      fullWidth
      fullScreen={isMobile}
      slotProps={{
        backdrop: { className: 'GoliathModalOverlay' },
        paper: { className: 'GoliathModalPaper GoliathSettingsPaper' },
        // Back to the first section once closed, after the fade so the
        // contents do not switch while still visible.
        transition: { onExited: () => setSection('general') },
      }}
    >
      <Box className="GoliathDialogHeader">
        <Typography component="h2" className="GoliathDialogHeading">
          Settings
        </Typography>
        <IconButton
          aria-label="Close settings"
          className="GoliathDialogClose"
          onClick={onClose}
          size="small"
        >
          <CloseTwoToneIcon fontSize="small" />
        </IconButton>
      </Box>
      {reloading && (
        <LinearProgress
          className="GoliathReloadProgress"
          aria-label="Refreshing subscriptions"
        />
      )}

      <Box className="GoliathSettingsBody">
        <Box component="nav" className="GoliathSettingsNav">
          {sections.map((s) => (
            <Box
              key={s.id}
              component="button"
              type="button"
              className={
                s.id === section
                  ? 'GoliathSettingsNavItem GoliathSettingsNavItemSelected'
                  : 'GoliathSettingsNavItem'
              }
              onClick={() => setSection(s.id)}
            >
              {s.icon}
              <span>{s.label}</span>
            </Box>
          ))}
        </Box>

        <Box className="GoliathSettingsContent">
          {section === 'general' ? (
            <GeneralSettings
              theme={theme}
              onToggleTheme={onToggleTheme}
              hideEmpty={hideEmpty}
              onToggleHideEmpty={onToggleHideEmpty}
              onShowKeybindings={onShowKeybindings}
              buildTimestamp={buildTimestamp}
              buildHash={buildHash}
            />
          ) : (
            <FeedSettings
              folderFeedView={folderFeedView}
              onAddFeed={onAddFeed}
              renameFeed={renameFeed}
              moveFeed={moveFeed}
              unsubscribeFeed={unsubscribeFeed}
              onFeedsChanged={onFeedsChanged}
            />
          )}
        </Box>
      </Box>

      <Box className="GoliathDialogFooter">
        <Button
          variant="contained"
          className="GoliathAccentButton"
          onClick={onClose}
        >
          Done
        </Button>
      </Box>
    </Dialog>
  );
};

/** A titled group of settings, drawn as one card. */
const SettingsGroup: React.FC<{ title: string; children: ReactNode }> = ({
  title,
  children,
}) => (
  <Box className="GoliathSettingsGroup">
    <Typography className="GoliathSettingsGroupTitle">{title}</Typography>
    <Box className="GoliathSettingsCard">{children}</Box>
  </Box>
);

/** One row in a settings card: a label and description with a control. */
const SettingsRow: React.FC<{
  label: string;
  description?: string;
  control: ReactNode;
  onClick?: () => void;
}> = ({ label, description, control, onClick }) => (
  <Box
    component={onClick ? 'button' : 'div'}
    type={onClick ? 'button' : undefined}
    className={
      onClick
        ? 'GoliathSettingsCardRow GoliathSettingsCardRowClickable'
        : 'GoliathSettingsCardRow'
    }
    onClick={onClick}
  >
    <Box className="GoliathSettingsRowText">
      <span className="GoliathSettingsRowLabel">{label}</span>
      {description && (
        <span className="GoliathSettingsRowDescription">{description}</span>
      )}
    </Box>
    <Box className="GoliathSettingsRowControl">{control}</Box>
  </Box>
);

interface GeneralSettingsProps {
  theme: GoliathTheme;
  onToggleTheme: () => void;
  hideEmpty: boolean;
  onToggleHideEmpty: () => void;
  onShowKeybindings: () => void;
  buildTimestamp: string;
  buildHash: string;
}

const GeneralSettings: React.FC<GeneralSettingsProps> = ({
  theme,
  onToggleTheme,
  hideEmpty,
  onToggleHideEmpty,
  onShowKeybindings,
  buildTimestamp,
  buildHash,
}) => (
  <>
    <SettingsGroup title="Appearance">
      <SettingsRow
        label="Dark theme"
        description="Use the dark colour scheme."
        control={
          <Switch
            checked={theme === GoliathTheme.Dark}
            onChange={onToggleTheme}
            className="GoliathAccentSwitch"
            slotProps={{ input: { 'aria-label': 'Dark theme' } }}
          />
        }
      />
      <SettingsRow
        label="Hide empty feeds"
        description="Show only feeds and folders with unread items in the sidebar."
        control={
          <Switch
            checked={hideEmpty}
            onChange={onToggleHideEmpty}
            className="GoliathAccentSwitch"
            slotProps={{
              input: { 'aria-label': 'Hide feeds with no unread items' },
            }}
          />
        }
      />
    </SettingsGroup>

    <SettingsGroup title="Keyboard">
      <SettingsRow
        label="Keyboard shortcuts"
        description="Every key the reader responds to."
        onClick={onShowKeybindings}
        control={<ChevronRightTwoToneIcon className="GoliathSettingsChevron" />}
      />
    </SettingsGroup>

    <Box className="GoliathSettingsAbout">
      <span>Goliath RSS</span>
      <span>
        Built at {buildTimestamp} &middot;{' '}
        <span className="GoliathSettingsBuildHash">{buildHash}</span>
      </span>
    </Box>
  </>
);

type RowMode = 'rename' | 'move' | 'unsubscribe';

interface FeedSettingsProps {
  folderFeedView: Map<FolderView, FeedView[]>;
  onAddFeed: () => void;
  renameFeed: (feedId: FeedId, title: string) => Promise<void>;
  moveFeed: (
    feedId: FeedId,
    toFolderId: FolderId,
    fromFolderId: FolderId
  ) => Promise<void>;
  unsubscribeFeed: (feedId: FeedId) => Promise<void>;
  onFeedsChanged: () => void;
}

/**
 * Every subscription, grouped by folder, with rename, move and unsubscribe
 * on each row. One row is editable at a time; the others stay as they are
 * until it is saved or abandoned.
 */
const FeedSettings: React.FC<FeedSettingsProps> = ({
  folderFeedView,
  onAddFeed,
  renameFeed,
  moveFeed,
  unsubscribeFeed,
  onFeedsChanged,
}) => {
  const [editing, setEditing] = useState<{
    feedId: FeedId;
    mode: RowMode;
  } | null>(null);

  const folders = Array.from(folderFeedView.keys());
  let feedCount = 0;
  folderFeedView.forEach((feeds) => (feedCount += feeds.length));

  return (
    <>
      <Box className="GoliathSettingsFeedsHeader">
        <Typography className="GoliathSettingsGroupTitle">
          Subscriptions
          <span className="GoliathSettingsCount">{feedCount}</span>
        </Typography>
        <Button
          variant="contained"
          className="GoliathAccentButton"
          size="small"
          startIcon={<AddTwoToneIcon />}
          onClick={onAddFeed}
        >
          Add feed
        </Button>
      </Box>
      {Array.from(folderFeedView, ([folder, feeds]) => (
        <SettingsGroup key={folder.id} title={folder.title}>
          {feeds.map((feed) => (
            <FeedRow
              key={feed.id}
              feed={feed}
              folders={folders}
              mode={editing?.feedId === feed.id ? editing.mode : null}
              onSetMode={(mode) =>
                setEditing(mode === null ? null : { feedId: feed.id, mode })
              }
              renameFeed={renameFeed}
              moveFeed={moveFeed}
              unsubscribeFeed={unsubscribeFeed}
              onFeedsChanged={onFeedsChanged}
            />
          ))}
        </SettingsGroup>
      ))}
    </>
  );
};

interface FeedRowProps {
  feed: FeedView;
  folders: FolderView[];
  mode: RowMode | null;
  onSetMode: (mode: RowMode | null) => void;
  renameFeed: (feedId: FeedId, title: string) => Promise<void>;
  moveFeed: (
    feedId: FeedId,
    toFolderId: FolderId,
    fromFolderId: FolderId
  ) => Promise<void>;
  unsubscribeFeed: (feedId: FeedId) => Promise<void>;
  onFeedsChanged: () => void;
}

const FeedRow: React.FC<FeedRowProps> = ({
  feed,
  folders,
  mode,
  onSetMode,
  renameFeed,
  moveFeed,
  unsubscribeFeed,
  onFeedsChanged,
}) => {
  const plainTitle = extractText(feed.title) || feed.title;
  const [title, setTitle] = useState(plainTitle);
  const [folderId, setFolderId] = useState<FolderId>(feed.folder_id);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Editing starts from the row's current values each time a mode is
  // entered, so an abandoned edit does not resurface the next time.
  const enterMode = (next: RowMode) => {
    setTitle(plainTitle);
    setFolderId(feed.folder_id);
    setError(null);
    onSetMode(next);
  };

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await action();
      onSetMode(null);
      onFeedsChanged();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const handleRename = (e: FormEvent) => {
    e.preventDefault();
    const trimmed = title.trim();
    if (trimmed === '' || busy) {
      return;
    }
    if (trimmed === plainTitle) {
      onSetMode(null);
      return;
    }
    run(() => renameFeed(feed.id, trimmed));
  };

  const handleMove = () => {
    if (busy) {
      return;
    }
    if (folderId === feed.folder_id) {
      onSetMode(null);
      return;
    }
    run(() => moveFeed(feed.id, folderId, feed.folder_id));
  };

  const handleUnsubscribe = () => {
    if (busy) {
      return;
    }
    run(() => unsubscribeFeed(feed.id));
  };

  const cancelButton = (
    <Button
      size="small"
      className="GoliathQuietButton"
      onClick={() => onSetMode(null)}
      disabled={busy}
    >
      Cancel
    </Button>
  );

  const confirmButton = (label: string, onClick?: () => void) => (
    <Button
      size="small"
      variant="contained"
      className="GoliathAccentButton"
      type={onClick === undefined ? 'submit' : 'button'}
      onClick={onClick}
      disabled={busy}
      startIcon={
        busy ? (
          <CircularProgress size={14} color="inherit" />
        ) : (
          <CheckTwoToneIcon />
        )
      }
    >
      {label}
    </Button>
  );

  const renderEditor = () => {
    switch (mode) {
      case 'rename':
        return (
          <form onSubmit={handleRename} className="GoliathSettingsFeedEditor">
            <TextField
              autoFocus
              size="small"
              fullWidth
              value={title}
              disabled={busy}
              onChange={(e) => setTitle(e.target.value)}
              className="GoliathAccentField"
              slotProps={{ htmlInput: { 'aria-label': 'Feed title' } }}
            />
            {confirmButton('Save')}
            {cancelButton}
          </form>
        );
      case 'move':
        return (
          <Box className="GoliathSettingsFeedEditor">
            <Select
              size="small"
              fullWidth
              value={folderId}
              disabled={busy}
              onChange={(e) => setFolderId(e.target.value as FolderId)}
              className="GoliathAccentField"
              inputProps={{ 'aria-label': 'Folder' }}
              MenuProps={{ classes: { paper: 'GoliathMenuPaper' } }}
            >
              {folders.map((f) => (
                <MenuItem key={f.id} value={f.id}>
                  {f.title}
                </MenuItem>
              ))}
            </Select>
            {confirmButton('Move', handleMove)}
            {cancelButton}
          </Box>
        );
      case 'unsubscribe':
        return (
          <Box className="GoliathSettingsFeedEditor">
            <Typography className="GoliathSettingsConfirmText">
              Unsubscribe and delete its articles?
            </Typography>
            <Button
              size="small"
              variant="contained"
              className="GoliathDangerButton"
              onClick={handleUnsubscribe}
              disabled={busy}
              startIcon={
                busy ? (
                  <CircularProgress size={14} color="inherit" />
                ) : (
                  <DeleteTwoToneIcon />
                )
              }
            >
              Unsubscribe
            </Button>
            {cancelButton}
          </Box>
        );
      default:
        return null;
    }
  };

  return (
    <Box className="GoliathSettingsCardRow GoliathSettingsFeedRow">
      <FeedIcon
        favicon={feed.favicon?.GetFavicon() || ''}
        feedTitle={feed.title}
        feedId={feed.id}
        size={18}
        alt={feed.title}
      />
      {mode === null ? (
        <>
          <Tooltip title={plainTitle} enterDelay={600}>
            <Typography className="GoliathSettingsFeedTitle">
              {plainTitle}
            </Typography>
          </Tooltip>
          <Box className="GoliathSettingsFeedActions">
            <Tooltip title="Rename">
              <IconButton
                size="small"
                aria-label={`Rename ${plainTitle}`}
                onClick={() => enterMode('rename')}
              >
                <EditTwoToneIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title="Move to folder">
              <IconButton
                size="small"
                aria-label={`Move ${plainTitle}`}
                onClick={() => enterMode('move')}
              >
                <DriveFileMoveTwoToneIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title="Unsubscribe">
              <IconButton
                size="small"
                className="GoliathDangerIconButton"
                aria-label={`Unsubscribe from ${plainTitle}`}
                onClick={() => enterMode('unsubscribe')}
              >
                <DeleteTwoToneIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        </>
      ) : (
        <Box className="GoliathSettingsFeedEditing">
          {renderEditor()}
          {error && (
            <Typography className="GoliathDialogError">{error}</Typography>
          )}
        </Box>
      )}
    </Box>
  );
};

export default SettingsModal;
