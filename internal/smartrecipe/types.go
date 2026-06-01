// Package smartrecipe 提供智能食谱功能，支持食材管理、营养分析和购物清单生成。
// 对标群晖 Synology Food，为 NAS 系统提供智能化厨房管理体验。
package smartrecipe

import "time"

// Category 菜谱分类
type Category string

const (
	CategoryChinese   Category = "chinese"   // 中餐
	CategoryWestern   Category = "western"   // 西餐
	CategoryJapanese  Category = "japanese"  // 日料
	CategoryKorean    Category = "korean"    // 韩餐
	CategoryDessert   Category = "dessert"   // 甜点
	CategorySoup      Category = "soup"      // 汤类
	CategorySalad     Category = "salad"     // 沙拉
	CategoryDrink     Category = "drink"     // 饮品
)

// Difficulty 难度
type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"   // 简单
	DifficultyMedium Difficulty = "medium" // 中等
	DifficultyHard   Difficulty = "hard"   // 困难
)

// IngredientCategory 食材分类
type IngredientCategory string

const (
	IngredientCategoryVegetable IngredientCategory = "vegetable" // 蔬菜
	IngredientCategoryMeat      IngredientCategory = "meat"      // 肉类
	IngredientCategorySeafood   IngredientCategory = "seafood"   // 海鲜
	IngredientCategoryDairy     IngredientCategory = "dairy"     // 乳制品
	IngredientCategoryGrain     IngredientCategory = "grain"     // 谷物
	IngredientCategoryFruit     IngredientCategory = "fruit"     // 水果
	IngredientCategorySpice     IngredientCategory = "spice"     // 调料
	IngredientCategoryOther     IngredientCategory = "other"     // 其他
)

// InventoryStatus 库存状态
type InventoryStatus string

const (
	InventoryStatusNormal     InventoryStatus = "normal"      // 正常
	InventoryStatusLow        InventoryStatus = "low"         // 库存不足
	InventoryStatusExpiringSoon InventoryStatus = "expiring_soon" // 即将过期
	InventoryStatusExpired    InventoryStatus = "expired"     // 已过期
)

// Nutrient 营养成分
type Nutrient struct {
	Calories float64 `json:"calories"` // 卡路里
	Protein  float64 `json:"protein"`  // 蛋白质(g)
	Fat      float64 `json:"fat"`      // 脂肪(g)
	Carb     float64 `json:"carb"`     // 碳水化合物(g)
	Fiber    float64 `json:"fiber"`    // 膳食纤维(g)
	Sodium   float64 `json:"sodium"`   // 钠(mg)
	VitaminA float64 `json:"vitamin_a"` // 维生素A(μg)
	VitaminC float64 `json:"vitamin_c"` // 维生素C(mg)
	Calcium  float64 `json:"calcium"`  // 钙(mg)
	Iron     float64 `json:"iron"`     // 铁(mg)
}

// Ingredient 食材
type Ingredient struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Category       IngredientCategory `json:"category"`
	Unit           string             `json:"unit"`            // g, ml, piece, etc.
	NutrientPer100 Nutrient           `json:"nutrient_per_100"` // 每100g营养成分
	ImageURL       string             `json:"image_url,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
}

// RecipeIngredient 菜谱中的食材
type RecipeIngredient struct {
	IngredientID string  `json:"ingredient_id"`
	Name         string  `json:"name"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	Optional     bool    `json:"optional,omitempty"`
}

// Recipe 菜谱
type Recipe struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Description     string             `json:"description,omitempty"`
	Category        Category           `json:"category"`
	Difficulty      Difficulty         `json:"difficulty"`
	PrepTime        int                `json:"prep_time"`        // 准备时间(分钟)
	CookTime        int                `json:"cook_time"`        // 烹饪时间(分钟)
	Servings        int                `json:"servings"`         // 份量
	Ingredients     []RecipeIngredient `json:"ingredients"`
	Steps           []string           `json:"steps"`
	Tags            []string           `json:"tags,omitempty"`
	ImageURL        string             `json:"image_url,omitempty"`
	Rating          float64            `json:"rating,omitempty"`
	TotalNutrition  Nutrient           `json:"total_nutrition"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// InventoryItem 库存项
type InventoryItem struct {
	ID             string          `json:"id"`
	IngredientID   string          `json:"ingredient_id"`
	IngredientName string          `json:"ingredient_name"`
	Quantity       float64         `json:"quantity"`
	Unit           string          `json:"unit"`
	Location       string          `json:"location"`    // fridge, freezer, pantry
	ExpiryDate     time.Time       `json:"expiry_date"`
	Status         InventoryStatus `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// MealPlan 膳食计划
