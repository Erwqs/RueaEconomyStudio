package auto

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"RueaES/alg"
	"RueaES/eruntime"
	"RueaES/typedef"
)

type ResourceKind string

const (
	ResourceOre   ResourceKind = "ore"
	ResourceWood  ResourceKind = "wood"
	ResourceFish  ResourceKind = "fish"
	ResourceCrops ResourceKind = "crops"
)

const (
	defenseFakeMediumCore = 1
	defenseFakeHighCore   = 2
	defenseActualMedium   = 2
	defenseActualHigh     = 4

	defenseSetLow      = 6
	defenseSetMedium   = 19
	defenseSetHigh     = 31
	defenseSetVeryHigh = 49
)

func defenseTarget(coreMin int, minSetLevel int) int {
	if coreMin < 0 {
		coreMin = 0
	}
	target := coreMin * 4
	if target < minSetLevel {
		target = minSetLevel
	}
	return target
}

type AutoConfig struct {
	GuildTag          string
	TerritoryNames    []string
	MaxIterations     int
	ResetExisting     bool
	ChokeWeight       float64
	ThroughputWeight  float64
	HighCountFraction float64
	Logger            func(format string, args ...any)
}

type AutoResult struct {
	Actions  []Action
	Warnings []string
}

type LoopMode int

const (
	LoopNone LoopMode = iota
	LoopEveryDuration
	LoopEveryTicks
)

type LoopConfig struct {
	Mode       LoopMode
	Every      time.Duration
	EveryTicks uint64
}

