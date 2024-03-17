import React, {ReactNode, ReactText} from 'react';
import {KeyAll, SelectionKey, SelectionType} from "../utils/types";
import {Box} from "@mui/material";
import InboxIcon from "@mui/icons-material/Inbox";
import {TreeView} from '@mui/x-tree-view/TreeView';
import {TreeItem} from '@mui/x-tree-view/TreeItem';
import RssFeedOutlinedIcon from '@mui/icons-material/RssFeedOutlined';
import FolderIcon from '@mui/icons-material/Folder';
import {FolderView} from "../models/folder";
import {FeedView} from "../models/feed";

export interface FolderFeedListProps {
  //tree: Map<FolderId, Folder>;
  folderFeedView: Map<FolderView, FeedView[]>;
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
      keyCache: this.precomputeIdToSelectionKey(this.props.folderFeedView)
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
    const tree = this.props.folderFeedView;
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
          expanded={Array.from(tree.keys(), (k: FolderView) => k.id)}
          defaultCollapseIcon={<FolderIcon/>}
        >
          {
            Array.from(
              tree, ([k, v]) => (
                <TreeItem
                  key={k.id}
                  nodeId={k.id}
                  label={this.renderFolder(k)}
                  className="GoliathFolderRow"
                >
                  {v.map(this.renderFeed)}
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

  renderFolder(folderView: FolderView) {
    if (folderView.unread_count === 0) {
      return <span className="GoliathFolderTitle">{folderView.title}</span>;
    } else {
      return <span className="GoliathFolderTitle">
        <b>{`(${folderView.unread_count})  ${folderView.title}`}</b>
      </span>;
    }
  }

  renderFeed(feedView: FeedView) {
    let img: ReactNode;
    if (feedView.favicon === null) {
      img = <RssFeedOutlinedIcon fontSize="small"/>
    } else {
      img = <img
        src={`data:${feedView.favicon.GetFavicon()}`}
        height={16} width={16} alt=''/>
    }
    img = <span className="GoliathFeedIcon">{img}</span>;

    let title: ReactNode;
    if (feedView.unread_count === 0) {
      title = feedView.title;
      title = <span dangerouslySetInnerHTML={{__html: feedView.title}}/>
    } else {
      title = <b>{`(${feedView.unread_count}) `}
        <span dangerouslySetInnerHTML={{__html: feedView.title}}/></b>
    }

    return <TreeItem
      key={feedView.id}
      nodeId={feedView.id}
      label={<span className="GoliathFeedTitle">{title}</span>}
      className="GoliathFeedRow"
      icon={img}/>
  }

  precomputeIdToSelectionKey(folderFeedView: Map<FolderView, FeedView[]>): Map<string, [SelectionType, SelectionKey]> {
    const cache: Map<string, [SelectionType, SelectionKey]> = new Map();

    // Add a special entry for selecting everything.
    cache.set(KeyAll, [SelectionType.All, KeyAll]);

    folderFeedView.forEach((value: FeedView[], key: FolderView) => {
      cache.set(key.id, [SelectionType.Folder, key.id]);
      value.forEach((feedView: FeedView) => {
        cache.set(feedView.id, [SelectionType.Feed, [feedView.id, key.id]]);
      })
    });

    return cache;
  }
}