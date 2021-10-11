import React, {ReactNode, ReactText} from 'react';
import {
  Feed,
  Folder,
  FolderId,
  KeyAll,
  SelectionKey,
  SelectionType
} from "../utils/types";
import {Box} from "@mui/material";
import InboxIcon from "@mui/icons-material/Inbox";
import TreeView from '@mui/lab/TreeView';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import {TreeItem} from "@mui/lab";

export interface FolderFeedListProps {
  tree: Map<FolderId, Folder>;
  unreadCount: number;
  selectedKey: SelectionKey;
  selectionType: SelectionType;
  handleSelect: (type: SelectionType, key: SelectionKey) => void;
}

export interface FolderFeedListState {
  keyCache: Map<ReactText, [SelectionType, SelectionKey]>;
}

export default class FolderFeedList extends React.Component<FolderFeedListProps, FolderFeedListState> {
  constructor(props: FolderFeedListProps) {
    super(props);

    this.state = {
      keyCache: precomputeIdToSelectionKey(this.props.tree)
    };

    this.handleSelect = this.handleSelect.bind(this);
  }

  handleSelect = (_: any, key: string[] | string) => {
    let selectionKey: SelectionKey;
    let selectionType: SelectionType;

    if (typeof key !== 'string') {
      throw new Error("Unexpected type: " + key)
    }

    let entry = this.state.keyCache.get(key);
    if (entry === undefined) {
      throw new Error("Unknown tree key: " + key);
    }
    [selectionType, selectionKey] = entry;

    this.props.handleSelect(selectionType, selectionKey);
  };

  render() {
    const tree = this.props.tree;
    let selectedKeys: string[], allSelectedClass: string;

    if (!this.props.selectedKey || this.props.selectedKey === KeyAll) {
      selectedKeys = [];
      allSelectedClass = 'GoliathAllItemsSelected';
    } else {
      switch (this.props.selectionType) {
      case SelectionType.Article:
        throw new Error(
          "Cannot render folder feed list with article selection");
      case SelectionType.Folder:
        selectedKeys = [this.props.selectedKey as string];
        allSelectedClass = 'GoliathAllItems';
        break;
      case SelectionType.Feed:
        const feedId = this.props.selectedKey[0];
        selectedKeys = [feedId as string];
        allSelectedClass = 'GoliathAllItems';
        break;
      case SelectionType.All: // fallthrough
      default:
        selectedKeys = [];
        allSelectedClass = 'GoliathAllItemsSelected';
      }
    }

    return (
      <Box>
        <Box
          onClick={() => this.handleSelect(null, KeyAll)}
          className={allSelectedClass}>
          <InboxIcon fontSize="small"/>
          <Box className={allSelectedClass}>
            {this.renderAllItemsTitle()}
          </Box>
        </Box>

        {/* TODO: Enable collapsing? */}
        <TreeView
          className="goliath-ant-tree"
          onNodeSelect={this.handleSelect}
          selected={selectedKeys}
          expanded={Array.from(tree.keys(), (k) => k.toString())}
          defaultExpandIcon={<ChevronRightIcon/>}
          defaultCollapseIcon={<ExpandMoreIcon/>}
        >
          {
            Array.from(
              tree.entries(), ([k, v]) => (
                <TreeItem
                  nodeId={k.toString()}
                  label={renderFolderTitle(v)}
                >
                  {Array.from(v.feeds.values()).map(makeFeedRow)}
                </TreeItem>))
          }
        </TreeView>
      </Box>
    )
  }

  renderAllItemsTitle() {
    const unreadCount = this.props.unreadCount;
    if (unreadCount === 0) {
      return 'All items';
    } else {
      return <b>{`(${unreadCount})  All items`}</b>;
    }
  }
}

function precomputeIdToSelectionKey(structure: Map<FolderId, Folder>): Map<string, [SelectionType, SelectionKey]> {
  const cache = new Map<string, [SelectionType, SelectionKey]>();

  // Add a special entry for selecting everything.
  cache.set(KeyAll, [SelectionType.All, KeyAll]);

  structure.forEach(
    (folder: Folder, folderId: FolderId) => {
      cache.set(folderId as string, [SelectionType.Folder, folderId]);
      folder.feeds.forEach(
        (feed: Feed) => {
          cache.set(feed.id as string, [SelectionType.Feed, [feed.id, folderId]]);
        }
      )
    });

  return cache;
}

function renderFolderTitle(folder: Folder) {
  if (folder.unread_count === 0) {
    return folder.title;
  } else {
    return <b>{`(${folder.unread_count})  ${folder.title}`}</b>;
  }
}

function makeFeedRow(feed: Feed) {
  let img: ReactNode;
  if (feed.favicon === '') {
    img = <i className="fas fa-rss-square"/>
  } else {
    img = <img src={`data:${feed.favicon}`} height={16} width={16} alt=''/>
  }
  img = <span className="goliath-feed-icon">{img}</span>;

  let title: ReactNode;
  if (feed.unread_count === 0) {
    title = feed.title;
  } else {
    title = <b>{`(${feed.unread_count})  ${feed.title}`}</b>
  }

  return <TreeItem
    nodeId={feed.id.toString()}
    label={<span className="goliath-feed-title">{title}</span>}
    icon={img}/>
}