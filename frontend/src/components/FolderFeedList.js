import React from 'react';
import { Tree } from 'antd';
const TreeNode = Tree.TreeNode;

class FolderFeedList extends React.Component {
  render() {
    var tree = this.props.tree;
    return (
        <Tree defaultExpandAll>
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
    return <TreeNode key={f.favicon_id} title={f.title} isLeaf />;
  }
}

export default FolderFeedList;
