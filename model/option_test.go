package model

import (
	"path/filepath"
	"testing"

	sqlitedriver "github.com/glebarez/go-sqlite"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRetrySQLiteWriteRetriesBusySnapshot(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "options.db") + "?mode=rwc&_pragma=journal_mode(WAL)&_busy_timeout=0"
	db1, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	db2, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	db1SQL, err := db1.DB()
	require.NoError(t, err)
	db2SQL, err := db2.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db1SQL.Close()
		_ = db2SQL.Close()
	})

	require.NoError(t, db1.AutoMigrate(&Option{}))
	require.NoError(t, db1.Create(&Option{Key: "test.option", Value: "old"}).Error)

	attempts := 0
	var firstError error
	err = retrySQLiteWrite(func() error {
		attempts++
		return db1.Transaction(func(tx *gorm.DB) error {
			var option Option
			require.NoError(t, tx.First(&option, "key = ?", "test.option").Error)
			if attempts == 1 {
				require.NoError(t, db2.Model(&Option{}).Where("key = ?", "test.option").Update("value", "concurrent").Error)
			}
			option.Value = "new"
			err := tx.Save(&option).Error
			if attempts == 1 {
				firstError = err
			}
			return err
		})
	})
	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
	var sqliteErr *sqlitedriver.Error
	require.ErrorAs(t, firstError, &sqliteErr)
	assert.Equal(t, 517, sqliteErr.Code())

	var option Option
	require.NoError(t, db1.First(&option, "key = ?", "test.option").Error)
	assert.Equal(t, "new", option.Value)
}
