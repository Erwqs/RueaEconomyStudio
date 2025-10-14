package app

import (
	"RueaES/eruntime"
	"RueaES/typedef"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// TerritorySnapshot represents a snapshot of territory state for undo/redo
type TerritorySnapshot struct {
	TerritoryName string                 `json:"territory_name"`
	Timestamp     time.Time              `json:"timestamp"`
	Changes       map[string]interface{} `json:"changes"` // Complete state
}

// UndoNode represents a node in the undo tree
type UndoNode struct {
	ID             string            `json:"id"`
	BeforeSnapshot TerritorySnapshot `json:"before_snapshot"` // State BEFORE this edit
	AfterSnapshot  TerritorySnapshot `json:"after_snapshot"`  // State AFTER this edit
	Description    string            `json:"description"`     // Human-readable description
	Parent         *UndoNode         `json:"-"`               // Parent node (not serialized to prevent cycles)
	ParentID       string            `json:"parent_id"`       // Parent ID for serialization
	Children       []*UndoNode       `json:"-"`               // Child nodes (branches)
	ChildrenIDs    []string          `json:"children_ids"`
	X              float64           `json:"x"` // Position in graph for visualization
	Y              float64           `json:"y"`
	IsActive       bool              `json:"is_active"` // Current node in this branch
}

// UndoTree manages the undo/redo tree structure
type UndoTree struct {
	Root        *UndoNode            `json:"-"`
	RootID      string               `json:"root_id"`
	CurrentNode *UndoNode            `json:"-"`
	CurrentID   string               `json:"current_id"`
	Nodes       map[string]*UndoNode `json:"-"` // All nodes by ID
	NodesData   []*UndoNode          `json:"nodes"`
	mu          sync.RWMutex
	nextID      int
}

// UndoManager manages the undo system
type UndoManager struct {
	tree              *UndoTree
	lastTerritoryName string
	lastSnapshot      *TerritorySnapshot
	mu                sync.RWMutex
	isBusy            bool // Prevents concurrent undo/redo operations
}

var (
	globalUndoManager *UndoManager
	undoManagerOnce   sync.Once
)

// GetUndoManager returns the global undo manager instance
func GetUndoManager() *UndoManager {
	undoManagerOnce.Do(func() {
		globalUndoManager = NewUndoManager()
	})
	return globalUndoManager
}

// NewUndoManager creates a new undo manager
func NewUndoManager() *UndoManager {
	tree := &UndoTree{
		Nodes:  make(map[string]*UndoNode),
		nextID: 0,
	}

	// Create root node
	root := &UndoNode{
		ID:          "root",
		Description: "Initial State",
		Children:    make([]*UndoNode, 0),
		X:           0,
		Y:           0,
		IsActive:    true,
	}

	tree.Root = root
	tree.CurrentNode = root
	tree.Nodes["root"] = root
	tree.RootID = "root"
	tree.CurrentID = "root"

	return &UndoManager{
		tree: tree,
	}
}

// CaptureTerritoryState captures the current state of a territory
func (um *UndoManager) CaptureTerritoryState(territoryName string) *TerritorySnapshot {
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		return nil
	}

	territory.Mu.RLock()
	defer territory.Mu.RUnlock()

	// Capture current state
	changes := make(map[string]interface{})

	// Upgrades
	changes["damage"] = territory.Options.Upgrade.Set.Damage
	changes["attack"] = territory.Options.Upgrade.Set.Attack
	changes["health"] = territory.Options.Upgrade.Set.Health
	changes["defence"] = territory.Options.Upgrade.Set.Defence

	// Bonuses
	changes["stronger_minions"] = territory.Options.Bonus.Set.StrongerMinions
	changes["tower_multi_attack"] = territory.Options.Bonus.Set.TowerMultiAttack
	changes["tower_aura"] = territory.Options.Bonus.Set.TowerAura
	changes["tower_volley"] = territory.Options.Bonus.Set.TowerVolley
	changes["gathering_experience"] = territory.Options.Bonus.Set.GatheringExperience
	changes["mob_experience"] = territory.Options.Bonus.Set.MobExperience
	changes["mob_damage"] = territory.Options.Bonus.Set.MobDamage
	changes["pvp_damage"] = territory.Options.Bonus.Set.PvPDamage
	changes["xp_seeking"] = territory.Options.Bonus.Set.XPSeeking
	changes["tome_seeking"] = territory.Options.Bonus.Set.TomeSeeking
	changes["emerald_seeking"] = territory.Options.Bonus.Set.EmeraldSeeking
	changes["larger_resource_storage"] = territory.Options.Bonus.Set.LargerResourceStorage
	changes["larger_emerald_storage"] = territory.Options.Bonus.Set.LargerEmeraldStorage
	changes["efficient_resource"] = territory.Options.Bonus.Set.EfficientResource
	changes["efficient_emerald"] = territory.Options.Bonus.Set.EfficientEmerald
	changes["resource_rate"] = territory.Options.Bonus.Set.ResourceRate
	changes["emerald_rate"] = territory.Options.Bonus.Set.EmeraldRate

	// Tax and border
	changes["tax"] = territory.Tax.Tax
	changes["ally_tax"] = territory.Tax.Ally
	changes["border"] = int(territory.Border)
	changes["routing_mode"] = int(territory.RoutingMode)
	changes["hq"] = territory.HQ

	// Treasury override
	changes["treasury_override"] = int(territory.TreasuryOverride)

	// Storage (for manual edits)
	changes["storage_emeralds"] = territory.Storage.At.Emeralds
	changes["storage_ores"] = territory.Storage.At.Ores
	changes["storage_wood"] = territory.Storage.At.Wood
	changes["storage_fish"] = territory.Storage.At.Fish
	changes["storage_crops"] = territory.Storage.At.Crops

	return &TerritorySnapshot{
		TerritoryName: territoryName,
		Timestamp:     time.Now(),
		Changes:       changes,
	}
}

