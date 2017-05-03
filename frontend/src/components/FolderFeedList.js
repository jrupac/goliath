import React from 'react';
import { Tree, Icon } from 'antd';
const TreeNode = Tree.TreeNode;

class FolderFeedList extends React.Component {
  constructor(props) {
    super(props);
    this.handleSelect = this.handleSelect.bind(this);
  }

  handleSelect = (key, info) => {
    var type;
    if (info === "all") {
      this.props.handleSelect("all", key);
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
    var tree = this.props.tree;
    var selectedKeys, allSelectedClass;
    if (this.props.selectedKey) {
      selectedKeys = [this.props.selectedKey];
      allSelectedClass = "all-items";
    } else {
      selectedKeys = [];
      allSelectedClass = "all-items-selected";
    }

    return (
        <div>
          <div
              onClick={() => this.handleSelect(null, "all")}
              className={allSelectedClass}>
            <Icon type="inbox" />
            <div className="all-items-text">
              All items
            </div>
          </div>
          <Tree
              defaultExpandAll
              selectedKeys={selectedKeys}
              onSelect={this.handleSelect}>
            {
              Array.from(tree.keys(), k => (
                <TreeNode key={k} title={this.renderFolderTitle(tree.get(k))}>
                  {tree.get(k).feeds.map(this.renderFeed)}
                </TreeNode>
              ))
            }
          </Tree>
        </div>
    )
  }

  renderFolderTitle(folder) {
    var unreadCount = folder.feeds.reduce(
        (a, b) => a + b.unread_count, 0);
    if (unreadCount === 0) {
      return folder.title;
    } else {
      return <b>{"(" + unreadCount + ") " + folder.title}</b>;
    }
  }

  renderFeed(f) {
    var img;
    if (f.favicon === "") {
      img = <img src="/favicon.ico" height={16} width={16} alt=""/>
    } else {
      img = <img src={"data:" + f.favicon} height={16} width={16} alt=""/>
    }

    var title;
    if (f.unread_count === 0) {
      title = f.title;
    } else {
      title = <b>{"(" + f.unread_count + ") " + f.title}</b>;
    }

    var elem = (
      <div className="feed-row">
        <div className="feed-icon">
          {img}
        </div>
        <div className="feed-title">
          {title}
        </div>
      </div>
    );
    return <TreeNode key={f.id} title={elem} isLeaf />;
  }
}

export default FolderFeedList;
