package logger

// CommError 记录设备通信错误（仅写入 comm-YYYY-MM-DD.log，可通过配置关闭）。
// 仅通信失败时记录：写失败、超时、设备拒绝命令、读错误；正常收发不落盘，
// 避免高频采集帧淹没错误日志并造成日志文件快速膨胀。
func CommError(msg string, args ...any) {
	if !commOn.Load() {
		return
	}
	if l := commLogger.Load(); l != nil {
		l.Error(msg, args...)
	}
}
