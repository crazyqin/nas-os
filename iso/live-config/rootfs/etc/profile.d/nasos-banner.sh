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
fi
