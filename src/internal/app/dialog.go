package app

import "strings"

// isDialogCancelled 判断对话框错误是否为用户主动取消。
//
// Wails v3 在用户取消 SaveFile/OpenFile 对话框时返回内部 sentinel
// cfd.ErrorCancelled（Error() == "cancelled by user"），而非空字符串 + nil。
// cfd 是 internal 包无法直接 import，故按错误文本判定。
// 调用方应在收到该错误时静默返回 nil，避免把用户取消当作失败上报前端。
func isDialogCancelled(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cancelled by user")
}
