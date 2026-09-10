import React, { ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { extractText } from '../utils/helpers';
import {
  FeedSelection,
  FolderSelection,
  KeyUnread,
  KeyAllItems,
  KeySaved,
  SelectionKey,
  SelectionType,
} from '../utils/types';
import { Box, IconButton, Tooltip } from '@mui/material';
import InboxTwoToneIcon from '@mui/icons-material/InboxTwoTone';
import ListTwoToneIcon from '@mui/icons-material/ListTwoTone';
import { SimpleTreeView } from '@mui/x-tree-view/SimpleTreeView';
import { TreeItem } from '@mui/x-tree-view/TreeItem';
import FolderOpenTwoToneIcon from '@mui/icons-material/FolderOpenTwoTone';
import AddTwoToneIcon from '@mui/icons-material/AddTwoTone';
import { FolderView } from '../models/folder';
import { FeedId, FeedTitle, FeedView } from '../models/feed';
import StarTwoToneIcon from '@mui/icons-material/StarTwoTone';
import FeedIcon from './FeedIcon';

function precomputeIdToSelectionKey(
  folderFeedView: Map<FolderView, FeedView[]>
): Map<string, [SelectionType, SelectionKey]> {
  const cache: Map<string, [SelectionType, SelectionKey]> = new Map();
  cache.set(KeyUnread, [SelectionType.Unread, KeyUnread]);
  cache.set(KeyAllItems, [SelectionType.All, KeyAllItems]);
  cache.set(KeySaved, [SelectionType.Saved, KeySaved]);
  folderFeedView.forEach((value: FeedView[], key: FolderView) => {
    cache.set(key.id, [SelectionType.Folder, key.id]);
    value.forEach((feedView: FeedView) => {
      cache.set(feedView.id, [SelectionType.Feed, [feedView.id, key.id]]);
    });
  });
  return cache;
}

interface FeedTreeItemProps {
  feedId: FeedId;
  plainTitle: string;
  rawTitle: FeedTitle;
  faviconSrc: string;
  unreadCount: number;
  isSelected: boolean;
}

/**
 * A single feed row in the sidebar tree.
 *
 * Memoized, which matters more than it looks. Marking one article read calls
 * setState on App, so the whole tree re-renders on every j/k keypress, and this
 * list holds one row per feed. Without the bail-out, a keypress re-renders a
 * TreeItem, a Tooltip and several Boxes for every feed the user subscribes to,
 * when at most one row's unread count has actually changed.
 *
 * The props are deliberately primitive. FeedCls hands out a single FeedView
 * object that it mutates in place, so memoizing on the view object would
 * compare equal on every render and display stale unread counts.
 */
const FeedTreeItem = React.memo(function FeedTreeItem({
  feedId,
  plainTitle,
  rawTitle,
  faviconSrc,
  unreadCount,
  isSelected,
}: FeedTreeItemProps) {
  const hasUnread = unreadCount > 0;

  const pillClass = hasUnread
    ? isSelected
      ? 'GoliathSidebarPill'
      : 'GoliathSidebarPillPlain'
    : 'GoliathSidebarPillPlaceholder';

  // A fresh `slots` object would give TreeItem a new icon component type on
  // every render, remounting the favicon, so keep its identity stable.
  const slots = useMemo(
    () => ({
      icon: () => (
        <FeedIcon
          favicon={faviconSrc}
          feedTitle={rawTitle}
          feedId={feedId}
          size={16}
          alt={rawTitle}
        />
      ),
    }),
    [faviconSrc, rawTitle, feedId]
  );

  return (
    <TreeItem
      itemId={feedId}
      label={
        <span
          className={
            hasUnread
              ? 'GoliathFeedRowHasUnread GoliathFeedTitle'
              : 'GoliathFeedTitle'
          }
        >
          <Box className="GoliathFeedTitleRow">
            <Tooltip title={plainTitle}>
              <Box className="GoliathFeedTitleText">{plainTitle}</Box>
            </Tooltip>
            <Box className={pillClass}>{hasUnread && unreadCount}</Box>
          </Box>
        </span>
      }
      className="GoliathFeedRow"
      slots={slots}
    />
  );
});

export interface FolderFeedListProps {
  folderFeedView: Map<FolderView, FeedView[]>;
  unreadCount: number;
  selectedKey: SelectionKey;
  selectionType: SelectionType;
  handleSelect: (type: SelectionType, key: SelectionKey) => void;
  hideEmpty?: boolean;
  onAddFeed?: () => void;
}

const FolderFeedList: React.FC<FolderFeedListProps> = ({
  folderFeedView,
  unreadCount,
  selectedKey,
  selectionType,
  handleSelect,
  hideEmpty = false,
  onAddFeed,
}) => {
  // Derived from folderFeedView, so it is memoized rather than held in state
  // and synced by an effect. The effect version re-rendered the whole sidebar a
  // second time on every change, and folderFeedView gets a new identity on
  // every article mark.
  const keyCache = useMemo(
    () => precomputeIdToSelectionKey(folderFeedView),
    [folderFeedView]
  );
  const [isScrolled, setIsScrolled] = useState(false);
  const treeViewRef = useRef<HTMLUListElement>(null);

  useEffect(() => {
    const handleScroll = () => {
      if (treeViewRef.current) {
        setIsScrolled(treeViewRef.current.scrollTop > 0);
      }
    };

    const treeViewElement = treeViewRef.current;
    if (treeViewElement) {
      treeViewElement.addEventListener('scroll', handleScroll);
    }

    return () => {
      if (treeViewElement) {
        treeViewElement.removeEventListener('scroll', handleScroll);
      }
    };
  }, []);

  const shouldRenderItem = (item: FolderView | FeedView): boolean => {
    if (!hideEmpty || item.unread_count > 0) {
      return true;
    }

    let folderId, feedId;

    switch (selectionType) {
      case SelectionType.Folder:
        folderId = selectedKey as FolderSelection;
        return (item as FolderView).id === folderId;
      case SelectionType.Feed:
        [feedId, folderId] = selectedKey as FeedSelection;
        return (
          (item as FeedView).id === feedId ||
          (item as FolderView).id === folderId
        );
      default:
        // This item is not selected, so don't render it regardless of what the
        // selectedKey actually is.
        return false;
    }
  };

  const plainTitles = useMemo(() => {
    const map = new Map<string, string>();
    folderFeedView.forEach((feeds) => {
      feeds.forEach((feed) => {
        map.set(feed.id, extractText(feed.title) || feed.title);
      });
    });
    return map;
  }, [folderFeedView]);

  const expandedItems = useMemo(
    () => Array.from(folderFeedView.keys(), (k: FolderView) => k.id),
    [folderFeedView]
  );

  const selectedKeyString = useMemo(() => {
    let key = '';
    switch (selectionType) {
      case SelectionType.Folder:
        key = selectedKey as string;
        break;
      case SelectionType.Feed: {
        const feedId = (selectedKey as string[])[0];
        key = feedId;
        break;
      }
    }
    return key;
  }, [selectedKey, selectionType]);

  const handleItemSelect = (
    _: React.SyntheticEvent | null,
    itemId: string | null
  ) => {
    if (itemId === null) {
      return;
    }

    let entry = keyCache.get(itemId);
    if (entry === undefined) {
      throw new Error('Unknown tree key: ' + itemId);
    }
    const [selType, selKey] = entry;

    handleSelect(selType, selKey);
  };

  const renderUnreadTitle = () => {
    return (
      <Box className="GoliathStreamContent">
        <span>Unread items</span>
        {unreadCount > 0 && (
          <Box className="GoliathSidebarPill">{unreadCount}</Box>
        )}
      </Box>
    );
  };

  const renderAllTitle = () => {
    return (
      <Box className="GoliathStreamContent">
        <span>All items</span>
      </Box>
    );
  };

  const renderSavedItemsTitle = () => {
    // TODO: Support showing number of saved items.
    return (
      <Box className="GoliathStreamContent">
        <span>Saved items</span>
      </Box>
    );
  };

  const renderFolder = (folderView: FolderView) => {
    const isSelected = selectedKeyString === folderView.id;
    const hasUnread = folderView.unread_count > 0;

    return (
      <Box className="GoliathFolderRowContent">
        <span
          className={
            hasUnread
              ? 'GoliathFolderTitleHasUnread GoliathFolderTitle'
              : 'GoliathFolderTitle'
          }
        >
          <Tooltip title={folderView.title}>
            <Box
              component="span"
              className="GoliathFolderTitleText"
              sx={{
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {folderView.title}
            </Box>
          </Tooltip>
          {hasUnread && (
            <Box
              className={
                isSelected ? 'GoliathSidebarPill' : 'GoliathSidebarPillPlain'
              }
            >
              {folderView.unread_count}
            </Box>
          )}
        </span>
        {!hasUnread && <Box className="GoliathSidebarPillPlaceholder" />}
      </Box>
    );
  };

  const renderFeed = (feedView: FeedView): ReactNode => {
    if (!shouldRenderItem(feedView)) {
      return null;
    }

    return (
      <FeedTreeItem
        key={feedView.id}
        feedId={feedView.id}
        plainTitle={plainTitles.get(feedView.id) || feedView.title}
        rawTitle={feedView.title}
        faviconSrc={feedView.favicon?.GetFavicon() || ''}
        unreadCount={feedView.unread_count}
        isSelected={selectedKeyString === feedView.id}
      />
    );
  };

  const unreadSelectedClass =
    !selectedKey ||
    (selectedKey === KeyUnread && selectionType === SelectionType.Unread)
      ? 'GoliathStreamSelectorSelected'
      : 'GoliathStreamSelector';

  const allSelectedClass =
    selectedKey === KeyAllItems && selectionType === SelectionType.All
      ? 'GoliathStreamSelectorSelected'
      : 'GoliathStreamSelector';

  // TODO: Support saved items CSS classes.
  const savedSelectedClass =
    selectedKey === KeySaved
      ? 'GoliathStreamSelectorSelected'
      : 'GoliathStreamSelector';

  const scrolledClass = isScrolled ? 'GoliathDrawerActionBarScrolled' : '';

  return (
    <>
      <Box className="GoliathFolderFeedHeader">
        <p className="GoliathFolderFeedTitle">streams</p>
      </Box>

      <Box
        onClick={(e) => handleItemSelect(e, KeyUnread)}
        className={unreadSelectedClass}
      >
        <InboxTwoToneIcon fontSize="small" />
        {renderUnreadTitle()}
      </Box>

      <Box
        onClick={(e) => handleItemSelect(e, KeyAllItems)}
        className={allSelectedClass}
      >
        <ListTwoToneIcon fontSize="small" />
        {renderAllTitle()}
      </Box>

      <Box
        onClick={(e) => handleItemSelect(e, KeySaved)}
        className={savedSelectedClass}
      >
        <StarTwoToneIcon fontSize="small" />
        {renderSavedItemsTitle()}
      </Box>

      <Box className={`${scrolledClass} GoliathFolderFeedHeader `}>
        <p className="GoliathFolderFeedTitle">feeds</p>
        <Tooltip title="Add feed">
          <IconButton
            className="GoliathAddFeedButton"
            aria-label="Add feed"
            onClick={onAddFeed}
            size="small"
          >
            <AddTwoToneIcon />
          </IconButton>
        </Tooltip>
      </Box>

      <SimpleTreeView
        ref={treeViewRef}
        className="GoliathFolderFeedList"
        onSelectedItemsChange={handleItemSelect}
        selectedItems={selectedKeyString}
        expandedItems={expandedItems}
        slots={{ collapseIcon: FolderOpenTwoToneIcon }}
      >
        {Array.from(folderFeedView, ([k, v]) => {
          const feedsToRender = v.map(renderFeed).filter(Boolean);
          if (!shouldRenderItem(k)) {
            // If we're not rendering the folder, don't render any of the
            // feeds inside of it either.
            return null;
          }
          return (
            <TreeItem
              key={k.id}
              itemId={k.id}
              label={renderFolder(k)}
              className="GoliathFolderRow"
            >
              {feedsToRender}
            </TreeItem>
          );
        })}
      </SimpleTreeView>
    </>
  );
};

export default FolderFeedList;
