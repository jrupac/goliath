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
import FilterAltOutlinedIcon from '@mui/icons-material/FilterAltOutlined';
import FilterAltIcon from '@mui/icons-material/FilterAlt';
import { FolderView } from '../models/folder';
import { FeedView } from '../models/feed';
import BookmarkTwoToneIcon from '@mui/icons-material/BookmarkTwoTone';
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

export interface FolderFeedListProps {
  folderFeedView: Map<FolderView, FeedView[]>;
  unreadCount: number;
  selectedKey: SelectionKey;
  selectionType: SelectionType;
  handleSelect: (type: SelectionType, key: SelectionKey) => void;
  hideEmpty?: boolean;
  toggleHideEmpty?: () => void;
}

const FolderFeedList: React.FC<FolderFeedListProps> = ({
  folderFeedView,
  unreadCount,
  selectedKey,
  selectionType,
  handleSelect,
  hideEmpty = false,
  toggleHideEmpty,
}) => {
  const [keyCache, setKeyCache] = useState<
    Map<string, [SelectionType, SelectionKey]>
  >(() => precomputeIdToSelectionKey(folderFeedView));
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

  useEffect(() => {
    setKeyCache(precomputeIdToSelectionKey(folderFeedView));
  }, [folderFeedView]);

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

  const renderAllTitle = () => 'All items';

  const renderSavedItemsTitle = () => {
    // TODO: Support showing number of saved items.
    return 'Saved items';
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

    const faviconSrc = feedView.favicon?.GetFavicon();
    const img = (
      <FeedIcon
        favicon={faviconSrc || ''}
        feedTitle={feedView.title}
        feedId={feedView.id}
        size={16}
        alt={feedView.title}
      />
    );

    const plainTitle = plainTitles.get(feedView.id) || feedView.title;
    const isSelected = selectedKeyString === feedView.id;
    const hasUnread = feedView.unread_count > 0;

    const pillClass = hasUnread
      ? isSelected
        ? 'GoliathSidebarPill'
        : 'GoliathSidebarPillPlain'
      : 'GoliathSidebarPillPlaceholder';

    return (
      <TreeItem
        key={feedView.id}
        itemId={feedView.id}
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
              <Box className={pillClass}>
                {hasUnread && feedView.unread_count}
              </Box>
            </Box>
          </span>
        }
        className="GoliathFeedRow"
        slots={{ icon: () => img }}
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
        <Box>{renderAllTitle()}</Box>
      </Box>

      <Box
        onClick={(e) => handleItemSelect(e, KeySaved)}
        className={savedSelectedClass}
      >
        <BookmarkTwoToneIcon fontSize="small" />
        <Box>{renderSavedItemsTitle()}</Box>
      </Box>

      <Box className={`${scrolledClass} GoliathFolderFeedHeader `}>
        <p className="GoliathFolderFeedTitle">feeds</p>
        <Tooltip title="Hide feeds with no unread items">
          <IconButton
            className={hideEmpty ? 'GoliathHideEmptyButton' : ''}
            onClick={toggleHideEmpty}
            size="small"
          >
            {hideEmpty ? <FilterAltIcon /> : <FilterAltOutlinedIcon />}
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
