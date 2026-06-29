// 与真实 bindings/yx-daq/internal/app/index.ts 结构一致，re-export 所有 service mock
import * as CalibrationService from './calibrationservice'
import * as ConfigService from './configservice'
import * as DataService from './dataservice'
import * as DeviceService from './deviceservice'
import * as FiveHoleService from './fiveholeservice'
import * as MotionService from './motionservice'
import * as ThreeHoleService from './threeholeservice'

export {
  CalibrationService,
  ConfigService,
  DataService,
  DeviceService,
  FiveHoleService,
  MotionService,
  ThreeHoleService,
}
