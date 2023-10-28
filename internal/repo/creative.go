package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/aidea-server/internal/repo/model"
	"github.com/mylxsw/asteria/log"
	"github.com/mylxsw/eloquent"
	"github.com/mylxsw/eloquent/query"
	"github.com/mylxsw/go-utils/array"
	"gopkg.in/guregu/null.v3"
)

type IslandType int64

const (
	IslandTypeText              IslandType = 1
	IslandTypeImage             IslandType = 2
	IslandTypeVideo             IslandType = 3
	IslandTypeAudio             IslandType = 4
	IslandTypeUpscale           IslandType = 5
	IslandTypeImageColorization IslandType = 6
)

type IslandHistorySharedStatus int64

const (
	IslandHistorySharedStatusNotShared IslandHistorySharedStatus = 0
	IslandHistorySharedStatusShared    IslandHistorySharedStatus = 1
)

type IslandStatus int64

const (
	IslandStatusDisabled IslandStatus = 0
	IslandStatusEnabled  IslandStatus = 1
)

type CreativeStatus int64

const (
	CreativeStatusPending    CreativeStatus = 1
	CreativeStatusProcessing CreativeStatus = 2
	CreativeStatusSuccess    CreativeStatus = 3
	CreativeStatusFailed     CreativeStatus = 4
	// CreativeStatusForbid  资源封禁
	CreativeStatusForbid CreativeStatus = 5
)

type CreativeRepo struct {
	db *sql.DB
}

func NewCreativeRepo(db *sql.DB) *CreativeRepo {
	return &CreativeRepo{db: db}
}

type CreativeItem struct {
	ID          int64          `json:"id"`
	IslandId    string         `json:"island_id"`
	IslandType  IslandType     `json:"island_type"`
	IslandModel string         `json:"island_model"`
	Arguments   string         `json:"arguments"`
	Prompt      string         `json:"prompt"`
	Answer      string         `json:"answer"`
	TaskId      string         `json:"task_id"`
	Status      CreativeStatus `json:"status"`
}

// CreativeIslandExt CreativeIsland 扩展字段
type CreativeIslandExt struct {
	// AIRewrite 默认是否开启 AI 重写
	AIRewrite         bool        `json:"ai_rewrite,omitempty"`
	ShowAIRewrite     bool        `json:"show_ai_rewrite,omitempty"`
	AIPrompt          string      `json:"ai_prompt,omitempty"`
	UpscaleBy         string      `json:"upscale_by,omitempty"`
	ShowNegativeText  bool        `json:"show_negative_text,omitempty"`
	ShowAdvanceButton bool        `json:"show_advance_button,omitempty"`
	AllowSizes        []AllowSize `json:"allow_sizes,omitempty"`
	DefaultWidth      int         `json:"default_width,omitempty"`
	DefaultHeight     int         `json:"default_height,omitempty"`
	DefaultSteps      int         `json:"default_steps,omitempty"`
}

func (ext CreativeIslandExt) GetDefaultWidth(defaultValue int) int {
	if ext.DefaultWidth > 0 {
		return ext.DefaultWidth
	}

	return defaultValue
}

func (ext CreativeIslandExt) GetDefaultHeight(defaultValue int) int {
	if ext.DefaultHeight > 0 {
		return ext.DefaultHeight
	}

	return defaultValue
}

func (ext CreativeIslandExt) GetDefaultSteps(defaultValue int) int {
	if ext.DefaultSteps > 0 {
		return ext.DefaultSteps
	}

	return defaultValue
}

func (ext CreativeIslandExt) Init() CreativeIslandExt {
	sizes := array.Filter(ext.AllowSizes, func(size AllowSize, _ int) bool {
		return !(size.Width <= 0 && size.Height <= 0)
	})
	ext.AllowSizes = array.Map(sizes, func(size AllowSize, _ int) AllowSize {
		if size.Width <= 0 {
			size.Width = size.Height
		}

		if size.Height <= 0 {
			size.Height = size.Width
		}

		if size.AspectRatio == "" {
			size.AspectRatio = helper.ResolveAspectRatio(size.Width, size.Height)
		}

		return size
	})

	return ext
}

type AllowSize struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
}
type CreativeIsland struct {
	Id                     int64  `json:"id"`
	IslandId               string `json:"island_id"`
	Title                  string `json:"title"`
	TitleColor             string `json:"title_color"`
	Description            string `json:"description"`
	Category               string `json:"category"`
	ModelType              string `json:"model_type"`
	WordCount              int64  `json:"word_count"`
	Hint                   string `json:"hint"`
	Vendor                 string `json:"vendor"`
	Model                  string `json:"model"`
	StylePreset            string `json:"style_preset,omitempty"`
	Prompt                 string `json:"prompt"`
	BgImage                string `json:"bg_image,omitempty"`
	BgEmbeddedImage        string `json:"bg_embedded_image,omitempty"`
	Label                  string `json:"label,omitempty"`
	LabelColor             string `json:"label_color,omitempty"`
	SubmitBtnText          string `json:"submit_btn_text,omitempty"`
	PromptInputTitle       string `json:"prompt_input_title,omitempty"`
	WaitSeconds            int64  `json:"wait_seconds,omitempty"`
	ShowImageStyleSelector int64  `json:"show_image_style_selector,omitempty"`
	NoPrompt               int64  `json:"no_prompt,omitempty"`
	VersionMin             string `json:"version_min,omitempty"`
	VersionMax             string `json:"version_max,omitempty"`
	Status                 int64  `json:"status"`
	Priority               int64  `json:"priority,omitempty"`
	CreatedAt              time.Time
	UpdatedAt              time.Time

	// 注意，不要添加与 model.CreativeIsland 相同的字段，否则会导致 json 序列化失败
	Extension CreativeIslandExt `json:"extension,omitempty"`
}

