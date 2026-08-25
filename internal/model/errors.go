package model

import (
	"errors"
	"fmt"
)

// 领域错误，供 service / httpapi 做错误映射。
var (
	// ErrNotFound 实体不存在。
	ErrNotFound = errors.New("entity not found")
	// ErrConflict 并发或序号冲突。
	ErrConflict = errors.New("conflict")
	// ErrInvalidArgument 入参非法。
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrSealed 实体已封存，拒绝写入。
	ErrSealed = errors.New("entity sealed")
	// ErrTransition 状态机流转不合法。
	ErrTransition = errors.New("illegal state transition")
	// ErrUnknownTest 未知健康测试类别。
	ErrUnknownTest = errors.New("unknown health test category")
)

// IsNotFound 判断是否为“不存在”类错误。
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsConflict 判断是否为“冲突”类错误。
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}

// IsSealed 判断是否为“已封存”类错误。
func IsSealed(err error) bool {
	return errors.Is(err, ErrSealed)
}

// IsTransition 判断是否为“非法流转”类错误。
func IsTransition(err error) bool {
	return errors.Is(err, ErrTransition)
}

// IsInvalidArgument 判断是否为“入参非法”类错误。
func IsInvalidArgument(err error) bool {
	return errors.Is(err, ErrInvalidArgument)
}

// ConflictError 携带冲突实体的标识，供调用方明确报告“已存在”而非模糊成功。
// 用于同名同设备熵源重复注册等场景：保留首次注册结果，其余请求以 409 报告冲突。
type ConflictError struct {
	// ExistingID 已存在的实体 ID。
	ExistingID int64
	// Cause 触发冲突的领域错误（通常为 ErrConflict）。
	Cause error
}

// Error 实现 error 接口。
func (e *ConflictError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("conflict: existing entity id=%d: %v", e.ExistingID, e.Cause)
	}
	return fmt.Sprintf("conflict: existing entity id=%d", e.ExistingID)
}

// Unwrap 暴露底层领域错误，使 IsConflict 等判定生效。
func (e *ConflictError) Unwrap() error {
	if e.Cause != nil {
		return e.Cause
	}
	return ErrConflict
}

// NewConflictError 构造一个携带已存在 ID 的冲突错误。
func NewConflictError(existingID int64) *ConflictError {
	return &ConflictError{ExistingID: existingID, Cause: ErrConflict}
}