type Runner struct {
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

var (
	defaultRunner Runner
	stateMu       sync.Mutex
	lastConfig    AutoConfig
	lastLoop      LoopConfig
)

// StartAuto kicks off the optimizer loop (non-blocking).
func StartAuto(cfg AutoConfig, loop LoopConfig) {
	stateMu.Lock()
	lastConfig = cfg
	lastLoop = loop
	stateMu.Unlock()

	defaultRunner.Start(cfg, loop)
}

// StopAuto stops the optimizer loop if running (non-blocking).
func StopAuto() {
	defaultRunner.Stop()
}

// ResumeAuto resumes the last loop config if available.
func ResumeAuto() error {
	stateMu.Lock()
	cfg := lastConfig
	loop := lastLoop
	stateMu.Unlock()

	if cfg.GuildTag == "" {
		return fmt.Errorf("no previous config to resume")
	}
	defaultRunner.Start(cfg, loop)
	return nil
}

// RestartAuto clears all upgrades/bonuses for the claim and restarts the loop (non-blocking).
func RestartAuto() error {
	stateMu.Lock()
	cfg := lastConfig
	loop := lastLoop
	stateMu.Unlock()

	if cfg.GuildTag == "" {
		return fmt.Errorf("no previous config to restart")
	}

	defaultRunner.Stop()
	go func() {
		cfg.ResetExisting = true
		_, _ = InitializeClaimEco(cfg)
		defaultRunner.Start(cfg, loop)
	}()

	return nil
}

// RunAutoOnce runs a single optimization pass asynchronously.
func RunAutoOnce(cfg AutoConfig) {
	go func() {
		_, _ = InitializeClaimEco(cfg)
	}()
}

// IsAutoRunning reports whether the auto optimizer loop is currently running.
func IsAutoRunning() bool {
	return defaultRunner.IsRunning()
}

func (r *Runner) Start(cfg AutoConfig, loop LoopConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return
	}
	r.running = true
	r.stopCh = make(chan struct{})

	go func() {
		defer func() {
			r.mu.Lock()
			r.running = false
			r.mu.Unlock()
		}()

		lastTick := uint64(0)
		for {
			select {
			case <-r.stopCh:
				return
			default:
			}

			_, _ = InitializeClaimEco(cfg)

			switch loop.Mode {
			case LoopEveryDuration:
				if loop.Every <= 0 {
					loop.Every = time.Second
				}
				select {
				case <-time.After(loop.Every):
				case <-r.stopCh:
					return
				}
			case LoopEveryTicks:
				if loop.EveryTicks == 0 {
					loop.EveryTicks = 60
				}
				for {
					select {
					case <-r.stopCh:
						return
					default:
					}
					current := eruntime.Tick()
					if lastTick == 0 {
						lastTick = current
					}
					if current-lastTick >= loop.EveryTicks {
						lastTick = current
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
			default:
				return
			}
		}
	}()
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}
	close(r.stopCh)
	if r.running {
		r.running = false
	}
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

type Action struct {
	Territory string
	Kind      string
	Details   string
}

type claim struct {
	GuildTag     string
	Territories  []*typedef.Territory
	HQ           *typedef.Territory
	Cities       []*typedef.Territory
	ResourceSets map[ResourceKind][]*typedef.Territory
	Doubles      []*typedef.Territory
	Rainbows     []*typedef.Territory
}

func InitializeClaimEco(cfg AutoConfig) (*AutoResult, error) {
	if cfg.GuildTag == "" {
		return nil, fmt.Errorf("guild tag is required")
	}

	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 12
	}
	if cfg.ChokeWeight <= 0 {
		cfg.ChokeWeight = 0.55
	}
	if cfg.ThroughputWeight <= 0 {
		cfg.ThroughputWeight = 0.45
	}
	if cfg.HighCountFraction <= 0 {
		cfg.HighCountFraction = 0.16
	}

	result := &AutoResult{}
	logf := cfg.Logger
	if logf == nil {
		logf = func(string, ...any) {}
	}

	cl, err := buildClaim(cfg.GuildTag, cfg.TerritoryNames)
	if err != nil {
		return result, err
	}

	if err := validateClaim(cl); err != nil {
		return result, err
	}

	if cfg.ResetExisting {
		resetClaimOptions(cl, result)
	}

	ensureHQStorage(cl, result)
	ensureHQDefense(cl, result)

	applyCityEmeraldBuffs(cl, result)

	applyFakeDefense(cl.Territories, defenseTarget(defenseFakeMediumCore, 0), claimNetWeights(cl.Territories), result)

	fixDrain(cl, cfg.MaxIterations, result)

	applyFakeHighOnCriticalProducers(cl, result)

	rebalanceDefense(cl, cfg, result)

	fixDrain(cl, cfg.MaxIterations, result)

	ensureTerritoryStorage(cl, result)

	fixDrain(cl, cfg.MaxIterations, result)

	logf("Auto eco finished for %d territories", len(cl.Territories))
	return result, nil
}

func buildClaim(guildTag string, territoryNames []string) (*claim, error) {
	all := eruntime.GetTerritories()
	byName := make(map[string]*typedef.Territory, len(all))
	for _, t := range all {
		if t != nil {
			byName[t.Name] = t
		}
	}

	territories := make([]*typedef.Territory, 0)
	if len(territoryNames) > 0 {
		for _, name := range territoryNames {
			t := byName[name]
			if t == nil {
				continue
			}
			t.Mu.RLock()
			tag := t.Guild.Tag
			t.Mu.RUnlock()
			if tag == guildTag {
				territories = append(territories, t)
			}
		}
	} else {
		for _, t := range all {
			if t == nil {
				continue
			}
			t.Mu.RLock()
			tag := t.Guild.Tag
			t.Mu.RUnlock()
			if tag == guildTag {
				territories = append(territories, t)
			}
		}
	}

	if len(territories) == 0 {
		return nil, fmt.Errorf("no territories found for guild %s", guildTag)
	}

	cl := &claim{
		GuildTag:     guildTag,
		Territories:  territories,
		ResourceSets: make(map[ResourceKind][]*typedef.Territory),
		Doubles:      []*typedef.Territory{},
		Rainbows:     []*typedef.Territory{},
		Cities:       []*typedef.Territory{},
	}

	for _, t := range territories {
		if t == nil {
			continue
		}

		if isCity(t) {
			cl.Cities = append(cl.Cities, t)
		}

		kind, doubleTerritory, rainbow := classifyResource(t)
		if rainbow {
			cl.Rainbows = append(cl.Rainbows, t)
		} else if doubleTerritory {
			cl.Doubles = append(cl.Doubles, t)
		} else if kind != "" {
			cl.ResourceSets[kind] = append(cl.ResourceSets[kind], t)
		}

		t.Mu.RLock()
		isHQ := t.HQ
		t.Mu.RUnlock()
		if isHQ {
			cl.HQ = t
		}
	}

	if cl.HQ == nil {
		return nil, fmt.Errorf("no HQ found for guild %s", guildTag)
	}

	sortByDistanceToHQ(cl.Cities, cl.HQ)
	for _, list := range cl.ResourceSets {
		sortByDistanceToHQ(list, cl.HQ)
	}
	sortByDistanceToHQ(cl.Doubles, cl.HQ)
	sortByDistanceToHQ(cl.Rainbows, cl.HQ)

	return cl, nil
}

func validateClaim(cl *claim) error {
	if len(cl.Cities) == 0 {
		return fmt.Errorf("claim needs at least one city")
	}

	hasAll := len(cl.ResourceSets[ResourceOre]) > 0 && len(cl.ResourceSets[ResourceCrops]) > 0 && len(cl.ResourceSets[ResourceWood]) > 0 && len(cl.ResourceSets[ResourceFish]) > 0
	if !hasAll && len(cl.Rainbows) == 0 && len(cl.Doubles) == 0 {
		return fmt.Errorf("claim needs at least one of each resource territory or a rainbow territory")
	}
	return nil
}

func resetClaimOptions(cl *claim, result *AutoResult) {
	for _, t := range cl.Territories {
		if t == nil {
			continue
		}
		t.Mu.RLock()
		opts := typedef.TerritoryOptions{
			Upgrades:    typedef.Upgrade{},
			Bonuses:     typedef.Bonus{},
			Tax:         t.Tax,
			RoutingMode: t.RoutingMode,
			Border:      t.Border,
			HQ:          t.HQ,
		}
		t.Mu.RUnlock()
		eruntime.Set(t.Name, opts)
		result.Actions = append(result.Actions, Action{Territory: t.Name, Kind: "reset", Details: "cleared upgrades/bonuses"})
	}
}

func ensureHQStorage(cl *claim, result *AutoResult) {
	if cl.HQ == nil {
		return
	}

	target := recommendedHQStorageLevel(len(cl.Territories))
	current := getBonusLevel(cl.HQ, func(b *typedef.Bonus) int { return b.LargerResourceStorage })
	currentE := getBonusLevel(cl.HQ, func(b *typedef.Bonus) int { return b.LargerEmeraldStorage })

	for current < target {
		current++
		eruntime.SetTerritoryBonus(cl.HQ.Name, "largerResourceStorage", current)
		result.Actions = append(result.Actions, Action{Territory: cl.HQ.Name, Kind: "hq_storage", Details: fmt.Sprintf("resource storage -> %d", current)})
	}
	for currentE < target {
		currentE++
		eruntime.SetTerritoryBonus(cl.HQ.Name, "largerEmeraldStorage", currentE)
		result.Actions = append(result.Actions, Action{Territory: cl.HQ.Name, Kind: "hq_storage", Details: fmt.Sprintf("emerald storage -> %d", currentE)})
	}
}

func ensureHQDefense(cl *claim, result *AutoResult) {
	if cl.HQ == nil {
		return
	}

	weights := claimNetWeights(cl.Territories)
	applyDefenseTarget(cl.HQ, defenseSetVeryHigh, weights, result, defenseActualHigh, 1)

	connections := hqConnections(cl)
	for _, t := range connections {
		applyDefenseTarget(t, defenseTarget(defenseFakeHighCore, 0), weights, result, defenseFakeHighCore, 1)
	}
}

func ensureTerritoryStorage(cl *claim, result *AutoResult) {
	if cl == nil {
		return
	}
	for _, t := range cl.Territories {
		if t == nil {
			continue
		}
		if isRainbow(t) {
			continue
		}
		applyStorageForTerritory(t, result)
	}
}

func applyStorageForTerritory(t *typedef.Territory, result *AutoResult) {
	if t == nil {
		return
	}

	stats := eruntime.GetTerritoryStats(t.Name)
	if stats == nil {
		return
	}

	defLevel := calcLevelInt(getSetLevels(t))
	highDefense := defLevel >= defenseSetHigh
	doubleTerritory := isDouble(t)

	resourceBuffer := 1.0
	if doubleTerritory {
		resourceBuffer = 1.5
	}
	if highDefense {
		resourceBuffer = math.Max(resourceBuffer, 1.25)
	}

	resourceInterval := intervalSeconds(stats.ResourceDeltaTime)
	emeraldInterval := intervalSeconds(stats.EmeraldDeltaTime)
	resourceNeed := maxResourcePerSecond(stats.GenerationPerSecond) * resourceInterval * resourceBuffer

	emeraldBuffer := 1.0
	if highDefense {
		emeraldBuffer = math.Max(emeraldBuffer, 1.25)
	}
	if isCity(t) {
		emeraldBuffer = math.Max(emeraldBuffer, 1.75)
	}
	emeraldNeed := stats.GenerationPerSecond.Emeralds * emeraldInterval * emeraldBuffer

	ensureResourceStorageLevel(t, resourceNeed, highDefense, result)
	if emeraldNeed > 0 {
		ensureEmeraldStorageLevel(t, emeraldNeed, highDefense, result)
	}
}

func ensureResourceStorageLevel(t *typedef.Territory, requiredPerMinute float64, highDefense bool, result *AutoResult) {
	if t == nil || requiredPerMinute <= 0 {
		return
	}

	costs := eruntime.GetCost()
	maxLevel := costs.Bonuses.LargerResourceStorage.MaxLevel
	current := getBonusLevel(t, func(b *typedef.Bonus) int { return b.LargerResourceStorage })
	needed := storageLevelFor(resourcesCapacityValues(costs.Bonuses.LargerResourceStorage.Value), requiredPerMinute)
	if highDefense && needed < 1 {
		needed = 1
	}
	if needed > maxLevel {
		needed = maxLevel
	}
	for current < needed {
		current++
		eruntime.SetTerritoryBonus(t.Name, "largerResourceStorage", current)
		result.Actions = append(result.Actions, Action{Territory: t.Name, Kind: "storage", Details: fmt.Sprintf("resource storage -> %d", current)})
	}
}

func ensureEmeraldStorageLevel(t *typedef.Territory, requiredPerMinute float64, highDefense bool, result *AutoResult) {
	if t == nil || requiredPerMinute <= 0 {
		return
	}

	costs := eruntime.GetCost()
	maxLevel := costs.Bonuses.LargerEmeraldsStorage.MaxLevel
	current := getBonusLevel(t, func(b *typedef.Bonus) int { return b.LargerEmeraldStorage })
	needed := storageLevelFor(emeraldCapacityValues(costs.Bonuses.LargerEmeraldsStorage.Value), requiredPerMinute)
	if highDefense && needed < 1 {
		needed = 1
	}
	if needed > maxLevel {
		needed = maxLevel
	}
	for current < needed {
		current++
		eruntime.SetTerritoryBonus(t.Name, "largerEmeraldStorage", current)
		result.Actions = append(result.Actions, Action{Territory: t.Name, Kind: "storage", Details: fmt.Sprintf("emerald storage -> %d", current)})
	}
}

func hqConnections(cl *claim) []*typedef.Territory {
	if cl == nil || cl.HQ == nil {
		return nil
	}
	byName := make(map[string]*typedef.Territory, len(cl.Territories))
	for _, t := range cl.Territories {
		if t != nil {
			byName[t.Name] = t
		}
	}

	cl.HQ.Mu.RLock()
	direct := make([]string, 0, len(cl.HQ.Links.Direct))
	for name := range cl.HQ.Links.Direct {
		direct = append(direct, name)
	}
	cl.HQ.Mu.RUnlock()

	result := make([]*typedef.Territory, 0, len(direct))
	for _, name := range direct {
		if t := byName[name]; t != nil {
			result = append(result, t)
		}
	}
	return result
}

func recommendedHQStorageLevel(territoryCount int) int {
	switch {
	case territoryCount <= 8:
		return 4
	case territoryCount <= 20:
		return 5
	default:
		return 6
	}
}

func applyCityEmeraldBuffs(cl *claim, result *AutoResult) {
	for _, city := range cl.Cities {
		if city == nil {
			continue
		}
		prevEff := getBonusLevel(city, func(b *typedef.Bonus) int { return b.EfficientEmerald })
		prevRate := getBonusLevel(city, func(b *typedef.Bonus) int { return b.EmeraldRate })

		before := claimNet(cl.Territories)
		applyCityBuff(city, 3, result)
		after := claimNet(cl.Territories)

		if after.Ores >= 0 && after.Crops >= 0 && (before.Ores >= 0 && before.Crops >= 0) {
			eruntime.SetTerritoryBonus(city.Name, "efficientEmerald", prevEff)
			eruntime.SetTerritoryBonus(city.Name, "emeraldRate", prevRate)
			result.Actions = append(result.Actions, Action{Territory: city.Name, Kind: "city_prod", Details: "emerald prod reverted"})
			continue
		}

		if after.Ores < 0 {
			fixResourceDrain(cl, ResourceOre, result)
		}
		if after.Crops < 0 {
			fixResourceDrain(cl, ResourceCrops, result)
		}
	}
}

func applyCityBuff(t *typedef.Territory, level int, result *AutoResult) {
	if t == nil {
		return
	}
	eruntime.SetTerritoryBonus(t.Name, "efficientEmerald", level)
	eruntime.SetTerritoryBonus(t.Name, "emeraldRate", level)
	if level == 0 {
		result.Actions = append(result.Actions, Action{Territory: t.Name, Kind: "city_prod", Details: "emerald prod -> 00"})
	} else {
		result.Actions = append(result.Actions, Action{Territory: t.Name, Kind: "city_prod", Details: fmt.Sprintf("emerald prod -> %d%d", level, level)})
	}
}

func applyProdBuff(t *typedef.Territory, level int, result *AutoResult) {
	if t == nil {
		return
	}
	if level == 0 {
		eruntime.SetTerritoryBonus(t.Name, "efficientResource", 0)
		eruntime.SetTerritoryBonus(t.Name, "resourceRate", 0)
		result.Actions = append(result.Actions, Action{Territory: t.Name, Kind: "resource_prod", Details: "resource prod -> 00"})
		return
	}

	eruntime.SetTerritoryBonus(t.Name, "efficientResource", level)
	eruntime.SetTerritoryBonus(t.Name, "resourceRate", 3)
	result.Actions = append(result.Actions, Action{Territory: t.Name, Kind: "resource_prod", Details: fmt.Sprintf("resource prod -> %d3", level)})
}

func fixDrain(cl *claim, iterations int, result *AutoResult) {
	for i := 0; i < iterations; i++ {
		net := claimNet(cl.Territories)
		drainKind := mostNegativeResource(net)
		if drainKind == "" {
			break
		}
		fixResourceDrain(cl, drainKind, result)
	}
}

func fixResourceDrain(cl *claim, kind ResourceKind, result *AutoResult) {
	candidates := make([]*resourceCandidate, 0)
	for _, t := range cl.Doubles {
		if t == nil {
			continue
		}
		if territoryHasResource(t, kind) {
			candidates = append(candidates, &resourceCandidate{territory: t, classRank: 2})
		}
	}
	for _, t := range cl.ResourceSets[kind] {
		if t != nil {
			candidates = append(candidates, &resourceCandidate{territory: t, classRank: 1})
		}
	}
	for _, t := range cl.Rainbows {
		if t != nil {
			candidates = append(candidates, &resourceCandidate{territory: t, classRank: 0})
		}
	}
	if len(candidates) == 0 {
		return
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].classRank != candidates[j].classRank {
			return candidates[i].classRank > candidates[j].classRank
		}
		return resourcePriority(candidates[i].territory, kind) > resourcePriority(candidates[j].territory, kind)
	})

	net := claimNet(cl.Territories)
	allAt33 := true
	for _, t := range cl.ResourceSets[kind] {
		if t == nil {
			continue
		}
		if getBonusLevel(t, func(b *typedef.Bonus) int { return b.EfficientResource }) < 3 {
			allAt33 = false
			break
		}
	}

	for _, candidate := range candidates {
		t := candidate.territory
		if t == nil {
			continue
		}

		if isRainbow(t) {
			if getBonusLevel(t, func(b *typedef.Bonus) int { return b.EfficientResource }) > 0 {
				continue
			}
			applyProdBuff(t, 3, result)
			return
		}

		current := getBonusLevel(t, func(b *typedef.Bonus) int { return b.EfficientResource })
		if current >= 3 && net.Emeralds < 0 {
			continue
		}

		desired := 3
		if net.Emeralds < 0 {
			desired = 2
		}
		if current >= 3 && net.Emeralds >= 0 && allAt33 {
			desired = 4
		}

		if desired != current {
			applyProdBuff(t, desired, result)
		}
		return
	}
}

