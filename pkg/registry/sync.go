package registry

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"cis/pkg/types"
)

// ImageSyncer 处理镜像同步
type ImageSyncer struct {
	source  types.RegistryAuth
	targets []types.RegistryAuth
}

func NewImageSyncer(source types.RegistryAuth, targets []types.RegistryAuth) *ImageSyncer {
	return &ImageSyncer{
		source:  source,
		targets: targets,
	}
}

// 添加一个验证 registry URL 的方法
func validateRegistryURL(url string) error {
	// 检查 URL 是否为空
	if url == "" {
		return fmt.Errorf("registry URL 不能为空")
	}

	// 检查 URL 是否包含协议（不应该包含）
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return fmt.Errorf("registry URL 不应包含协议前缀 (http:// 或 https://)")
	}

	return nil
}

func (s *ImageSyncer) SyncImage(ctx context.Context, imageRef string) error {
	// 验证源仓库 URL
	if err := validateRegistryURL(s.source.URL); err != nil {
		return err
	}

	// 验证目标仓库 URL
	for _, target := range s.targets {
		if err := validateRegistryURL(target.URL); err != nil {
			return err
		}
	}

	fmt.Printf("开始同步镜像: %s\n", imageRef)

	// 1. 解析源镜像引用
	sourceRef, err := name.ParseReference(imageRef, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("解析源镜像引用失败: %w", err)
	}

	fmt.Printf("源镜像引用: %s\n", sourceRef.String())

	// 2. 配置源仓库认证（如果有）
	var sourceAuth authn.Authenticator
	if s.source.Auth != nil {
		sourceAuth = authn.FromConfig(authn.AuthConfig{
			Username: s.source.Auth.Username,
			Password: s.source.Auth.Password,
		})
	} else {
		sourceAuth = authn.Anonymous
	}

	// 配置源仓库传输层
	sourceTransport := http.DefaultTransport.(*http.Transport).Clone()

	// 获取源镜像
	sourceImage, err := remote.Image(sourceRef,
		remote.WithAuth(sourceAuth),
		remote.WithContext(ctx),
		remote.WithTransport(sourceTransport))
	if err != nil {
		return fmt.Errorf("获取源镜像失败: %w", err)
	}

	// 获取并打印镜像摘要，用于验证
	digest, err := sourceImage.Digest()
	if err == nil {
		fmt.Printf("源镜像摘要: %s\n", digest.String())
	}

	// 4. 同步到每个目标仓库
	for _, target := range s.targets {
		// 构建目标镜像引用
		targetRef, err := s.buildTargetReference(imageRef, target)
		if err != nil {
			return fmt.Errorf("构建目标镜像引用失败: %w", err)
		}

		fmt.Printf("目标镜像引用: %s\n", targetRef.String())

		// 配置目标仓库认证
		var targetAuth authn.Authenticator
		if target.Auth != nil {
			targetAuth = authn.FromConfig(authn.AuthConfig{
				Username: target.Auth.Username,
				Password: target.Auth.Password,
			})
		} else {
			targetAuth = authn.Anonymous
		}

		// 配置目标仓库传输层
		targetTransport := http.DefaultTransport.(*http.Transport).Clone()

		// 推送镜像到目标仓库
		if err := remote.Write(targetRef, sourceImage,
			remote.WithAuth(targetAuth),
			remote.WithContext(ctx),
			remote.WithTransport(targetTransport)); err != nil {
			return fmt.Errorf("推送镜像到目标仓库失败: %w", err)
		}

		fmt.Printf("镜像推送成功: %s\n", targetRef.String())
	}

	return nil
}

// buildTargetReference 构建目标镜像引用
func (s *ImageSyncer) buildTargetReference(sourceRef string, target types.RegistryAuth) (name.Reference, error) {
	// 解析源镜像引用以获取仓库名和标签
	srcRef, err := name.ParseReference(sourceRef, name.WeakValidation)
	if err != nil {
		return nil, fmt.Errorf("解析源镜像引用失败: %w", err)
	}

	// 获取仓库名和标签
	repository := srcRef.Context().RepositoryStr()
	tag := srcRef.Identifier()

	// 如果指定了目标命名空间，替换原有的命名空间
	if target.Namespace != "" {
		// 获取镜像名称（不包含命名空间）
		parts := strings.Split(repository, "/")
		imageName := parts[len(parts)-1]
		// 使用新的命名空间
		repository = fmt.Sprintf("%s/%s", target.Namespace, imageName)
	}

	// 构建目标镜像引用
	targetRef := fmt.Sprintf("%s/%s:%s", target.URL, repository, tag)
	fmt.Printf("同步镜像: %s -> %s\n", srcRef, targetRef)

	return name.ParseReference(targetRef)
} 