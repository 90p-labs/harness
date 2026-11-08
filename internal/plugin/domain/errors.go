package domain

import "errors"

var (
	ErrUnknownEvent = errors.New("unknown event")
	ErrPluginFailed = errors.New("plugin failed")
)
