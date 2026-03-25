package lxc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ImageManager handles LXC image operations.
type ImageManager struct {
	manager *Manager
}

// NewImageManager creates a new ImageManager.
func NewImageManager(manager *Manager) *ImageManager {
	return &ImageManager{manager: manager}
}

// ListImages lists all available images.
func (i *ImageManager) ListImages(ctx context.Context) ([]*Image, error) {
	cmd := i.manager.cmd("image", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	var raw []struct {
		Fingerprint  string            `json:"fingerprint"`
		Filename     string            `json:"filename"`
		Size         uint64            `json:"size"`
		Architecture string            `json:"architecture"`
		CreatedAt    time.Time         `json:"created_at"`
		UploadedAt   time.Time         `json:"uploaded_at"`
		Properties   map[string]string `json:"properties"`
		Public       bool              `json:"public"`
		AutoUpdate   bool              `json:"auto_update"`
		Aliases      []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"aliases"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse image list: %w", err)
	}

	var images []*Image
	for _, r := range raw {
		image := &Image{
			ID:         r.Fingerprint[:12],
			Name:       r.Filename,
			OS:         r.Properties["os"],
			Release:    r.Properties["release"],
			Arch:       r.Architecture,
			Size:       r.Size / (1024 * 1024), // Convert to MB
			CreatedAt:  r.CreatedAt,
			Properties: r.Properties,
		}

		for _, alias := range r.Aliases {
			image.Aliases = append(image.Aliases, alias.Name)
			if image.Name == "" {
				image.Name = alias.Name
			}
		}

		images = append(images, image)
	}

	return images, nil
}

// GetImage retrieves a specific image by fingerprint or alias.
func (i *ImageManager) GetImage(ctx context.Context, imageRef string) (*Image, error) {
	cmd := i.manager.cmd("image", "show", imageRef, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get image %s: %w", imageRef, err)
	}

	var raw struct {
		Fingerprint  string            `json:"fingerprint"`
		Filename     string            `json:"filename"`
		Size         uint64            `json:"size"`
		Architecture string            `json:"architecture"`
		CreatedAt    time.Time         `json:"created_at"`
		UploadedAt   time.Time         `json:"uploaded_at"`
		Properties   map[string]string `json:"properties"`
		Aliases      []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"aliases"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse image info: %w", err)
	}

	image := &Image{
		ID:         raw.Fingerprint[:12],
		Name:       raw.Filename,
		OS:         raw.Properties["os"],
		Release:    raw.Properties["release"],
		Arch:       raw.Architecture,
		Size:       raw.Size / (1024 * 1024),
		CreatedAt:  raw.CreatedAt,
		Properties: raw.Properties,
	}

	for _, alias := range raw.Aliases {
		image.Aliases = append(image.Aliases, alias.Name)
	}

	return image, nil
}

// CopyImage copies an image from a remote server.
func (i *ImageManager) CopyImage(ctx context.Context, remote, image string, autoUpdate bool, aliases []string) (*Image, error) {
	args := []string{"image", "copy", fmt.Sprintf("%s:%s", remote, image), "local:"}

	if autoUpdate {
		args = append(args, "--auto-update")
	}

	for _, alias := range aliases {
		args = append(args, "--alias", alias)
	}

	// Add public flag
	args = append(args, "--public")

	cmd := i.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to copy image: %w, output: %s", err, string(output))
	}

	// Return the copied image
	if len(aliases) > 0 {
		return i.GetImage(ctx, aliases[0])
	}
	return nil, nil
}

// PullImage pulls an image from a remote server (alias for CopyImage).
func (i *ImageManager) PullImage(ctx context.Context, imageRef string, autoUpdate bool) (*Image, error) {
	// Parse image reference
	server, image, alias, err := ParseImageRef(imageRef)
	if err != nil {
		return nil, err
	}

	if server == "" {
		server = "images" // Default to images.linuxcontainers.org
	}

	aliases := []string{alias}
	if image != alias {
		aliases = append(aliases, image)
	}

	return i.CopyImage(ctx, server, image, autoUpdate, aliases)
}

// DeleteImage deletes an image.
func (i *ImageManager) DeleteImage(ctx context.Context, imageRef string) error {
	cmd := i.manager.cmd("image", "delete", imageRef)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete image: %w, output: %s", err, string(output))
	}
	return nil
}

// AddImageAlias adds an alias to an image.
func (i *ImageManager) AddImageAlias(ctx context.Context, imageRef, alias string) error {
	cmd := i.manager.cmd("image", "alias", "create", alias, imageRef)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add alias: %w, output: %s", err, string(output))
	}
	return nil
}

// DeleteImageAlias deletes an image alias.
func (i *ImageManager) DeleteImageAlias(ctx context.Context, alias string) error {
	cmd := i.manager.cmd("image", "alias", "delete", alias)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete alias: %w, output: %s", err, string(output))
	}
	return nil
}

// RefreshImage refreshes an image from its source.
func (i *ImageManager) RefreshImage(ctx context.Context, imageRef string) error {
	cmd := i.manager.cmd("image", "refresh", imageRef)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to refresh image: %w, output: %s", err, string(output))
	}
	return nil
}

// ExportImage exports an image to a file.
func (i *ImageManager) ExportImage(ctx context.Context, imageRef, outputPath string) error {
	cmd := i.manager.cmd("image", "export", imageRef, outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to export image: %w, output: %s", err, string(output))
	}
	return nil
}

