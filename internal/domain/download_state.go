package domain

import "time"

// DownloadState 断点续传状态
type DownloadState struct {
	ID               string
	TaskID           string
	ProductID        string
	DestPath         string
	TotalBytes       int64
	ReceivedBytes    int64
	ChecksumExpected string
	Status           string // pending / downloading / paused / completed / failed
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
