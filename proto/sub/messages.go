package sub

import (
	"github.com/Symantec/Dominator/lib/filesystem"
	"github.com/Symantec/Dominator/lib/hash"
	"github.com/Symantec/Dominator/lib/objectcache"
	"github.com/Symantec/Dominator/lib/triggers"
	"github.com/Symantec/Dominator/proto/common"
)

type Configuration struct {
	ScanSpeedPercent    uint
	NetworkSpeedPercent uint
	ScanExclusionList   []string
}

type FetchRequest struct {
	ServerAddress string
	Hashes        []hash.Hash
}

type FetchResponse common.StatusResponse

type GetConfigurationRequest struct {
}

type GetConfigurationResponse Configuration

type PollRequest struct {
	HaveGeneration uint64
}

type PollResponse struct {
	NetworkSpeed                 uint64
	FetchInProgress              bool // Fetch() and Update() mutually exclusive
	UpdateInProgress             bool
	LastUpdateHadTriggerFailures bool
	GenerationCount              uint64
	FileSystem                   *filesystem.FileSystem
	FileSystemFollows            bool
	ObjectCache                  objectcache.ObjectCache
} // FileSystem is encoded afterwards.

type SetConfigurationRequest Configuration

type SetConfigurationResponse common.StatusResponse

type FileToCopyToCache struct {
	Name string
	Hash hash.Hash
}

type Hardlink struct {
	NewLink string
	Target  string
}

type Inode struct {
	Name string
	filesystem.GenericInode
}

type UpdateRequest struct {
	// The ordering here reflects the ordering that the sub is expected to use.
	FilesToCopyToCache  []FileToCopyToCache
	DirectoriesToMake   []Inode
	InodesToMake        []Inode
	HardlinksToMake     []Hardlink
	PathsToDelete       []string
	InodesToChange      []Inode
	MultiplyUsedObjects map[hash.Hash]uint64
	Triggers            *triggers.Triggers
}

type UpdateResponse struct{}

type CleanupRequest struct {
	Hashes []hash.Hash
}

type CleanupResponse struct{}
