package main

import (
	"fmt"
	"os"
	"time"
)

// Called only after the native loop's deferred session/GPU/window cleanup.
// Keep this independent of the renderer: a lost device cannot draw an error UI.
func graphicsFailureMessage(cause error, logPath string) string {
	detail := fmt.Sprintf("图形系统已停止，游戏已退出。\n\n原因：%v\n\n已执行会话退出清理，但此提示不代表存档成功。请检查保存日志。", cause)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		_, err = fmt.Fprintf(f, "%s Native WebGPU failure: %v\n", time.Now().Format(time.RFC3339), cause)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return detail + fmt.Sprintf("\n\n诊断日志写入失败：%v", err)
	}
	return detail + "\n\n诊断日志：" + logPath
}