func buildCreativeIslandFromModel(item model.CreativeIslandN) CreativeIsland {
	var ext CreativeIslandExt
	if !item.Ext.IsZero() && item.Ext.String != "" {
		if err := json.Unmarshal([]byte(item.Ext.ValueOrZero()), &ext); err != nil {
			log.With(item).Errorf("unmarshal creative island ext failed: %v", err)
		}
	}
	return CreativeIsland{
		Id:                     item.Id.ValueOrZero(),
		IslandId:               item.IslandId.ValueOrZero(),
		Title:                  item.Title.ValueOrZero(),
		TitleColor:             item.TitleColor.ValueOrZero(),
		Description:            item.Description.ValueOrZero(),
		Category:               item.Category.ValueOrZero(),
		ModelType:              item.ModelType.ValueOrZero(),
		WordCount:              item.WordCount.ValueOrZero(),
		Hint:                   item.Hint.ValueOrZero(),
		Vendor:                 item.Vendor.ValueOrZero(),
		Model:                  item.Model.ValueOrZero(),
		StylePreset:            item.StylePreset.ValueOrZero(),
		Prompt:                 item.Prompt.ValueOrZero(),
		BgImage:                item.BgImage.ValueOrZero(),
		BgEmbeddedImage:        item.BgEmbeddedImage.ValueOrZero(),
		Label:                  item.Label.ValueOrZero(),
		LabelColor:             item.LabelColor.ValueOrZero(),
		SubmitBtnText:          item.SubmitBtnText.ValueOrZero(),
		PromptInputTitle:       item.PromptInputTitle.ValueOrZero(),
		WaitSeconds:            item.WaitSeconds.ValueOrZero(),
		ShowImageStyleSelector: item.ShowImageStyleSelector.ValueOrZero(),
		NoPrompt:               item.NoPrompt.ValueOrZero(),
		VersionMin:             item.VersionMin.ValueOrZero(),
		VersionMax:             item.VersionMax.ValueOrZero(),
		Status:                 item.Status.ValueOrZero(),
		Priority:               item.Priority.ValueOrZero(),
		CreatedAt:              item.CreatedAt.ValueOrZero(),
		UpdatedAt:              item.UpdatedAt.ValueOrZero(),

		Extension: ext,
	}
}

func (r *CreativeRepo) Islands(ctx context.Context) ([]CreativeIsland, error) {
	q := query.Builder().
		Where(model.FieldCreativeIslandStatus, int64(IslandStatusEnabled)).
		OrderBy(model.FieldCreativeIslandPriority, "DESC").
		OrderBy(model.FieldCreativeIslandId, "ASC")
	items, err := model.NewCreativeIslandModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	return array.Map(items, func(item model.CreativeIslandN, _ int) CreativeIsland {
		return buildCreativeIslandFromModel(item)
	}), nil
}

func (r *CreativeRepo) Island(ctx context.Context, islandId string) (*CreativeIsland, error) {
	q := query.Builder().Where(model.FieldCreativeIslandIslandId, islandId)
	item, err := model.NewCreativeIslandModel(r.db).First(ctx, q)
	if err != nil {
		if err == query.ErrNoResult {
			return nil, ErrNotFound
		}
		return nil, err
	}

	island := buildCreativeIslandFromModel(*item)
	return &island, nil
}

func (r *CreativeRepo) CreateRecord(ctx context.Context, userId int64, item *CreativeItem) (int64, error) {
	return model.NewCreativeHistoryModel(r.db).Create(ctx, query.KV{
		model.FieldCreativeHistoryUserId:      userId,
		model.FieldCreativeHistoryIslandId:    item.IslandId,
		model.FieldCreativeHistoryIslandType:  int64(item.IslandType),
		model.FieldCreativeHistoryIslandModel: item.IslandModel,
		model.FieldCreativeHistoryArguments:   item.Arguments,
		model.FieldCreativeHistoryPrompt:      item.Prompt,
		model.FieldCreativeHistoryAnswer:      item.Answer,
		model.FieldCreativeHistoryTaskId:      item.TaskId,
		model.FieldCreativeHistoryStatus:      int64(item.Status),
	})
}