// CompareSnapshots returns only the differences between two snapshots
func CompareSnapshots(old, new *TerritorySnapshot) map[string]interface{} {
	if old == nil {
		return new.Changes
	}

	diff := make(map[string]interface{})
	for key, newValue := range new.Changes {
		if oldValue, exists := old.Changes[key]; !exists || oldValue != newValue {
			diff[key] = newValue
		}
	}

	return diff
}

// StartEdit begins tracking changes to a territory
func (um *UndoManager) StartEdit(territoryName string) {
	um.mu.Lock()
	defer um.mu.Unlock()

	um.lastTerritoryName = territoryName
	um.lastSnapshot = um.CaptureTerritoryState(territoryName)
	
	fmt.Printf("[UNDO] Started tracking edits for %s\n", territoryName)
}

// EndEdit saves changes if any were made
func (um *UndoManager) EndEdit(territoryName string) {
	um.mu.Lock()
	defer um.mu.Unlock()

	if um.lastSnapshot == nil || um.lastTerritoryName != territoryName {
		return
	}

	// Capture current state (AFTER editing)
	currentSnapshot := um.CaptureTerritoryState(territoryName)
	if currentSnapshot == nil {
		return
	}

	// Compare and only save if there are differences
	diff := CompareSnapshots(um.lastSnapshot, currentSnapshot)
	if len(diff) == 0 {
		// No changes, don't create a node
		um.lastSnapshot = nil
		return
	}

	// Store both BEFORE (lastSnapshot) and AFTER (currentSnapshot) states
	beforeSnapshot := &TerritorySnapshot{
		TerritoryName: territoryName,
		Timestamp:     um.lastSnapshot.Timestamp,
		Changes:       um.lastSnapshot.Changes, // State BEFORE editing
	}
	
	afterSnapshot := &TerritorySnapshot{
		TerritoryName: territoryName,
		Timestamp:     time.Now(),
		Changes:       currentSnapshot.Changes, // State AFTER editing
	}

	// Create description based on changes (using diff for readability)
	description := um.generateDescription(territoryName, diff)

	// Add to tree with both snapshots
	node := um.tree.AddNode(beforeSnapshot, afterSnapshot, description)
	fmt.Printf("[UNDO] Saved edit for %s: %s (node %s, %d changes)\n", territoryName, description, node.ID, len(diff))

	// Clear last snapshot
	um.lastSnapshot = nil
}

