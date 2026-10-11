package service

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ListCatalogChannels 在用户分组授权完成后读取目录的渠道配置快照。
// 只保留绑定授权分组的活跃渠道，不调用旧视图的展示合成价或共享价格缓存。
func (s *ChannelService) ListCatalogChannels(ctx context.Context, authorizedGroups []Group) ([]Channel, error) {
	allowed := make(map[int64]struct{}, len(authorizedGroups))
	for _, group := range authorizedGroups {
		if group.IsActive() {
			allowed[group.ID] = struct{}{}
		}
	}
	out := make([]Channel, 0)
	if len(allowed) == 0 {
		return out, nil
	}
	channels, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取目录渠道配置: %w", err)
	}
	bindings := make(map[int64]int64, len(allowed))
	for i := range channels {
		ch := &channels[i]
		if !ch.IsActive() {
			continue
		}
		included := false
		for _, id := range ch.GroupIDs {
			if _, ok := allowed[id]; ok {
				// 对齐 channel_groups 的唯一分组约束；不按仓库返回顺序猜报价。
				if previous, exists := bindings[id]; exists && previous != ch.ID {
					return nil, infraerrors.New(http.StatusInternalServerError,
						"CATALOG_CHANNEL_BINDING_CONFLICT", "模型目录的渠道绑定存在冲突").
						WithCause(fmt.Errorf("目录分组 %d 同时绑定渠道 %d 和 %d", id, previous, ch.ID))
				}
				bindings[id] = ch.ID
				included = true
			}
		}
		if included {
			out = append(out, *ch.Clone())
		}
	}
	return out, nil
}

// CatalogModelsForGroup 在既有有限模型集合内做平台与公开请求名白名单过滤。
// 调用方必须先完成用户授权；此处的绑定检查不授予用户权限。
func (c *Channel) CatalogModelsForGroup(group *Group) []SupportedModel {
	out := make([]SupportedModel, 0)
	if c == nil || group == nil || !c.IsActive() || !group.IsActive() || !slices.Contains(c.GroupIDs, group.ID) {
		return out
	}
	for _, model := range c.SupportedModels() {
		if !isConcreteRequestPlatform(model.Platform) ||
			(group.Platform != PlatformComposite && group.Platform != model.Platform) ||
			!group.ModelAllowlist.Allows(model.Name) {
			continue
		}
		out = append(out, model)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Platform != out[j].Platform {
			return out[i].Platform < out[j].Platform
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}