func (r *CreativeRepo) CreateRecordWithArguments(ctx context.Context, userId int64, item *CreativeItem, arg *CreativeRecordArguments) (int64, error) {
	if arg != nil {
		arguments, _ := json.Marshal(arg)
		item.Arguments = string(arguments)
	}

	id, err := model.NewCreativeHistoryModel(r.db).Create(ctx, query.KV{
		model.FieldCreativeHistoryUserId:      userId,
		model.FieldCreativeHistoryIslandId:    item.IslandId,
		model.FieldCreativeHistoryIslandType:  int64(item.IslandType),
		model.FieldCreativeHistoryIslandModel: item.IslandModel,
		model.FieldCreativeHistoryArguments:   item.Arguments,
		model.FieldCreativeHistoryPrompt:      item.Prompt,
		model.FieldCreativeHistoryAnswer:      item.Answer,
		model.FieldCreativeHistoryTaskId:      item.TaskId,
		model.FieldCreativeHistoryStatus:      int64(item.Status),
	})
	if err != nil {
		return 0, err
	}

	// 这里故意不使用事务，因为这个操作不是很重要，如果失败了，也不会影响其他操作
	if arg != nil && arg.GalleryCopyID > 0 {
		_, err := r.db.ExecContext(ctx, "UPDATE creative_gallery SET hot_value = hot_value + 1, ref_count = ref_count + 1  WHERE id = ?", arg.GalleryCopyID)
		if err != nil {
			log.With(err).Errorf("update gallery hot value failed")
		}
	}

	return id, nil
}

func (r *CreativeRepo) UpdateRecordByID(ctx context.Context, userId, id int64, answer string, quotaUsed int64, status CreativeStatus) error {
	q := query.Builder().Where(model.FieldCreativeHistoryId, id).
		Where(model.FieldCreativeHistoryUserId, userId)

	_, err := model.NewCreativeHistoryModel(r.db).Update(ctx, q, model.CreativeHistoryN{
		Answer:    null.StringFrom(answer),
		Status:    null.IntFrom(int64(status)),
		QuotaUsed: null.IntFrom(quotaUsed),
	})
	return err
}

func (r *CreativeRepo) UpdateRecordStatusByID(ctx context.Context, id int64, answer string, status CreativeStatus) error {
	q := query.Builder().Where(model.FieldCreativeHistoryId, id)
	_, err := model.NewCreativeHistoryModel(r.db).Update(ctx, q, model.CreativeHistoryN{
		Status: null.IntFrom(int64(status)),
		Answer: null.StringFrom(answer),
	})
	return err
}

func (r *CreativeRepo) UpdateRecordAnswerByTaskID(ctx context.Context, userId int64, taskID string, answer string) error {
	q := query.Builder().Where(model.FieldCreativeHistoryTaskId, taskID).
		Where(model.FieldCreativeHistoryUserId, userId)

	_, err := model.NewCreativeHistoryModel(r.db).Update(ctx, q, model.CreativeHistoryN{
		Answer: null.StringFrom(answer),
	})
	return err
}

func (r *CreativeRepo) UpdateRecordAnswerByID(ctx context.Context, userId int64, historyID int64, answer string) error {
	q := query.Builder().Where(model.FieldCreativeHistoryId, historyID).
		Where(model.FieldCreativeHistoryUserId, userId)

	_, err := model.NewCreativeHistoryModel(r.db).Update(ctx, q, model.CreativeHistoryN{
		Answer: null.StringFrom(answer),
	})
	return err
}

type CreativeRecordUpdateRequest struct {
	Answer       string                       `json:"answer"`
	QuotaUsed    int64                        `json:"quota_used"`
	Status       CreativeStatus               `json:"status"`
	ExtArguments *CreativeRecordUpdateExtArgs `json:"ext_arguments"`
}

type CreativeRecordUpdateExtArgs struct {
	RealPrompt         string `json:"real_prompt,omitempty"`
	RealNegativePrompt string `json:"real_negative_prompt,omitempty"`
}

func (r *CreativeRepo) UpdateRecordArgumentsByTaskID(ctx context.Context, userId int64, taskID string, ext CreativeRecordUpdateExtArgs) error {
	q := query.Builder().Where(model.FieldCreativeHistoryTaskId, taskID).
		Where(model.FieldCreativeHistoryUserId, userId)

	original, err := model.NewCreativeHistoryModel(r.db).First(ctx, q)
	if err != nil {
		return err
	}

	var arg CreativeRecordArguments
	if !original.Arguments.IsZero() && original.Arguments.String != "" {
		if err := json.Unmarshal([]byte(original.Arguments.ValueOrZero()), &arg); err != nil {
			log.With(original).Errorf("unmarshal creative island ext failed: %v", err)
		}
	}

	if ext.RealPrompt != "" {
		arg.RealPrompt = ext.RealPrompt
	}

	if ext.RealNegativePrompt != "" {
		arg.RealNegativePrompt = ext.RealNegativePrompt
	}

	argData, _ := json.Marshal(arg)
	update := model.CreativeHistoryN{
		Arguments: null.StringFrom(string(argData)),
	}

	_, err = model.NewCreativeHistoryModel(r.db).Update(ctx, q, update)
	return err
}

