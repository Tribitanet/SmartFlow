package logger

import "log"

func Section(name string) {
	log.Printf("=== %s ===", name)
}

func Done(name string) {
	log.Printf("=== %s завершён ===", name)
}

func Info(format string, a ...any) {
	log.Printf("------ "+format, a...)
}

func Warn(format string, a ...any) {
	log.Printf("------ [WARN] "+format, a...)
}

func Error(format string, a ...any) {
	log.Printf("------ [ERROR] "+format, a...)
}
