package smartrecipe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 智能菜谱管理器
type Manager struct {
	mu            sync.RWMutex
	recipes       map[string]*Recipe
	ingredients   map[string]*Ingredient
	inventory     map[string]*InventoryItem
	mealPlans     map[string]*MealPlan
	shoppingLists map[string]*ShoppingList
	logger        Logger
	ctx           context.Context
	cancel        context.CancelFunc
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewManager 创建菜谱管理器
func NewManager(logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		recipes:       make(map[string]*Recipe),
		ingredients:   make(map[string]*Ingredient),
		inventory:     make(map[string]*InventoryItem),
		mealPlans:     make(map[string]*MealPlan),
		shoppingLists: make(map[string]*ShoppingList),
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// ==================== 菜谱管理 ====================

// CreateRecipe 创建菜谱
func (m *Manager) CreateRecipe(recipe *Recipe) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if recipe.ID == "" {
		recipe.ID = generateID("recipe")
	}
	recipe.CreatedAt = time.Now()
	recipe.UpdatedAt = time.Now()

	// 计算总营养成分
	recipe.TotalNutrition = m.calculateRecipeNutrition(recipe)

	m.recipes[recipe.ID] = recipe
	m.logger.Info("菜谱创建成功: %s (%s)", recipe.Name, recipe.ID)
	return nil
}

// UpdateRecipe 更新菜谱
func (m *Manager) UpdateRecipe(recipe *Recipe) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.recipes[recipe.ID]
	if !ok {
		return fmt.Errorf("菜谱不存在: %s", recipe.ID)
	}

	recipe.CreatedAt = existing.CreatedAt
	recipe.UpdatedAt = time.Now()
	recipe.TotalNutrition = m.calculateRecipeNutrition(recipe)
	m.recipes[recipe.ID] = recipe
	m.logger.Info("菜谱更新成功: %s (%s)", recipe.Name, recipe.ID)
	return nil
}

// DeleteRecipe 删除菜谱
func (m *Manager) DeleteRecipe(recipeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.recipes[recipeID]; !ok {
		return fmt.Errorf("菜谱不存在: %s", recipeID)
	}

	delete(m.recipes, recipeID)
	m.logger.Info("菜谱删除成功: %s", recipeID)
	return nil
}

// GetRecipe 获取菜谱
func (m *Manager) GetRecipe(recipeID string) (*Recipe, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recipe, ok := m.recipes[recipeID]
	if !ok {
		return nil, fmt.Errorf("菜谱不存在: %s", recipeID)
	}
	return recipe, nil
}

// ListRecipes 列出菜谱（支持筛选）
func (m *Manager) ListRecipes(category Category, difficulty Difficulty, keyword string) []*Recipe {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recipes := make([]*Recipe, 0)
	for _, recipe := range m.recipes {
		if category != "" && recipe.Category != category {
			continue
		}
		if difficulty != "" && recipe.Difficulty != difficulty {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(recipe.Name), strings.ToLower(keyword)) &&
			!strings.Contains(strings.ToLower(recipe.Description), strings.ToLower(keyword)) {
			continue
		}
		recipes = append(recipes, recipe)
	}

	sort.Slice(recipes, func(i, j int) bool {
		return recipes[i].Rating > recipes[j].Rating
	})
	return recipes
}

// ==================== 食材库管理 ====================

// CreateIngredient 创建食材
func (m *Manager) CreateIngredient(ingredient *Ingredient) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ingredient.ID == "" {
		ingredient.ID = generateID("ing")
	}
	ingredient.CreatedAt = time.Now()

	m.ingredients[ingredient.ID] = ingredient
	m.logger.Info("食材创建成功: %s (%s)", ingredient.Name, ingredient.ID)
	return nil
}

// UpdateIngredient 更新食材
func (m *Manager) UpdateIngredient(ingredient *Ingredient) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.ingredients[ingredient.ID]; !ok {
		return fmt.Errorf("食材不存在: %s", ingredient.ID)
	}

	m.ingredients[ingredient.ID] = ingredient
	return nil
}

// DeleteIngredient 删除食材
func (m *Manager) DeleteIngredient(ingredientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.ingredients[ingredientID]; !ok {
		return fmt.Errorf("食材不存在: %s", ingredientID)
	}

	delete(m.ingredients, ingredientID)
	return nil
}

