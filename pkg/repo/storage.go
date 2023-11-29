package repo

import (
	"context"
	"database/sql"
	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/pkg/misc"
	"github.com/mylxsw/aidea-server/pkg/repo/model"
	"github.com/mylxsw/eloquent/query"
)

const (
	StorageFileStatusEnabled  = int64(1)
	StorageFileStatusDisabled = int64(2)
	StorageFileStatusReview   = int64(3)
)

type FileStorageRepo struct {
	db   *sql.DB
	conf *config.Config
}

func NewFileStorageRepo(db *sql.DB, conf *config.Config) *FileStorageRepo {
	return &FileStorageRepo{db: db, conf: conf}
}

func (repo *FileStorageRepo) Save(ctx context.Context, file model.StorageFile) (int64, error) {
	return model.NewStorageFileModel(repo.db).Create(ctx, query.KV{
		model.FieldStorageFileName:     misc.WordTruncate(file.Name, 100),
		model.FieldStorageFileHash:     file.Hash,
		model.FieldStorageFileFileKey:  file.FileKey,
		model.FieldStorageFileFileSize: file.FileSize,
		model.FieldStorageFileBucket:   file.Bucket,
		model.FieldStorageFileUserId:   file.UserId,
		model.FieldStorageFileNote:     file.Note,
		model.FieldStorageFileStatus:   file.Status,
		model.FieldStorageFileChannel:  file.Channel,
	})
}

func (repo *FileStorageRepo) UpdateByKey(ctx context.Context, fileKey string, status int64, note string) error {
	q := query.Builder().Where(model.FieldStorageFileFileKey, fileKey)
	_, err := model.NewStorageFileModel(repo.db).UpdateFields(ctx, query.KV{
		model.FieldStorageFileNote:   note,
		model.FieldStorageFileStatus: status,
	}, q)
	return err
}
