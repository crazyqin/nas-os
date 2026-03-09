package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"nas-os/internal/nfs"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/users"
	"nas-os/internal/web"
)

func main() {
	log.Println("🚀 NAS-OS 启动中...")

	// 初始化用户管理
	userMgr, err := users.NewManager("/mnt")
	if err != nil {
		log.Fatalf("用户管理初始化失败：%v", err)
	}
	log.Println("✅ 用户管理模块就绪")

	// 初始化存储管理
	storMgr, err := storage.NewManager()
	if err != nil {
		log.Fatalf("存储管理初始化失败：%v", err)
	}
	log.Println("✅ 存储管理模块就绪")

	// 初始化 SMB 共享
	smbMgr, err := smb.NewManager(userMgr, "/etc/samba/smb.conf")
	if err != nil {
		log.Fatalf("SMB 管理初始化失败：%v", err)
	}
	log.Println("✅ SMB 共享模块就绪")

	// 初始化 NFS 共享
	nfsMgr, err := nfs.NewManager("/etc/exports")
	if err != nil {
		log.Fatalf("NFS 管理初始化失败：%v", err)
	}
	log.Println("✅ NFS 共享模块就绪")

	// 初始化 Web 服务
	webServer := web.NewServer(storMgr, userMgr, smbMgr, nfsMgr)

	// 启动 Web 服务
	go func() {
		if err := webServer.Start(":8080"); err != nil {
			log.Fatalf("Web 服务启动失败：%v", err)
		}
	}()

	log.Println("✅ NAS-OS 就绪 - Web 管理界面：http://localhost:8080")

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("👋 NAS-OS 正在关闭...")
	webServer.Stop()
}
