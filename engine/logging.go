package engine

import (
	"os"
)

const LogFile string = "../log.txt"

func InitLog() {
	//call before starting new session
	//truncate old session's log file
	os.Remove(LogFile)
}

func GetLogFileHandle() (*os.File, error) {
	return os.OpenFile(LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
}