type MealPlan struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	StartDate string     `json:"start_date"` // YYYY-MM-DD
	EndDate   string     `json:"end_date"`   // YYYY-MM-DD
	Days      []DayPlan  `json:"days"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// DayPlan 每日计划
type DayPlan struct {
	Date  string      `json:"date"` // YYYY-MM-DD
	Meals []MealEntry `json:"meals"`
}

// MealEntry 餐食条目
type MealEntry struct {
	MealType string `json:"meal_type"` // breakfast, lunch, dinner, snack
	RecipeID string `json:"recipe_id"`
	Servings int    `json:"servings"`
}

// NutritionSummary 营养摘要
type NutritionSummary struct {
	StartDate    string  `json:"start_date"`
	EndDate      string  `json:"end_date"`
	TotalCalories float64 `json:"total_calories"`
	TotalProtein  float64 `json:"total_protein"`
	TotalFat      float64 `json:"total_fat"`
	TotalCarb     float64 `json:"total_carb"`
	TotalFiber    float64 `json:"total_fiber"`
	AvgCalories   float64 `json:"avg_calories"`
	MealCount     int     `json:"meal_count"`
}

// RecommendationRequest 推荐请求
type RecommendationRequest struct {
	AvailableIngredients []string   `json:"available_ingredients,omitempty"`
	MaxPrepTime          int        `json:"max_prep_time,omitempty"`
	MaxCookTime          int        `json:"max_cook_time,omitempty"`
	PreferredDifficulty  Difficulty `json:"preferred_difficulty,omitempty"`
	PreferredCategory    Category   `json:"preferred_category,omitempty"`
	MaxCalories          float64    `json:"max_calories,omitempty"`
	ExcludeTags          []string   `json:"exclude_tags,omitempty"`
	Limit                int        `json:"limit,omitempty"`
}

// RecommendationResult 推荐结果
type RecommendationResult struct {
	Recipe             Recipe  `json:"recipe"`
	MatchScore         float64 `json:"match_score"`          // 匹配度百分比
	MatchedIngredients int     `json:"matched_ingredients"`  // 匹配的食材数
	MissingIngredients int     `json:"missing_ingredients"`  // 缺少的食材数
}

// ShoppingItem 购物项
type ShoppingItem struct {
	IngredientID   string   `json:"ingredient_id"`
	IngredientName string   `json:"ingredient_name"`
	Quantity       float64  `json:"quantity"`
	Unit           string   `json:"unit"`
	Reason         string   `json:"reason"`            // 缺少, 不足
	RecipeNames    []string `json:"recipe_names"`      // 关联的菜谱
	Checked        bool     `json:"checked"`
}

// ShoppingList 购物清单
type ShoppingList struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Items     []ShoppingItem `json:"items"`
	PlanID    string         `json:"plan_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// ExpiryAlert 过期提醒
type ExpiryAlert struct {
	ID             string    `json:"id"`
	IngredientID   string    `json:"ingredient_id"`
	IngredientName string    `json:"ingredient_name"`
	ExpiryDate     time.Time `json:"expiry_date"`
	DaysLeft       int       `json:"days_left"`
	Level          AlertLevel `json:"level"`
}

// AlertLevel 提醒级别
type AlertLevel string

const (
	AlertLevelExpired AlertLevel = "expired"   // 已过期
	AlertLevelSoon    AlertLevel = "soon"      // 即将过期（1-3天）
	AlertLevelWarning AlertLevel = "warning"   // 预警（4-7天）
)

// RecipeRecommendation 菜谱推荐（简化版）
type RecipeRecommendation struct {
	Recipe      Recipe   `json:"recipe"`
	MatchScore  float64  `json:"match_score"`
	MatchReason string   `json:"match_reason"`
	HasAllIngs  bool     `json:"has_all_ingredients"`
	MissingIngs []string `json:"missing_ingredients,omitempty"`
}

// NutritionInfo 营养信息（详细版）
type NutritionInfo struct {
	Calories    int     `json:"calories"`
	Protein     float64 `json:"protein"`
	Carbs       float64 `json:"carbs"`
	Fat         float64 `json:"fat"`
	Fiber       float64 `json:"fiber"`
	Sugar       float64 `json:"sugar"`
	Sodium      float64 `json:"sodium"`
	Cholesterol float64 `json:"cholesterol"`
	VitaminA    float64 `json:"vitamin_a"`
	VitaminC    float64 `json:"vitamin_c"`
	Calcium     float64 `json:"calcium"`
	Iron        float64 `json:"iron"`
}
