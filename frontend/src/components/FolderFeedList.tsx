import React, {ReactNode, ReactText} from 'react';
import {KeyAll, SelectionKey, SelectionType} from "../utils/types";
import {Box} from "@mui/material";
import InboxIcon from "@mui/icons-material/Inbox";
import {TreeView} from '@mui/x-tree-view/TreeView';
import {TreeItem} from '@mui/x-tree-view/TreeItem';
import RssFeedOutlinedIcon from '@mui/icons-material/RssFeedOutlined';
import FolderIcon from '@mui/icons-material/Folder';
import {Folder, FolderId} from "../models/folder";
import {Feed} from "../models/feed";

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
      keyCache: this.precomputeIdToSelectionKey(this.props.tree)
    };

    this.handleSelect = this.handleSelect.bind(this);
  }

  handleSelect = (_: any, key: string[] | string) => {
    let selectionKey: SelectionKey;
    let selectionType: SelectionType;

    // Since the tree is not multi-select, we should receive a single string.
    if (typeof key !== 'string') {
      throw new Error("Unexpected selection key: " + key)
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
    let selectedKey: string, allSelectedClass: string;

    if (!this.props.selectedKey || this.props.selectedKey === KeyAll) {
      selectedKey = '';
      allSelectedClass = 'GoliathAllItemsSelected';
    } else {
      switch (this.props.selectionType) {
      case SelectionType.Article:
        throw new Error(
          "Cannot render folder feed list with article selection");
      case SelectionType.Folder:
        selectedKey = this.props.selectedKey as string;
        allSelectedClass = 'GoliathAllItems';
        break;
      case SelectionType.Feed: {
        const feedId = this.props.selectedKey[0];
        selectedKey = feedId as string;
        allSelectedClass = 'GoliathAllItems';
        break;
      }
      case SelectionType.All: // fallthrough
      default:
        selectedKey = '';
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

        <TreeView
          className="GoliathFolderFeedList"
          onNodeSelect={this.handleSelect}
          selected={selectedKey}
          expanded={Array.from(tree.keys(), (k) => k.toString())}
          defaultCollapseIcon={<FolderIcon/>}
        >
          {
            Array.from(
              tree.entries(), ([k, v]) => (
                <TreeItem
                  key={k.toString()}
                  nodeId={k.toString()}
                  label={this.renderFolder(v)}
                  className="GoliathFolderRow"
                >
                  {Array.from(v.feeds.values()).map(this.renderFeed)}
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

  renderFolder(folder: Folder) {
    if (folder.unread_count === 0) {
      return <span className="GoliathFolderTitle">{folder.title}</span>;
    } else {
      return <span className="GoliathFolderTitle">
        <b>{`(${folder.unread_count})  ${folder.title}`}</b>
      </span>;
    }
  }

  renderFeed(feed: Feed) {
    let img: ReactNode;
    if (feed.favicon === '') {
      img = <RssFeedOutlinedIcon fontSize="small"/>
    } else {
      img = <img src={`data:${feed.favicon}`} height={16} width={16} alt=''/>
    }
    img = <span className="GoliathFeedIcon">{img}</span>;

    let title: ReactNode;
    if (feed.unread_count === 0) {
      title = feed.title;
      title = <span dangerouslySetInnerHTML={{__html: feed.title}}/>
    } else {
      title = <b>{`(${feed.unread_count}) `}
        <span dangerouslySetInnerHTML={{__html: feed.title}}/></b>
    }

    return <TreeItem
      key={feed.id.toString()}
      nodeId={feed.id.toString()}
      label={<span className="GoliathFeedTitle">{title}</span>}
      className="GoliathFeedRow"
      icon={img}/>
  }

  precomputeIdToSelectionKey(structure: Map<FolderId, Folder>): Map<string, [SelectionType, SelectionKey]> {
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
}