// generateDescription creates a human-readable description of changes
func (um *UndoManager) generateDescription(territoryName string, changes map[string]interface{}) string {
	if len(changes) == 0 {
		return fmt.Sprintf("%s: No changes", territoryName)
	}

	// Find the most significant change
	if _, ok := changes["hq"]; ok {
		return fmt.Sprintf("%s: Set HQ", territoryName)
	}
	if _, ok := changes["damage"]; ok {
		return fmt.Sprintf("%s: Changed Damage", territoryName)
	}
	if _, ok := changes["attack"]; ok {
		return fmt.Sprintf("%s: Changed Attack", territoryName)
	}
	if _, ok := changes["health"]; ok {
		return fmt.Sprintf("%s: Changed Health", territoryName)
	}
	if _, ok := changes["defence"]; ok {
		return fmt.Sprintf("%s: Changed Defence", territoryName)
	}
	if _, ok := changes["tax"]; ok {
		return fmt.Sprintf("%s: Changed Tax", territoryName)
	}

	// Count how many things changed
	return fmt.Sprintf("%s: %d changes", territoryName, len(changes))
}

// AddNode adds a new node to the tree
func (tree *UndoTree) AddNode(beforeSnapshot *TerritorySnapshot, afterSnapshot *TerritorySnapshot, description string) *UndoNode {
	tree.mu.Lock()
	defer tree.mu.Unlock()

	// Generate new ID
	tree.nextID++
	id := fmt.Sprintf("node_%d", tree.nextID)

	// Create new node (position will be calculated by layout algorithm)
	node := &UndoNode{
		ID:             id,
		BeforeSnapshot: *beforeSnapshot,
		AfterSnapshot:  *afterSnapshot,
		Description:    description,
		Parent:         tree.CurrentNode,
		ParentID:       tree.CurrentNode.ID,
		Children:       make([]*UndoNode, 0),
		ChildrenIDs:    make([]string, 0),
		X:              0,
		Y:              0,
		IsActive:       true,
	}

	// Deactivate all current children
	for _, child := range tree.CurrentNode.Children {
		child.IsActive = false
	}

	// Add to parent's children
	tree.CurrentNode.Children = append(tree.CurrentNode.Children, node)
	tree.CurrentNode.ChildrenIDs = append(tree.CurrentNode.ChildrenIDs, id)

	// Add to nodes map
	tree.Nodes[id] = node

	// Set as current
	tree.CurrentNode = node
	tree.CurrentID = id

	// Recalculate layout for entire tree
	tree.layoutTree()

	return node
}

// layoutTree recalculates positions for all nodes to prevent overlaps
func (tree *UndoTree) layoutTree() {
	if tree.Root == nil {
		return
	}
	
	// Position root at origin
	tree.Root.X = 0
	tree.Root.Y = 0
	
	// Layout children recursively with collision detection
	tree.layoutSubtree(tree.Root)
}

// layoutSubtree positions a node's children in an inverted-V pattern with collision avoidance
func (tree *UndoTree) layoutSubtree(node *UndoNode) {
	const verticalSpacing = 120.0
	const horizontalAngle = 150.0 // Base horizontal spacing between siblings
	const minNodeSpacing = 180.0  // Minimum distance between any two nodes
	
	numChildren := len(node.Children)
	if numChildren == 0 {
		return
	}
	
	// Calculate initial positions for children in a spread pattern
	// Center child at index 0, spread others left and right
	for i, child := range node.Children {
		// Position relative to parent
		offset := float64(i) - float64(numChildren-1)/2.0
		child.X = node.X + offset*horizontalAngle
		child.Y = node.Y + verticalSpacing
	}
	
	// Get all nodes at this level and check for collisions
	childLevel := int(node.Y/verticalSpacing) + 1
	nodesAtLevel := tree.getNodesAtLevel(childLevel)
	
	// Resolve collisions by shifting overlapping subtrees
	for _, child := range node.Children {
		tree.resolveCollisions(child, nodesAtLevel, minNodeSpacing)
	}
	
	// Recursively layout children's subtrees
	for _, child := range node.Children {
		tree.layoutSubtree(child)
	}
}

// getNodesAtLevel returns all nodes at a specific depth level
func (tree *UndoTree) getNodesAtLevel(level int) []*UndoNode {
	var nodes []*UndoNode
	const verticalSpacing = 120.0
	
	for _, node := range tree.Nodes {
		nodeLevel := int(node.Y / verticalSpacing)
		if nodeLevel == level {
			nodes = append(nodes, node)
		}
	}
	
	return nodes
}

