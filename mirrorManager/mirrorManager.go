package mirrorManager

var AllMirrors map[int]MirrorStatus = getMap()

const (
	MirrorStatusDownloading = "Downloading"
	MirrorStatusUploading   = "Uploading"
)

func getMap() map[int]MirrorStatus {
	return make(map[int]MirrorStatus)
}

func GetAllMirrors() []MirrorStatus {
	var dls []MirrorStatus
	for _, dl := range AllMirrors {
		dls = append(dls, dl)
	}
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

func RemoveMirrorLocal(messageId int) {
	_, ok := AllMirrors[messageId]
	if ok {
		delete(AllMirrors, messageId)
	}
}
