import React from 'react';
import { Tree } from 'antd';
const TreeNode = Tree.TreeNode;

class FolderFeedList extends React.Component {
  handleSelect = (key) => {
    key = Number(key);
    var type;
    if (this.props.tree.has(key)) {
      type = 'folder';
    } else {
      type = 'feed';
    }
    this.props.handleSelect(key, type);
  };

  render() {
    var tree = this.props.tree;
    return (
        <Tree defaultExpandAll onSelect={this.handleSelect}>
          {
            Array.from(tree.keys(), k => (
              <TreeNode key={k} title={tree.get(k).title}>
                {tree.get(k).feeds.map(this.renderFeed)}
              </TreeNode>
            ))
          }
        </Tree>
    )
  }

  renderFeed(f) {
    var img;
    if (f.favicon === "") {
      img = <img src="/favicon.ico" height={16} width={16} alt=""/>
    } else {
      img = <img src={"data:" + f.favicon} height={16} width={16} alt=""/>
    }

    var elem = (
      <div className="feed-row">
        <div className="feed-icon">
          {img}
        </div>
        <div className="feed-title">
          {f.title}
        </div>
      </div>
    );
    return <TreeNode key={f.id} title={elem} isLeaf />;
  }
}

export default FolderFeedList;
