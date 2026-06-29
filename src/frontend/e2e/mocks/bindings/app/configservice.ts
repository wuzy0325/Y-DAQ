/** E2E mock: ConfigService（配置持久化服务） */
import { mockState, __logCall } from '../../state'

function defaultThreeHoleConfig(): any {
  return {
    layout: {
      pattern: 'rectangle',
      origin: { x: 0, y: 0 },
      width: 100,
      height: 100,
      stepX: 10,
      stepY: 10,
    },
    speed: 20,
    points: [],
  }
}

export function LoadThreeHoleProbe1Config(): Promise<any> {
  __logCall('LoadThreeHoleProbe1Config', [])
  return Promise.resolve(mockState.threeHoleConfigs['probe1'] || defaultThreeHoleConfig())
}

export function LoadThreeHoleProbe2Config(): Promise<any> {
  __logCall('LoadThreeHoleProbe2Config', [])
  return Promise.resolve(mockState.threeHoleConfigs['probe2'] || defaultThreeHoleConfig())
}

export function SaveThreeHoleProbe1Config(config: any): Promise<void> {
  __logCall('SaveThreeHoleProbe1Config', [config])
  mockState.threeHoleConfigs['probe1'] = config
  return Promise.resolve()
}

export function SaveThreeHoleProbe2Config(config: any): Promise<void> {
  __logCall('SaveThreeHoleProbe2Config', [config])
  mockState.threeHoleConfigs['probe2'] = config
  return Promise.resolve()
}

export function SelectDataSavePath(): Promise<string> {
  __logCall('SelectDataSavePath', [])
  const path = 'D:/testdata/e2e'
  mockState.dataSavePath = path
  return Promise.resolve(path)
}

export function SetDataSavePath(path: string): Promise<void> {
  __logCall('SetDataSavePath', [path])
  mockState.dataSavePath = path
  return Promise.resolve()
}
