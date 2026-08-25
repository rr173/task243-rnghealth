package model

import "errors"

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
