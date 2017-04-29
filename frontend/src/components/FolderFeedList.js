import React from 'react';
import { Tree } from 'antd';
const TreeNode = Tree.TreeNode;

class FolderFeedList extends React.Component {
  render() {
    var tree = this.props.tree;
    return (
        <Tree defaultExpandAll>
          {
            Array.from(tree.keys(), k => {
                var v = tree.get(k);
                return <TreeNode key={k} title={v.title}>
                {v.feeds.map(e => (
                    <TreeNode key={e.favicon_id} title={e.title} />))}
              </TreeNode>
            })
          }
        </Tree>
    )
  }
}

export default FolderFeedList;