// GetIngredient 获取食材
func (m *Manager) GetIngredient(ingredientID string) (*Ingredient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ingredient, ok := m.ingredients[ingredientID]
	if !ok {
		return nil, fmt.Errorf("食材不存在: %s", ingredientID)
	}
	return ingredient, nil
}

// ListIngredients 列出食材
func (m *Manager) ListIngredients(category IngredientCategory) []*Ingredient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ingredients := make([]*Ingredient, 0)
	for _, ing := range m.ingredients {
		if category != "" && ing.Category != category {
			continue
		}
		ingredients = append(ingredients, ing)
	}
	return ingredients
}

// ==================== 库存管理 ====================

// AddInventory 添加库存
func (m *Manager) AddInventory(item *InventoryItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item.ID == "" {
		item.ID = generateID("inv")
	}

	// 获取食材名称
	if ing, ok := m.ingredients[item.IngredientID]; ok {
		item.IngredientName = ing.Name
		item.Unit = ing.Unit
	}

	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()
	m.inventory[item.ID] = item
	m.logger.Info("库存添加成功: %s x%v %s", item.IngredientName, item.Quantity, item.Unit)
	return nil
}

// UpdateInventory 更新库存
func (m *Manager) UpdateInventory(item *InventoryItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.inventory[item.ID]
	if !ok {
		return fmt.Errorf("库存项不存在: %s", item.ID)
	}

	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = time.Now()
	m.inventory[item.ID] = item
	return nil
}

// DeleteInventory 删除库存
func (m *Manager) DeleteInventory(itemID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.inventory[itemID]; !ok {
		return fmt.Errorf("库存项不存在: %s", itemID)
	}

	delete(m.inventory, itemID)
	return nil
}

// GetInventory 获取库存列表
func (m *Manager) GetInventory(location string, statusFilter InventoryStatus) []*InventoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*InventoryItem, 0)
	now := time.Now()

	for _, item := range m.inventory {
		if location != "" && item.Location != location {
			continue
		}

		status := m.getInventoryStatus(item, now)
		if statusFilter != "" && status != statusFilter {
			continue
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ExpiryDate.Before(items[j].ExpiryDate)
	})
	return items
}

// GetInventoryStatus 获取库存状态
func (m *Manager) getInventoryStatus(item *InventoryItem, now time.Time) InventoryStatus {
	if item.ExpiryDate.Before(now) {
		return InventoryStatusExpired
	}
	if item.ExpiryDate.Sub(now).Hours() < 72 { // 3天内过期
		return InventoryStatusExpiringSoon
	}
	if item.Quantity < 100 { // 简单判断库存不足
		return InventoryStatusLow
	}
	return InventoryStatusNormal
}

// GetExpiringItems 获取即将过期的食材
func (m *Manager) GetExpiringItems(days int) []*InventoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*InventoryItem, 0)
	deadline := time.Now().AddDate(0, 0, days)

	for _, item := range m.inventory {
		if item.ExpiryDate.Before(deadline) && item.ExpiryDate.After(time.Now()) {
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ExpiryDate.Before(items[j].ExpiryDate)
	})
	return items
}

// ==================== 智能推荐 ====================