// resolveCollisions shifts a node and its subtree if it collides with others
func (tree *UndoTree) resolveCollisions(node *UndoNode, nodesAtLevel []*UndoNode, minSpacing float64) {
	const maxIterations = 10
	
	for iteration := 0; iteration < maxIterations; iteration++ {
		hasCollision := false
		
		for _, other := range nodesAtLevel {
			if other == node || other.Parent == node.Parent {
				// Skip self and siblings (siblings are positioned by parent)
				continue
			}
			
			// Check distance
			dx := node.X - other.X
			distance := math.Abs(dx)
			
			if distance < minSpacing {
				// Collision detected! Shift this node away
				hasCollision = true
				shift := minSpacing - distance
				if dx < 0 {
					// Node is to the left of other, shift left
					tree.shiftSubtree(node, -shift)
				} else {
					// Node is to the right of other, shift right
					tree.shiftSubtree(node, shift)
				}
			}
		}
		
		if !hasCollision {
			break
		}
	}
}

// shiftSubtree shifts a node and all its descendants horizontally
func (tree *UndoTree) shiftSubtree(node *UndoNode, deltaX float64) {
	node.X += deltaX
	
	for _, child := range node.Children {
		tree.shiftSubtree(child, deltaX)
	}
}

// Undo moves to the parent node and reverts changes
func (um *UndoManager) Undo() (string, bool) {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Check if already busy
	if um.isBusy {
		fmt.Printf("[UNDO] Operation in progress, skipping\n")
		return "", false
	}
	um.isBusy = true
	defer func() { um.isBusy = false }()

	if um.tree.CurrentNode.Parent == nil {
		return "", false // Already at root
	}

	// Apply the BEFORE snapshot of the current node (reverting this edit)
	territoryName := um.tree.CurrentNode.BeforeSnapshot.TerritoryName
	if territoryName != "" {
		fmt.Printf("[UNDO] Reverting edit: %s\n", um.tree.CurrentNode.Description)
		um.applySnapshot(&um.tree.CurrentNode.BeforeSnapshot)
	}

	// Move to parent
	um.tree.CurrentNode.IsActive = false
	um.tree.CurrentNode = um.tree.CurrentNode.Parent
	um.tree.CurrentNode.IsActive = true
	um.tree.CurrentID = um.tree.CurrentNode.ID

	return territoryName, true
}

// Redo moves to the active child node and applies changes
func (um *UndoManager) Redo() (string, bool) {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Check if already busy
	if um.isBusy {
		fmt.Printf("[REDO] Operation in progress, skipping\n")
		return "", false
	}
	um.isBusy = true
	defer func() { um.isBusy = false }()

	// Find active child
	var activeChild *UndoNode
	for _, child := range um.tree.CurrentNode.Children {
		if child.IsActive {
			activeChild = child
			break
		}
	}

	if activeChild == nil {
		return "", false // No active child to redo to
	}

	// Apply the AFTER snapshot of the child (reapplying that edit)
	territoryName := activeChild.AfterSnapshot.TerritoryName
	if territoryName != "" {
		fmt.Printf("[REDO] Reapplying edit: %s\n", activeChild.Description)
		um.applySnapshot(&activeChild.AfterSnapshot)
	}

	// Move to child
	um.tree.CurrentNode = activeChild
	um.tree.CurrentID = activeChild.ID

	return territoryName, true
}