func applyFakeDefense(territories []*typedef.Territory, target int, weights resourceWeights, result *AutoResult) {
	for _, t := range territories {
		if t == nil {
			continue
		}
		applyDefenseTarget(t, target, weights, result, defenseFakeMediumCore, 0)
	}
}

type defenseStep struct {
	kind      string
	resource  string
	nextLevel int
	delta     int
	cost      float64
}

func applyDefenseTarget(t *typedef.Territory, target int, weights resourceWeights, result *AutoResult, coreMin int, auraVolleyMin int) {
	if t == nil {
		return
	}
	if shouldSkipDefense(t) {
		return
	}

	current := getSetLevels(t)
	if calcLevelInt(current) >= target {
		return
	}

	for calcLevelInt(current) < target {
		steps := availableDefenseSteps(current)
		if len(steps) == 0 {
			break
		}

		if coreMin > 0 {
			missingCore := coresBelowMin(current, coreMin)
			if len(missingCore) > 0 {
				picked := pickBestStep(current, steps, weights, func(step defenseStep) bool {
					return stepIsCoreUpgrade(step) && missingCore[step.kind]
				})
				if picked != nil {
					applyDefenseStep(t, current, *picked, result)
					continue
				}
			}
		}

		if auraVolleyMin > 0 {
			missing := missingAuraVolley(current, auraVolleyMin)
			if len(missing) > 0 {
				picked := pickBestStep(current, steps, weights, func(step defenseStep) bool {
					return step.kind == "aura" && missing["aura"] || step.kind == "volley" && missing["volley"]
				})
				if picked != nil {
					applyDefenseStep(t, current, *picked, result)
					continue
				}
			}
		}

		best := steps[0]
		bestScore := bestDefenseComboScore(current, steps, 0, weights)
		for i := 1; i < len(steps); i++ {
			score := bestDefenseComboScore(current, steps, i, weights)
			if score > bestScore {
				best = steps[i]
				bestScore = score
			}
		}

		applyDefenseStep(t, current, best, result)
	}
}

