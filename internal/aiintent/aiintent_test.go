package aiintent

import (
	"context"
	"testing"
)

func TestEngine_ParseIntent_StorageCreate(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intent, err := engine.ParseIntent(ctx, "创建一个新的存储池，容量10TB")
	if err != nil {
		t.Fatalf("解析意图失败: %v", err)
	}

	if intent.Type != IntentStorageCreate {
		t.Errorf("期望类型 %s, 实际 %s", IntentStorageCreate, intent.Type)
	}
	if intent.Confidence == 0 {
		t.Error("置信度不应为0")
	}
	if intent.Parameters["capacity"] != "10TB" {
		t.Errorf("期望容量 10TB, 实际 %s", intent.Parameters["capacity"])
	}
}

func TestEngine_ParseIntent_BackupSchedule(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intent, err := engine.ParseIntent(ctx, "设置每天自动备份 /data 目录")
	if err != nil {
		t.Fatalf("解析意图失败: %v", err)
	}

	if intent.Type != IntentBackupSchedule {
		t.Errorf("期望类型 %s, 实际 %s", IntentBackupSchedule, intent.Type)
	}
	if intent.Parameters["frequency"] != "daily" {
		t.Errorf("期望频率 daily, 实际 %s", intent.Parameters["frequency"])
	}
	if intent.Parameters["path"] != "/data" {
		t.Errorf("期望路径 /data, 实际 %s", intent.Parameters["path"])
	}
}

func TestEngine_ParseIntent_PerformanceTune(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intent, err := engine.ParseIntent(ctx, "优化存储性能，加速读写速度")
	if err != nil {
		t.Fatalf("解析意图失败: %v", err)
	}

	if intent.Type != IntentPerformanceTune {
		t.Errorf("期望类型 %s, 实际 %s", IntentPerformanceTune, intent.Type)
	}
}

func TestEngine_ParseIntent_EmptyText(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	_, err := engine.ParseIntent(ctx, "")
	if err == nil {
		t.Error("空文本应返回错误")
	}
}

func TestEngine_ParseIntent_Unknown(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intent, err := engine.ParseIntent(ctx, "今天天气不错")
	if err != nil {
		t.Fatalf("解析意图失败: %v", err)
	}

	if intent.Type != IntentUnknown {
		t.Errorf("期望类型 %s, 实际 %s", IntentUnknown, intent.Type)
	}
}

