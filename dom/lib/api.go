/*
	Package lib implements some of the core computations in the dominator.

	Package lib provides functions for computing differences between a sub and
	desired image to generate lists of objects for fetching and pushing and
	update requests. It contains some common code that both the dominator and
	the push-image subcommand of subtool share.
*/
package lib

import (
	"errors"
	"github.com/Symantec/Dominator/lib/filesystem"
	"github.com/Symantec/Dominator/lib/hash"
	"github.com/Symantec/Dominator/lib/image"
	"github.com/Symantec/Dominator/lib/objectcache"
	"github.com/Symantec/Dominator/lib/objectserver"
	"github.com/Symantec/Dominator/lib/srpc"
	subproto "github.com/Symantec/Dominator/proto/sub"
	"log"
)

var (
	ErrorFailedToGetObject = errors.New("get object failed")
)

// Sub should be initialised with data to be used in the package functions.
type Sub struct {
	Hostname                string
	Client                  *srpc.Client
	FileSystem              *filesystem.FileSystem
	ComputedInodes          map[string]*filesystem.RegularInode
	ObjectCache             objectcache.ObjectCache
	ObjectGetter            objectserver.ObjectGetter
	requiredInodeToSubInode map[uint64]uint64
	inodesChanged           map[uint64]bool   // Required inode number.
	inodesCreated           map[uint64]string // Required inode number.
	subObjectCacheUsage     map[hash.Hash]uint64
	requiredFS              *filesystem.FileSystem
}

// BuildMissingLists will construct lists of objects to be fetched by the sub
// from an object server and the list of computed objects that should be pushed
// to the sub. The lists are generated by comparing the contents of
// sub.FileSystem and sub.ObjectCache with the desired image.
// If pushComputedFiles is true then the list of computed files to be pushed is
// generated.
// If ignoreMissingComputedFiles is true then missing computed files are
// ignored, otherwise these missing files lead to an error and early termination
// of the function.
// Computed file metadata are specified by sub.ComputedInodes.
// BuildMissingLists returns a slice of objects to fetch and a map of files to
// push. The map is nil if there are missing computed files.
func BuildMissingLists(sub Sub, image *image.Image, pushComputedFiles bool,
	ignoreMissingComputedFiles bool, logger *log.Logger) (
	[]hash.Hash, map[hash.Hash]struct{}) {
	return sub.buildMissingLists(image, pushComputedFiles,
		ignoreMissingComputedFiles, logger)
}

// BuildUpdateRequest will build an update request which can be sent to the sub.
// If deleteMissingComputedFiles is true then missing computed files are deleted
// on the sub, else missing computed files lead to the function failing.
// It returns true if the function failed due to missing computed files.
func BuildUpdateRequest(sub Sub, image *image.Image,
	request *subproto.UpdateRequest, deleteMissingComputedFiles bool,
	logger *log.Logger) bool {
	return sub.buildUpdateRequest(image, request, deleteMissingComputedFiles,
		logger)
}

// PushObjects will push the list of files given by objectsToPush to the sub.
// File data are obtained from sub.ObjectGetter.
func PushObjects(sub Sub, objectsToPush map[hash.Hash]struct{},
	logger *log.Logger) error {
	return sub.pushObjects(objectsToPush, logger)
}

// String returns the hostname of the sub.
func (sub *Sub) String() string {
	return sub.Hostname
}