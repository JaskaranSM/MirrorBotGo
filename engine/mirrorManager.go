package engine

import (
	"sort"
)

var AllMirrors map[int]MirrorStatus = getMap()
var CanceledMirrors map[int]MirrorStatus = getMap()
var GlobalMirrorIndex int = 0

const (
	MirrorStatusDownloading = "Downloading"
	MirrorStatusUploading   = "Uploading"
	MirrorStatusArchiving   = "Archiving"
)

func getMap() map[int]MirrorStatus {
	return make(map[int]MirrorStatus)
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

func GetMirrorByUid(uid int) MirrorStatus {
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

func AddMirrorLocal(messageId int, dl MirrorStatus) {
	AllMirrors[messageId] = dl
}

func MoveMirrorToCancel(messageId int, dl MirrorStatus) {
	CanceledMirrors[messageId] = dl
}

func RemoveMirrorLocal(messageId int) {
	_, ok := AllMirrors[messageId]
	if ok {
		delete(AllMirrors, messageId)
	}
}
