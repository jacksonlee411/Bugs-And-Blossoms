import { Box, Paper, Typography } from '@mui/material'
import { SimpleTreeView } from '@mui/x-tree-view/SimpleTreeView'
import { TreeItem } from '@mui/x-tree-view/TreeItem'

export interface TreePanelNode {
  id: string
  label: string
  children?: TreePanelNode[]
}

interface TreePanelProps {
  title: string
  nodes: TreePanelNode[]
  onSelect: (nodeId: string) => void
  selectedItemId?: string
  loading?: boolean
  loadingLabel: string
  emptyLabel: string
  minWidth?: number
}

function renderNodes(nodes: TreePanelNode[], onSelect: (nodeId: string) => void) {
  return nodes.map((node) => (
    <TreeItem
      itemId={node.id}
      key={node.id}
      label={
        <Box
          onClick={() => onSelect(node.id)}
          sx={{ cursor: 'pointer', py: 0.5 }}
        >
          {node.label}
        </Box>
      }
    >
      {node.children ? renderNodes(node.children, onSelect) : null}
    </TreeItem>
  ))
}

export function TreePanel({
  title,
  nodes,
  onSelect,
  selectedItemId,
  loading = false,
  loadingLabel,
  emptyLabel,
  minWidth = 260
}: TreePanelProps) {
  return (
    <Paper sx={{ minWidth, p: 2 }} variant='outlined'>
      <Typography sx={{ mb: 1 }} variant='subtitle2'>
        {title}
      </Typography>
      {nodes.length === 0 ? (
        <Typography color='text.secondary' variant='body2'>
          {emptyLabel}
        </Typography>
      ) : (
        <SimpleTreeView selectedItems={selectedItemId}>
          {renderNodes(nodes, onSelect)}
        </SimpleTreeView>
      )}
      {loading ? (
        <Typography color='text.secondary' sx={{ mt: 1 }} variant='body2'>
          {loadingLabel}
        </Typography>
      ) : null}
    </Paper>
  )
}