func applyDefenseStep(t *typedef.Territory, current setLevels, step defenseStep, result *AutoResult) {
	switch step.kind {
	case "damage":
		current.Upgrades.Damage = step.nextLevel
		eruntime.SetTerritoryUpgrade(t.Name, "damage", step.nextLevel)
	case "attack":
		current.Upgrades.Attack = step.nextLevel
		eruntime.SetTerritoryUpgrade(t.Name, "attack", step.nextLevel)
	case "health":
		current.Upgrades.Health = step.nextLevel
		eruntime.SetTerritoryUpgrade(t.Name, "health", step.nextLevel)
	case "defence":
		current.Upgrades.Defence = step.nextLevel
		eruntime.SetTerritoryUpgrade(t.Name, "defence", step.nextLevel)
	case "aura":
		current.Bonuses.TowerAura = step.nextLevel
		eruntime.SetTerritoryBonus(t.Name, "towerAura", step.nextLevel)
	case "volley":
		current.Bonuses.TowerVolley = step.nextLevel
		eruntime.SetTerritoryBonus(t.Name, "towerVolley", step.nextLevel)
	}

	result.Actions = append(result.Actions, Action{Territory: t.Name, Kind: "defense", Details: fmt.Sprintf("%s -> %d", step.kind, step.nextLevel)})
}

