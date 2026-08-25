package store

import (
	"database/sql"
	"errors"
	"strings"

	"task243-rnghealth/internal/model"
)

// scannable 是 *sql.Row 与 *sql.Rows 共有的 Scan 接口。
type scannable interface {
	Scan(dest ...interface{}) error
}

// mapErr 将底层数据库错误映射到领域错误。
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unique constraint"):
		return model.ErrConflict
	case strings.Contains(msg, "constraint failed"):
		return model.ErrConflict
	case strings.Contains(msg, "no such"):
		return model.ErrNotFound
	}
	return err
}
