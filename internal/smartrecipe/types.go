// Package smartrecipe 智能菜谱管理
// 提供菜谱管理、食材库、智能推荐、膳食计划、营养分析、购物清单功能
package smartrecipe

import "time"

// Difficulty 菜谱难度
type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"   // 简单
	DifficultyMedium Difficulty = "medium" // 中等
	DifficultyHard   Difficulty = "hard"   // 困难
)

// Category 菜谱分类
type Category string

const (
	CategoryBreakfast  Category = "breakfast"  // 早餐
	CategoryLunch      Category = "lunch"      // 午餐
	CategoryDinner     Category = "dinner"     // 晚餐
	CategorySnack      Category = "snack"      // 小吃
	CategoryDessert    Category = "dessert"    // 甜点
	CategorySoup       Category = "soup"       // 汤品
	CategorySalad      Category = "salad"      // 沙拉
	CategoryDrink      Category = "drink"      // 饮品
	CategoryStaple     Category = "staple"     // 主食
	CategorySideDish   Category = "side_dish"  // 配菜
)

// IngredientCategory 食材分类
type IngredientCategory string

const (
	IngCategoryVegetable IngredientCategory = "vegetable" // 蔬菜
	IngCategoryMeat      IngredientCategory = "meat"      // 肉类
	IngCategorySeafood   IngredientCategory = "seafood"   // 海鲜
	IngCategoryFruit     IngredientCategory = "fruit"     // 水果
	IngCategoryGrain     IngredientCategory = "grain"     // 谷物
	IngCategoryDairy     IngredientCategory = "dairy"     // 乳制品
	IngCategorySpice     IngredientCategory = "spice"     // 调味料
	IngCategoryOil       IngredientCategory = "oil"       // 油脂
	IngCategoryOther     IngredientCategory = "other"     // 其他
)

// Nutrient 营养成分（每100g）
type Nutrient struct {
	Calories  float64 `json:"calories"`  // 热量(kcal)
	Protein   float64 `json:"protein"`   // 蛋白质(g)
	Fat       float64 `json:"fat"`       // 脂肪(g)
	Carb      float64 `json:"carb"`      // 碳水化合物(g)
	Fiber     float64 `json:"fiber"`     // 膳食纤维(g)
	Sodium    float64 `json:"sodium"`    // 钠(mg)
	VitaminA  float64 `json:"vitamin_a"` // 维生素A(μg)
	VitaminC  float64 `json:"vitamin_c"` // 维生素C(mg)
	Calcium   float64 `json:"calcium"`   // 钙(mg)
	Iron      float64 `json:"iron"`      // 铁(mg)
}