func TestEngine_ExecuteIntent(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	// 注册处理器
	engine.RegisterHandler(IntentStorageCreate, func(ctx context.Context, intent *Intent) error {
		intent.Result = "存储池创建成功"
		return nil
	})

	intent, _ := engine.ParseIntent(ctx, "创建一个新的存储池，容量5TB")
	result, err := engine.ExecuteIntent(ctx, intent.ID)
	if err != nil {
		t.Fatalf("执行意图失败: %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("期望状态 %s, 实际 %s", StatusCompleted, result.Status)
	}
}

func TestEngine_ExecuteIntent_NotFound(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	_, err := engine.ExecuteIntent(ctx, "non-existent")
	if err == nil {
		t.Error("不存在的意图应返回错误")
	}
}

func TestEngine_ExecuteIntent_NoHandler(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intent, _ := engine.ParseIntent(ctx, "创建一个新的存储池")
	_, err := engine.ExecuteIntent(ctx, intent.ID)
	if err == nil {
		t.Error("无处理器应返回错误")
	}
}

func TestEngine_GetIntent(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intent, _ := engine.ParseIntent(ctx, "查看存储状态")
	fetched, err := engine.GetIntent(intent.ID)
	if err != nil {
		t.Fatalf("获取意图失败: %v", err)
	}
	if fetched.ID != intent.ID {
		t.Errorf("ID不匹配: %s != %s", fetched.ID, intent.ID)
	}
}

func TestEngine_GetIntent_NotFound(t *testing.T) {
	engine := NewEngine()

	_, err := engine.GetIntent("non-existent")
	if err == nil {
		t.Error("不存在的意图应返回错误")
	}
}

func TestEngine_ListIntents(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	engine.ParseIntent(ctx, "创建存储池")
	engine.ParseIntent(ctx, "备份数据")
	engine.ParseIntent(ctx, "优化性能")

	intents := engine.ListIntents(0)
	if len(intents) != 3 {
		t.Errorf("期望3个意图, 实际 %d", len(intents))
	}

	limited := engine.ListIntents(2)
	if len(limited) != 2 {
		t.Errorf("期望2个意图, 实际 %d", len(limited))
	}
}

func TestEngine_CancelIntent(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intent, _ := engine.ParseIntent(ctx, "创建存储池")
	err := engine.CancelIntent(intent.ID)
	if err != nil {
		t.Fatalf("取消意图失败: %v", err)
	}

	fetched, _ := engine.GetIntent(intent.ID)
	if fetched.Status != StatusCancelled {
		t.Errorf("期望状态 %s, 实际 %s", StatusCancelled, fetched.Status)
	}
}

func TestEngine_CancelIntent_AlreadyCompleted(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	engine.RegisterHandler(IntentStorageCreate, func(ctx context.Context, intent *Intent) error {
		return nil
	})

	intent, _ := engine.ParseIntent(ctx, "创建存储池")
	engine.ExecuteIntent(ctx, intent.ID)

	err := engine.CancelIntent(intent.ID)
	if err == nil {
		t.Error("已完成的意图不应能取消")
	}
}

func TestEngine_GetStats(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	engine.ParseIntent(ctx, "创建存储池")
	engine.ParseIntent(ctx, "备份数据")

	stats := engine.GetStats()
	if stats["total_intents"] != 2 {
		t.Errorf("期望2个意图, 实际 %v", stats["total_intents"])
	}
	if stats["total_parsed"] != 2 {
		t.Errorf("期望解析2次, 实际 %v", stats["total_parsed"])
	}
}

func TestEngine_RegisterHandler(t *testing.T) {
	engine := NewEngine()

	called := false
	engine.RegisterHandler(IntentQuery, func(ctx context.Context, intent *Intent) error {
		called = true
		return nil
	})

	ctx := context.Background()
	intent, _ := engine.ParseIntent(ctx, "查看存储状态")
	engine.ExecuteIntent(ctx, intent.ID)

	if !called {
		t.Error("处理器未被调用")
	}
}

func TestEngine_MultipleIntents(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	engine.RegisterHandler(IntentStorageCreate, func(ctx context.Context, intent *Intent) error {
		return nil
	})
	engine.RegisterHandler(IntentBackupSchedule, func(ctx context.Context, intent *Intent) error {
		return nil
	})

	intents := []string{
		"创建一个10TB的存储池",
		"设置每天备份",
		"优化存储性能",
	}

	for _, text := range intents {
		intent, err := engine.ParseIntent(ctx, text)
		if err != nil {
			t.Fatalf("解析意图失败: %v", err)
		}
		if intent.Type == IntentUnknown {
			t.Errorf("未能识别意图: %s", text)
		}
	}

	stats := engine.GetStats()
	if stats["total_parsed"] != 3 {
		t.Errorf("期望解析3次, 实际 %v", stats["total_parsed"])
	}
}

func TestEngine_ExtractParameters_Capacity(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intent, _ := engine.ParseIntent(ctx, "扩容存储到50TB")
	if intent.Parameters["capacity"] != "50TB" {
		t.Errorf("期望容量 50TB, 实际 %s", intent.Parameters["capacity"])
	}
}

func TestEngine_ExtractParameters_Path(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	intent, _ := engine.ParseIntent(ctx, "备份 /home/data 目录")
	if intent.Parameters["path"] != "/home/data" {
		t.Errorf("期望路径 /home/data, 实际 %s", intent.Parameters["path"])
	}
}
