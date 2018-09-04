import React from 'react';
import Tree from 'antd/lib/tree';
import {EnclosingType, KeyAll} from '../App';

const TreeNode = Tree.TreeNode;

export default class FolderFeedList extends React.Component {
  constructor(props) {
    super(props);
    this.handleSelect = this.handleSelect.bind(this);
  }

  handleSelect = (key, type) => {
    if (type === EnclosingType.All) {
      this.props.handleSelect(EnclosingType.All, key);
      return;
    } else if (key && key.length === 1) {
      key = key[0];
    } else {
      return;
    }

    if (this.props.tree.has(key)) {
      type = EnclosingType.Folder;
    } else {
      type = EnclosingType.Feed;
    }
    this.props.handleSelect(type, key);
  };

  render() {
    const tree = this.props.tree;
    let selectedKeys, allSelectedClass;
    if (!this.props.selectedKey || this.props.selectedKey === KeyAll) {
      selectedKeys = [];
      allSelectedClass = 'all-items-selected';
    } else if (this.props.selectedKey) {
      selectedKeys = [this.props.selectedKey];
      allSelectedClass = 'all-items';
    }

    // TODO: Implement folder expansion with override correctly.
    // const expandedKeys = Array.from(tree.entries()).map(
    //     ([k, v]) => ( (v.unread_count > 0) ? k : null));

    return (
      <div>
        <div
          onClick={() => this.handleSelect(KeyAll, EnclosingType.All)}
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
              <TreeNode key={k} title={renderFolderTitle(v)}>
                {v.feeds.map(renderFeed)}
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

function renderFolderTitle(folder) {
  if (folder.unread_count === 0) {
    return folder.title;
  } else {
    return <b>{`(${folder.unread_count})  ${folder.title}`}</b>;
  }
}

function renderFeed(feed) {
  let img;
  if (feed.favicon === '') {
    img = <i className="fas fa-rss-square" />
  } else {
    img = <img src={`data:${feed.favicon}`} height={16} width={16} alt=''/>
  }

  let title;
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
      <div className='feed-title'>
        {title}
      </div>
    </div>
  );
  return <TreeNode key={feed.id} title={elem} isLeaf/>;
}