// RecommendRecipes 推荐菜谱
func (m *Manager) RecommendRecipes(req RecommendationRequest) []RecommendationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]RecommendationResult, 0)

	// 如果没有指定食材，使用库存中的食材
	availableIDs := req.AvailableIngredients
	if len(availableIDs) == 0 {
		for _, inv := range m.inventory {
			if inv.ExpiryDate.After(time.Now()) {
				availableIDs = append(availableIDs, inv.IngredientID)
			}
		}
	}

	for _, recipe := range m.recipes {
		// 筛选条件
		if req.MaxPrepTime > 0 && recipe.PrepTime > req.MaxPrepTime {
			continue
		}
		if req.MaxCookTime > 0 && recipe.CookTime > req.MaxCookTime {
			continue
		}
		if req.PreferredDifficulty != "" && recipe.Difficulty != req.PreferredDifficulty {
			continue
		}
		if req.PreferredCategory != "" && recipe.Category != req.PreferredCategory {
			continue
		}
		if req.MaxCalories > 0 && recipe.TotalNutrition.Calories > req.MaxCalories {
			continue
		}

		// 检查排除标签
		excluded := false
		for _, tag := range req.ExcludeTags {
			for _, rTag := range recipe.Tags {
				if strings.EqualFold(tag, rTag) {
					excluded = true
					break
				}
			}
			if excluded {
				break
			}
		}
		if excluded {
			continue
		}

		// 计算匹配度
		matched := 0
		missing := 0
		for _, ri := range recipe.Ingredients {
			if ri.Optional {
				continue
			}
			found := false
			for _, availID := range availableIDs {
				if ri.IngredientID == availID {
					found = true
					break
				}
			}
			if found {
				matched++
			} else {
				missing++
			}
		}

		totalRequired := matched + missing
		if totalRequired == 0 {
			continue
		}

		matchScore := float64(matched) / float64(totalRequired) * 100

		results = append(results, RecommendationResult{
			Recipe:             *recipe,
			MatchScore:         matchScore,
			MatchedIngredients: matched,
			MissingIngredients: missing,
		})
	}

	// 按匹配度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].MatchScore > results[j].MatchScore
	})

	// 限制返回数量
	if req.Limit > 0 && len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results
}

// ==================== 膳食计划 ====================

// CreateMealPlan 创建膳食计划
func (m *Manager) CreateMealPlan(plan *MealPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if plan.ID == "" {
		plan.ID = generateID("plan")
	}
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	m.mealPlans[plan.ID] = plan
	m.logger.Info("膳食计划创建成功: %s (%s)", plan.Name, plan.ID)
	return nil
}

// UpdateMealPlan 更新膳食计划
func (m *Manager) UpdateMealPlan(plan *MealPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.mealPlans[plan.ID]
	if !ok {
		return fmt.Errorf("膳食计划不存在: %s", plan.ID)
	}

	plan.CreatedAt = existing.CreatedAt
	plan.UpdatedAt = time.Now()
	m.mealPlans[plan.ID] = plan
	return nil
}

// DeleteMealPlan 删除膳食计划
func (m *Manager) DeleteMealPlan(planID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.mealPlans[planID]; !ok {
		return fmt.Errorf("膳食计划不存在: %s", planID)
	}

	delete(m.mealPlans, planID)
	return nil
}

// GetMealPlan 获取膳食计划
func (m *Manager) GetMealPlan(planID string) (*MealPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.mealPlans[planID]
	if !ok {
		return nil, fmt.Errorf("膳食计划不存在: %s", planID)
	}
	return plan, nil
}

// ListMealPlans 列出膳食计划
func (m *Manager) ListMealPlans() []*MealPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]*MealPlan, 0, len(m.mealPlans))
	for _, plan := range m.mealPlans {
		plans = append(plans, plan)
	}
	return plans
}

// ==================== 营养分析 ====================

// AnalyzeNutrition 分析膳食计划的营养成分
func (m *Manager) AnalyzeNutrition(planID string) (*NutritionSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.mealPlans[planID]
	if !ok {
		return nil, fmt.Errorf("膳食计划不存在: %s", planID)
	}

	summary := &NutritionSummary{
		StartDate: plan.StartDate,
		EndDate:   plan.EndDate,
	}

	mealCount := 0
	for _, day := range plan.Days {
		for _, meal := range day.Meals {
			recipe, ok := m.recipes[meal.RecipeID]
			if !ok {
				continue
			}

			// 按份量计算
			servings := float64(meal.Servings)
			if servings == 0 {
				servings = 1
			}

			summary.TotalCalories += recipe.TotalNutrition.Calories * servings
			summary.TotalProtein += recipe.TotalNutrition.Protein * servings
			summary.TotalFat += recipe.TotalNutrition.Fat * servings
			summary.TotalCarb += recipe.TotalNutrition.Carb * servings
			summary.TotalFiber += recipe.TotalNutrition.Fiber * servings
			mealCount++
		}
	}

	summary.MealCount = mealCount

	// 计算日均
	if len(plan.Days) > 0 {
		days := float64(len(plan.Days))
		summary.AvgCalories = summary.TotalCalories / days
	}

	return summary, nil
}

