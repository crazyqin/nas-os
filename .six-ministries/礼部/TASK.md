# 礼部任务：WebShare 与用户体验

## 目标
对标 TrueNAS 26 WebShare + TrueSearch，提升用户界面体验

## 任务清单
1. **WebShare 文件浏览器**
   - 纯浏览器文件访问（无需 SMB/NFS 客户端）
   - 文件上传/下载/管理
   - 文件夹创建、过滤
   - 快照时间线查看
   - 可分享链接生成
   - 隐藏文件切换

2. **TrueSearch 搜索增强**
   - 文件名搜索
   - 文件内容搜索
   - 文件类型过滤
   - 加密数据集排除索引

3. **认证体验优化**
   - Passkey 认证支持
   - 多因素认证流程优化

## 交付物
- `webui/src/pages/webshare/` - WebShare 前端
- `internal/web/webshare/` - WebShare 后端
- `internal/search/truesearch.go` - 搜索增强
- 用户文档

## 竞品参考
- TrueNAS 26 WebShare with TrueSearch
- Synology Photos UI
- Dropbox Web UI

## 负责人
礼部尚书

## 截止
本轮开发周期结束前