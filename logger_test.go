package utils

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	zap "github.com/Laisky/zap"
	"github.com/Laisky/zap/zapcore"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	// zap "github.com/Laisky/zap"
)

func TestNewLogger(t *testing.T) {
	logger, err := CreateNewDefaultLogger("123", LoggerLevelDebug)
	require.NoError(t, err)

	lvl := logger.Level()
	require.Equal(t, zap.DebugLevel, lvl)

	_, err = NewLogger()
	require.NoError(t, err)

	logger = logger.Clone().Named("sample")
	for i := 0; i < 100; i++ {
		logger.DebugSample(1, "test")
		logger.InfoSample(1, "test")
		logger.WarnSample(1, "test")
	}
}

func TestWriteToFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestWriteToFile")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("create directory: %v", dir)
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "test.log")
	logger, err := NewLogger(
		WithLoggerOutputPaths([]string{file}),
	)
	require.NoError(t, err)

	logger.Info("yoo")
	logger.Sync()

	content, err := ioutil.ReadFile(file)
	require.NoError(t, err)
	require.Contains(t, string(content), "go-utils/logger_test.go")
	require.Contains(t, string(content), "yoo\n")
}

func TestSetupLogger(t *testing.T) {
	var err error
	Logger, err := NewConsoleLoggerWithName("test", "debug")
	if err != nil {
		t.Fatal(err)
	}

	Logger.Info("test", zap.String("arg", "111"))
	require.NoError(t, Logger.ChangeLevel(LoggerLevelDebug))
	require.NoError(t, Logger.ChangeLevel(LoggerLevelWarn))
	require.NoError(t, Logger.ChangeLevel(LoggerLevelError))
	require.NoError(t, Logger.ChangeLevel(LoggerLevelFatal))
	require.NoError(t, Logger.ChangeLevel(LoggerLevelPanic))
	require.Error(t, Logger.ChangeLevel("xxx"))
	require.NoError(t, Logger.ChangeLevel(LoggerLevelInfo))
	Logger.Info("test", zap.String("arg", "222"), zap.String("color", "\033[1;34m colored \033[0m"))
	Logger.Debug("test", zap.String("arg", "333"))
	// if err := Logger.Sync(); err != nil {
	// 	t.Fatalf("%+v", err)
	// }

	logger := Logger.With(zap.String("yo", "hello"))
	logger.Warn("test")

	// if err = logger.Sync(); err != nil {
	// 	t.Fatal(err)
	// }

	// t.Error()
}

// func setupLogger(level string) *zap2.Logger {
// 	var loglevel zap2.AtomicLevel
// 	switch level {
// 	case "debug":
// 		loglevel = zap2.NewAtomicLevelAt(zap2.DebugLevel)
// 	case "info":
// 		loglevel = zap2.NewAtomicLevelAt(zap2.InfoLevel)
// 	case "warn":
// 		loglevel = zap2.NewAtomicLevelAt(zap2.WarnLevel)
// 	case "error":
// 		loglevel = zap2.NewAtomicLevelAt(zap2.ErrorLevel)
// 	default:
// 		panic(errors.Errorf("log level only be debug/info/warn/error"))
// 	}

// 	cfg := zap2.Config{
// 		Level:       loglevel,
// 		Development: false,
// 		Sampling: &zap2.SamplingConfig{
// 			Initial:    100,
// 			Thereafter: 100,
// 		},
// 		Encoding:         "json",
// 		EncoderConfig:    zap2.NewProductionEncoderConfig(),
// 		OutputPaths:      []string{"stdout"},
// 		ErrorOutputPaths: []string{"stderr"},
// 	}
// 	cfg.EncoderConfig.MessageKey = "message"
// 	cfg.EncoderConfig.EncodeTime = zapcore2.ISO8601TimeEncoder

// 	logger, err := cfg.Build()
// 	if err != nil {
// 		panic(err)
// 	}

// 	defer logger.Sync()
// 	// logger.Info("Logger construction succeeded", zap2.String("level", level))

// 	return logger
// }

// func BenchmarkLogger(b *testing.B) {
// 	Logger.ChangeLevel("info")
// 	b.Run("origin zap", func(b *testing.B) {
// 		for i := 0; i < b.N; i++ {
// 			Logger.Debug("yooo")
// 		}
// 	})

// 	logger := setupLogger("info")
// 	b.Run("new zap", func(b *testing.B) {
// 		for i := 0; i < b.N; i++ {
// 			logger.Debug("yooo")
// 		}
// 	})
// }

