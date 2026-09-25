package auth

import "errors"

// ErrNotFound - пользователь не найден
var ErrNotFound = errors.New("user not found")

// ErrWrongPassword — старый пароль не совпал.
var ErrWrongPassword = errors.New("wrong password")

// ErrWeakPassword — пароль слишком короткий.
var ErrWeakPassword = errors.New("weak password")

// ErrLastAdmin — попытка снять себе админ-роль (последнему админу нельзя).
var ErrLastAdmin = errors.New("cannot demote last admin")

// ErrSelfLockout — попытка заблокировать саму себя.
var ErrSelfLockout = errors.New("cannot block yourself")