func stepIsCoreUpgrade(step defenseStep) bool {
	return step.kind == "damage" || step.kind == "attack" || step.kind == "health" || step.kind == "defence"
}

func coresBelowMin(levels setLevels, minLevel int) map[string]bool {
	result := map[string]bool{}
	if levels.Upgrades.Damage < minLevel {
		result["damage"] = true
	}
	if levels.Upgrades.Attack < minLevel {
		result["attack"] = true
	}
	if levels.Upgrades.Health < minLevel {
		result["health"] = true
	}
	if levels.Upgrades.Defence < minLevel {
		result["defence"] = true
	}
	return result
}

func missingAuraVolley(levels setLevels, minLevel int) map[string]bool {
	result := map[string]bool{}
	if levels.Bonuses.TowerAura < minLevel {
		result["aura"] = true
	}
	if levels.Bonuses.TowerVolley < minLevel {
		result["volley"] = true
	}
	return result
}

func pickBestStep(levels setLevels, steps []defenseStep, weights resourceWeights, allow func(step defenseStep) bool) *defenseStep {
	var best *defenseStep
	bestScore := -1.0
	for i := range steps {
		step := steps[i]
		if !allow(step) {
			continue
		}
		score := defenseStepScore(levels, step, weights)
		if score > bestScore {
			bestScore = score
			best = &steps[i]
		}
	}
	return best
}

func defenseBalanceMultiplier(levels setLevels, step defenseStep) float64 {
	coreLevels := []int{levels.Upgrades.Damage, levels.Upgrades.Attack, levels.Upgrades.Health, levels.Upgrades.Defence}
	coreMin := coreLevels[0]
	coreMax := coreLevels[0]
	for _, v := range coreLevels[1:] {
		if v < coreMin {
			coreMin = v
		}
		if v > coreMax {
			coreMax = v
		}
	}

	mult := 1.0

	coreSpread := coreMax - coreMin
	coreIsBehind := coreMin < 2
	stepIsCore := step.kind == "damage" || step.kind == "attack" || step.kind == "health" || step.kind == "defence"

	switch step.kind {
	case "damage":
		if step.nextLevel > coreMin+2 {
			mult *= 5.0
		}
		if levels.Upgrades.Attack == 0 && step.nextLevel > 0 {
			mult *= 1.15
		}
	case "attack":
		if step.nextLevel > coreMin+2 {
			mult *= 5.0
		}
		if levels.Upgrades.Damage == 0 && step.nextLevel > 0 {
			mult *= 1.5
		}
	case "health":
		if step.nextLevel > coreMin+2 {
			mult *= 5.0
		}
	case "defence":
		if step.nextLevel > coreMin+2 {
			mult *= 5.0
		}
	case "aura", "volley":
		if coreMin < 2 {
			mult *= 3.0
		} else {
			mult *= 0.85
		}
		if coreMax >= 7 {
			mult *= 0.9
		}
	}

	if coreIsBehind {
		if stepIsCore {
			mult *= 0.5
		} else {
			mult *= 3.0
		}
	}
	if coreSpread > 2 && !stepIsCore {
		mult *= 2.5
	}

	return mult
}

func defenseStepValue(step defenseStep) float64 {
	base := 1.0
	switch step.kind {
	case "damage":
		base = 1.35
	case "attack":
		base = 1.25
	case "health":
		base = 0.9
	case "defence":
		base = 1.1
	case "aura":
		base = 1.2
	case "volley":
		base = 1.15
	}
	return base * float64(step.delta)
}

func defenseStepScore(levels setLevels, step defenseStep, weights resourceWeights) float64 {
	weighted := weightedCost(step.cost, step.resource, weights)
	if weighted <= 0 {
		return 0
	}
	adjustedCost := weighted * defenseBalanceMultiplier(levels, step)
	if adjustedCost <= 0 {
		return 0
	}
	return defenseStepValue(step) / adjustedCost
}

func bestDefenseComboScore(levels setLevels, steps []defenseStep, idx int, weights resourceWeights) float64 {
	if idx < 0 || idx >= len(steps) {
		return 0
	}
	first := steps[idx]
	best := defenseStepScore(levels, first, weights)
	value1 := defenseStepValue(first)
	cost1 := weightedCost(first.cost, first.resource, weights) * defenseBalanceMultiplier(levels, first)

	for j := 0; j < len(steps); j++ {
		if j == idx {
			continue
		}
		second := steps[j]
		value2 := defenseStepValue(second)
		cost2 := weightedCost(second.cost, second.resource, weights) * defenseBalanceMultiplier(levels, second)
		totalCost := cost1 + cost2
		if totalCost <= 0 {
			continue
		}
		comboScore := (value1 + value2) / totalCost
		if comboScore > best {
			best = comboScore
		}
	}

	return best
}

