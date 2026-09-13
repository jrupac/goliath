import React, { FormEvent, useMemo, useState } from 'react';
import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from '@mui/material';
import CloseTwoToneIcon from '@mui/icons-material/CloseTwoTone';
import { FolderView } from '../models/folder';
import { FeedId, FeedView } from '../models/feed';
import { AddedFeed, FolderSummary } from '../api/interface';
import { FolderId } from '../models/folder';

// A feed the user has filed nowhere is presented under this name. The server
// reserves it, so a folder answering to it is the one such feeds land in and
// no user-made folder can be confused with it.
export const UnfiledFolderTitle = 'Uncategorized';

// Sentinel for a folder picker's "New folder…" choice. Real folder IDs are
// numeric, so this cannot collide with one.
export const NewFolderChoice = 'new-folder';

// folderNameProblem says what is wrong with a name for a new or renamed
// folder, or returns null if nothing is. The server has the final say; this
// catches what it would refuse or misread before a request is made.
export const folderNameProblem = (name: string): string | null => {
  if (name === '') {
    return 'Enter a name for the folder.';
  }
  // Folders are named to the server by ID, and an ID is a number, so a name
  // that is also a number would be read as an ID.
  if (/^[+-]?\d+$/.test(name)) {
    return 'A folder name cannot be just a number.';
  }
  if (name.toLowerCase() === UnfiledFolderTitle.toLowerCase()) {
    return `${UnfiledFolderTitle} is a reserved name.`;
  }
  return null;
};

// Sentinel for the folder picker meaning the unfiled folder, whether or not
// the client has seen it yet. Real folder IDs are numeric, so this cannot
// collide with one.
const KeepUnfiled = 'unfiled';

export interface QuickAddDialogProps {
  open: boolean;
  onClose: () => void;
  folderFeedView: Map<FolderView, FeedView[]>;
  listFolders: () => Promise<FolderSummary[]>;
  addFeed: (url: string) => Promise<AddedFeed>;
  moveFeed: (feedId: FeedId, toFolderId: FolderId) => Promise<void>;
  moveFeedToNewFolder: (feedId: FeedId, folderName: string) => Promise<void>;
  unsubscribeFeed: (feedId: FeedId) => Promise<void>;
  // Called once the subscription list has changed and is worth re-reading.
  onChanged: () => void;
}

type Step = 'url' | 'folder';

/**
 * Adds a feed in two steps: the address, then the folder.
 *
 * The server has no way to inspect a feed without subscribing to it, so the
 * first step both validates the address and creates the subscription, unfiled.
 * The second step is a move. Cancelling between the two undoes the first step,
 * so backing out leaves nothing behind -- unless the address was already
 * subscribed, in which case the add created nothing and there is nothing to
 * undo. The server reports both cases identically, so which one it was is
 * decided by whether the feed it names was already known here.
 *
 * The folder step starts at the folder the feed is actually in. That is the
 * unfiled one for a new subscription, but not for an address already
 * subscribed, nor for a feed the user had removed and got back.
 */
