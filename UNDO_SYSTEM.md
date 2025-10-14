# Undo System Documentation

## Overview

The RueaES now includes a comprehensive undo/redo system with a visual graph explorer that allows you to navigate through your editing history in a non-linear way.

## Features

### 1. **Automatic Change Tracking**
- Changes to territories are automatically tracked when you:
  - Edit territory settings in the Edge Menu
  - Switch between territories
  - Close a territory menu

### 2. **Smart Diff System**
- Only changed values are stored (not the entire state)
- Minimal memory usage (~few KB per change instead of 60KB for full state)
- Tracks changes to:
  - Tower upgrades (Damage, Attack, Health, Defence)
  - Tower bonuses (all 17 bonus types)
  - Tax settings (tax rate, ally tax rate)
  - Border status (open/closed)
  - Routing mode (cheapest/fastest)
  - HQ status
  - Treasury override
  - Resource storage (manual edits)

### 3. **Non-Linear Undo Tree**
- Traditional undo systems lose your "redo" history when you make a new change
- This system creates a **tree** instead of a linear history
- You can:
  - Undo changes to a territory
  - Make new edits
  - Later navigate back to see what you had before the undo
  - Explore different "branches" of your editing history

## Keyboard Shortcuts

### Quick Undo/Redo
- **`Ctrl+Z`**: Undo the last edit
  - Shows a toast notification with the territory name
  - Can undo back to the initial state (root node)

- **`Ctrl+Y`**: Redo the last undone edit
  - Only works if you haven't branched off to a new edit path
  - Shows a toast notification with the territory name

### Visual Explorer
- **`Z`**: Open/close the Undo History Explorer
  - Visual graph showing all your edits as connected nodes
  - Current state is highlighted in bright green
  - Active path is shown in blue
  - Inactive branches are shown in gray

## Using the Undo Explorer

### Navigation
- **Mouse Wheel**: Zoom in/out
  - Zoom range: 0.2x to 3.0x
  - Zooms towards your mouse cursor position

- **Left Click + Drag**: Pan the view
  - Move around to see different parts of the tree
  - Especially useful for large editing histories

- **Click on Node**: Jump to that state instantly
  - All territory changes are applied immediately
  - Works across branches (non-linear navigation!)

### Visual Cues
- **Bright Green**: Your current state
- **Blue Nodes/Lines**: Nodes in the active path from root to current
- **Gray Nodes/Lines**: Inactive branches (alternative histories)
- **Orange**: Node you're hovering over
- **Yellow Border**: Current node border

### Node Labels
- Each node shows a description like:
  - "Detlas: Changed Damage"
  - "Ragni: Set HQ"
  - "Corkus City: 3 changes"
  - Territory name is always included

### Graph Layout
- Root node (initial state) is at the top-left
- Children branch downward and to the right
- Automatic spacing prevents overlap

## How It Works

### Change Detection
1. When you open a territory menu, the system captures the current state
2. As you make changes, they're pending in memory
3. When you close the menu or switch territories:
   - New state is captured
   - Only differences are stored
   - If no changes were made, nothing is saved

### Tree Structure
```
     Root (Initial)
        |
     Edit 1 (Territory A)
        |
     Edit 2 (Territory B)
       / \
      /   \
 Edit 3   Edit 4  <- You undid Edit 2, made Edit 4, but Edit 3 still exists!
     |
 Edit 5
```

### Memory Efficiency
- **Traditional approach**: Each save = ~60KB (full state)
- **This approach**: Each save = ~few KB (only changes)
- Example: Changing one upgrade level = ~100 bytes
- Even 100 edits ≈ 10KB instead of 6MB

## Use Cases

### 1. Experiment Safely
```
1. Edit Territory A's upgrades
2. Don't like it? Ctrl+Z
3. Try different upgrades
4. Still curious about the first version? Press Z, click that node
```

### 2. Compare Configurations
```
1. Set up configuration A
2. Undo and set up configuration B
3. Use Explorer to quickly switch between A and B
4. Keep both configurations accessible
```

### 3. Branch Your Workflow
```
1. Base configuration (root)
2. Try optimization for production (branch 1)
3. Undo to base
4. Try optimization for defense (branch 2)
5. Switch between them anytime via Explorer
```

## Tips

- **Press Z regularly** to visualize your editing history
- **Use Ctrl+Z for quick undos** of recent changes
- **Use the Explorer** for bigger jumps or exploring old branches
- **Zoom out** to see the overall structure of your edits
- **Zoom in** to read node labels more clearly
- The system automatically starts when you open any territory menu
- No manual save needed - everything is tracked automatically

## Technical Details

### Node Data Structure
Each node contains:
- Unique ID
- Territory name
- Timestamp
- Description
- Parent reference
- Children references
- X, Y position (for visualization)
- Active/inactive flag

### Change Storage Format
```json
{
  "territory_name": "Detlas",
  "timestamp": "2025-10-11T12:34:56Z",
  "changes": {
    "damage": 10,
    "attack": 5,
    "tax": 0.15
  }
}
```

### Path Navigation Algorithm
- Finds common ancestor between current and target nodes
- Applies snapshots in correct order to reach target
- Handles arbitrary tree structures (not just linear paths)

## Limitations

- Undo history is not persisted across sessions (restarting the app clears it)
- Maximum practical tree size: ~1000 nodes (memory limits)
- Cannot undo changes made outside the territory menu (e.g., API imports, batch operations)
- Cannot undo guild ownership changes (only territory settings)

## Future Enhancements (Not Yet Implemented)

- Save/load undo history to disk
- Search nodes by territory name or description
- Diff view showing exact changes between two nodes
- Bookmark important nodes
- Prune old branches to save memory
- Undo for guild management operations

---

**Remember**: The undo system is your safety net. Experiment freely - you can always go back!
