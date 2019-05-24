import React, {ReactNode} from 'react';
import Tree from 'antd/lib/tree';
import {FolderData} from '../App';
import {
  FeedType,
  FolderId,
  KeyAll,
  SelectionKey,
  SelectionType
} from "../utils/types";

const TreeNode = Tree.TreeNode;

export interface FolderFeedListProps {
  tree: Map<FolderId, FolderData>;
  handleSelect: (type: SelectionType, key: SelectionKey) => void;
  selectedKey: SelectionKey;
  selectionType: SelectionType;
  unreadCount: number;
}

export interface FolderFeedListState {
  keyCache: Map<string, [SelectionType, SelectionKey]>;
}

export default class FolderFeedList extends React.Component<FolderFeedListProps, FolderFeedListState> {
  constructor(props: FolderFeedListProps) {
    super(props);

    this.state = {
      keyCache: precomputeIdToSelectionKey(this.props.tree)
    };

    this.handleSelect = this.handleSelect.bind(this);
  }

  handleSelect = (keys: string[]) => {
    let selectionKey: SelectionKey;
    let selectionType: SelectionType;
    let key: string;

    // This tree can only have one node selected at a time.
    if (keys && keys.length === 1) {
      key = keys[0];
    } else {
      throw new Error("Unexpected tree node selection: " + keys.toString());
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
      allSelectedClass = 'all-items-selected';
    } else {
      switch (this.props.selectionType) {
        case SelectionType.Article:
          throw new Error(
            "Cannot render folder feed list with article selection");
        case SelectionType.Folder:
          selectedKeys = [this.props.selectedKey as string];
          allSelectedClass = 'all-items';
          break;
        case SelectionType.Feed:
          const feedId = this.props.selectedKey[0];
          selectedKeys = [feedId as string];
          allSelectedClass = 'all-items';
          break;
        case SelectionType.All: // fallthrough
        default:
          selectedKeys = [];
          allSelectedClass = 'all-items-selected';
      }
    }

    // TODO: Implement folder expansion with override correctly.
    // const expandedKeys = Array.from(tree.entries()).map(
    //     ([k, v]) => ( (v.unread_count > 0) ? k : null));

    return (
      <div>
        <div
          onClick={() => this.handleSelect([KeyAll])}
          className={allSelectedClass}>
          <i
            className="fas fa-inbox"
            aria-hidden="true"/>
          <div className='all-items-text'>
            {this.renderAllItemsTitle()}
          </div>
        </div>
        <Tree
          defaultExpandAll
          selectedKeys={selectedKeys}
          onSelect={this.handleSelect}>
          {
            Array.from(tree.entries(), ([k, v]) => (
              <TreeNode
                key={k.toString()}
                title={renderFolderTitle(v)}>
                {Array.from(v.feedMap.values()).map(renderFeed)}
              </TreeNode>
            ))
          }
        </Tree>
      </div>
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

function precomputeIdToSelectionKey(structure: Map<FolderId, FolderData>): Map<string, [SelectionType, SelectionKey]> {
  const cache = new Map<string, [SelectionType, SelectionKey]>();

  // Add a special entry for selecting everything.
  cache.set(KeyAll, [SelectionType.All, KeyAll]);

  structure.forEach(
    (folder: FolderData, folderId: FolderId) => {
      cache.set(folderId as string, [SelectionType.Folder, folderId]);
      folder.feedMap.forEach(
        (feed: FeedType) => {
          cache.set(feed.id as string, [SelectionType.Feed, [feed.id, folderId]]);
        }
      )
    });

  return cache;
}

function renderFolderTitle(folder: FolderData) {
  if (folder.unread_count === 0) {
    return folder.title;
  } else {
    return <b>{`(${folder.unread_count})  ${folder.title}`}</b>;
  }
}

function renderFeed(feed: FeedType) {
  let img: ReactNode;
  if (feed.favicon === '') {
    img = <i className="fas fa-rss-square"/>
  } else {
    img = <img src={`data:${feed.favicon}`} height={16} width={16} alt=''/>
  }

  let title: ReactNode;
  if (feed.unread_count === 0) {
    title = feed.title;
  } else {
    title = <b>{`(${feed.unread_count})  ${feed.title}`}</b>
  }

  const elem = (
    <div className='feed-row'>
      <div className='feed-icon'>
        {img}
      </div>
      <div className='feed-title' title={feed.title}>
        {title}
      </div>
    </div>
  );
  return <TreeNode key={feed.id.toString()} title={elem} isLeaf/>;
}