func BenchmarkLogger(b *testing.B) {
	var err error
	if err = Logger.ChangeLevel("error"); err != nil {
		b.Fatalf("set level: %+v", err)
	}
	b.Run("low level log", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			Logger.Debug("yooo")
		}
	})

	if err = Logger.ChangeLevel("error"); err != nil {
		b.Fatalf("set level: %+v", err)
	}
	// b.Run("log", func(b *testing.B) {
	// 	for i := 0; i < b.N; i++ {
	// 		Logger.Info("yooo")
	// 	}
	// })
}

func BenchmarkSampleLogger(b *testing.B) {
	var err error
	if err = Logger.ChangeLevel("error"); err != nil {
		b.Fatalf("set level: %+v", err)
	}
	b.Run("low level log", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			Logger.DebugSample(100, "yooo")
		}
	})
}

func TestAlertHook(t *testing.T) {
	pusher, err := NewAlertPusherWithAlertType(
		context.Background(),
		"https://blog.laisky.com/graphql/query/",
		"hello",
		"rwkpVuAgaBZQBASKndHK",
	)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	defer pusher.Close()
	logger := Logger.WithOptions(
		zap.Fields(zap.String("logger", "test")),
		zap.HooksWithFields(pusher.GetZapHook()),
	)

	logger.Debug("DEBUG", zap.String("yo", "hello"))
	logger.Info("Info", zap.String("yo", "hello"))
	logger.Warn("Warn", zap.String("yo", "hello"))
	logger.Error("Error", zap.String("yo", "hello"), zap.Bool("bool", true), zap.Error(errors.Errorf("xxx")))
	// t.Error()

	time.Sleep(5 * time.Second)
}
func ExampleAlertPusher() {
	pusher, err := NewAlertPusherWithAlertType(
		context.Background(),
		"https://blog.laisky.com/graphql/query/",
		"hello",
		"rwkpVuAgaBZQBASKndHK",
	)
	if err != nil {
		Logger.Panic("create alert pusher", zap.Error(err))
	}
	defer pusher.Close()
	logger := Logger.WithOptions(
		zap.Fields(zap.String("logger", "test")),
		zap.HooksWithFields(pusher.GetZapHook()),
	)

	logger.Debug("DEBUG", zap.String("yo", "hello"))
	logger.Info("Info", zap.String("yo", "hello"))
	logger.Warn("Warn", zap.String("yo", "hello"))
	logger.Error("Error", zap.String("yo", "hello"))

	time.Sleep(1 * time.Second)
}

// func TestPateoAlertPusher(t *testing.T) {
// 	ctx := context.Background()

// 	Settings.SetupFromFile("/Users/laisky/repo/pateo/configs/go-fluentd/settings.yml")

// 	alert, err := NewPateoAlertPusher(
// 		ctx,
// 		Settings.GetString("settings.pateo_logger.push_api"),
// 		Settings.GetString("settings.pateo_logger.token"),
// 	)
// 	if err != nil {
// 		t.Fatalf("%+v", err)
// 	}

// 	// if err = alert.Send("test", "test content", Clock.GetUTCNow()); err != nil {
// 	// 	t.Fatalf("%+v", err)
// 	// }

// 	logger := Logger.WithOptions(zap.HooksWithFields(alert.GetZapHook("test")))
// 	logger.Error("test content", zap.String("field", "value"))

// 	time.Sleep(1 * time.Second)
// 	t.Error()
// }

func TestChangeLoggerLevel(t *testing.T) {
	var allLogs []string
	logger, err := NewLogger(
		WithLoggerZapOptions(zap.Hooks(func(e zapcore.Entry) error {
			allLogs = append(allLogs, e.Message)
			return nil
		})),
		WithLoggerLevel(LoggerLevelDebug),
	)
	require.NoError(t, err)

	// case: normal log
	{
		msg := RandomStringWithLength(50)
		logger.Debug(msg)
		require.Equal(t, msg, allLogs[len(allLogs)-1])
	}

	// case: change level
	{
		msg := RandomStringWithLength(50)
		err = logger.ChangeLevel(LoggerLevelInfo)
		require.NoError(t, err)
		logger.Debug(msg)
		require.Len(t, allLogs, 1)
		require.NotEqual(t, msg, allLogs[len(allLogs)-1])
		err = logger.ChangeLevel(LoggerLevelDebug)
		require.NoError(t, err)
	}

	// case: change level for child logger
	{
		msg := RandomStringWithLength(50)
		childLogger := logger.Clone()
		err = childLogger.ChangeLevel(LoggerLevelInfo)
		require.NoError(t, err)
		logger.Debug(msg)
		require.NotEqual(t, msg, allLogs[len(allLogs)-1])

		msg = RandomStringWithLength(50)
		childLogger.Info(msg)
		require.Equal(t, msg, allLogs[len(allLogs)-1])

		msg = RandomStringWithLength(50)
		childLogger.Debug(msg)
		require.NotEqual(t, msg, allLogs[len(allLogs)-1])
	}

}
