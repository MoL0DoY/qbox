package log

import (
	"github.com/go-ozzo/ozzo-log"
	"fmt"
)

type LoggerService struct {
	logger *log.Logger
}

func (l *LoggerService) Open(devEnv bool, silentMode bool) error {
	l.logger = log.NewLogger()
	target := log.NewFileTarget()
	target.FileName = "app.log"

	if silentMode {
		target.MaxLevel = log.LevelError
	} else {
		if devEnv {
			target.MaxLevel = log.LevelDebug
		} else {
			target.MaxLevel = log.LevelInfo
		}
	}

	l.logger.Targets = append(l.logger.Targets, target)
	err := l.logger.Open()
	if err != nil {
		return err
	}
	l.Check("app")
	l.Info("Начато")
	return nil
}

func (l *LoggerService) Close() {
	l.Check("app")
	l.Info("Закончено")
	l.Info("------------------------")
	l.logger.Close()
}

func (l *LoggerService) Check(category string) {
	l.logger = l.logger.GetLogger(category)
}

func (l LoggerService) Info(format string, a ...interface{}) {
	if len(a) > 0 {
		l.logger.Info(format, a...)
	} else {
		l.logger.Info(format)
	}
}

func (l LoggerService) Fatal(format string, a ...interface{}) {
	if len(a) > 0 {
		l.logger.Emergency(format, a...)
		fmt.Printf(format+"\n", a)
	} else {
		l.logger.Emergency(format)
		fmt.Println(format)
	}
}

func (l LoggerService) Error(format string, a ...interface{}) {
	if len(a) > 0 {
		l.logger.Error(format, a...)
	} else {
		l.logger.Error(format)
	}
}

func (l LoggerService) Debug(format string, a ...interface{}) {
	if len(a) > 0 {
		l.logger.Debug(format, a...)
	} else {
		l.logger.Debug(format)
	}
}

func (l LoggerService) Notice(format string, a ...interface{}) {
	if len(a) > 0 {
		l.logger.Notice(format, a...)
	} else {
		l.logger.Notice(format)
	}
}
