package hub

import "errors"

var (
	ErrNotConnected = errors.New("client is not connected")

	ErrNotInitialized = errors.New("service not initialized")

	ErrInvalidQuestID = errors.New("invalid quest ID")

	ErrInvalidTemplateID = errors.New("invalid template ID")

	ErrConnectionTimeout = errors.New("connection timeout")

	ErrInvokeFailed = errors.New("hub method invocation failed")

	ErrQuestNotFound = errors.New("quest not found")

	ErrBundleNotFound = errors.New("bundle not found")
)
