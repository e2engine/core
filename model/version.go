package model

import "time"

type Version struct {
	Version      string    `json:"version" yaml:"version"`
	BuildTime    time.Time `json:"build_time" yaml:"build_time"`
	GitCommit    string    `json:"git_commit" yaml:"git_commit"`
	GitTreeDirty bool      `json:"git_tree_dirty" yaml:"git_tree_dirty"`
	GoVersion    string    `json:"go_version" yaml:"go_version"`
	Compiler     string    `json:"compiler" yaml:"compiler"`
	Platform     string    `json:"platform" yaml:"platform"`
}