// ImportImage imports an image from a file or URL.
func (i *ImageManager) ImportImage(ctx context.Context, source string, aliases []string) (*Image, error) {
	args := make([]string, 0, 3+2*len(aliases))
	args = append(args, "image", "import", source)

	for _, alias := range aliases {
		args = append(args, "--alias", alias)
	}

	cmd := i.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to import image: %w, output: %s", err, string(output))
	}

	if len(aliases) > 0 {
		return i.GetImage(ctx, aliases[0])
	}
	return nil, nil
}

// SearchRemoteImages searches for images on a remote server.
func (i *ImageManager) SearchRemoteImages(ctx context.Context, remote, query string) ([]*Image, error) {
	cmd := i.manager.cmd("image", "list", remote+":", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list remote images: %w", err)
	}

	var raw []struct {
		Properties map[string]string `json:"properties"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse image list: %w", err)
	}

	var images []*Image
	for _, r := range raw {
		image := &Image{
			OS:         r.Properties["os"],
			Release:    r.Properties["release"],
			Arch:       r.Properties["architecture"],
			Variant:    r.Properties["variant"],
			Properties: r.Properties,
		}

		// Build name from properties
		if image.OS != "" {
			image.Name = image.OS
			if image.Release != "" {
				image.Name += "/" + image.Release
			}
		}

		// Filter by query
		if query != "" {
			query = strings.ToLower(query)
			if !strings.Contains(strings.ToLower(image.Name), query) &&
				!strings.Contains(strings.ToLower(image.OS), query) {
				continue
			}
		}

		images = append(images, image)
	}

	return images, nil
}

// GetRemoteImageInfo gets detailed info about a remote image.
func (i *ImageManager) GetRemoteImageInfo(ctx context.Context, remote, image string) (*Image, error) {
	cmd := i.manager.cmd("image", "info", fmt.Sprintf("%s:%s", remote, image), "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get remote image info: %w", err)
	}

	var raw struct {
		Fingerprint  string            `json:"fingerprint"`
		Filename     string            `json:"filename"`
		Size         uint64            `json:"size"`
		Architecture string            `json:"architecture"`
		CreatedAt    time.Time         `json:"created_at"`
		Properties   map[string]string `json:"properties"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse image info: %w", err)
	}

	return &Image{
		ID:         raw.Fingerprint[:12],
		Name:       image,
		OS:         raw.Properties["os"],
		Release:    raw.Properties["release"],
		Arch:       raw.Architecture,
		Size:       raw.Size / (1024 * 1024),
		CreatedAt:  raw.CreatedAt,
		Properties: raw.Properties,
	}, nil
}

// SetImageAutoUpdate enables or disables auto-update for an image.
func (i *ImageManager) SetImageAutoUpdate(ctx context.Context, imageRef string, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}

	cmd := i.manager.cmd("image", "set-property", imageRef, "auto_update", value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set auto-update: %w, output: %s", err, string(output))
	}
	return nil
}

// PopularImages returns a list of commonly used images.
func PopularImages() []ImageRecommendation {
	return []ImageRecommendation{
		{
			Name:        "ubuntu/22.04",
			OS:          "ubuntu",
			Release:     "22.04",
			Description: "Ubuntu 22.04 LTS (Jammy Jellyfish)",
			Recommended: true,
		},
		{
			Name:        "ubuntu/24.04",
			OS:          "ubuntu",
			Release:     "24.04",
			Description: "Ubuntu 24.04 LTS (Noble Numbat)",
			Recommended: true,
		},
		{
			Name:        "debian/12",
			OS:          "debian",
			Release:     "12",
			Description: "Debian 12 (Bookworm)",
			Recommended: true,
		},
		{
			Name:        "alpine/3.19",
			OS:          "alpine",
			Release:     "3.19",
			Description: "Alpine Linux 3.19 (minimal)",
			Recommended: true,
		},
		{
			Name:        "centos/9-Stream",
			OS:          "centos",
			Release:     "9-Stream",
			Description: "CentOS Stream 9",
		},
		{
			Name:        "rockylinux/9",
			OS:          "rockylinux",
			Release:     "9",
			Description: "Rocky Linux 9",
		},
		{
			Name:        "fedora/40",
			OS:          "fedora",
			Release:     "40",
			Description: "Fedora 40",
		},
		{
			Name:        "archlinux",
			OS:          "archlinux",
			Release:     "",
			Description: "Arch Linux (rolling)",
		},
	}
}

// ImageRecommendation represents a recommended image.
type ImageRecommendation struct {
	Name        string `json:"name"`
	OS          string `json:"os"`
	Release     string `json:"release"`
	Description string `json:"description"`
	Recommended bool   `json:"recommended"`
}

// QuickPull pulls a commonly used image by name.
func (i *ImageManager) QuickPull(ctx context.Context, os, release string) (*Image, error) {
	imageRef := os
	if release != "" {
		imageRef += "/" + release
	}
	return i.PullImage(ctx, imageRef, true)
}

// GetImageByProperties finds an image matching the given properties.
func (i *ImageManager) GetImageByProperties(ctx context.Context, os, release, arch string) (*Image, error) {
	images, err := i.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	for _, img := range images {
		if os != "" && img.OS != os {
			continue
		}
		if release != "" && img.Release != release {
			continue
		}
		if arch != "" && img.Arch != arch {
			continue
		}
		return img, nil
	}

	return nil, fmt.Errorf("no matching image found for os=%s, release=%s, arch=%s", os, release, arch)
}