const QuickAddDialog: React.FC<QuickAddDialogProps> = ({
  open,
  onClose,
  folderFeedView,
  listFolders,
  addFeed,
  moveFeed,
  moveFeedToNewFolder,
  unsubscribeFeed,
  onChanged,
}) => {
  const [step, setStep] = useState<Step>('url');
  const [url, setUrl] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [added, setAdded] = useState<AddedFeed | null>(null);
  const [alreadySubscribed, setAlreadySubscribed] = useState(false);
  const [folderId, setFolderId] = useState<FolderId>(KeepUnfiled);
  // Where the feed is, in the picker's terms, so that Done moves it only if
  // a different folder was chosen.
  const [currentFolderId, setCurrentFolderId] = useState<FolderId>(KeepUnfiled);
  const [newFolderName, setNewFolderName] = useState('');
  const [listed, setListed] = useState<FolderSummary[]>([]);

  // Runs once the dialog has finished closing, so the next open starts
  // afresh without the contents changing while it is still fading out.
  const reset = () => {
    setStep('url');
    setUrl('');
    setBusy(false);
    setError(null);
    setAdded(null);
    setAlreadySubscribed(false);
    setFolderId(KeepUnfiled);
    setCurrentFolderId(KeepUnfiled);
    setNewFolderName('');
    setListed([]);
  };

  const knownFeedIds = useMemo(() => {
    const ids = new Set<FeedId>();
    folderFeedView.forEach((feeds) => feeds.forEach((f) => ids.add(f.id)));
    return ids;
  }, [folderFeedView]);

  const folders = useMemo(
    () => Array.from(folderFeedView.keys()),
    [folderFeedView]
  );
  const hasUnfiledFolder = folders.some((f) => f.title === UnfiledFolderTitle);

  // Folders holding no feeds are missing from the view, which is built from
  // subscriptions, but are as good a place to file a feed as any other. The
  // unfiled folder is already offered as the default.
  const emptyFolders = listed.filter(
    (f) => f.title !== UnfiledFolderTitle && !folders.some((v) => v.id === f.id)
  );

  // A failure here leaves only the folders already known on offer, which is
  // no reason to report the add itself as having failed.
  const loadFolders = async (): Promise<FolderSummary[]> => {
    try {
      const found = await listFolders();
      setListed(found);
      return found;
    } catch (err) {
      console.error(`Failed to list folders: ${err}`);
      return [];
    }
  };

  const unfiledFolderId = [...folders, ...listed].find(
    (f) => f.title === UnfiledFolderTitle
  )?.id;

  // The picker's value for the folder a feed is in. A folder the picker does
  // not offer is taken as unfiled, which leaves the feed where it is.
  const pickerValueFor = (
    feedFolderId: FolderId | undefined,
    found: FolderSummary[]
  ): FolderId => {
    const offered = [...folders, ...found].find((f) => f.id === feedFolderId);
    if (offered === undefined || offered.title === UnfiledFolderTitle) {
      return KeepUnfiled;
    }
    return offered.id;
  };

  const handleSubmitUrl = async (e: FormEvent) => {
    e.preventDefault();
    const trimmed = url.trim();
    if (trimmed === '' || busy) {
      return;
    }

    setBusy(true);
    setError(null);
    try {
      const feed = await addFeed(trimmed);
      const found = await loadFolders();
      const current = pickerValueFor(feed.folderId, found);
      setAdded(feed);
      setAlreadySubscribed(knownFeedIds.has(feed.id));
      setCurrentFolderId(current);
      setFolderId(current);
      setStep('folder');
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const handleDone = async () => {
    if (added === null || busy) {
      return;
    }
    if (folderId === currentFolderId) {
      onChanged();
      onClose();
      return;
    }

    let move: () => Promise<void>;
    if (folderId === NewFolderChoice) {
      const name = newFolderName.trim();
      const problem = folderNameProblem(name);
      if (problem !== null) {
        setError(problem);
        return;
      }
      move = () => moveFeedToNewFolder(added.id, name);
    } else if (folderId === KeepUnfiled) {
      // Reached only for a feed already filed somewhere, which is being
      // taken out of its folder.
      if (unfiledFolderId === undefined) {
        setError(`Could not find the ${UnfiledFolderTitle} folder.`);
        return;
      }
      move = () => moveFeed(added.id, unfiledFolderId);
    } else {
      move = () => moveFeed(added.id, folderId);
    }

    setBusy(true);
    setError(null);
    try {
      await move();
      onChanged();
      onClose();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const handleCancel = async () => {
    if (busy) {
      return;
    }
    // Only a subscription this dialog created is undone. One that already
    // existed is the user's, and backing out of a duplicate add must not
    // take it with it.
    if (step === 'folder' && added !== null && !alreadySubscribed) {
      setBusy(true);
      try {
        await unsubscribeFeed(added.id);
      } catch (err) {
        // The subscription stays; say so rather than pretend it went.
        setError((err as Error).message);
        setBusy(false);
        return;
      }
      setBusy(false);
    }
    onClose();
  };

  const renderUrlStep = () => (
    <form onSubmit={handleSubmitUrl} className="GoliathQuickAddForm">
      <Box className="GoliathDialogContent">
        <TextField
          autoFocus
          fullWidth
          name="url"
          label="Feed address"
          placeholder="https://example.com/feed.xml"
          value={url}
          error={error !== null}
          helperText={error ?? 'The feed is fetched before it is added.'}
          disabled={busy}
          onChange={(e) => setUrl(e.target.value)}
          className="GoliathAccentField GoliathDialogField"
          slotProps={{ htmlInput: { inputMode: 'url', spellCheck: false } }}
        />
      </Box>
      <Box className="GoliathDialogFooter">
        <Button
          className="GoliathQuietButton"
          onClick={handleCancel}
          disabled={busy}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="contained"
          className="GoliathAccentButton"
          disabled={busy || url.trim() === ''}
          startIcon={
            busy ? <CircularProgress size={14} color="inherit" /> : undefined
          }
        >
          {busy ? 'Checking' : 'Add'}
        </Button>
      </Box>
    </form>
  );

  const renderFolderStep = () => (
    <>
      <Box className="GoliathDialogContent">
        <Typography className="GoliathDialogText">
          {alreadySubscribed ? 'Already subscribed to ' : 'Subscribed to '}
          <strong>{added?.title}</strong>.
        </Typography>
        <FormControl
          fullWidth
          className="GoliathAccentField GoliathDialogField"
        >
          <InputLabel id="goliath-quickadd-folder-label">Folder</InputLabel>
          <Select
            labelId="goliath-quickadd-folder-label"
            label="Folder"
            value={folderId}
            disabled={busy}
            onChange={(e) => {
              setFolderId(e.target.value as FolderId);
              setError(null);
            }}
            MenuProps={{ classes: { paper: 'GoliathMenuPaper' } }}
          >
            {!hasUnfiledFolder && (
              <MenuItem value={KeepUnfiled}>{UnfiledFolderTitle}</MenuItem>
            )}
            {folders.map((f) => (
              <MenuItem
                key={f.id}
                value={f.title === UnfiledFolderTitle ? KeepUnfiled : f.id}
              >
                {f.title}
              </MenuItem>
            ))}
            {emptyFolders.map((f) => (
              <MenuItem key={f.id} value={f.id}>
                {f.title}
              </MenuItem>
            ))}
            <MenuItem
              value={NewFolderChoice}
              className="GoliathNewFolderChoice"
            >
              New folder…
            </MenuItem>
          </Select>
        </FormControl>
        {folderId === NewFolderChoice && (
          <TextField
            autoFocus
            fullWidth
            label="New folder name"
            value={newFolderName}
            disabled={busy}
            onChange={(e) => setNewFolderName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                handleDone();
              }
            }}
            className="GoliathAccentField GoliathDialogField"
          />
        )}
        {error && (
          <Typography className="GoliathDialogError">{error}</Typography>
        )}
      </Box>
      <Box className="GoliathDialogFooter">
        <Button
          className="GoliathQuietButton"
          onClick={handleCancel}
          disabled={busy}
        >
          {alreadySubscribed ? 'Close' : 'Cancel'}
        </Button>
        <Button
          variant="contained"
          className="GoliathAccentButton"
          onClick={handleDone}
          disabled={busy}
          startIcon={
            busy ? <CircularProgress size={14} color="inherit" /> : undefined
          }
        >
          Done
        </Button>
      </Box>
    </>
  );

  return (
    <Dialog
      open={open}
      onClose={handleCancel}
      maxWidth="xs"
      fullWidth
      slotProps={{
        backdrop: { className: 'GoliathModalOverlay' },
        paper: { className: 'GoliathModalPaper' },
        transition: { onExited: reset },
      }}
    >
      <Box className="GoliathDialogHeader">
        <Typography component="h2" className="GoliathDialogHeading">
          Add feed
        </Typography>
        <IconButton
          aria-label="Close dialog"
          className="GoliathDialogClose"
          onClick={handleCancel}
          size="small"
          disabled={busy}
        >
          <CloseTwoToneIcon fontSize="small" />
        </IconButton>
      </Box>
      {step === 'url' ? renderUrlStep() : renderFolderStep()}
    </Dialog>
  );
};

export default QuickAddDialog;
