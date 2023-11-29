package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	model2 "github.com/mylxsw/aidea-server/pkg/repo/model"

	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/go-utils/array"
)

// PromptRepo 提示语相关的数据库操作
type PromptRepo struct {
	db *sql.DB
}

func NewPromptRepo(db *sql.DB) *PromptRepo {
	return &PromptRepo{db: db}
}

// DrawPromptTag 提示语标签
type DrawPromptTag struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// DrawPromptCategory 提示语标签分类
type DrawPromptCategory struct {
	Name        string `json:"name"`
	childrenMap map[string]DrawPromptCategory
	Children    []DrawPromptCategory `json:"children,omitempty"`
	Tags        []DrawPromptTag      `json:"tags,omitempty"`
}

// TagType 提示语标签类型
type TagType int

const (
	// TagTypeCommon 通用标签，适用于正向、负向
	TagTypeCommon TagType = 0
	// TagTypePrompt 仅适用于正向提示语
	TagTypePrompt TagType = 1
	// TagTypeNegativePrompt 仅适用于负向提示语
	TagTypeNegativePrompt TagType = 2
)

// DrawTags 获取文生图、图生图 Prompt 生成器标签列表
func (r *PromptRepo) DrawTags(ctx context.Context, tagType ...TagType) ([]DrawPromptCategory, error) {
	q := query.Builder().Where(model2.FieldPromptTagsStatus, 1)
	if len(tagType) > 0 {
		q = q.WhereIn(model2.FieldPromptTagsTagType, tagType)
	}

	tags, err := model2.NewPromptTagsModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	categories := array.ToMap(
		array.Map(
			array.UniqBy(tags, func(tag model2.PromptTagsN) string { return tag.Category.ValueOrZero() }),
			func(tag model2.PromptTagsN, _ int) DrawPromptCategory {
				return DrawPromptCategory{
					Name:        tag.Category.ValueOrZero(),
					childrenMap: make(map[string]DrawPromptCategory),
				}
			},
		),
		func(cat DrawPromptCategory, _ int) string {
			return cat.Name
		},
	)

	for _, tag := range tags {
		cat := categories[tag.Category.ValueOrZero()]
		subCat, ok := cat.childrenMap[tag.CategorySub.ValueOrZero()]
		if !ok {
			subCat = DrawPromptCategory{
				Name: tag.CategorySub.ValueOrZero(),
				Tags: make([]DrawPromptTag, 0),
			}
		}

		subCat.Tags = append(subCat.Tags, DrawPromptTag{
			Name:  tag.TagName.ValueOrZero(),
			Value: tag.TagValue.ValueOrZero(),
		})

		cat.childrenMap[tag.CategorySub.ValueOrZero()] = subCat
	}

	res := array.FromMap(categories)
	for i, cat := range res {
		res[i].Children = array.FromMap(cat.childrenMap)
	}

	return res, nil
}

// ChatSystemPrompts 获取数字人模型的所有系统提示语示例
func (r *PromptRepo) ChatSystemPromptExamples(ctx context.Context) ([]model2.ChatSysPromptExample, error) {
	examples, err := model2.NewChatSysPromptExampleModel(r.db).Get(ctx, query.Builder())
	if err != nil {
		return nil, err
	}

	return array.Map(examples, func(model model2.ChatSysPromptExampleN, _ int) model2.ChatSysPromptExample {
		return model.ToChatSysPromptExample()
	}), nil
}

// PromptExample 用户提示语示例
type PromptExample struct {
	// Title  标题，返回值必须包含该字段，即使为空字符串（客户端未做兼容）
	Title   string   `json:"title" yaml:"title"`
	Content string   `json:"content,omitempty" yaml:"content,omitempty"`
	Models  []string `json:"models,omitempty" yaml:"models,omitempty"`
	Tags    []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// CommonPromptExamples 获取通用的提示语示例
// TODO  这里时间充足了应该修改为根据查询条件获取部分数据或者是增加缓存层，否则数据量大的时候会有性能问题
func (r *PromptRepo) CommonPromptExamples(ctx context.Context) ([]PromptExample, error) {
	examples, err := model2.NewPromptExampleModel(r.db).Get(ctx, query.Builder())
	if err != nil {
		return nil, err
	}

	return array.Map(examples, func(example model2.PromptExampleN, _ int) PromptExample {
		var models []string
		if example.Models.ValueOrZero() != "" {
			if err := json.Unmarshal([]byte(example.Models.ValueOrZero()), &models); err != nil {
				log.With(example).Errorf("unmarshal models failed: %v", err)
			}
		}

		var tags []string
		if example.Tags.ValueOrZero() != "" {
			if err := json.Unmarshal([]byte(example.Tags.ValueOrZero()), &tags); err != nil {
				log.With(example).Errorf("unmarshal tags failed: %v", err)
			}
		}

		return PromptExample{
			Title:   example.Title.ValueOrZero(),
			Content: example.Content.ValueOrZero(),
			Models:  models,
			Tags:    tags,
		}
	}), nil
}
