package logger

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"k8s.io/klog/v2"
)

// fmtVerbRE 匹配 printf 格式動詞（%v %s %d %f 等）
var fmtVerbRE = regexp.MustCompile(`%[vsTdxXfFgeEbcqp]`)

// Init 初始化日誌系統
// level: debug | info | warn | error
// format 由環境變數 LOG_FORMAT=json|text 控制，預設 text
func Init(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))

	// 將 klog（client-go 使用）輸出導向 stdout
	klog.InitFlags(nil)
	klog.SetOutput(os.Stdout)

	Info("日誌系統初始化完成", "level", level, "format", os.Getenv("LOG_FORMAT"))
}

// dispatch 統一分派：若 format 含 printf 動詞則先 Sprintf，否則視為 slog key-value pairs
func dispatch(level slog.Level, format string, args ...any) {
	if len(args) > 0 && fmtVerbRE.MatchString(format) {
		slog.Log(nil, level, fmt.Sprintf(format, args...)) //nolint:sloglint
	} else {
		slog.Log(nil, level, format, args...) //nolint:sloglint
	}
}

// Debug 除錯日誌
func Debug(format string, args ...any) {
	dispatch(slog.LevelDebug, format, args...)
}

// Info 資訊日誌
func Info(format string, args ...any) {
	dispatch(slog.LevelInfo, format, args...)
}

// Warn 警告日誌
func Warn(format string, args ...any) {
	dispatch(slog.LevelWarn, format, args...)
}

// Error 錯誤日誌
func Error(format string, args ...any) {
	dispatch(slog.LevelError, format, args...)
}

// Fatal 致命錯誤（輸出後退出）
func Fatal(format string, args ...any) {
	dispatch(slog.LevelError, format, args...)
	os.Exit(1)
}
