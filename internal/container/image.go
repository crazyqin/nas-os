package container

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Image 镜像信息
type Image struct {
	ID          string    `json:"id"`
	Repository  string    `json:"repository"`
	Tag         string    `json:"tag"`
	FullName    string    `json:"fullName"`
	Size        uint64    `json:"size"`
	SizeHuman   string    `json:"sizeHuman"`
	Created     time.Time `json:"created"`
	Containers  int       `json:"containers"` // 使用该镜像的容器数
	Labels      map[string]string `json:"labels"`
	Architecture string   `json:"architecture"`
	OS          string    `json:"os"`
}

// ImagePullProgress 镜像拉取进度
type ImagePullProgress struct {
	Status         string `json:"status"`
	ProgressDetail struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"progressDetail"`
	Progress string `json:"progress"`
	ID       string `json:"id"`
}

// ImageConfig 镜像创建配置
type ImageConfig struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag,omitempty"`
	Platform   string `json:"platform,omitempty"` // "linux/amd64", "linux/arm64"
}

// ImageManager 镜像管理器
type ImageManager struct {
	manager *Manager
}

// NewImageManager 创建镜像管理器
func NewImageManager(mgr *Manager) *ImageManager {
	return &ImageManager{
		manager: mgr,
	}
}

// ListImages 列出所有镜像
func (im *ImageManager) ListImages() ([]*Image, error) {
	cmd := exec.Command("docker", "images", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出镜像：%w", err)
	}

	var images []*Image
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		var raw struct {
			ID         string `json:"ID"`
			Repository string `json:"Repository"`
			Tag        string `json:"Tag"`
			Size       string `json:"Size"`
			CreatedAt  string `json:"CreatedAt"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		// 获取详细信息
		image, err := im.GetImage(raw.ID)
		if err != nil {
			// 如果无法获取详细信息，使用基本信息
			image = &Image{
				ID:         raw.ID,
				Repository: raw.Repository,
				Tag:        raw.Tag,
				FullName:   fmt.Sprintf("%s:%s", raw.Repository, raw.Tag),
				Size:       parseSize(raw.Size),
				SizeHuman:  raw.Size,
			}
		}

		images = append(images, image)
	}

	return images, nil
}

// GetImage 获取镜像详情
func (im *ImageManager) GetImage(id string) (*Image, error) {
	cmd := exec.Command("docker", "inspect", "--type", "image", "--format", "{{json .}}", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取镜像信息：%w", err)
	}

	var raw struct {
		ID          string    `json:"Id"`
		RepoTags    []string  `json:"RepoTags"`
		Size        uint64    `json:"Size"`
		Created     time.Time `json:"Created"`
		Architecture string  `json:"Architecture"`
		OS          string    `json:"Os"`
		Config      struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	image := &Image{
		ID:           raw.ID,
		Size:         raw.Size,
		SizeHuman:    formatSize(raw.Size),
		Created:      raw.Created,
		Architecture: raw.Architecture,
		OS:           raw.OS,
		Labels:       raw.Config.Labels,
	}

	// 解析仓库标签
	if len(raw.RepoTags) > 0 && raw.RepoTags[0] != "<none>:<none>" {
		parts := strings.Split(raw.RepoTags[0], ":")
		if len(parts) >= 2 {
			image.Repository = strings.Join(parts[:len(parts)-1], ":")
			image.Tag = parts[len(parts)-1]
			image.FullName = raw.RepoTags[0]
		}
	} else {
		image.Repository = "<none>"
		image.Tag = "<none>"
		image.FullName = "<none>"
	}

	// 统计使用该镜像的容器数
	containers, err := im.manager.ListContainers(true)
	if err == nil {
		count := 0
		for _, c := range containers {
			if c.Image == image.FullName || strings.HasPrefix(c.Image, image.ID[:12]) {
				count++
			}
		}
		image.Containers = count
	}

	return image, nil
}

// PullImage 拉取镜像
func (im *ImageManager) PullImage(config *ImageConfig) error {
	image := config.Repository
	if config.Tag != "" {
		image += ":" + config.Tag
	}

	args := []string{"pull", image}
	if config.Platform != "" {
		args = append(args, "--platform", config.Platform)
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("拉取镜像失败：%w, %s", err, string(output))
	}
	return nil
}

// PushImage 推送镜像到仓库
func (im *ImageManager) PushImage(image string) error {
	cmd := exec.Command("docker", "push", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("推送镜像失败：%w, %s", err, string(output))
	}
	return nil
}

// RemoveImage 删除镜像
func (im *ImageManager) RemoveImage(id string, force bool, prune bool) error {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	if prune {
		args = append(args, "--prune")
	}
	args = append(args, id)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除镜像失败：%w, %s", err, string(output))
	}
	return nil
}

// TagImage 给镜像打标签
func (im *ImageManager) TagImage(source, target string) error {
	cmd := exec.Command("docker", "tag", source, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("标记镜像失败：%w, %s", err, string(output))
	}
	return nil
}

// BuildImage 构建镜像
func (im *ImageManager) BuildImage(contextPath, dockerfilePath, tag string, args map[string]string) error {
	buildArgs := []string{"build", "-t", tag}
	if dockerfilePath != "" {
		buildArgs = append(buildArgs, "-f", dockerfilePath)
	}
	for k, v := range args {
		buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	buildArgs = append(buildArgs, contextPath)

	cmd := exec.Command("docker", buildArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("构建镜像失败：%w, %s", err, string(output))
	}
	return nil
}

// SaveImage 保存镜像到文件
func (im *ImageManager) SaveImage(image, outputPath string) error {
	cmd := exec.Command("docker", "save", "-o", outputPath, image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("保存镜像失败：%w, %s", err, string(output))
	}
	return nil
}

// LoadImage 从文件加载镜像
func (im *ImageManager) LoadImage(inputPath string) error {
	cmd := exec.Command("docker", "load", "-i", inputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("加载镜像失败：%w, %s", err, string(output))
	}
	return nil
}

// PruneImages 清理悬空镜像
func (im *ImageManager) PruneImages(all bool) (uint64, error) {
	args := []string{"image", "prune", "-f"}
	if all {
		args = append(args, "-a")
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("清理镜像失败：%w", err)
	}

	// 解析回收的空间
	var reclaimed uint64
	outputStr := string(output)
	if strings.Contains(outputStr, "Total reclaimed space:") {
		parts := strings.Split(outputStr, "Total reclaimed space:")
		if len(parts) > 1 {
			sizeStr := strings.TrimSpace(parts[1])
			reclaimed = parseSize(sizeStr)
		}
	}

	return reclaimed, nil
}

// SearchImages 搜索 Docker Hub 镜像
func (im *ImageManager) SearchImages(term string, limit int) ([]*Image, error) {
	cmd := exec.Command("docker", "search", "--format", "{{json .}}", "--limit", fmt.Sprintf("%d", limit), term)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("搜索镜像失败：%w", err)
	}

	var images []*Image
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		var raw struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			StarCount   int    `json:"star_count"`
			IsOfficial  bool   `json:"is_official"`
			IsAutomated bool   `json:"is_automated"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		images = append(images, &Image{
			Repository: raw.Name,
			Tag:        "latest",
			FullName:   raw.Name,
			Labels: map[string]string{
				"description":   raw.Description,
				"star_count":    fmt.Sprintf("%d", raw.StarCount),
				"is_official":   fmt.Sprintf("%t", raw.IsOfficial),
				"is_automated":  fmt.Sprintf("%t", raw.IsAutomated),
			},
		})
	}

	return images, nil
}

// formatSize 格式化大小
func formatSize(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