func shouldSkipDefense(t *typedef.Territory) bool {
	if t == nil {
		return true
	}

	stats := eruntime.GetTerritoryStats(t.Name)
	if stats == nil {
		return true
	}

	if stats.HQ {
		return false
	}

	if stats.RouteTax > 0 {
		return true
	}

	if len(stats.TradingRoutes) == 0 {
		return true
	}

	return false
}

type setLevels struct {
	Upgrades typedef.Upgrade
	Bonuses  typedef.Bonus
}

func getSetLevels(t *typedef.Territory) setLevels {
	t.Mu.RLock()
	defer t.Mu.RUnlock()
	return setLevels{Upgrades: t.Options.Upgrade.Set, Bonuses: t.Options.Bonus.Set}
}

func calcLevelInt(levels setLevels) int {
	auraBonus := 0
	if levels.Bonuses.TowerAura > 0 {
		auraBonus = 4 + levels.Bonuses.TowerAura
	}
	volleyBonus := 0
	if levels.Bonuses.TowerVolley > 0 {
		volleyBonus = 2 + levels.Bonuses.TowerVolley
	}

	return levels.Upgrades.Damage + levels.Upgrades.Attack + levels.Upgrades.Health + levels.Upgrades.Defence +
		levels.Bonuses.TowerAura + levels.Bonuses.TowerVolley + auraBonus + volleyBonus
}

func availableDefenseSteps(levels setLevels) []defenseStep {
	steps := make([]defenseStep, 0, 6)

	if levels.Upgrades.Damage < 11 {
		cost, res := eruntime.GetUpgradeCost("damage", levels.Upgrades.Damage+1)
		steps = append(steps, defenseStep{kind: "damage", resource: res, nextLevel: levels.Upgrades.Damage + 1, delta: 1, cost: float64(cost)})
	}
	if levels.Upgrades.Attack < 11 {
		cost, res := eruntime.GetUpgradeCost("attack", levels.Upgrades.Attack+1)
		steps = append(steps, defenseStep{kind: "attack", resource: res, nextLevel: levels.Upgrades.Attack + 1, delta: 1, cost: float64(cost)})
	}
	if levels.Upgrades.Health < 11 {
		cost, res := eruntime.GetUpgradeCost("health", levels.Upgrades.Health+1)
		steps = append(steps, defenseStep{kind: "health", resource: res, nextLevel: levels.Upgrades.Health + 1, delta: 1, cost: float64(cost)})
	}
	if levels.Upgrades.Defence < 11 {
		cost, res := eruntime.GetUpgradeCost("defence", levels.Upgrades.Defence+1)
		steps = append(steps, defenseStep{kind: "defence", resource: res, nextLevel: levels.Upgrades.Defence + 1, delta: 1, cost: float64(cost)})
	}

	if levels.Bonuses.TowerAura < 3 {
		cost, res := eruntime.GetBonusCost("towerAura", levels.Bonuses.TowerAura+1)
		delta := 6
		if levels.Bonuses.TowerAura == 1 || levels.Bonuses.TowerAura == 2 {
			delta = 2
		}
		steps = append(steps, defenseStep{kind: "aura", resource: res, nextLevel: levels.Bonuses.TowerAura + 1, delta: delta, cost: float64(cost)})
	}
	if levels.Bonuses.TowerVolley < 3 {
		cost, res := eruntime.GetBonusCost("towerVolley", levels.Bonuses.TowerVolley+1)
		delta := 4
		if levels.Bonuses.TowerVolley == 1 || levels.Bonuses.TowerVolley == 2 {
			delta = 2
		}
		steps = append(steps, defenseStep{kind: "volley", resource: res, nextLevel: levels.Bonuses.TowerVolley + 1, delta: delta, cost: float64(cost)})
	}

	return steps
}

type resourceWeights struct {
	Emeralds float64
	Ores     float64
	Wood     float64
	Fish     float64
	Crops    float64
}

func claimNetWeights(territories []*typedef.Territory) resourceWeights {
	net := claimNet(territories)
	return resourceWeights{
		Emeralds: scarcityWeight(net.Emeralds),
		Ores:     scarcityWeight(net.Ores),
		Wood:     scarcityWeight(net.Wood),
		Fish:     scarcityWeight(net.Fish),
		Crops:    scarcityWeight(net.Crops),
	}
}

func scarcityWeight(net float64) float64 {
	if net <= 0 {
		return 10
	}
	return 1 / math.Max(net/1000.0, 1)
}

func weightedCost(cost float64, resource string, weights resourceWeights) float64 {
	switch resource {
	case "emeralds":
		return cost * weights.Emeralds
	case "ore", "ores":
		return cost * weights.Ores
	case "wood":
		return cost * weights.Wood
	case "fish":
		return cost * weights.Fish
	case "crops":
		return cost * weights.Crops
	default:
		return cost
	}
}

func applyFakeHighOnCriticalProducers(cl *claim, result *AutoResult) {
	critical := criticalProductionTerritories(cl)
	weights := claimNetWeights(cl.Territories)
	for _, t := range critical {
		applyDefenseTarget(t, defenseTarget(defenseFakeHighCore, 0), weights, result, defenseFakeHighCore, 1)
	}
}