// applySnapshot applies a snapshot to a territory
func (um *UndoManager) applySnapshot(snapshot *TerritorySnapshot) {
	territory := eruntime.GetTerritory(snapshot.TerritoryName)
	if territory == nil {
		return
	}

	territory.Mu.Lock()
	defer territory.Mu.Unlock()

	// Apply changes
	for key, value := range snapshot.Changes {
		switch key {
		// Upgrades
		case "damage":
			territory.Options.Upgrade.Set.Damage = toInt(value)
		case "attack":
			territory.Options.Upgrade.Set.Attack = toInt(value)
		case "health":
			territory.Options.Upgrade.Set.Health = toInt(value)
		case "defence":
			territory.Options.Upgrade.Set.Defence = toInt(value)

		// Bonuses
		case "stronger_minions":
			territory.Options.Bonus.Set.StrongerMinions = toInt(value)
		case "tower_multi_attack":
			territory.Options.Bonus.Set.TowerMultiAttack = toInt(value)
		case "tower_aura":
			territory.Options.Bonus.Set.TowerAura = toInt(value)
		case "tower_volley":
			territory.Options.Bonus.Set.TowerVolley = toInt(value)
		case "gathering_experience":
			territory.Options.Bonus.Set.GatheringExperience = toInt(value)
		case "mob_experience":
			territory.Options.Bonus.Set.MobExperience = toInt(value)
		case "mob_damage":
			territory.Options.Bonus.Set.MobDamage = toInt(value)
		case "pvp_damage":
			territory.Options.Bonus.Set.PvPDamage = toInt(value)
		case "xp_seeking":
			territory.Options.Bonus.Set.XPSeeking = toInt(value)
		case "tome_seeking":
			territory.Options.Bonus.Set.TomeSeeking = toInt(value)
		case "emerald_seeking":
			territory.Options.Bonus.Set.EmeraldSeeking = toInt(value)
		case "larger_resource_storage":
			territory.Options.Bonus.Set.LargerResourceStorage = toInt(value)
		case "larger_emerald_storage":
			territory.Options.Bonus.Set.LargerEmeraldStorage = toInt(value)
		case "efficient_resource":
			territory.Options.Bonus.Set.EfficientResource = toInt(value)
		case "efficient_emerald":
			territory.Options.Bonus.Set.EfficientEmerald = toInt(value)
		case "resource_rate":
			territory.Options.Bonus.Set.ResourceRate = toInt(value)
		case "emerald_rate":
			territory.Options.Bonus.Set.EmeraldRate = toInt(value)

		// Tax and border
		case "tax":
			territory.Tax.Tax = toFloat64(value)
		case "ally_tax":
			territory.Tax.Ally = toFloat64(value)
		case "border":
			territory.Border = typedef.Border(toInt(value))
		case "routing_mode":
			territory.RoutingMode = typedef.Routing(toInt(value))
		case "hq":
			territory.HQ = toBool(value)

		// Treasury override
		case "treasury_override":
			territory.TreasuryOverride = typedef.TreasuryOverride(toInt(value))

		// Storage
		case "storage_emeralds":
			territory.Storage.At.Emeralds = toFloat64(value)
		case "storage_ores":
			territory.Storage.At.Ores = toFloat64(value)
		case "storage_wood":
			territory.Storage.At.Wood = toFloat64(value)
		case "storage_fish":
			territory.Storage.At.Fish = toFloat64(value)
		case "storage_crops":
			territory.Storage.At.Crops = toFloat64(value)
		}
	}

	// Territory options need to be sent to the channel for recalculation
	territory.SetCh <- typedef.TerritoryOptions{
		Upgrades:    territory.Options.Upgrade.Set,
		Bonuses:     territory.Options.Bonus.Set,
		Tax:         territory.Tax,
		RoutingMode: territory.RoutingMode,
		Border:      territory.Border,
		HQ:          territory.HQ,
	}
}

// GetTree returns the undo tree for visualization
func (um *UndoManager) GetTree() *UndoTree {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.tree
}

// IsBusy returns whether an undo/redo operation is in progress
func (um *UndoManager) IsBusy() bool {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.isBusy
}

// SetCurrentNode sets the current node (for graph navigation)
func (um *UndoManager) SetCurrentNode(nodeID string) bool {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Check if already busy
	if um.isBusy {
		fmt.Printf("[UNDO] Jump operation in progress, skipping\n")
		return false
	}
	um.isBusy = true
	defer func() { um.isBusy = false }()

	node, exists := um.tree.Nodes[nodeID]
	if !exists {
		return false
	}

	// Determine the direction and path
	commonAncestor := um.findCommonAncestor(um.tree.CurrentNode, node)
	
	// First, undo from current to common ancestor (apply BEFORE snapshots)
	current := um.tree.CurrentNode
	for current != commonAncestor {
		if current.BeforeSnapshot.TerritoryName != "" {
			fmt.Printf("[UNDO] Reverting: %s\n", current.Description)
			um.applySnapshot(&current.BeforeSnapshot)
		}
		current = current.Parent
		if current == nil {
			break
		}
	}
	
	// Build path from common ancestor to target
	pathToTarget := make([]*UndoNode, 0)
	temp := node
	for temp != commonAncestor && temp != nil {
		pathToTarget = append([]*UndoNode{temp}, pathToTarget...)
		temp = temp.Parent
	}
	
	// Apply AFTER snapshots from common ancestor to target (in forward order)
	for _, pathNode := range pathToTarget {
		if pathNode.AfterSnapshot.TerritoryName != "" {
			fmt.Printf("[REDO] Applying: %s\n", pathNode.Description)
			um.applySnapshot(&pathNode.AfterSnapshot)
		}
	}

	// Deactivate old branch
	um.deactivateBranch(um.tree.CurrentNode)

	// Set new current and activate branch
	um.tree.CurrentNode = node
	um.tree.CurrentID = node.ID
	um.activateBranch(node)

	return true
}

