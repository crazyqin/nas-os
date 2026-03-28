#!/bin/bash
# nas-os Ollama 一键安装脚本
# 支持 NVIDIA/Intel/AMD GPU 和 CPU 模式

set -e

echo "🤖 nas-os Ollama 一键部署脚本"
echo "================================"

# 检测GPU类型
detect_gpu() {
    echo "检测GPU类型..."
    
    # NVIDIA
    if command -v nvidia-smi &> /dev/null; then
        echo "✅ 发现 NVIDIA GPU"
        nvidia-smi --query-gpu=name,memory.total --format=csv,noheader
        return "nvidia"
    fi
    
    # Intel (检查i915驱动)
    if ls /dev/dri/card* 2>/dev/null | grep -q .; then
        if lspci | grep -qi "intel.*graphics"; then
            echo "✅ 发现 Intel GPU"
            lspci | grep -i "vga" | grep -i "intel"
            return "intel"
        fi
    fi
    
    # AMD (检查amdgpu驱动)
    if ls /dev/dri/card* 2>/dev/null | grep -q .; then
        if lspci | grep -qi "amd.*vga"; then
            echo "✅ 发现 AMD GPU"
            lspci | grep -i "vga" | grep -i "amd"
            return "amd"
        fi
    fi
    
    echo "⚠️ 未发现GPU，使用CPU模式"
    return "cpu"
}

# 安装NVIDIA容器工具包
install_nvidia_container_toolkit() {
    echo "安装 NVIDIA Container Toolkit..."
    
    if command -v apt-get &> /dev/null; then
        curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
            gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
        
        curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
            sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
            tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
        
        apt-get update
        apt-get install -y nvidia-container-toolkit
        
        nvidia-ctk runtime configure --runtime=docker
        systemctl restart docker
    fi
}

# 安装Ollama
install_ollama() {
    echo "安装 Ollama..."
    
    # 使用官方安装脚本
    curl -fsSL https://ollama.com/install.sh | sh
    
    # 或使用Docker
    # docker pull ollama/ollama:latest
}

# 配置Ollama服务
configure_ollama() {
    local gpu_type=$1
    
    echo "配置 Ollama 服务..."
    
    # 创建systemd服务配置
    mkdir -p /etc/systemd/system/ollama.service.d
    
    case $gpu_type in
        nvidia)
            cat > /etc/systemd/system/ollama.service.d/gpu.conf << EOF
[Service]
Environment="OLLAMA_GPU_TYPE=nvidia"
EOF
            ;;
        intel)
            cat > /etc/systemd/system/ollama.service.d/gpu.conf << EOF
[Service]
Environment="OLLAMA_GPU_TYPE=intel"
Environment="OLLAMA_INTEL_GPU=true"
EOF
            ;;
        amd)
            cat > /etc/systemd/system/ollama.service.d/gpu.conf << EOF
[Service]
Environment="OLLAMA_GPU_TYPE=amd"
EOF
            ;;
        cpu)
            cat > /etc/systemd/system/ollama.service.d/gpu.conf << EOF
[Service]
Environment="OLLAMA_GPU_TYPE=cpu"
EOF
            ;;
    esac
    
    systemctl daemon-reload
    systemctl enable ollama
    systemctl start ollama
}

# 下载推荐模型
download_models() {
    echo "下载推荐模型..."
    
    local models=(
        "llama3.2"
        "nomic-embed-text"
    )
    
    for model in "${models[@]}"; do
        echo "下载模型: $model"
        ollama pull "$model"
    done
}

# 配置nas-os集成
configure_nas_os() {
    echo "配置 nas-os Ollama 集成..."
    
    # 创建配置文件
    mkdir -p /etc/nas-os
    
    cat > /etc/nas-os/ai-config.yaml << EOF
ollama:
  url: http://localhost:11434
  gpu_type: ${OLLAMA_GPU_TYPE:-cpu}
  default_model: llama3.2
  embedding_model: nomic-embed-text

face_recognition:
  enabled: true
  model_path: /opt/nas-os/models/face_detection
  gpu_acceleration: ${OLLAMA_GPU_TYPE:-cpu}

embedding:
  provider: ollama
  model: nomic-embed-text
  dimension: 768

vector_db:
  type: qdrant
  url: http://localhost:6333
EOF
    
    echo "✅ 配置文件已创建: /etc/nas-os/ai-config.yaml"
}

# 显示状态
show_status() {
    echo ""
    echo "================================"
    echo "🎉 Ollama 安装完成!"
    echo ""
    echo "Ollama 状态:"
    systemctl status ollama --no-pager
    
    echo ""
    echo "可用模型:"
    ollama list
    
    echo ""
    echo "测试命令:"
    echo "  ollama run llama3.2 '你好'"
    echo ""
    echo "nas-os AI配置:"
    echo "  配置文件: /etc/nas-os/ai-config.yaml"
    echo ""
    echo "================================"
}

# 主流程
main() {
    GPU_TYPE=$(detect_gpu)
    
    case $GPU_TYPE in
        nvidia)
            install_nvidia_container_toolkit
            ;;
        intel)
            echo "Intel GPU 配置..."
            # Intel核显需要额外配置
            ;;
        amd)
            echo "AMD GPU 配置..."
            ;;
    esac
    
    install_ollama
    configure_ollama "$GPU_TYPE"
    
    # 等待Ollama启动
    sleep 5
    
    download_models
    configure_nas_os
    show_status
}

# 执行
main