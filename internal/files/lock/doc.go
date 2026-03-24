// Package lock 提供文件锁定机制，参考群晖 DSM Drive 实现
//
// 功能特性：
//   - 支持独占锁（写锁）和共享锁（读锁）
//   - 多用户协作锁协议
//   - 锁冲突检测与解决
//   - 锁自动续期与过期清理
//   - 审计日志完整记录
//   - SMB/NFS/WebDAV 协议适配器
//
// 使用示例：
//
//	manager := lock.NewManager(lock.DefaultConfig(), logger)
//	defer manager.Close()
//
//	// 获取独占锁
//	lock, conflict, err := manager.Lock(&lock.LockRequest{
//	    FilePath: "/data/important.doc",
//	    LockType: lock.LockTypeExclusive,
//	    Owner:    "user1",
//	    OwnerName: "张三",
//	})
//
//	// 释放锁
//	err = manager.Unlock(lock.ID, "user1")
package lock