// Ingredient 食材定义
type Ingredient struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Category       IngredientCategory `json:"category"`
	Unit           string             `json:"unit"`            // 默认单位: g, ml, 个, 片等
	NutrientPer100 Nutrient           `json:"nutrient_per_100"` // 每100g营养成分
	ShelfLife      int                `json:"shelf_life"`      // 保质期(天)
	StorageMethod  string             `json:"storage_method"`  // 存储方式: 冷藏/冷冻/常温
	Barcode        string             `json:"barcode,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
}

// InventoryItem 库存项
type InventoryItem struct {
	ID          string    `json:"id"`
	IngredientID string   `json:"ingredient_id"`
	IngredientName string `json:"ingredient_name"`
	Quantity    float64   `json:"quantity"`    // 数量(按食材默认单位)
	Unit        string    `json:"unit"`
	PurchaseDate time.Time `json:"purchase_date"`
	ExpiryDate  time.Time `json:"expiry_date"`
	Location    string    `json:"location"`    // 存储位置: 冰箱冷藏/冰箱冷冻/储物柜
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// InventoryStatus 库存状态
type InventoryStatus string

const (
	InventoryStatusNormal  InventoryStatus = "normal"  // 正常
	InventoryStatusLow     InventoryStatus = "low"     // 库存不足
	InventoryStatusExpired InventoryStatus = "expired" // 已过期
	InventoryStatusExpiringSoon InventoryStatus = "expiring_soon" // 即将过期
)

// RecipeStep 菜谱步骤
type RecipeStep struct {
	Step        int    `json:"step"`        // 步骤序号
	Description string `json:"description"` // 步骤描述
	Duration    int    `json:"duration"`    // 预计时间(分钟)
	Tips        string `json:"tips,omitempty"` // 小贴士
}

// RecipeIngredient 菜谱所需食材
type RecipeIngredient struct {
	IngredientID   string  `json:"ingredient_id"`
	IngredientName string  `json:"ingredient_name"`
	Quantity       float64 `json:"quantity"`
	Unit           string  `json:"unit"`
	Optional       bool    `json:"optional"`        // 是否可选
	Substitute     string  `json:"substitute,omitempty"` // 替代食材ID
}

// Recipe 菜谱
type Recipe struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Category    Category           `json:"category"`
	Difficulty  Difficulty         `json:"difficulty"`
	PrepTime    int                `json:"prep_time"`    // 准备时间(分钟)
	CookTime    int                `json:"cook_time"`    // 烹饪时间(分钟)
	Servings    int                `json:"servings"`     // 份量(人数)
	Ingredients []RecipeIngredient `json:"ingredients"`
	Steps       []RecipeStep       `json:"steps"`
	Tags        []string           `json:"tags,omitempty"`
	ImageURL    string             `json:"image_url,omitempty"`
	Source      string             `json:"source,omitempty"` // 来源
	Rating      float64            `json:"rating"`           // 评分(0-5)
	TimesCooked int                `json:"times_cooked"`     // 烹饪次数
	TotalNutrition Nutrient        `json:"total_nutrition"`  // 整份营养成分
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// MealType 餐次类型
type MealType string

const (
	MealTypeBreakfast MealType = "breakfast"
	MealTypeLunch     MealType = "lunch"
	MealTypeDinner    MealType = "dinner"
	MealTypeSnack     MealType = "snack"
)

// MealPlanItem 膳食计划项
type MealPlanItem struct {
	ID       string   `json:"id"`
	RecipeID string   `json:"recipe_id"`
	RecipeName string `json:"recipe_name"`
	MealType MealType `json:"meal_type"`
	Servings int      `json:"servings"`
	Note     string   `json:"note,omitempty"`
}

// DayPlan 日计划
type DayPlan struct {
	Date  string          `json:"date"` // YYYY-MM-DD
	Meals []MealPlanItem  `json:"meals"`
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

// ShoppingItem 购物清单项
type ShoppingItem struct {
	IngredientID   string  `json:"ingredient_id"`
	IngredientName string  `json:"ingredient_name"`
	Quantity       float64 `json:"quantity"`
	Unit           string  `json:"unit"`
	Reason         string  `json:"reason"`         // 需要原因: 缺少/不足
	RecipeNames    []string `json:"recipe_names"`   // 来自哪些菜谱
	Checked        bool    `json:"checked"`         // 是否已购买
	EstimatedPrice float64 `json:"estimated_price,omitempty"` // 预估价格
}

// ShoppingList 购物清单
type ShoppingList struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Items     []ShoppingItem `json:"items"`
	PlanID    string         `json:"plan_id,omitempty"` // 关联膳食计划
	TotalEst  float64        `json:"total_estimated"`   // 预估总价
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// NutritionSummary 营养摘要
type NutritionSummary struct {
	TotalCalories float64 `json:"total_calories"`
	TotalProtein  float64 `json:"total_protein"`
	TotalFat      float64 `json:"total_fat"`
	TotalCarb     float64 `json:"total_carb"`
	TotalFiber    float64 `json:"total_fiber"`
	AvgCalories   float64 `json:"avg_calories"` // 日均热量
	MealCount     int     `json:"meal_count"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
}

// RecommendationRequest 推荐请求
type RecommendationRequest struct {
	AvailableIngredients []string `json:"available_ingredients,omitempty"` // 食材ID列表
	MaxPrepTime          int      `json:"max_prep_time,omitempty"`        // 最大准备时间
	MaxCookTime          int      `json:"max_cook_time,omitempty"`        // 最大烹饪时间
	PreferredDifficulty  Difficulty `json:"preferred_difficulty,omitempty"`
	PreferredCategory    Category   `json:"preferred_category,omitempty"`
	MaxCalories          float64    `json:"max_calories,omitempty"`
	ExcludeTags          []string   `json:"exclude_tags,omitempty"`
	Limit                int        `json:"limit"` // 返回数量限制
}

// RecommendationResult 推荐结果
type RecommendationResult struct {
	Recipe         Recipe  `json:"recipe"`
	MatchScore     float64 `json:"match_score"`     // 匹配度(0-100)
	MatchedIngredients int `json:"matched_ingredients"` // 匹配的食材数
	MissingIngredients int `json:"missing_ingredients"` // 缺少的食材数
}
