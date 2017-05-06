import React from 'react';
import Icon from 'antd/lib/icon';
import Tree from 'antd/lib/tree';
import favicon from '../../public/favicon.ico';

const TreeNode = Tree.TreeNode;

export default class FolderFeedList extends React.Component {
  constructor(props) {
    super(props);
    this.handleSelect = this.handleSelect.bind(this);
  }

  handleSelect = (key, info) => {
    var type;
    if (info === 'all') {
      this.props.handleSelect('all', key);
      return;
    } else if (key && key.length === 1) {
      key = key[0];
    } else {
      return;
    }

    if (this.props.tree.has(key)) {
      type = 'folder';
    } else {
      type = 'feed';
    }
    this.props.handleSelect(type, key);
  };

  render() {
    const tree = this.props.tree;
    var selectedKeys, allSelectedClass;
    if (this.props.selectedKey) {
      selectedKeys = [this.props.selectedKey];
      allSelectedClass = 'all-items';
    } else {
      selectedKeys = [];
      allSelectedClass = 'all-items-selected';
    }

    // TODO: Implement folder expansion with override correctly.
    // const expandedKeys = Array.from(tree.entries()).map(
    //     ([k, v]) => ( (v.unread_count > 0) ? k : null));

    return (
        <div>
          <div
              onClick={() => this.handleSelect(null, 'all')}
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
      img = <img src={favicon} height={16} width={16} alt=''/>
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
