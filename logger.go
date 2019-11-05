package utils

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	zap "github.com/Laisky/zap"
	"github.com/Laisky/zap/zapcore"
	"github.com/shurcooL/graphql"
)

var (
	/*Logger logging tool.

	* Info(msg string, fields ...Field)
	* Debug(msg string, fields ...Field)
	* Warn(msg string, fields ...Field)
	* Error(msg string, fields ...Field)
	* Panic(msg string, fields ...Field)
	* DebugSample(sample int, msg string, fields ...zap.Field)
	* InfoSample(sample int, msg string, fields ...zap.Field)
	* WarnSample(sample int, msg string, fields ...zap.Field)
	 */
	Logger *LoggerType
)

// SampleRateDenominator sample rate = sample / SampleRateDenominator
const SampleRateDenominator = 1000

// LoggerType extend from zap.Logger
type LoggerType struct {
	*zap.Logger
	level zap.AtomicLevel
}

// NewLogger create new logger
func NewLogger(level string, opts ...zap.Option) (l *LoggerType, err error) {
	zl := zap.NewAtomicLevel()
	cfg := zap.Config{
		Level:            zl,
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	cfg.EncoderConfig.MessageKey = "message"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	zapLogger, err := cfg.Build(opts...)
	if err != nil {
		return nil, fmt.Errorf("build zap logger: %+v", err)
	}

	l = &LoggerType{
		Logger: zapLogger,
		level:  zl,
	}
	return l, l.ChangeLevel(level)
}

// ChangeLevel change logger level
func (l *LoggerType) ChangeLevel(level string) (err error) {
	switch level {
	case "debug":
		l.level.SetLevel(zap.DebugLevel)
	case "info":
		l.level.SetLevel(zap.InfoLevel)
	case "warn":
		l.level.SetLevel(zap.WarnLevel)
	case "error":
		l.level.SetLevel(zap.ErrorLevel)
	default:
		return fmt.Errorf("log level only be debug/info/warn/error")
	}

	return
}

// DebugSample emit debug log with propability sample/SampleRateDenominator.
// sample could be [0, 1000], less than 0 means never, great than 1000 means certainly
func (l *LoggerType) DebugSample(sample int, msg string, fields ...zap.Field) {
	if rand.Intn(SampleRateDenominator) > sample {
		return
	}

	l.Debug(msg, fields...)
}

// InfoSample emit info log with propability sample/SampleRateDenominator
func (l *LoggerType) InfoSample(sample int, msg string, fields ...zap.Field) {
	if rand.Intn(SampleRateDenominator) > sample {
		return
	}

	l.Info(msg, fields...)
}

// WarnSample emit warn log with propability sample/SampleRateDenominator
func (l *LoggerType) WarnSample(sample int, msg string, fields ...zap.Field) {
	if rand.Intn(SampleRateDenominator) > sample {
		return
	}

	l.Warn(msg, fields...)
}

func init() {
	var err error
	if Logger, err = NewLogger("info"); err != nil {
		panic(fmt.Sprintf("create logger: %+v", err))
	}
	Logger.Info("create logger", zap.String("level", "info"))
}

type alertMutation struct {
	TelegramMonitorAlert struct {
		Name graphql.String
	} `graphql:"TelegramMonitorAlert(type: $type, token: $token, msg: $msg)"`
}

// AlertPusher send alert to laisky's alert API
//
// https://github.com/Laisky/laisky-blog-graphql/tree/master/telegram
type AlertPusher struct {
	cli        *graphql.Client
	stopChan   chan struct{}
	senderChan chan *alertMsg

	token, alertType string

	pushAPI string
	timeout time.Duration
}

type alertMsg struct {
	alertType,
	pushToken,
	msg string
}

const (
	defaultAlertPusherTimeout = 10 * time.Second
)

// AlertPushOption is AlertPusher's options
type AlertPushOption func(*AlertPusher)

// WithAlertPushTimeout set AlertPusher HTTP timeout
func WithAlertPushTimeout(timeout time.Duration) AlertPushOption {
	return func(a *AlertPusher) {
		a.timeout = timeout
	}
}

// NewAlertPusher create new AlertPusher
func NewAlertPusher(ctx context.Context, pushAPI string, opts ...AlertPushOption) (a *AlertPusher) {
	Logger.Debug("create new AlertPusher", zap.String("pushAPI", pushAPI))
	a = &AlertPusher{
		stopChan:   make(chan struct{}),
		senderChan: make(chan *alertMsg, 100),

		timeout: defaultAlertPusherTimeout,
		pushAPI: pushAPI,
	}
	for _, opt := range opts {
		opt(a)
	}

	a.cli = graphql.NewClient(a.pushAPI, &http.Client{
		Timeout: a.timeout,
	})

	a.runSender(ctx)
	return a
}

// NewAlertPusherWithAlertType create new AlertPusher with default type and token
func NewAlertPusherWithAlertType(url string, alertType, pushToken string) *AlertPusher {
	Logger.Debug("create new AlertPusher", zap.String("url", url))
	return &AlertPusher{
		cli:        graphql.NewClient(url, httpClient),
		stopChan:   make(chan struct{}),
		senderChan: make(chan *alertMsg, 100),
		token:      pushToken,
		alertType:  alertType,
	}
}

// Close close AlertPusher
func (a *AlertPusher) Close() {
	close(a.senderChan)
	close(a.stopChan)
}

// SendWithType send alert with specific type, token and msg
func (a *AlertPusher) SendWithType(alertType, pushToken, msg string) (err error) {
	select {
	case a.senderChan <- &alertMsg{
		alertType: alertType,
		pushToken: pushToken,
		msg:       msg,
	}:
	default:
		return fmt.Errorf("send channel overflow")
	}

	return nil
}

func (a *AlertPusher) runSender(ctx context.Context) {
	var (
		payload *alertMsg
		err     error
		query   = new(alertMutation)
		vars    = map[string]interface{}{}
	)
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopChan:
			return
		case payload = <-a.senderChan:
		}

		vars["type"] = graphql.String(payload.alertType)
		vars["token"] = graphql.String(payload.pushToken)
		vars["msg"] = graphql.String(payload.msg)
		if err = a.cli.Mutate(ctx, query, vars); err != nil {
			Logger.Error("send alert mutation", zap.Error(err))
		}

		Logger.Debug("send telegram msg",
			zap.String("alert", payload.alertType),
			zap.String("msg", payload.msg))
	}
}

