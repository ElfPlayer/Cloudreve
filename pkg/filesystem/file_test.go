package filesystem

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	model "github.com/HFO4/cloudreve/models"
	"github.com/HFO4/cloudreve/pkg/filesystem/fsctx"
	"github.com/HFO4/cloudreve/pkg/filesystem/local"
	"github.com/HFO4/cloudreve/pkg/serializer"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestFileSystem_AddFile(t *testing.T) {
	asserts := assert.New(t)
	file := local.FileStream{
		Size: 5,
		Name: "1.txt",
	}
	folder := model.Folder{
		Model: gorm.Model{
			ID: 1,
		},
	}
	fs := FileSystem{
		User: &model.User{
			Model: gorm.Model{
				ID: 1,
			},
			Policy: model.Policy{
				Model: gorm.Model{
					ID: 1,
				},
			},
		},
	}
	ctx := context.WithValue(context.Background(), fsctx.FileHeaderCtx, file)
	ctx = context.WithValue(ctx, fsctx.SavePathCtx, "/Uploads/1_sad.txt")

	_, err := fs.AddFile(ctx, &folder)

	asserts.Error(err)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	f, err := fs.AddFile(ctx, &folder)

	asserts.NoError(err)
	asserts.Equal("/Uploads/1_sad.txt", f.SourceName)
}

