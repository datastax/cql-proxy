package proxycore

import "go.uber.org/zap"

func GetOrCreateNopLogger(logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}
	return logger
}