// AnalyzeRecipeNutrition 分析单个菜谱营养
func (m *Manager) AnalyzeRecipeNutrition(recipeID string, servings int) (*Nutrient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recipe, ok := m.recipes[recipeID]
	if !ok {
		return nil, fmt.Errorf("菜谱不存在: %s", recipeID)
	}

	if servings <= 0 {
		servings = 1
	}

	factor := float64(servings) / float64(recipe.Servings)
	if recipe.Servings == 0 {
		factor = float64(servings)
	}

	return &Nutrient{
		Calories: recipe.TotalNutrition.Calories * factor,
		Protein:  recipe.TotalNutrition.Protein * factor,
		Fat:      recipe.TotalNutrition.Fat * factor,
		Carb:     recipe.TotalNutrition.Carb * factor,
		Fiber:    recipe.TotalNutrition.Fiber * factor,
	}, nil
}

// ==================== 购物清单 ====================

// GenerateShoppingList 根据膳食计划生成购物清单
func (m *Manager) GenerateShoppingList(planID string, name string) (*ShoppingList, error) {
	m.mu.RLock()
	plan, ok := m.mealPlans[planID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("膳食计划不存在: %s", planID)
	}

	// 汇总所需食材
	ingredientNeeds := make(map[string]struct {
		Quantity    float64
		Unit        string
		RecipeNames []string
	})

	for _, day := range plan.Days {
		for _, meal := range day.Meals {
			recipe, ok := m.recipes[meal.RecipeID]
			if !ok {
				continue
			}

			servings := float64(meal.Servings)
			if servings == 0 {
				servings = 1
			}
			factor := servings / float64(recipe.Servings)
			if recipe.Servings == 0 {
				factor = servings
			}

			for _, ri := range recipe.Ingredients {
				existing := ingredientNeeds[ri.IngredientID]
				existing.Quantity += ri.Quantity * factor
				existing.Unit = ri.Unit
				found := false
				for _, rn := range existing.RecipeNames {
					if rn == recipe.Name {
						found = true
						break
					}
				}
				if !found {
					existing.RecipeNames = append(existing.RecipeNames, recipe.Name)
				}
				ingredientNeeds[ri.IngredientID] = existing
			}
		}
	}
	m.mu.RUnlock()

	// 对比库存，找出需要购买的
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]ShoppingItem, 0)
	for ingID, need := range ingredientNeeds {
		// 检查库存
		inStock := 0.0
		for _, inv := range m.inventory {
			if inv.IngredientID == ingID && inv.ExpiryDate.After(time.Now()) {
				inStock += inv.Quantity
			}
		}

		// 计算缺口
		deficit := need.Quantity - inStock
		if deficit <= 0 {
			continue
		}

		ingName := ingID
		if ing, ok := m.ingredients[ingID]; ok {
			ingName = ing.Name
		}

		reason := "不足"
		if inStock == 0 {
			reason = "缺少"
		}

		items = append(items, ShoppingItem{
			IngredientID:   ingID,
			IngredientName: ingName,
			Quantity:       deficit,
			Unit:           need.Unit,
			Reason:         reason,
			RecipeNames:    need.RecipeNames,
			Checked:        false,
		})
	}

	if name == "" {
		name = fmt.Sprintf("购物清单-%s", plan.Name)
	}

	list := &ShoppingList{
		ID:        generateID("shop"),
		Name:      name,
		Items:     items,
		PlanID:    planID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.shoppingLists[list.ID] = list
	m.logger.Info("购物清单生成成功: %s, 共%d项", list.Name, len(items))
	return list, nil
}

// GetShoppingList 获取购物清单
func (m *Manager) GetShoppingList(listID string) (*ShoppingList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list, ok := m.shoppingLists[listID]
	if !ok {
		return nil, fmt.Errorf("购物清单不存在: %s", listID)
	}
	return list, nil
}

// UpdateShoppingItem 更新购物清单项（标记已购买等）
func (m *Manager) UpdateShoppingItem(listID string, itemIndex int, checked bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	list, ok := m.shoppingLists[listID]
	if !ok {
		return fmt.Errorf("购物清单不存在: %s", listID)
	}

	if itemIndex < 0 || itemIndex >= len(list.Items) {
		return fmt.Errorf("购物项索引越界: %d", itemIndex)
	}

	list.Items[itemIndex].Checked = checked
	list.UpdatedAt = time.Now()
	return nil
}

// ==================== 辅助函数 ====================