func (r *CreativeRepo) UpdateRecordByTaskID(ctx context.Context, userId int64, taskID string, req CreativeRecordUpdateRequest) error {
	q := query.Builder().Where(model.FieldCreativeHistoryTaskId, taskID).
		Where(model.FieldCreativeHistoryUserId, userId)

	update := model.CreativeHistoryN{
		Answer:    null.StringFrom(req.Answer),
		Status:    null.IntFrom(int64(req.Status)),
		QuotaUsed: null.IntFrom(req.QuotaUsed),
	}

	if req.ExtArguments != nil {
		original, err := model.NewCreativeHistoryModel(r.db).First(ctx, q)
		if err != nil {
			return err
		}

		var arg CreativeRecordArguments
		if !original.Arguments.IsZero() && original.Arguments.String != "" {
			if err := json.Unmarshal([]byte(original.Arguments.ValueOrZero()), &arg); err != nil {
				log.With(original).Errorf("unmarshal creative island ext failed: %v", err)
			}
		}

		if req.ExtArguments.RealPrompt != "" {
			arg.RealPrompt = req.ExtArguments.RealPrompt
		}

		if req.ExtArguments.RealNegativePrompt != "" {
			arg.RealNegativePrompt = req.ExtArguments.RealNegativePrompt
		}

		argData, _ := json.Marshal(arg)
		update.Arguments = null.StringFrom(string(argData))
	}

	_, err := model.NewCreativeHistoryModel(r.db).Update(ctx, q, update)
	return err
}

func (r *CreativeRepo) FindHistoryRecordByTaskId(ctx context.Context, userId int64, taskId string) (*model.CreativeHistory, error) {
	q := query.Builder().
		Where(model.FieldCreativeHistoryUserId, userId).
		Where(model.FieldCreativeHistoryTaskId, taskId).
		OrderBy(model.FieldCreativeHistoryId, "DESC")

	item, err := model.NewCreativeHistoryModel(r.db).First(ctx, q)
	if err != nil {
		if err == query.ErrNoResult {
			return nil, ErrNotFound
		}
		return nil, err
	}

	ret := item.ToCreativeHistory()
	return &ret, nil
}

func (r *CreativeRepo) FindHistoryRecord(ctx context.Context, userId, id int64) (*CreativeHistoryItem, error) {
	q := query.Builder().
		Where(model.FieldCreativeHistoryId, id)

	if userId > 0 {
		q = q.Where(model.FieldCreativeHistoryUserId, userId)
	}

	item, err := model.NewCreativeHistoryModel(r.db).First(ctx, q)
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &CreativeHistoryItem{
		Id:         item.Id.ValueOrZero(),
		UserID:     item.UserId.ValueOrZero(),
		IslandId:   item.IslandId.ValueOrZero(),
		IslandType: item.IslandType.ValueOrZero(),
		Arguments:  item.Arguments.ValueOrZero(),
		Prompt:     item.Prompt.ValueOrZero(),
		Answer:     item.Answer.ValueOrZero(),
		QuotaUsed:  item.QuotaUsed.ValueOrZero(),
		Status:     item.Status.ValueOrZero(),
		Shared:     item.Shared.ValueOrZero(),
		CreatedAt:  item.CreatedAt.ValueOrZero(),
		UpdatedAt:  item.UpdatedAt.ValueOrZero(),
	}, nil
}