func criticalProductionTerritories(cl *claim) []*typedef.Territory {
	critical := []*typedef.Territory{}
	prodCounts := map[ResourceKind]int{}
	for kind, list := range cl.ResourceSets {
		prodCounts[kind] = len(list)
	}

	buffed := map[string]bool{}
	for _, t := range cl.Territories {
		if t == nil {
			continue
		}
		level := getBonusLevel(t, func(b *typedef.Bonus) int { return b.EfficientResource })
		if level > 0 {
			buffed[t.Name] = true
		}
	}

	for kind, list := range cl.ResourceSets {
		if len(list) <= 2 {
			for _, t := range list {
				if t != nil && buffed[t.Name] {
					critical = append(critical, t)
				}
			}
			continue
		}

		buffedCount := 0
		for _, t := range list {
			if t != nil && buffed[t.Name] {
				buffedCount++
			}
		}

		if buffedCount >= prodCounts[kind]-1 {
			for _, t := range list {
				if t != nil && buffed[t.Name] {
					critical = append(critical, t)
				}
			}
		}
	}

	for _, city := range cl.Cities {
		if city == nil {
			continue
		}
		if len(cl.Cities) <= 2 {
			critical = append(critical, city)
		}
	}

	return critical
}

func rebalanceDefense(cl *claim, cfg AutoConfig, result *AutoResult) {
	for i := 0; i < cfg.MaxIterations; i++ {
		importance := combinedImportance(cl, cfg)
		if len(importance) == 0 {
			return
		}

		targets := topImportantTerritories(cl.Territories, importance, cfg.HighCountFraction)
		weights := claimNetWeights(cl.Territories)
		for _, t := range targets {
			applyDefenseTarget(t, defenseTarget(defenseActualHigh, defenseSetHigh), weights, result, defenseActualHigh, 1)
		}

		mediumFraction := cfg.HighCountFraction * 2.0
		if mediumFraction > 1.0 {
			mediumFraction = 1.0
		}
		mediumTargets := topImportantTerritories(cl.Territories, importance, mediumFraction)
		for _, t := range mediumTargets {
			applyDefenseTarget(t, defenseTarget(defenseActualMedium, defenseSetMedium), weights, result, defenseActualMedium, 1)
		}

		moved := moveProductionToImportant(cl, targets, importance, result)
		if !moved {
			return
		}
	}
}

func combinedImportance(cl *claim, cfg AutoConfig) map[string]float64 {
	territories := eruntime.GetAllTerritories()
	territoryMap := make(map[string]*typedef.Territory, len(territories))
	for _, t := range territories {
		if t != nil {
			territoryMap[t.Name] = t
		}
	}

	runtimeOpts := eruntime.GetRuntimeOptions()
	chokepoints, _ := alg.ComputeChokepoints(cfg.GuildTag, territoryMap, eruntime.GetTradingRoutes(), runtimeOpts.ChokepointEmeraldWeight, runtimeOpts.ChokepointIncludeDownstream)

	throughput := eruntime.GetInGuildTransitTotals()
	maxThroughput := 0.0
	for _, t := range cl.Territories {
		if t == nil {
			continue
		}
		if throughput[t.Name] > maxThroughput {
			maxThroughput = throughput[t.Name]
		}
	}

	combined := make(map[string]float64)
	for _, t := range cl.Territories {
		if t == nil {
			continue
		}
		choke := 0.0
		if report, ok := chokepoints[t.Name]; ok {
			choke = report.Importance
		}
		thr := 0.0
		if maxThroughput > 0 {
			thr = throughput[t.Name] / maxThroughput
		}
		combined[t.Name] = cfg.ChokeWeight*choke + cfg.ThroughputWeight*thr
	}

	return combined
}

func topImportantTerritories(territories []*typedef.Territory, importance map[string]float64, fraction float64) []*typedef.Territory {
	sorted := make([]*typedef.Territory, 0, len(territories))
	for _, t := range territories {
		if t != nil {
			sorted = append(sorted, t)
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return importance[sorted[i].Name] > importance[sorted[j].Name]
	})

	count := int(math.Ceil(float64(len(sorted)) * fraction))
	if count < 1 {
		count = 1
	}
	if count > len(sorted) {
		count = len(sorted)
	}
	return sorted[:count]
}

func moveProductionToImportant(cl *claim, targets []*typedef.Territory, importance map[string]float64, result *AutoResult) bool {
	moved := false
	targetSet := map[string]bool{}
	for _, t := range targets {
		if t != nil {
			targetSet[t.Name] = true
		}
	}

	for _, target := range targets {
		if target == nil {
			continue
		}
		kind, doubleTerritory, rainbow := classifyResource(target)
		if rainbow || doubleTerritory || kind == "" {
			continue
		}

		if getBonusLevel(target, func(b *typedef.Bonus) int { return b.EfficientResource }) > 0 {
			continue
		}

		donor := findProductionDonor(cl.ResourceSets[kind], targetSet, importance)
		if donor == nil {
			continue
		}

		level := getBonusLevel(donor, func(b *typedef.Bonus) int { return b.EfficientResource })
		if level == 0 {
			continue
		}

		applyProdBuff(donor, 0, result)
		applyProdBuff(target, level, result)
		moved = true
	}

	return moved
}

func findProductionDonor(list []*typedef.Territory, targetSet map[string]bool, importance map[string]float64) *typedef.Territory {
	var donor *typedef.Territory
	best := math.MaxFloat64
	for _, t := range list {
		if t == nil || targetSet[t.Name] {
			continue
		}
		level := getBonusLevel(t, func(b *typedef.Bonus) int { return b.EfficientResource })
		if level == 0 {
			continue
		}
		score := importance[t.Name]
		if score < best {
			best = score
			donor = t
		}
	}
	return donor
}

func claimNet(territories []*typedef.Territory) typedef.BasicResources {
	net := typedef.BasicResources{}
	for _, t := range territories {
		if t == nil {
			continue
		}
		stats := eruntime.GetTerritoryStats(t.Name)
		if stats == nil {
			continue
		}
		net.Emeralds += stats.CurrentGeneration.Emeralds - stats.TotalCosts.Emeralds
		net.Ores += stats.CurrentGeneration.Ores - stats.TotalCosts.Ores
		net.Wood += stats.CurrentGeneration.Wood - stats.TotalCosts.Wood
		net.Fish += stats.CurrentGeneration.Fish - stats.TotalCosts.Fish
		net.Crops += stats.CurrentGeneration.Crops - stats.TotalCosts.Crops
	}
	return net
}

