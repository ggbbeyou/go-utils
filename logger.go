package utils

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/Laisky/graphql"
	zap "github.com/Laisky/zap"
	"github.com/Laisky/zap/buffer"
	"github.com/Laisky/zap/zapcore"
	"github.com/pkg/errors"
)

const (
	// SampleRateDenominator sample rate = sample / SampleRateDenominator
	SampleRateDenominator = 1000

	defaultAlertPusherTimeout = 10 * time.Second
	defaultAlertPusherBufSize = 20
	defaultAlertHookLevel     = zapcore.ErrorLevel

	// LoggerLevelInfo Logger level info
	LoggerLevelInfo string = "info"
	// LoggerLevelDebug Logger level debug
	LoggerLevelDebug string = "debug"
	// LoggerLevelWarn Logger level warn
	LoggerLevelWarn string = "warn"
	// LoggerLevelError Logger level error
	LoggerLevelError string = "error"
	// LoggerLevelFatal Logger level fatal
	LoggerLevelFatal string = "fatal"
	// LoggerLevelPanic Logger level panic
	LoggerLevelPanic string = "panic"
)

var (
	/*Logger logging tool.

	* Info(msg string, fields ...Field)
	* Debug(msg string, fields ...Field)
	* Warn(msg string, fields ...Field)
	* Error(msg string, fields ...Field)
	* Panic(msg string, fields ...Field)
	* DebugSample(sample int, msg string, fields ...zapcore.Field)
	* InfoSample(sample int, msg string, fields ...zapcore.Field)
	* WarnSample(sample int, msg string, fields ...zapcore.Field)
	 */
	Logger LoggerItf
)

type zapLoggerItf interface {
	Debug(msg string, fields ...zapcore.Field)
	Info(msg string, fields ...zapcore.Field)
	Warn(msg string, fields ...zapcore.Field)
	Error(msg string, fields ...zapcore.Field)
	DPanic(msg string, fields ...zapcore.Field)
	Panic(msg string, fields ...zapcore.Field)
	Fatal(msg string, fields ...zapcore.Field)
	Sync() error
	Core() zapcore.Core
}

type LoggerItf interface {
	zapLoggerItf
	Level() zapcore.Level
	ChangeLevel(level string) (err error)
	DebugSample(sample int, msg string, fields ...zapcore.Field)
	InfoSample(sample int, msg string, fields ...zapcore.Field)
	WarnSample(sample int, msg string, fields ...zapcore.Field)
	Clone() LoggerItf
	Named(s string) LoggerItf
	With(fields ...zapcore.Field) LoggerItf
	WithOptions(opts ...zap.Option) LoggerItf
}

// LoggerType extend from zap.Logger
type LoggerType struct {
	*zap.Logger

	// level level of current logger
	//
	// zap logger do not expose api to change log's level,
	// so we have to save the pointer of zap.AtomicLevel.
	level zap.AtomicLevel
}

// CreateNewDefaultLogger set default utils.Logger
func CreateNewDefaultLogger(name, level string, opts ...zap.Option) (l LoggerItf, err error) {
	if l, err = NewLoggerWithName(name, level, opts...); err != nil {
		return nil, errors.Wrap(err, "create new logger")
	}

	Logger = l
	return Logger, nil
}

// NewLoggerWithName create new logger with name
func NewLoggerWithName(name, level string, opts ...zap.Option) (l LoggerItf, err error) {
	return NewLogger(
		WithLoggerName(name),
		WithLoggerEncoding(LoggerEncodingJSON),
		WithLoggerLevel(level),
		WithLoggerZapOptions(opts...),
	)
}

// NewConsoleLoggerWithName create new logger with name
func NewConsoleLoggerWithName(name, level string, opts ...zap.Option) (l LoggerItf, err error) {
	return NewLogger(
		WithLoggerName(name),
		WithLoggerEncoding(LoggerEncodingConsole),
		WithLoggerLevel(level),
		WithLoggerZapOptions(opts...),
	)
}