// buildPath finds the path between two nodes
func (um *UndoManager) buildPath(from, to *UndoNode) []*UndoNode {
	if from == to {
		return []*UndoNode{}
	}

	// Find common ancestor
	fromAncestors := um.getAncestors(from)
	toAncestors := um.getAncestors(to)

	var commonAncestor *UndoNode
	for _, fa := range fromAncestors {
		for _, ta := range toAncestors {
			if fa == ta {
				commonAncestor = fa
				goto found
			}
		}
	}
found:

	if commonAncestor == nil {
		return nil
	}

	// Build path: go up from 'from' to common ancestor, then down to 'to'
	path := make([]*UndoNode, 0)

	// Get path to common ancestor (going up)
	current := from
	for current != commonAncestor && current != nil {
		current = current.Parent
	}

	// Get path from common ancestor to target (going down)
	pathDown := make([]*UndoNode, 0)
	current = to
	for current != commonAncestor && current != nil {
		pathDown = append([]*UndoNode{current}, pathDown...)
		current = current.Parent
	}

	path = append(path, pathDown...)
	return path
}

// getAncestors returns all ancestors of a node
func (um *UndoManager) getAncestors(node *UndoNode) []*UndoNode {
	ancestors := make([]*UndoNode, 0)
	current := node
	for current != nil {
		ancestors = append(ancestors, current)
		current = current.Parent
	}
	return ancestors
}

// findCommonAncestor finds the lowest common ancestor of two nodes
func (um *UndoManager) findCommonAncestor(node1, node2 *UndoNode) *UndoNode {
	ancestors1 := um.getAncestors(node1)
	ancestors2 := um.getAncestors(node2)
	
	// Find first common node
	for _, a1 := range ancestors1 {
		for _, a2 := range ancestors2 {
			if a1 == a2 {
				return a1
			}
		}
	}
	
	return nil
}

// deactivateBranch deactivates all nodes in a branch
func (um *UndoManager) deactivateBranch(node *UndoNode) {
	if node == nil {
		return
	}
	node.IsActive = false
	for _, child := range node.Children {
		if child.IsActive {
			um.deactivateBranch(child)
		}
	}
}

// activateBranch activates all nodes up to root
func (um *UndoManager) activateBranch(node *UndoNode) {
	if node == nil {
		return
	}
	node.IsActive = true
	if node.Parent != nil {
		um.activateBranch(node.Parent)
	}
}

// ExportToJSON exports the undo tree to JSON
func (um *UndoManager) ExportToJSON() ([]byte, error) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	// Prepare tree for serialization
	um.tree.NodesData = make([]*UndoNode, 0, len(um.tree.Nodes))
	for _, node := range um.tree.Nodes {
		um.tree.NodesData = append(um.tree.NodesData, node)
	}

	return json.MarshalIndent(um.tree, "", "  ")
}

// ImportFromJSON imports the undo tree from JSON
func (um *UndoManager) ImportFromJSON(data []byte) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	tree := &UndoTree{}
	if err := json.Unmarshal(data, tree); err != nil {
		return err
	}

	// Rebuild node map and relationships
	tree.Nodes = make(map[string]*UndoNode)
	for _, node := range tree.NodesData {
		tree.Nodes[node.ID] = node
	}

	// Rebuild parent-child relationships
	for _, node := range tree.NodesData {
		if node.ParentID != "" {
			node.Parent = tree.Nodes[node.ParentID]
		}

		node.Children = make([]*UndoNode, 0, len(node.ChildrenIDs))
		for _, childID := range node.ChildrenIDs {
			if child, exists := tree.Nodes[childID]; exists {
				node.Children = append(node.Children, child)
			}
		}
	}

	// Set root and current
	tree.Root = tree.Nodes[tree.RootID]
	tree.CurrentNode = tree.Nodes[tree.CurrentID]

	um.tree = tree
	return nil
}

// Helper functions for type conversion (handles both int and float64 from JSON)
func toInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	default:
		return 0
	}
}

func toFloat64(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0.0
	}
}

func toBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}
