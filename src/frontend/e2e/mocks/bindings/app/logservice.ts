/** E2E mock: LogService（日志查看与配置服务） */
import { mockState, __logCall } from '../../state'

export function GetLoggingConfig(): Promise<any> {
  __logCall('GetLoggingConfig', [])
  return Promise.resolve({ ...mockState.loggingConfig })
}

export function SetLoggingConfig(cfg: any): Promise<void> {
  __logCall('SetLoggingConfig', [cfg])
  mockState.loggingConfig = { ...cfg }
  return Promise.resolve()
}

export function ListLogFiles(): Promise<any[]> {
  __logCall('ListLogFiles', [])
  return Promise.resolve([])
}

export function ReadLogTail(fileName: string, maxLines: number): Promise<string> {
  __logCall('ReadLogTail', [fileName, maxLines])
  return Promise.resolve('')
}

export function GetLogDir(): Promise<string> {
  __logCall('GetLogDir', [])
  return Promise.resolve('C:/Users/test/.yx-daq/logs')
}

export function OpenLogDir(): Promise<void> {
  __logCall('OpenLogDir', [])
  return Promise.resolve()
}

export function ExportLogs(): Promise<string> {
  __logCall('ExportLogs', [])
  return Promise.resolve('')
}

export function ClearLogs(): Promise<void> {
  __logCall('ClearLogs', [])
  return Promise.resolve()
}

export function WriteFrontendError(level: string, message: string, stack: string): Promise<void> {
  __logCall('WriteFrontendError', [level, message, stack])
  return Promise.resolve()
}