type loggerOption struct {
	zap.Config
	zapOptions []zap.Option
	Name       string
}

func (o *loggerOption) fillDefault() *loggerOption {
	o.Name = "app"
	o.Config = zap.Config{
		Level:            zap.NewAtomicLevel(),
		Development:      false,
		Encoding:         string(LoggerEncodingConsole),
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	o.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	o.EncoderConfig.MessageKey = "message"
	o.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	o.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return o
}

func (o *loggerOption) applyOpts(optfs ...LoggerOptFunc) (*loggerOption, error) {
	for _, optf := range optfs {
		if err := optf(o); err != nil {
			return nil, err
		}
	}

	return o, nil
}

type LoggerEncoding string

const (
	LoggerEncodingConsole = "console"
	LoggerEncodingJSON    = "json"
)

type LoggerOptFunc func(l *loggerOption) error

// WithLoggerOutputPaths set output path
//
// like "stdout"
func WithLoggerOutputPaths(paths []string) LoggerOptFunc {
	return func(c *loggerOption) error {
		c.OutputPaths = append(paths, "stdout")
		return nil
	}
}

// WithLoggerErrorOutputPaths set error logs output path
//
// like "stderr"
func WithLoggerErrorOutputPaths(paths []string) LoggerOptFunc {
	return func(c *loggerOption) error {
		c.ErrorOutputPaths = append(paths, "stderr")
		return nil
	}
}

// WithLoggerEncoding set logger encoding formet
func WithLoggerEncoding(format LoggerEncoding) LoggerOptFunc {
	return func(c *loggerOption) error {
		switch format {
		case LoggerEncodingConsole:
			c.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		case LoggerEncodingJSON:
			c.Encoding = string(LoggerEncodingJSON)
		default:
			return errors.Errorf("invalid format: %s", format)
		}

		return nil
	}
}

// WithLoggerZapOptions set logger with zap.Option
func WithLoggerZapOptions(opts ...zap.Option) LoggerOptFunc {
	return func(c *loggerOption) error {
		c.zapOptions = opts
		return nil
	}
}

// WithLoggerName set logger name
func WithLoggerName(name string) LoggerOptFunc {
	return func(c *loggerOption) error {
		c.Name = name
		return nil
	}
}

// ParseLoggerLevel
func ParseLoggerLevel(level string) (zapcore.Level, error) {
	switch level {
	case LoggerLevelInfo:
		return zap.InfoLevel, nil
	case LoggerLevelDebug:
		return zap.DebugLevel, nil
	case LoggerLevelWarn:
		return zap.WarnLevel, nil
	case LoggerLevelError:
		return zap.ErrorLevel, nil
	case LoggerLevelFatal:
		return zap.FatalLevel, nil
	case LoggerLevelPanic:
		return zap.PanicLevel, nil
	default:
		return 0, errors.Errorf("invalid level: %s", level)
	}
}

// WithLoggerLevel set logger level
func WithLoggerLevel(level string) LoggerOptFunc {
	return func(c *loggerOption) error {
		lvl, err := ParseLoggerLevel(level)
		if err != nil {
			return err
		}

		c.Level.SetLevel(lvl)
		return nil
	}
}

// NewLogger create new logger
func NewLogger(optfs ...LoggerOptFunc) (l LoggerItf, err error) {
	opt, err := new(loggerOption).fillDefault().applyOpts(optfs...)
	if err != nil {
		return nil, err
	}

	zapLogger, err := opt.Build(opt.zapOptions...)
	if err != nil {
		return nil, errors.Errorf("build zap logger: %+v", err)
	}
	zapLogger = zapLogger.Named(opt.Name)

	l = &LoggerType{
		Logger: zapLogger,
		level:  opt.Level,
	}

	return l, nil
}

// Level get current level of logger
func (l *LoggerType) Level() zapcore.Level {
	return l.level.Level()
}

// ChangeLevel change logger level
//
// all children logger share the same level of their parent logger,
// so if you change any logger's level, all its parent and
// children logger's level will be changed.
func (l *LoggerType) ChangeLevel(level string) (err error) {
	lvl, err := ParseLoggerLevel(level)
	if err != nil {
		return err
	}

	l.level.SetLevel(lvl)
	l.Debug("set logger level", zap.String("level", level))
	return
}

// DebugSample emit debug log with propability sample/SampleRateDenominator.
// sample could be [0, 1000], less than 0 means never, great than 1000 means certainly
func (l *LoggerType) DebugSample(sample int, msg string, fields ...zapcore.Field) {
	if rand.Intn(SampleRateDenominator) > sample {
		return
	}

	l.Debug(msg, fields...)
}

// InfoSample emit info log with propability sample/SampleRateDenominator
func (l *LoggerType) InfoSample(sample int, msg string, fields ...zapcore.Field) {
	if rand.Intn(SampleRateDenominator) > sample {
		return
	}

	l.Info(msg, fields...)
}

// WarnSample emit warn log with propability sample/SampleRateDenominator
func (l *LoggerType) WarnSample(sample int, msg string, fields ...zapcore.Field) {
	if rand.Intn(SampleRateDenominator) > sample {
		return
	}

	l.Warn(msg, fields...)
}

// Clone clone new Logger that inherit all config
func (l *LoggerType) Clone() LoggerItf {
	return &LoggerType{
		Logger: l.Logger.With(),
		level:  l.level,
	}
}

// Named adds a new path segment to the logger's name. Segments are joined by
// periods. By default, Loggers are unnamed.
func (l *LoggerType) Named(s string) LoggerItf {
	return &LoggerType{
		Logger: l.Logger.Named(s),
		level:  l.level,
	}
}

// With creates a child logger and adds structured context to it. Fields added
// to the child don't affect the parent, and vice versa.
func (l *LoggerType) With(fields ...zapcore.Field) LoggerItf {
	return &LoggerType{
		Logger: l.Logger.With(fields...),
		level:  l.level,
	}
}

// WithOptions clones the current Logger, applies the supplied Options, and
// returns the resulting Logger. It's safe to use concurrently.
func (l *LoggerType) WithOptions(opts ...zap.Option) LoggerItf {
	return &LoggerType{
		Logger: l.Logger.WithOptions(opts...),
		level:  l.level,
	}
}

func init() {
	var err error
	if Logger, err = NewConsoleLoggerWithName("go-utils", "info"); err != nil {
		panic(fmt.Sprintf("create logger: %+v", err))
	}

	Logger.Info("create logger", zap.String("level", "info"))
}

// ================================
// alert pusher hook
// ================================

type alertMutation struct {
	TelegramMonitorAlert struct {
		Name graphql.String
	} `graphql:"TelegramMonitorAlert(type: $type, token: $token, msg: $msg)"`
}

// AlertPusher send alert to laisky's alert API
//
// https://github.com/Laisky/laisky-blog-graphql/tree/master/telegram
type AlertPusher struct {
	*alertHookOption

	cli        *graphql.Client
	stopChan   chan struct{}
	senderChan chan *alertMsg

	token, alertType,
	pushAPI string
}

type alertHookOption struct {
	encPool *sync.Pool
	level   zapcore.LevelEnabler
	timeout time.Duration
}

func (o *alertHookOption) fillDefault() *alertHookOption {
	o.encPool = &sync.Pool{
		New: func() interface{} {
			return zapcore.NewJSONEncoder(zapcore.EncoderConfig{})
		},
	}
	o.level = defaultAlertHookLevel
	o.timeout = defaultAlertPusherTimeout
	return o
}

func (o *alertHookOption) applyOpts(opts ...AlertHookOptFunc) *alertHookOption {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// AlertHookOptFunc option for create AlertHook
type AlertHookOptFunc func(*alertHookOption)

// WithAlertHookLevel level to trigger AlertHook
func WithAlertHookLevel(level zapcore.Level) AlertHookOptFunc {
	if level.Enabled(zap.DebugLevel) {
		// because AlertPusher will use `debug` logger,
		// hook with debug will cause infinite recursive
		Logger.Panic("level should higher than debug")
	}
	if level.Enabled(zap.WarnLevel) {
		Logger.Warn("level is better higher than warn")
	}

	return func(a *alertHookOption) {
		a.level = level
	}
}

// WithAlertPushTimeout set AlertPusher HTTP timeout
func WithAlertPushTimeout(timeout time.Duration) AlertHookOptFunc {
	return func(a *alertHookOption) {
		a.timeout = timeout
	}
}

type alertMsg struct {
	alertType,
	pushToken,
	msg string
}

// NewAlertPusher create new AlertPusher
func NewAlertPusher(ctx context.Context, pushAPI string, opts ...AlertHookOptFunc) (a *AlertPusher, err error) {
	Logger.Debug("create new AlertPusher", zap.String("pushAPI", pushAPI))
	if pushAPI == "" {
		return nil, errors.Errorf("pushAPI should nout empty")
	}

	opt := new(alertHookOption).fillDefault().applyOpts(opts...)
	a = &AlertPusher{
		alertHookOption: opt,
		stopChan:        make(chan struct{}),
		senderChan:      make(chan *alertMsg, defaultAlertPusherBufSize),

		pushAPI: pushAPI,
	}

	a.cli = graphql.NewClient(a.pushAPI, &http.Client{
		Timeout: a.timeout,
	})

	go a.runSender(ctx)
	return a, nil
}

// NewAlertPusherWithAlertType create new AlertPusher with default type and token
func NewAlertPusherWithAlertType(ctx context.Context,
	pushAPI string,
	alertType,
	pushToken string,
	opts ...AlertHookOptFunc,
) (a *AlertPusher, err error) {
	Logger.Debug("create new AlertPusher with alert type",
		zap.String("pushAPI", pushAPI),
		zap.String("type", alertType))
	if a, err = NewAlertPusher(ctx, pushAPI, opts...); err != nil {
		return nil, err
	}

	a.alertType = alertType
	a.token = pushToken
	return a, nil
}

// Close close AlertPusher
func (a *AlertPusher) Close() {
	close(a.stopChan) // should close stopChan first
	close(a.senderChan)
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
		return errors.Errorf("send channel overflow")
	}

	return nil
}

func (a *AlertPusher) runSender(ctx context.Context) {
	var (
		ok      bool
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
		case payload, ok = <-a.senderChan:
			if !ok {
				return
			}
		}

		// only allow use debug level logger
		Logger.Debug("send alert", zap.String("type", payload.alertType))
		vars["type"] = graphql.String(payload.alertType)
		vars["token"] = graphql.String(payload.pushToken)
		vars["msg"] = graphql.String(payload.msg)
		if err = a.cli.Mutate(ctx, query, vars); err != nil {
			Logger.Debug("send alert mutation", zap.Error(err))
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

// GetZapHook get hook for zap logger
func (a *AlertPusher) GetZapHook() func(zapcore.Entry, []zapcore.Field) (err error) {
	return func(e zapcore.Entry, fs []zapcore.Field) (err error) {
		if !a.level.Enabled(e.Level) {
			return nil
		}

		var bb *buffer.Buffer
		enc := a.encPool.Get().(zapcore.Encoder)
		if bb, err = enc.EncodeEntry(e, fs); err != nil {
			Logger.Debug("zapcore encode fields got error", zap.Error(err))
			return nil
		}
		fsb := bb.String()
		bb.Reset()
		a.encPool.Put(enc)

		msg := "logger: " + e.LoggerName + "\n" +
			"time: " + e.Time.Format(time.RFC3339Nano) + "\n" +
			"level: " + e.Level.String() + "\n" +
			"caller: " + e.Caller.FullPath() + "\n" +
			"stack: " + e.Stack + "\n" +
			"message: " + e.Message + "\n" +
			fsb
		if err = a.Send(msg); err != nil {
			Logger.Debug("send alert got error", zap.Error(err))
			return nil
		}

		return nil
	}
}
