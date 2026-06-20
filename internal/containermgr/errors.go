package containermgr

import "errors"

// ========== 通用错误 ==========

var (
	// ErrContainerNotFound 容器未找到.
	ErrContainerNotFound = errors.New("容器未找到")
	// ErrContainerAlreadyRunning 容器已在运行.
	ErrContainerAlreadyRunning = errors.New("容器已在运行")
	// ErrContainerNotRunning 容器未运行.
	ErrContainerNotRunning = errors.New("容器未运行")
	// ErrContainerCreateFailed 创建容器失败.
	ErrContainerCreateFailed = errors.New("创建容器失败")
	// ErrContainerStartFailed 启动容器失败.
	ErrContainerStartFailed = errors.New("启动容器失败")
	// ErrContainerStopFailed 停止容器失败.
	ErrContainerStopFailed = errors.New("停止容器失败")
	// ErrContainerRestartFailed 重启容器失败.
	ErrContainerRestartFailed = errors.New("重启容器失败")
	// ErrContainerRemoveFailed 删除容器失败.
	ErrContainerRemoveFailed = errors.New("删除容器失败")
)

// ========== 镜像错误 ==========

var (
	// ErrImageNotFound 镜像未找到.
	ErrImageNotFound = errors.New("镜像未找到")
	// ErrImagePullFailed 拉取镜像失败.
	ErrImagePullFailed = errors.New("拉取镜像失败")
	// ErrImageBuildFailed 构建镜像失败.
	ErrImageBuildFailed = errors.New("构建镜像失败")
	// ErrImageRemoveFailed 删除镜像失败.
	ErrImageRemoveFailed = errors.New("删除镜像失败")
	// ErrImageTagFailed 镜像标签操作失败.
	ErrImageTagFailed = errors.New("镜像标签操作失败")
	// ErrImageInUse 镜像正在使用中.
	ErrImageInUse = errors.New("镜像正在使用中")
)

// ========== 网络错误 ==========

var (
	// ErrNetworkNotFound 网络未找到.
	ErrNetworkNotFound = errors.New("网络未找到")
	// ErrNetworkCreateFailed 创建网络失败.
	ErrNetworkCreateFailed = errors.New("创建网络失败")
	// ErrNetworkRemoveFailed 删除网络失败.
	ErrNetworkRemoveFailed = errors.New("删除网络失败")
	// ErrNetworkConnectFailed 连接网络失败.
	ErrNetworkConnectFailed = errors.New("连接网络失败")
	// ErrNetworkDisconnectFailed 断开网络失败.
	ErrNetworkDisconnectFailed = errors.New("断开网络失败")
	// ErrNetworkNameConflict 网络名称冲突.
	ErrNetworkNameConflict = errors.New("网络名称冲突")
	// ErrNetworkInUse 网络正在使用中.
	ErrNetworkInUse = errors.New("网络正在使用中")
)

// ========== 卷错误 ==========

var (
	// ErrVolumeNotFound 卷未找到.
	ErrVolumeNotFound = errors.New("卷未找到")
	// ErrVolumeCreateFailed 创建卷失败.
	ErrVolumeCreateFailed = errors.New("创建卷失败")
	// ErrVolumeRemoveFailed 删除卷失败.
	ErrVolumeRemoveFailed = errors.New("删除卷失败")
	// ErrVolumeInUse 卷正在使用中.
	ErrVolumeInUse = errors.New("卷正在使用中")
	// ErrVolumeBackupFailed 备份卷失败.
	ErrVolumeBackupFailed = errors.New("备份卷失败")
	// ErrVolumeRestoreFailed 恢复卷失败.
	ErrVolumeRestoreFailed = errors.New("恢复卷失败")
)

// ========== 运行时错误 ==========

var (
	// ErrRuntimeNotSupported 运行时不支持.
	ErrRuntimeNotSupported = errors.New("运行时不支持")
	// ErrRuntimeNotAvailable 运行时不可用.
	ErrRuntimeNotAvailable = errors.New("运行时不可用")
	// ErrRuntimeStartFailed 启动运行时失败.
	ErrRuntimeStartFailed = errors.New("启动运行时失败")
)

// ========== Compose 错误 ==========

var (
	// ErrComposeProjectNotFound Compose 项目未找到.
	ErrComposeProjectNotFound = errors.New("Compose 项目未找到")
	// ErrComposeUpFailed Compose Up 失败.
	ErrComposeUpFailed = errors.New("Compose Up 失败")
	// ErrComposeDownFailed Compose Down 失败.
	ErrComposeDownFailed = errors.New("Compose Down 失败")
	// ErrComposeConfigInvalid Compose 配置无效.
	ErrComposeConfigInvalid = errors.New("Compose 配置无效")
	// ErrComposeTemplateNotFound Compose 模板未找到.
	ErrComposeTemplateNotFound = errors.New("Compose 模板未找到")
	// ErrComposeScaleFailed Compose 扩缩容失败.
	ErrComposeScaleFailed = errors.New("Compose 扩缩容失败")
)

// ========== 资源监控错误 ==========

var (
	// ErrStatsFailed 获取统计信息失败.
	ErrStatsFailed = errors.New("获取统计信息失败")
	// ErrMonitorFailed 监控失败.
	ErrMonitorFailed = errors.New("监控失败")
)

// ========== 参数错误 ==========

var (
	// ErrInvalidConfig 无效配置.
	ErrInvalidConfig = errors.New("无效配置")
	// ErrInvalidName 无效名称.
	ErrInvalidName = errors.New("无效名称")
	// ErrInvalidImage 无效镜像.
	ErrInvalidImage = errors.New("无效镜像")
	// ErrInvalidNetwork 无效网络.
	ErrInvalidNetwork = errors.New("无效网络")
	// ErrInvalidVolume 无效卷.
	ErrInvalidVolume = errors.New("无效卷")
	// ErrNameRequired 名称必填.
	ErrNameRequired = errors.New("名称必填")
	// ErrImageRequired 镜像必填.
	ErrImageRequired = errors.New("镜像必填")
)
