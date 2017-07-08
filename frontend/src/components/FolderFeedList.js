import defaultFavicon from '../favicon.ico';
import React from 'react';
import Icon from 'antd/lib/icon';
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
    var selectedKeys, allSelectedClass;
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
            <Icon type='inbox' />
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
                <TreeNode key={k} title={this.renderFolderTitle(v)}>
                  {v.feeds.map(this.renderFeed)}
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

  renderFolderTitle(folder) {
    if (folder.unread_count === 0) {
      return folder.title;
    } else {
      return <b>{`(${folder.unread_count})  ${folder.title}`}</b>;
    }
  }

  renderFeed(f) {
    var img;
    if (f.favicon === '') {
      img = <img src={defaultFavicon} height={16} width={16} alt=''/>
    } else {
      img = <img src={`data:${f.favicon}`} height={16} width={16} alt=''/>
    }

    var title;
    if (f.unread_count === 0) {
      title = f.title;
    } else {
      title = <b>{`(${f.unread_count})  ${f.title}`}</b>
    }

    var elem = (
      <div className='feed-row'>
        <div className='feed-icon'>
          {img}
        </div>
        <div className='feed-title'>
          {title}
        </div>
      </div>
    );
    return <TreeNode key={f.id} title={elem} isLeaf />;
  }
}
