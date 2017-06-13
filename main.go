package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/satori/go.uuid"

	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

type lgrs map[loggerKey]*zap.Logger
type loggerKey int

const lotteryLoggerKey loggerKey = 1
const receivedLoggerKey loggerKey = 2

var loggers lgrs = map[loggerKey]*zap.Logger{
	lotteryLoggerKey:  newLogger("./lottery.log"),
	receivedLoggerKey: newLogger("./received.log"),
}

func main() {

	winRate := 0.9
	prize := 800

	addPrizeCh := make(chan int, 1000)
	changedPrizeCh := make(chan int, 1)

	ctx, _ := context.WithCancel(context.Background())

	startUpdatingPrize(ctx, prize, addPrizeCh, changedPrizeCh)

	for {

		fmt.Printf("prize: %d\n", prize)

		select {
		// update prize
		case v := <-changedPrizeCh:
			prize = v
		case <-ctx.Done():
			return
		default:
		}

		uid := uuid.NewV4().String()
		isWin := rand.Float64() <= winRate

		writeLotteryLog(loggers, uid, prize, isWin)

		time.Sleep(1 * time.Millisecond)

		if !isWin {
			continue
		}

		go func() {
			time.Sleep(time.Duration(rand.Intn(30)) * time.Second)

			if isReceivedByUser(prize) {
				addPrizeCh <- -10
				writeReceivedLog(loggers, uid)
			} else {
				addPrizeCh <- 5
			}
		}()
	}
}

func startUpdatingPrize(ctx context.Context, defaultPrize int, addPrizeCh chan int, changedPrizeCh chan int) {
	go func(prize int) {
		for {
			select {
			case a := <-addPrizeCh:

				prize += a

				select {
				case <-changedPrizeCh: // -> empty
				default:
				}

				changedPrizeCh <- prize

			case <-ctx.Done():
				return
			}
		}
	}(defaultPrize)
}

func writeLotteryLog(loggers lgrs, uid string, prize int, isWin bool) {
	loggers[lotteryLoggerKey].Info(
		"",
		zap.String("uid", uid),
		zap.Int("prize", prize),
		zap.Bool("isWin", isWin),
	)
}

func writeReceivedLog(loggers lgrs, uid string) {
	loggers[receivedLoggerKey].Info(
		"",
		zap.String("uid", uid),
	)
}

func isReceivedByUser(prize int) bool {
	return float64(prize)*(rand.Float64()+0.5) >= 1000
}

func newLogger(fileName string) *zap.Logger {

	config := zap.NewProductionConfig()
	config.EncoderConfig.MessageKey = "" // omitted
	config.EncoderConfig.LevelKey = ""

	enc := zapcore.NewJSONEncoder(config.EncoderConfig)

	sink := zapcore.AddSync(
		&lumberjack.Logger{
			Filename:   fileName,
			MaxSize:    500, // megabytes
			MaxBackups: 3,
			MaxAge:     3, //days
		},
	)

	return zap.New(
		zapcore.NewCore(enc, sink, config.Level),
	)
}