func mostNegativeResource(net typedef.BasicResources) ResourceKind {
	minVal := 0.0
	kind := ResourceKind("")
	if net.Ores < minVal {
		minVal = net.Ores
		kind = ResourceOre
	}
	if net.Crops < minVal {
		minVal = net.Crops
		kind = ResourceCrops
	}
	if net.Wood < minVal {
		minVal = net.Wood
		kind = ResourceWood
	}
	if net.Fish < minVal {
		minVal = net.Fish
		kind = ResourceFish
	}
	return kind
}

func classifyResource(t *typedef.Territory) (ResourceKind, bool, bool) {
	if t == nil {
		return "", false, false
	}
	t.Mu.RLock()
	base := t.ResourceGeneration.Base
	t.Mu.RUnlock()

	count := resourceCount(base)
	if count >= 3 {
		return "", false, true
	}
	if count == 2 {
		return "", true, false
	}

	switch {
	case base.Ores > 0:
		return ResourceOre, false, false
	case base.Wood > 0:
		return ResourceWood, false, false
	case base.Fish > 0:
		return ResourceFish, false, false
	case base.Crops > 0:
		return ResourceCrops, false, false
	default:
		return "", false, false
	}
}

func isRainbow(t *typedef.Territory) bool {
	_, _, rainbow := classifyResource(t)
	return rainbow
}

func territoryHasResource(t *typedef.Territory, kind ResourceKind) bool {
	if t == nil {
		return false
	}
	t.Mu.RLock()
	base := t.ResourceGeneration.Base
	t.Mu.RUnlock()

	switch kind {
	case ResourceOre:
		return base.Ores > 0
	case ResourceWood:
		return base.Wood > 0
	case ResourceFish:
		return base.Fish > 0
	case ResourceCrops:
		return base.Crops > 0
	default:
		return false
	}
}

func resourceCount(base typedef.BasicResources) int {
	count := 0
	if base.Ores > 0 {
		count++
	}
	if base.Wood > 0 {
		count++
	}
	if base.Fish > 0 {
		count++
	}
	if base.Crops > 0 {
		count++
	}
	return count
}

func isDouble(t *typedef.Territory) bool {
	_, doubleTerritory, _ := classifyResource(t)
	return doubleTerritory
}

func maxResourcePerMinute(gen typedef.BasicResources) float64 {
	return math.Max(
		math.Max(gen.Ores, gen.Wood),
		math.Max(gen.Fish, gen.Crops),
	) / 60.0
}

func maxResourcePerSecond(gen typedef.BasicResourcesSecond) float64 {
	return math.Max(
		math.Max(gen.Ores, gen.Wood),
		math.Max(gen.Fish, gen.Crops),
	)
}

func intervalSeconds(value uint8) float64 {
	if value == 0 {
		return 4.0
	}
	return float64(value)
}

func storageLevelFor(capacities []float64, requiredPerMinute float64) int {
	if len(capacities) == 0 {
		return 0
	}
	for level := 0; level < len(capacities); level++ {
		if capacities[level] >= requiredPerMinute {
			return level
		}
	}
	return len(capacities) - 1
}

func resourcesCapacityValues(multipliers []float64) []float64 {
	values := make([]float64, len(multipliers))
	for i, mult := range multipliers {
		values[i] = typedef.BaseResourceCapacity.Ores * mult
	}
	return values
}

func emeraldCapacityValues(multipliers []float64) []float64 {
	values := make([]float64, len(multipliers))
	for i, mult := range multipliers {
		values[i] = typedef.BaseResourceCapacity.Emeralds * mult
	}
	return values
}

func isCity(t *typedef.Territory) bool {
	if t == nil {
		return false
	}
	t.Mu.RLock()
	emeralds := t.ResourceGeneration.Base.Emeralds
	t.Mu.RUnlock()
	return math.Abs(emeralds-18000) < 0.01
}

func sortByDistanceToHQ(list []*typedef.Territory, hq *typedef.Territory) {
	if hq == nil {
		return
	}
	sort.SliceStable(list, func(i, j int) bool {
		return routeDistance(list[i]) < routeDistance(list[j])
	})
}

func routeDistance(t *typedef.Territory) int {
	if t == nil {
		return 9999
	}
	t.Mu.RLock()
	isHQ := t.HQ
	t.Mu.RUnlock()
	if isHQ {
		return 0
	}
	_, route, err := eruntime.GetTerritoryRoute(t.Name)
	if err != nil || route == nil {
		return 9999
	}
	if len(route) == 0 {
		return 9999
	}
	return len(route)
}

type resourceCandidate struct {
	territory *typedef.Territory
	classRank int
}

func resourcePriority(t *typedef.Territory, kind ResourceKind) float64 {
	if t == nil {
		return 0
	}
	dist := float64(routeDistance(t))
	if dist <= 0 {
		dist = 1
	}
	base := resourceBaseValue(t, kind)
	if base <= 0 {
		base = 1
	}
	return base / dist
}

func resourceBaseValue(t *typedef.Territory, kind ResourceKind) float64 {
	t.Mu.RLock()
	base := t.ResourceGeneration.Base
	t.Mu.RUnlock()
	switch kind {
	case ResourceOre:
		return base.Ores
	case ResourceWood:
		return base.Wood
	case ResourceFish:
		return base.Fish
	case ResourceCrops:
		return base.Crops
	default:
		return math.Max(math.Max(base.Ores, base.Wood), math.Max(base.Fish, base.Crops))
	}
}

func getBonusLevel(t *typedef.Territory, selectFn func(*typedef.Bonus) int) int {
	t.Mu.RLock()
	level := selectFn(&t.Options.Bonus.Set)
	t.Mu.RUnlock()
	return level
}