// calculateRecipeNutrition 计算菜谱总营养成分
func (m *Manager) calculateRecipeNutrition(recipe *Recipe) Nutrient {
	total := Nutrient{}

	for _, ri := range recipe.Ingredients {
		ing, ok := m.ingredients[ri.IngredientID]
		if !ok {
			continue
		}

		// 按重量计算营养（假设单位为g）
		factor := ri.Quantity / 100.0

		total.Calories += ing.NutrientPer100.Calories * factor
		total.Protein += ing.NutrientPer100.Protein * factor
		total.Fat += ing.NutrientPer100.Fat * factor
		total.Carb += ing.NutrientPer100.Carb * factor
		total.Fiber += ing.NutrientPer100.Fiber * factor
		total.Sodium += ing.NutrientPer100.Sodium * factor
		total.VitaminA += ing.NutrientPer100.VitaminA * factor
		total.VitaminC += ing.NutrientPer100.VitaminC * factor
		total.Calcium += ing.NutrientPer100.Calcium * factor
		total.Iron += ing.NutrientPer100.Iron * factor
	}

	return total
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// ==================== HTTP API ====================

// RegisterRoutes 注册HTTP路由
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	// 菜谱管理
	mux.HandleFunc("/api/recipes", m.handleRecipes)
	mux.HandleFunc("/api/recipes/", m.handleRecipeByID)
	mux.HandleFunc("/api/recipes/recommend", m.handleRecommendRecipes)

	// 食材管理
	mux.HandleFunc("/api/ingredients", m.handleIngredients)
	mux.HandleFunc("/api/ingredients/", m.handleIngredientByID)

	// 库存管理
	mux.HandleFunc("/api/inventory", m.handleInventory)
	mux.HandleFunc("/api/inventory/expiring", m.handleExpiringItems)

	// 膳食计划
	mux.HandleFunc("/api/meal-plans", m.handleMealPlans)
	mux.HandleFunc("/api/meal-plans/", m.handleMealPlanByID)

	// 营养分析
	mux.HandleFunc("/api/nutrition/plan/", m.handleNutritionAnalysis)
	mux.HandleFunc("/api/nutrition/recipe/", m.handleRecipeNutrition)

	// 购物清单
	mux.HandleFunc("/api/shopping-lists", m.handleShoppingLists)
	mux.HandleFunc("/api/shopping-lists/", m.handleShoppingListByID)
	mux.HandleFunc("/api/shopping-lists/generate", m.handleGenerateShoppingList)
}

// handleRecipes 处理菜谱列表/创建
func (m *Manager) handleRecipes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		category := Category(r.URL.Query().Get("category"))
		difficulty := Difficulty(r.URL.Query().Get("difficulty"))
		keyword := r.URL.Query().Get("keyword")
		recipes := m.ListRecipes(category, difficulty, keyword)
		writeJSON(w, recipes)
	case http.MethodPost:
		var recipe Recipe
		if err := json.NewDecoder(r.Body).Decode(&recipe); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateRecipe(&recipe); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, recipe)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRecipeByID 处理单个菜谱操作
func (m *Manager) handleRecipeByID(w http.ResponseWriter, r *http.Request) {
	recipeID := strings.TrimPrefix(r.URL.Path, "/api/recipes/")
	if recipeID == "" || recipeID == "recommend" {
		return
	}

	switch r.Method {
	case http.MethodGet:
		recipe, err := m.GetRecipe(recipeID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, recipe)
	case http.MethodPut:
		var recipe Recipe
		if err := json.NewDecoder(r.Body).Decode(&recipe); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		recipe.ID = recipeID
		if err := m.UpdateRecipe(&recipe); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, recipe)
	case http.MethodDelete:
		if err := m.DeleteRecipe(recipeID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRecommendRecipes 处理菜谱推荐
func (m *Manager) handleRecommendRecipes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RecommendationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	results := m.RecommendRecipes(req)
	writeJSON(w, results)
}

// handleIngredients 处理食材列表/创建
func (m *Manager) handleIngredients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		category := IngredientCategory(r.URL.Query().Get("category"))
		ingredients := m.ListIngredients(category)
		writeJSON(w, ingredients)
	case http.MethodPost:
		var ingredient Ingredient
		if err := json.NewDecoder(r.Body).Decode(&ingredient); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateIngredient(&ingredient); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, ingredient)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleIngredientByID 处理单个食材操作
