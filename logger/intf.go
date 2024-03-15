package logger

type iLogBackend interface {
	InfoDepth(depth int, msg string)
	WarningDepth(depth int, msg string)
	ErrorDepth(depth int, msg string)
}