func TestFileSystem_GetContent(t *testing.T) {
	asserts := assert.New(t)
	ctx := context.Background()
	fs := FileSystem{
		User: &model.User{
			Model: gorm.Model{
				ID: 1,
			},
			Policy: model.Policy{
				Model: gorm.Model{
					ID: 1,
				},
			},
		},
	}

	// 文件不存在
	rs, err := fs.GetContent(ctx, "not exist file")
	asserts.Equal(ErrObjectNotExist, err)
	asserts.Nil(rs)

	// 未知存储策略
	file, err := os.Create("TestFileSystem_GetContent.txt")
	asserts.NoError(err)
	_ = file.Close()

	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "policy_id"}).AddRow(1, "TestFileSystem_GetContent.txt", 1))
	mock.ExpectQuery("SELECT(.+)poli(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(1, "unknown"))

	rs, err = fs.GetContent(ctx, "/TestFileSystem_GetContent.txt")
	asserts.Error(err)
	asserts.NoError(mock.ExpectationsWereMet())

	// 打开文件失败
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "policy_id"}).AddRow(1, "TestFileSystem_GetContent.txt", 1))
	mock.ExpectQuery("SELECT(.+)poli(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "type", "source_name"}).AddRow(1, "local", "not exist"))

	rs, err = fs.GetContent(ctx, "/TestFileSystem_GetContent.txt")
	asserts.Equal(serializer.CodeIOFailed, err.(serializer.AppError).Code)
	asserts.NoError(mock.ExpectationsWereMet())

	// 打开成功
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "policy_id", "source_name"}).AddRow(1, "TestFileSystem_GetContent.txt", 1, "TestFileSystem_GetContent.txt"))
	mock.ExpectQuery("SELECT(.+)poli(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(1, "local"))

	rs, err = fs.GetContent(ctx, "/TestFileSystem_GetContent.txt")
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestFileSystem_GetDownloadContent(t *testing.T) {
	asserts := assert.New(t)
	ctx := context.Background()
	fs := FileSystem{
		User: &model.User{
			Model: gorm.Model{
				ID: 1,
			},
			Policy: model.Policy{
				Model: gorm.Model{
					ID: 1,
				},
			},
		},
	}
	file, err := os.Create("TestFileSystem_GetDownloadContent.txt")
	asserts.NoError(err)
	_ = file.Close()

	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "policy_id", "source_name"}).AddRow(1, "TestFileSystem_GetDownloadContent.txt", 1, "TestFileSystem_GetDownloadContent.txt"))
	mock.ExpectQuery("SELECT(.+)poli(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(1, "local"))

	// 无限速
	_, err = fs.GetDownloadContent(ctx, "/TestFileSystem_GetDownloadContent.txt")
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())

	// 有限速
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "policy_id", "source_name"}).AddRow(1, "TestFileSystem_GetDownloadContent.txt", 1, "TestFileSystem_GetDownloadContent.txt"))
	mock.ExpectQuery("SELECT(.+)poli(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(1, "local"))

	fs.User.Group.SpeedLimit = 1
	_, err = fs.GetDownloadContent(ctx, "/TestFileSystem_GetDownloadContent.txt")
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestFileSystem_GroupFileByPolicy(t *testing.T) {
	asserts := assert.New(t)
	ctx := context.Background()
	files := []model.File{
		model.File{
			PolicyID: 1,
			Name:     "1_1.txt",
		},
		model.File{
			PolicyID: 2,
			Name:     "2_1.txt",
		},
		model.File{
			PolicyID: 3,
			Name:     "3_1.txt",
		},
		model.File{
			PolicyID: 2,
			Name:     "2_2.txt",
		},
		model.File{
			PolicyID: 1,
			Name:     "1_2.txt",
		},
	}
	fs := FileSystem{}
	policyGroup := fs.GroupFileByPolicy(ctx, files)
	asserts.Equal(map[uint][]*model.File{
		1: {&files[0], &files[4]},
		2: {&files[1], &files[3]},
		3: {&files[2]},
	}, policyGroup)
}

func TestFileSystem_deleteGroupedFile(t *testing.T) {
	asserts := assert.New(t)
	ctx := context.Background()
	fs := FileSystem{}
	files := []model.File{
		{
			PolicyID:   1,
			Name:       "1_1.txt",
			SourceName: "1_1.txt",
			Policy:     model.Policy{Model: gorm.Model{ID: 1}, Type: "local"},
		},
		{
			PolicyID:   2,
			Name:       "2_1.txt",
			SourceName: "2_1.txt",
			Policy:     model.Policy{Model: gorm.Model{ID: 1}, Type: "local"},
		},
		{
			PolicyID:   3,
			Name:       "3_1.txt",
			SourceName: "3_1.txt",
			Policy:     model.Policy{Model: gorm.Model{ID: 1}, Type: "local"},
		},
		{
			PolicyID:   2,
			Name:       "2_2.txt",
			SourceName: "2_2.txt",
			Policy:     model.Policy{Model: gorm.Model{ID: 1}, Type: "local"},
		},
		{
			PolicyID:   1,
			Name:       "1_2.txt",
			SourceName: "1_2.txt",
			Policy:     model.Policy{Model: gorm.Model{ID: 1}, Type: "local"},
		},
	}

	// 全部失败
	{
		failed := fs.deleteGroupedFile(ctx, fs.GroupFileByPolicy(ctx, files))
		asserts.Equal(map[uint][]string{
			1: {"1_1.txt", "1_2.txt"},
			2: {"2_1.txt", "2_2.txt"},
			3: {"3_1.txt"},
		}, failed)
	}
	// 部分失败
	{
		file, err := os.Create("1_1.txt")
		asserts.NoError(err)
		_ = file.Close()
		failed := fs.deleteGroupedFile(ctx, fs.GroupFileByPolicy(ctx, files))
		asserts.Equal(map[uint][]string{
			1: {"1_2.txt"},
			2: {"2_1.txt", "2_2.txt"},
			3: {"3_1.txt"},
		}, failed)
	}
	// 部分失败,包含整组未知存储策略导致的失败
	{
		file, err := os.Create("1_1.txt")
		asserts.NoError(err)
		_ = file.Close()

		files[1].Policy.Type = "unknown"
		files[3].Policy.Type = "unknown"
		failed := fs.deleteGroupedFile(ctx, fs.GroupFileByPolicy(ctx, files))
		asserts.Equal(map[uint][]string{
			1: {"1_2.txt"},
			2: {"2_1.txt", "2_2.txt"},
			3: {"3_1.txt"},
		}, failed)
	}
}