func (m *Manager) handleIngredientByID(w http.ResponseWriter, r *http.Request) {
	ingredientID := strings.TrimPrefix(r.URL.Path, "/api/ingredients/")
	if ingredientID == "" {
		return
	}

	switch r.Method {
	case http.MethodGet:
		ingredient, err := m.GetIngredient(ingredientID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, ingredient)
	case http.MethodPut:
		var ingredient Ingredient
		if err := json.NewDecoder(r.Body).Decode(&ingredient); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ingredient.ID = ingredientID
		if err := m.UpdateIngredient(&ingredient); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, ingredient)
	case http.MethodDelete:
		if err := m.DeleteIngredient(ingredientID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleInventory 处理库存列表/添加
func (m *Manager) handleInventory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		location := r.URL.Query().Get("location")
		status := InventoryStatus(r.URL.Query().Get("status"))
		items := m.GetInventory(location, status)
		writeJSON(w, items)
	case http.MethodPost:
		var item InventoryItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddInventory(&item); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, item)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleExpiringItems 处理即将过期的食材
func (m *Manager) handleExpiringItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	days := 3
	if d := r.URL.Query().Get("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	items := m.GetExpiringItems(days)
	writeJSON(w, items)
}

// handleMealPlans 处理膳食计划列表/创建
func (m *Manager) handleMealPlans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		plans := m.ListMealPlans()
		writeJSON(w, plans)
	case http.MethodPost:
		var plan MealPlan
		if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateMealPlan(&plan); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, plan)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMealPlanByID 处理单个膳食计划操作
func (m *Manager) handleMealPlanByID(w http.ResponseWriter, r *http.Request) {
	planID := strings.TrimPrefix(r.URL.Path, "/api/meal-plans/")
	if planID == "" {
		return
	}

	switch r.Method {
	case http.MethodGet:
		plan, err := m.GetMealPlan(planID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, plan)
	case http.MethodPut:
		var plan MealPlan
		if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		plan.ID = planID
		if err := m.UpdateMealPlan(&plan); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, plan)
	case http.MethodDelete:
		if err := m.DeleteMealPlan(planID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleNutritionAnalysis 处理膳食计划营养分析
func (m *Manager) handleNutritionAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	planID := strings.TrimPrefix(r.URL.Path, "/api/nutrition/plan/")
	if planID == "" {
		http.Error(w, "plan_id is required", http.StatusBadRequest)
		return
	}

	summary, err := m.AnalyzeNutrition(planID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, summary)
}

// handleRecipeNutrition 处理菜谱营养分析
func (m *Manager) handleRecipeNutrition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/nutrition/recipe/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "recipe_id is required", http.StatusBadRequest)
		return
	}

	recipeID := parts[0]
	servings := 1
	if s := r.URL.Query().Get("servings"); s != "" {
		fmt.Sscanf(s, "%d", &servings)
	}

	nutrition, err := m.AnalyzeRecipeNutrition(recipeID, servings)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, nutrition)
}

// handleShoppingLists 处理购物清单列表
func (m *Manager) handleShoppingLists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.mu.RLock()
	lists := make([]*ShoppingList, 0, len(m.shoppingLists))
	for _, list := range m.shoppingLists {
		lists = append(lists, list)
	}
	m.mu.RUnlock()

	writeJSON(w, lists)
}

// handleShoppingListByID 处理单个购物清单操作
func (m *Manager) handleShoppingListByID(w http.ResponseWriter, r *http.Request) {
	listID := strings.TrimPrefix(r.URL.Path, "/api/shopping-lists/")
	if listID == "" || listID == "generate" {
		return
	}

	switch r.Method {
	case http.MethodGet:
		list, err := m.GetShoppingList(listID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, list)
	case http.MethodPut:
		var req struct {
			ItemIndex int  `json:"item_index"`
			Checked   bool `json:"checked"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.UpdateShoppingItem(listID, req.ItemIndex, req.Checked); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "updated"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGenerateShoppingList 处理生成购物清单
func (m *Manager) handleGenerateShoppingList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PlanID string `json:"plan_id"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	list, err := m.GenerateShoppingList(req.PlanID, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
