package engine

import (
	"sort"
	"sync"
)

var dlMutex sync.Mutex
var indexMutex sync.Mutex
var AllMirrors map[int64]MirrorStatus = getMap()
var CanceledMirrors map[int64]MirrorStatus = getMap()
var GlobalMirrorIndex int = 0

const (
	MirrorStatusDownloading  = "Downloading"
	MirrorStatusUploading    = "Uploading"
	MirrorStatusArchiving    = "Archiving"
	MirrorStatusUnArchiving  = "UnArchiving"
	MirrorStatusCloning      = "Cloning"
	MirrorStatusWaiting      = "Queued"
	MirrorStatusFailed       = "Failed"
	MirrorStatusCanceled     = "Canceled"
	MirrorStatusUploadQueued = "Queued for upload"
)

func getMap() map[int64]MirrorStatus {
	return make(map[int64]MirrorStatus)
}

func GetAllMirrors() []MirrorStatus {
	var dls []MirrorStatus
	for _, dl := range AllMirrors {
		dls = append(dls, dl)
	}
	sort.Slice(dls, func(i, j int) bool {
		return dls[i].Index() < dls[j].Index()
	})
	return dls
}

func GetMirrorByGid(gid string) MirrorStatus {
	for _, dl := range AllMirrors {
		if dl.Gid() == gid {
			return dl
		}
	}
	return nil
}

func GetMirrorByUid(uid int64) MirrorStatus {
	for i, dl := range AllMirrors {
		if i == uid {
			return dl
		}
	}
	return nil
}

func GetAllMirrorsCount() int {
	return len(GetAllMirrors())
}

func AddMirrorLocal(messageId int64, dl MirrorStatus) {
	dlMutex.Lock()
	defer dlMutex.Unlock()
	AllMirrors[messageId] = dl
}

func MoveMirrorToCancel(messageId int64, dl MirrorStatus) {
	dlMutex.Lock()
	defer dlMutex.Unlock()
	CanceledMirrors[messageId] = dl
}

func RemoveMirrorLocal(messageId int64) {
	dlMutex.Lock()
	defer dlMutex.Unlock()
	_, ok := AllMirrors[messageId]
	if ok {
		delete(AllMirrors, messageId)
	}
}

func GenerateMirrorIndex() int {
	indexMutex.Lock()
	defer func() {
		GlobalMirrorIndex = GlobalMirrorIndex + 1
		indexMutex.Unlock()
	}()
	return GlobalMirrorIndex
}

func GetAllMirrorsChunked(chunkSize int) (chunks [][]MirrorStatus) {
	//While there are more items remaining than chunkSize...
	items := GetAllMirrors()
	for chunkSize < len(items) {
		//We take a slice of size chunkSize from the items array and append it to the new array
		chunks = append(chunks, items[0:chunkSize])
		//Then we remove those elements from the items array
		items = items[chunkSize:]
	}
	//Finally we append the remaining items to the new array and return it
	return append(chunks, items)
}