type CreativeHistoryItem struct {
	Id          int64     `json:"id"`
	IslandId    string    `json:"island_id,omitempty"`
	IslandType  int64     `json:"island_type,omitempty"`
	IslandName  string    `json:"island_name,omitempty"`
	IslandTitle string    `json:"island_title,omitempty"`
	IslandModel string    `json:"-"`
	Arguments   string    `json:"arguments,omitempty"`
	Prompt      string    `json:"prompt,omitempty"`
	Answer      string    `json:"answer,omitempty"`
	QuotaUsed   int64     `json:"quota_used,omitempty"`
	Status      int64     `json:"status,omitempty"`
	UserID      int64     `json:"user_id,omitempty"`
	Shared      int64     `json:"shared,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type CreativeHistoryQuery struct {
	IslandId    string
	Mode        string
	IslandModel string
	Page        int64
	PerPage     int64
}

func (r *CreativeRepo) HistoryRecordPaginate(ctx context.Context, userId int64, req CreativeHistoryQuery) ([]CreativeHistoryItem, query.PaginateMeta, error) {
	q := query.Builder().
		OrderBy(model.FieldCreativeHistoryId, "DESC")

	if userId > 0 {
		q = q.Where(model.FieldCreativeHistoryUserId, userId)
	}

	switch req.Mode {
	case "creative-island":
		q = q.Where(model.FieldCreativeHistoryIslandType, int64(IslandTypeText))
	case "image-draw":
		q = q.Where(model.FieldCreativeHistoryIslandType, int64(IslandTypeImage))
	default:
	}

	if req.IslandId != "" {
		q = q.Where(model.FieldCreativeHistoryIslandId, req.IslandId)
	}

	if req.IslandModel != "" {
		q = q.Where(model.FieldCreativeHistoryIslandModel, req.IslandModel)
	}

	items, meta, err := model.NewCreativeHistoryModel(r.db).Paginate(ctx, req.Page, req.PerPage, q)
	if err != nil {
		return nil, query.PaginateMeta{}, err
	}

	islandIDNames := make(map[string]string)
	islandQ := query.Builder().Select(model.FieldCreativeIslandIslandId, model.FieldCreativeIslandTitle)
	if req.IslandId != "" {
		islandQ = islandQ.Where(model.FieldCreativeIslandIslandId, req.IslandId)
	}

	islands, err := model.NewCreativeIslandModel(r.db).Get(
		ctx,
		islandQ,
	)
	if err == nil {
		for _, island := range islands {
			islandIDNames[island.IslandId.ValueOrZero()] = island.Title.ValueOrZero()
		}
	}

	ret := array.Map(items, func(item model.CreativeHistoryN, _ int) CreativeHistoryItem {
		answer := item.Answer.ValueOrZero()
		if item.IslandType.ValueOrZero() == int64(IslandTypeText) {
			answer = helper.SubString(answer, 100)
		}

		return CreativeHistoryItem{
			Id:          item.Id.ValueOrZero(),
			IslandId:    item.IslandId.ValueOrZero(),
			IslandType:  item.IslandType.ValueOrZero(),
			IslandModel: item.IslandModel.ValueOrZero(),
			Arguments:   item.Arguments.ValueOrZero(),
			Prompt:      helper.SubString(item.Prompt.ValueOrZero(), 100),
			Answer:      answer,
			QuotaUsed:   item.QuotaUsed.ValueOrZero(),
			Status:      item.Status.ValueOrZero(),
			CreatedAt:   item.CreatedAt.ValueOrZero(),
			UpdatedAt:   item.UpdatedAt.ValueOrZero(),
			Shared:      item.Shared.ValueOrZero(),
			IslandName:  islandIDNames[item.IslandId.ValueOrZero()],
		}
	})

	return ret, meta, nil
}

func (r *CreativeRepo) DeleteHistoryRecord(ctx context.Context, userId, id int64) error {
	q := query.Builder().
		Where(model.FieldCreativeHistoryId, id).
		Where(model.FieldCreativeHistoryUserId, userId)

	_, err := model.NewCreativeHistoryModel(r.db).Delete(ctx, q)
	return err
}

func (r *CreativeRepo) UserGallery(ctx context.Context, userID int64, islandModel string, limit int64) ([]CreativeHistoryItem, error) {
	q := query.Builder().
		// Where(model.FieldCreativeHistoryStatus, int64(CreativeStatusSuccess)).
		Where(model.FieldCreativeHistoryIslandType, int64(IslandTypeImage)).
		Select(
			model.FieldCreativeHistoryId,
			model.FieldCreativeHistoryIslandId,
			model.FieldCreativeHistoryIslandType,
			model.FieldCreativeHistoryAnswer,
			model.FieldCreativeHistoryStatus,
			model.FieldCreativeHistoryUserId,
			model.FieldCreativeHistoryCreatedAt,
			model.FieldCreativeHistoryUpdatedAt,
		).
		OrderBy(model.FieldCreativeHistoryId, "DESC").
		Limit(limit)

	if islandModel != "" {
		q = q.Where(model.FieldCreativeHistoryIslandModel, islandModel)
	}

	if userID != 0 {
		q = q.Where(model.FieldCreativeHistoryUserId, userID)
	}

	items, err := model.NewCreativeHistoryModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	islandIDNames := make(map[string]string)
	islandQ := query.Builder().Select(model.FieldCreativeIslandIslandId, model.FieldCreativeIslandTitle)
	islands, err := model.NewCreativeIslandModel(r.db).Get(ctx, islandQ)
	if err == nil {
		for _, island := range islands {
			islandIDNames[island.IslandId.ValueOrZero()] = island.Title.ValueOrZero()
		}
	}

	ret := array.Map(items, func(item model.CreativeHistoryN, _ int) CreativeHistoryItem {
		return CreativeHistoryItem{
			Id:         item.Id.ValueOrZero(),
			IslandId:   item.IslandId.ValueOrZero(),
			IslandType: item.IslandType.ValueOrZero(),
			Answer:     item.Answer.ValueOrZero(),
			CreatedAt:  item.CreatedAt.ValueOrZero(),
			UpdatedAt:  item.UpdatedAt.ValueOrZero(),
			Status:     item.Status.ValueOrZero(),
			UserID:     item.UserId.ValueOrZero(),
			IslandName: islandIDNames[item.IslandId.ValueOrZero()],
		}
	})

	return ret, nil
}

type CreativeRecordArguments struct {
	NegativePrompt     string   `json:"negative_prompt,omitempty"`
	PromptTags         []string `json:"prompt_tags,omitempty"`
	Width              int64    `json:"width,omitempty"`
	Height             int64    `json:"height,omitempty"`
	ImageRatio         string   `json:"image_ratio,omitempty"`
	Steps              int64    `json:"steps,omitempty"`
	ImageCount         int64    `json:"image_count,omitempty"`
	StylePreset        string   `json:"style_preset,omitempty"`
	Mode               string   `json:"mode,omitempty"`
	Image              string   `json:"image,omitempty"`
	UpscaleBy          string   `json:"upscale_by,omitempty"`
	AIRewrite          bool     `json:"ai_rewrite,omitempty"`
	RealPrompt         string   `json:"real_prompt,omitempty"`
	RealNegativePrompt string   `json:"real_negative_prompt,omitempty"`
	ModelName          string   `json:"model_name,omitempty"`
	ModelID            string   `json:"model_id,omitempty"`
	FilterID           int64    `json:"filter_id,omitempty"`
	FilterName         string   `json:"filter_name,omitempty"`
	GalleryCopyID      int64    `json:"gallery_copy_id,omitempty"`
	Seed               int64    `json:"seed,omitempty"`
}

func (arg CreativeRecordArguments) ToGalleryMeta() GalleryMeta {
	return GalleryMeta{
		ImageRatio:         arg.ImageRatio,
		Steps:              arg.Steps,
		StylePreset:        arg.StylePreset,
		Mode:               arg.Mode,
		Image:              arg.Image,
		AIRewrite:          arg.AIRewrite,
		RealPrompt:         arg.RealPrompt,
		RealNegativePrompt: arg.RealNegativePrompt,
		ModelName:          arg.ModelName,
		ModelID:            arg.ModelID,
		FilterID:           arg.FilterID,
	}
}

const (
	CreativeGalleryStatusPending = 0
	CreativeGalleryStatusOK      = 1
	CreativeGalleryStatusDenied  = 2
	CreativeGalleryStatusDeleted = 3
)

func (r *CreativeRepo) Gallery(ctx context.Context, page, perPage int64) ([]model.CreativeGallery, query.PaginateMeta, error) {
	ids, meta, err := model.NewCreativeGalleryRandomModel(r.db).Paginate(ctx, page, perPage, query.Builder())
	if err != nil {
		return nil, meta, err
	}

	randomIds := array.Map(ids, func(item model.CreativeGalleryRandomN, _ int) any {
		return item.GalleryId.ValueOrZero()
	})

	if len(randomIds) == 0 {
		meta.LastPage = 1
		return []model.CreativeGallery{}, meta, nil
	}

	q := query.Builder().
		WhereIn(model.FieldCreativeGalleryId, randomIds).
		Select(
			model.FieldCreativeGalleryId,
			model.FieldCreativeGalleryUserId,
			model.FieldCreativeGalleryUsername,
			model.FieldCreativeGalleryCreativeType,
			model.FieldCreativeGalleryPrompt,
			model.FieldCreativeGalleryAnswer,
			model.FieldCreativeGalleryTags,
			model.FieldCreativeGalleryRefCount,
			model.FieldCreativeGalleryStarLevel,
			model.FieldCreativeGalleryHotValue,
			model.FieldCreativeGalleryCreatedAt,
			model.FieldCreativeGalleryUpdatedAt,
		).
		OrderBy(model.FieldCreativeGalleryHotValue, "DESC").
		OrderByRaw("RAND()")

	items, err := model.NewCreativeGalleryModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, meta, err
	}

	return array.Map(items, func(item model.CreativeGalleryN, _ int) model.CreativeGallery {
		return item.ToCreativeGallery()
	}), meta, err
}

func (r *CreativeRepo) GalleryByID(ctx context.Context, id int64) (*model.CreativeGallery, error) {
	q := query.Builder().
		Where(model.FieldCreativeGalleryId, id)

	item, err := model.NewCreativeGalleryModel(r.db).First(ctx, q)
	if err != nil {
		if errors.Is(err, query.ErrNoResult) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	ret := item.ToCreativeGallery()
	return &ret, nil
}

type GalleryMeta struct {
	ImageRatio         string `json:"image_ratio,omitempty"`
	Steps              int64  `json:"steps,omitempty"`
	StylePreset        string `json:"style_preset,omitempty"`
	Mode               string `json:"mode,omitempty"`
	Image              string `json:"image,omitempty"`
	AIRewrite          bool   `json:"ai_rewrite,omitempty"`
	RealPrompt         string `json:"real_prompt,omitempty"`
	RealNegativePrompt string `json:"real_negative_prompt,omitempty"`
	ModelName          string `json:"model_name,omitempty"`
	ModelID            string `json:"model_id,omitempty"`
	FilterID           int64  `json:"filter_id,omitempty"`
}

func (r *CreativeRepo) ShareCreativeHistoryToGallery(ctx context.Context, userID int64, username string, id int64) error {
	return eloquent.Transaction(r.db, func(tx query.Database) error {
		// 查询创作岛历史纪录信息
		q := query.Builder().
			Where(model.FieldCreativeHistoryId, id).
			Where(model.FieldCreativeHistoryUserId, userID)

		item, err := model.NewCreativeHistoryModel(tx).First(ctx, q)
		if err != nil {
			if errors.Is(err, query.ErrNoResult) {
				return ErrNotFound
			}

			return err
		}

		// 查询是否已经在 Gallery 中
		existItem, err := model.NewCreativeGalleryModel(tx).First(
			ctx,
			query.Builder().Where(model.FieldCreativeGalleryCreativeHistoryId, id),
		)
		if err != nil && !errors.Is(err, query.ErrNoResult) {
			return err
		}

		if !errors.Is(err, query.ErrNoResult) {
			// 已经存在，且已经删除，则恢复
			if existItem.Status.ValueOrZero() == CreativeGalleryStatusDeleted {
				item.Shared = null.IntFrom(int64(IslandHistorySharedStatusShared))
				if err := item.Save(ctx, model.FieldCreativeHistoryShared); err != nil {
					return err
				}

				existItem.Status = null.IntFrom(CreativeGalleryStatusOK)
				return existItem.Save(ctx, model.FieldCreativeGalleryStatus)
			}

			return nil
		}

		item.Shared = null.IntFrom(int64(IslandHistorySharedStatusShared))
		if err := item.Save(ctx, model.FieldCreativeHistoryShared); err != nil {
			return err
		}

		// 保存到 Gallery
		var arg CreativeRecordArguments
		if !item.Arguments.IsZero() && item.Arguments.String != "" {
			if err := json.Unmarshal([]byte(item.Arguments.ValueOrZero()), &arg); err != nil {
				log.With(item).Errorf("unmarshal creative island ext failed: %v", err)
			}
		}

		prompt := strings.Trim(item.Prompt.ValueOrZero(), ",")
		if len(arg.PromptTags) > 0 {
			prompt = prompt + "," + strings.Join(arg.PromptTags, ",")
		}

		meta, _ := json.Marshal(arg.ToGalleryMeta())
		_, err = model.NewCreativeGalleryModel(tx).Create(ctx, query.KV{
			model.FieldCreativeGalleryUserId:            userID,
			model.FieldCreativeGalleryUsername:          username,
			model.FieldCreativeGalleryCreativeHistoryId: id,
			model.FieldCreativeGalleryCreativeType:      item.IslandType.ValueOrZero(),
			model.FieldCreativeGalleryPrompt:            prompt,
			model.FieldCreativeGalleryAnswer:            item.Answer.ValueOrZero(),
			model.FieldCreativeGalleryStatus:            CreativeGalleryStatusOK,
			model.FieldCreativeGalleryNegativePrompt:    arg.NegativePrompt,
			model.FieldCreativeGalleryMeta:              string(meta),
		})
		return err
	})
}

func (r *CreativeRepo) CancelCreativeHistoryShare(ctx context.Context, userID int64, historyID int64) error {
	return eloquent.Transaction(r.db, func(tx query.Database) error {
		q := query.Builder().
			Where(model.FieldCreativeGalleryCreativeHistoryId, historyID)
		if userID > 0 {
			q = q.Where(model.FieldCreativeGalleryUserId, userID)
		}

		item, err := model.NewCreativeGalleryModel(tx).First(ctx, q)
		if err != nil {
			if errors.Is(err, query.ErrNoResult) {
				return nil
			}

			return err
		}

		historyItem, err := model.NewCreativeHistoryModel(tx).First(
			ctx,
			query.Builder().
				Where(model.FieldCreativeHistoryId, historyID).
				Where(model.FieldCreativeHistoryUserId, item.UserId),
		)
		if err != nil && !errors.Is(err, query.ErrNoResult) {
			return err
		}

		if historyItem != nil {
			historyItem.Shared = null.IntFrom(int64(IslandHistorySharedStatusNotShared))
			if err := historyItem.Save(ctx, model.FieldCreativeHistoryShared); err != nil {
				return err
			}
		}

		item.Status = null.IntFrom(CreativeGalleryStatusDeleted)
		return item.Save(ctx, model.FieldCreativeGalleryStatus)
	})
}

type ImageModel struct {
	model.ImageModel
	ImageMeta ImageModelMeta `json:"image_meta"`
}

type ImageModelMeta struct {
	Supports          []string             `json:"supports,omitempty"`
	Upscale           bool                 `json:"upscale,omitempty"`
	ShowStyle         bool                 `json:"show_style,omitempty"`
	ShowImageStrength bool                 `json:"show_image_strength,omitempty"`
	IntroURL          string               `json:"intro_url,omitempty"`
	ArtistStyle       string               `json:"artist_style,omitempty"`
	RatioDimensions   map[string]Dimension `json:"ratio_dimensions,omitempty"`
}

type Dimension struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

func (r *CreativeRepo) Model(ctx context.Context, vendor, realModel string) (*ImageModel, error) {
	q := query.Builder().Where(model.FieldImageModelVendor, vendor).Where(model.FieldImageModelRealModel, realModel)
	mod, err := model.NewImageModelModel(r.db).First(ctx, q)
	if err != nil {
		if err == query.ErrNoResult {
			return nil, ErrNotFound
		}

		return nil, err
	}

	item := mod.ToImageModel()

	var meta ImageModelMeta
	if item.Meta != "" {
		if err := json.Unmarshal([]byte(item.Meta), &meta); err != nil {
			log.With(item).Errorf("unmarshal creative island ext failed: %v", err)
		}
	}

	return &ImageModel{ImageModel: item, ImageMeta: meta}, nil
}

func (r *CreativeRepo) Models(ctx context.Context) ([]ImageModel, error) {
	q := query.Builder().
		Where(model.FieldImageModelStatus, 1).
		OrderBy(model.FieldImageModelVendor, "ASC").
		OrderBy(model.FieldImageModelModelName, "ASC")

	items, err := model.NewImageModelModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	return array.Map(items, func(item model.ImageModelN, _ int) ImageModel {
		m := item.ToImageModel()
		var meta ImageModelMeta
		if m.Meta != "" {
			if err := json.Unmarshal([]byte(m.Meta), &meta); err != nil {
				log.With(item).Errorf("unmarshal creative island ext failed: %v", err)
			}
		}
		return ImageModel{
			ImageModel: m,
			ImageMeta:  meta,
		}
	}), nil
}

type ImageFilter struct {
	model.ImageFilter
	Vendor    string          `json:"-"`
	ImageMeta ImageFilterMeta `json:"meta"`
}

type ImageFilterMeta struct {
	Prompt         string   `json:"prompt,omitempty"`
	NegativePrompt string   `json:"negative_prompt,omitempty"`
	Supports       []string `json:"supports,omitempty"`
	// UseTemplateWhenNotContain 当 prompt 不包含 UseTemplateWhenNotContain 时，自动应用提示语模板
	UseTemplateWhenNotContain []string `json:"use_template_when_not_contain,omitempty"`
	Template                  string   `json:"template,omitempty"`
	// Mode 用于图生图（ControlNet）
	// 可选值："canny", "mlsd", "pose", "scribble"
	Mode string `json:"mode,omitempty"`
}

func (meta ImageFilterMeta) ApplyTemplate(prompt string) string {
	if meta.Template == "" {
		return prompt
	}

	return fmt.Sprintf(meta.Template, prompt)
}

func (meta ImageFilterMeta) ShouldUseTemplate(prompt string) bool {
	if meta.Template == "" {
		return false
	}

	if len(meta.UseTemplateWhenNotContain) == 0 {
		return true
	}

	containsWords := array.Filter(meta.UseTemplateWhenNotContain, func(item string, _ int) bool {
		return strings.Contains(prompt, item)
	})

	return len(containsWords) == 0
}

// modelVendors 查询所有的模型（模型 id->模型服务商）
func (r *CreativeRepo) modelVendors(ctx context.Context) (map[string]string, error) {
	q := query.Builder().
		Where(model.FieldImageModelStatus, 1).
		Select(model.FieldImageModelModelId, model.FieldImageModelVendor)

	items, err := model.NewImageModelModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	ret := make(map[string]string)
	for _, item := range items {
		ret[item.ModelId.ValueOrZero()] = item.Vendor.ValueOrZero()
	}

	return ret, nil
}

func (r *CreativeRepo) Filters(ctx context.Context) ([]ImageFilter, error) {
	q := query.Builder().
		Where(model.FieldImageFilterStatus, 1).
		OrderBy(model.FieldImageFilterId, "DESC")

	items, err := model.NewImageFilterModel(r.db).Get(ctx, q)
	if err != nil {
		return nil, err
	}

	modelVenders, err := r.modelVendors(ctx)
	if err == nil {
		// 过滤掉模型不存在的风格
		items = array.Filter(items, func(item model.ImageFilterN, _ int) bool {
			return modelVenders[item.ModelId.ValueOrZero()] != ""
		})
	} else {
		log.Errorf("get model venders failed: %v", err)
	}

	return array.Map(items, func(item model.ImageFilterN, _ int) ImageFilter {
		m := item.ToImageFilter()
		var meta ImageFilterMeta
		if m.Meta != "" {
			if err := json.Unmarshal([]byte(m.Meta), &meta); err != nil {
				log.With(item).Errorf("unmarshal creative island ext failed: %v", err)
			}
		}

		return ImageFilter{
			ImageFilter: m,
			Vendor:      modelVenders[item.ModelId.ValueOrZero()],
			ImageMeta:   meta,
		}
	}), nil
}

func (r *CreativeRepo) Filter(ctx context.Context, id int64) (*ImageFilter, error) {
	q := query.Builder().
		Where(model.FieldImageFilterStatus, 1).
		Where(model.FieldImageFilterId, id)

	item, err := model.NewImageFilterModel(r.db).First(ctx, q)
	if err != nil {
		if err == query.ErrNoResult {
			return nil, ErrNotFound
		}

		return nil, err
	}

	// TODO 暂时无用，但是为了接口完整性，这里应该查询模型服务商

	m := item.ToImageFilter()
	var meta ImageFilterMeta
	if m.Meta != "" {
		if err := json.Unmarshal([]byte(m.Meta), &meta); err != nil {
			log.With(item).Errorf("unmarshal creative island ext failed: %v", err)
		}
	}

	return &ImageFilter{
		ImageFilter: m,
		ImageMeta:   meta,
	}, nil
}
