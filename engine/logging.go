package engine

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const LogFile string = "log.txt"

var LOGGER *zap.SugaredLogger

func GetLoggerObject() *zap.SugaredLogger {
	err := os.Remove(LogFile)
	if err != nil {
		log.Printf("GetLoggerObject: %s: %v\n", LogFile, err)
	}

	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeTime = zapcore.RFC3339TimeEncoder
	handleSync, _, err := zap.Open(LogFile)
	if err != nil {
		log.Println("Cannot open log file for zap: ", err)
	}
	core1 := zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), os.Stdout, zap.InfoLevel)
	core2 := zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), handleSync, zap.InfoLevel)
	logger := zap.New(zapcore.NewTee(core1, core2), zap.AddCaller())
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			log.Printf("Failed to sync zap logger: %v\n", err)
		}
	}(logger) // flushes buffer, if any
	return logger.Sugar()
}

func GetLogFileHandle() (*os.File, error) {
	return os.OpenFile(LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
}

func GetLogger() *zap.SugaredLogger {
	if LOGGER == nil {
		LOGGER = GetLoggerObject()
	}
	return LOGGER
}

func L() *zap.SugaredLogger {
	if LOGGER == nil {
		LOGGER = GetLoggerObject()
	}
	return LOGGER
}
