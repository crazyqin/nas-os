# NAS-OS 控制台横幅：每次交互式登录显示访问入口
# live 模式提示安装命令；已安装模式提示 WebUI 与初始密码位置
if [ -n "${PS1:-}" ] && [ "$(id -u)" = 0 ] && [ -t 1 ] 2>/dev/null; then
  MODE="已安装系统"
  [ -d /run/live ] && MODE="live 试用模式（断电即失，未写入磁盘）"
  IP=$(ip -4 -o addr show scope global 2>/dev/null | awk '{print $2": "$4}' | cut -d/ -f1 | paste -sd'  ' -)
  [ -n "$IP" ] || IP="（网络获取中，稍候可用 ip a 查看）"
  PWFILE=""
  for f in /etc/nas-os/.admin_password /var/lib/nas-os/.admin_password; do
    [ -s "$f" ] && PWFILE="$f"
  done
  echo
  echo "  ============================================================"
  echo "   NAS-OS — $MODE"
  echo "  ------------------------------------------------------------"
  echo "   Web 管理界面 : http://<本机 IP>:8080   本机地址: $IP"
  if [ -n "$PWFILE" ]; then
    echo "   初始管理员   : 用户名 admin，密码: cat $PWFILE"
  else
    echo "   初始管理员   : 用户名 admin（密码在首次启动后生成于"
    echo "                   /etc/nas-os/.admin_password）"
  fi
  if [ -d /run/live ]; then
    echo "   安装到硬盘   : 运行 nasos-install"
  fi
  echo "   主机名       : $(hostname)（局域网可试 http://nasos.local:8080）"
  echo "  ============================================================"
  echo

  # 后台探针（非交互，不改系统状态）：把 nasd 监听状态回报到当前控制
  # 台，供冒烟串口与真机排障观测——就绪打一行；10 分钟仍未监听则将
  # nas-os 单元状态与最近日志转储出来（否则这些只进 journal，串口
  # 冒烟里无从诊断）。依赖 bash 的 /dev/tcp，不适用时静默退出。
  (
    i=0
    while [ "$i" -lt 60 ]; do
      if (exec 3<>/dev/tcp/127.0.0.1/8080) 2>/dev/null; then
        echo "  [nasd] WebUI 已就绪：8080 端口可连（$(hostname) $(date '+%H:%M:%S')）"
        exit 0
      fi
      i=$((i+1))
      sleep 10
    done
    echo "  [nasd] 警告：10 分钟仍未监听 8080，诊断转储："
    systemctl status nas-os --no-pager -l 2>&1 | sed 's/^/  | /'
    journalctl -u nas-os -n 20 --no-pager 2>&1 | sed 's/^/  | /'
  ) &
fi
