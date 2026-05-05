import { Box, Paper, Typography } from '@mui/material'
import { SimpleTreeView } from '@mui/x-tree-view/SimpleTreeView'
import { TreeItem } from '@mui/x-tree-view/TreeItem'

export interface TreePanelNode {
  id: string
  label: string
  hasChildren: boolean
  children?: TreePanelNode[]
}

interface TreePanelProps {
  title: string
  nodes: TreePanelNode[]
  onSelect: (nodeId: string) => void
  onExpand?: (nodeId: string) => void
  selectedItemId?: string
  expandedItemIds?: string[]
  onExpandedItemIdsChange?: (itemIds: string[]) => void
  loading?: boolean
  loadingLabel: string
  emptyLabel: string
  minWidth?: number
}

const pendingChildPrefix = '__pending_child__'

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
      {node.children && node.children.length > 0 ? (
        renderNodes(node.children, onSelect)
      ) : node.hasChildren ? (
        <TreeItem
          disabled
          itemId={`${pendingChildPrefix}:${node.id}`}
          label={<Box sx={{ display: 'none' }} />}
          sx={{ display: 'none' }}
        />
      ) : null}
    </TreeItem>
  ))
}

export function TreePanel({
  title,
  nodes,
  onSelect,
  onExpand,
  selectedItemId,
  expandedItemIds,
  onExpandedItemIdsChange,
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
        <SimpleTreeView
          expandedItems={expandedItemIds}
          onItemExpansionToggle={(_event, itemId, isExpanded) => {
            if (onExpandedItemIdsChange && expandedItemIds) {
              const nextItems = isExpanded
                ? Array.from(new Set([...expandedItemIds, itemId]))
                : expandedItemIds.filter((expandedItemId) => expandedItemId !== itemId)
              onExpandedItemIdsChange(nextItems)
            }
            if (isExpanded && onExpand) {
              onExpand(itemId)
            }
          }}
          selectedItems={selectedItemId ?? null}
        >
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