// Send send with default alertType and pushToken
func (a *AlertPusher) Send(msg string) (err error) {
	return a.SendWithType(a.alertType, a.token, msg)
}

const (
	defaultAlertHookLevel = zapcore.ErrorLevel
)

// AlertHook hook for zap.Logger
type AlertHook struct {
	pusher *AlertPusher
	level  zapcore.LevelEnabler
}

// AlertHookOption option for create AlertHook
type AlertHookOption func(*AlertHook)

// WithAlertHookLevel level to trigger AlertHook
func WithAlertHookLevel(level zapcore.LevelEnabler) AlertHookOption {
	return func(a *AlertHook) {
		a.level = level
	}
}

// NewAlertHook create AlertHook
func NewAlertHook(pusher *AlertPusher, opts ...AlertHookOption) *AlertHook {
	return &AlertHook{
		pusher: pusher,
		level:  defaultAlertHookLevel,
	}
}

// GetZapHook get hook for zap logger
func (a *AlertHook) GetZapHook() func(zapcore.Entry) error {
	return func(e zapcore.Entry) error {
		if !a.level.Enabled(e.Level) {
			return nil
		}
		msg := "logger: " + e.LoggerName + "\n"
		msg += "time: " + e.Time.Format(time.RFC3339Nano) + "\n"
		msg += "level: " + e.Level.String() + "\n"
		msg += "stack: " + e.Stack + "\n"
		msg += "message: " + e.Message + "\n"
		return a.pusher.Send(msg)
	}
}
