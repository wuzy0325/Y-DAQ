import { CalibrationService, ConfigService, DataService, DeviceService, MotionService, ThreeHoleService } from '../../bindings/yx-daq/internal/app'
import type { AxisName, CalibrationConfig, DeviceProfile, MotionControllerProfile, ThreeHoleTraversalConfig } from '../../bindings/yx-daq/internal/types'

const defaultProbeID = 'probe1'

export const AddDeviceProfile = (profile: DeviceProfile) => DeviceService.AddDeviceProfile(profile)
export const AddMotionProfile = (profile: MotionControllerProfile) => MotionService.AddMotionProfile(profile)
export const ConnectDevice = (id: string) => DeviceService.ConnectDevice(id)
export const ConnectMotion = (id: string) => MotionService.ConnectMotion(id)
export const DisconnectDevice = (id: string) => DeviceService.DisconnectDevice(id)
export const DisconnectMotion = (id: string) => MotionService.DisconnectMotion(id)
export const ExportCalibrationPDF = () => DataService.ExportCalibrationPDF()
export const GetCalibrationStatus = () => CalibrationService.GetCalibrationStatus()
export const GetDataDir = () => DataService.GetDataDir()
export const GetDeviceProfiles = () => DeviceService.GetDeviceProfiles()
export const GetDeviceStatusAll = () => DeviceService.GetDeviceStatusAll()
export const GetLatestData = () => DeviceService.GetLatestData()
export const GetMotionProfiles = () => MotionService.GetMotionProfiles()
export const GetMotionStatusAll = () => MotionService.GetMotionStatusAll()
export const GetPublishRate = () => DataService.GetPublishRate()
export const GetThreeHoleCalibInfo = (probeID = defaultProbeID) => ThreeHoleService.GetThreeHoleCalibInfo(probeID)
export const GetThreeHoleTraversalStatus = (probeID = defaultProbeID) => ThreeHoleService.GetThreeHoleTraversalStatus(probeID)
export const IsRecording = () => DataService.IsRecording()
export const IsThreeHoleCalibLoaded = (probeID = defaultProbeID) => ThreeHoleService.IsThreeHoleCalibLoaded(probeID)
export const ListRecordingFiles = () => DataService.ListRecordingFiles()
export const LoadCSVFile = () => DataService.LoadCSVFile()
export const LoadThreeHoleCalibFiles = (probeIDOrFiles: string | string[], maybeFiles?: string[]) => {
  if (Array.isArray(probeIDOrFiles)) return ThreeHoleService.LoadThreeHoleCalibFiles(defaultProbeID, probeIDOrFiles)
  return ThreeHoleService.LoadThreeHoleCalibFiles(probeIDOrFiles, maybeFiles ?? [])
}
export const LoadThreeHoleConfig = (probeID = defaultProbeID) => {
  if (probeID === 'probe2') return ConfigService.LoadThreeHoleProbe2Config()
  return ConfigService.LoadThreeHoleProbe1Config()
}
export const MotionDefinePosition = (id: string, axis: AxisName, position: number) => MotionService.MotionDefinePosition(id, axis, position)
export const MotionEmergencyStop = (id: string) => MotionService.MotionEmergencyStop(id)
export const MotionGetLimitStatus = (id: string, axis: AxisName) => MotionService.MotionGetLimitStatus(id, axis)
export const MotionHome = (id: string, axis: AxisName) => MotionService.MotionHome(id, axis)
export const MotionIsAxisMoving = (id: string, axis: AxisName) => MotionService.MotionIsAxisMoving(id, axis)
export const MotionIsMoving = (id: string) => MotionService.MotionIsMoving(id)
export const MotionJog = (id: string, axis: AxisName, direction: number, distance: number, speed: number) => MotionService.MotionJog(id, axis, direction, distance, speed)
export const MotionMotorOff = (id: string) => MotionService.MotionMotorOff(id)
export const MotionMoveBy = (id: string, axis: AxisName, delta: number) => MotionService.MotionMoveBy(id, axis, delta)
export const MotionMoveTo = (id: string, axis: AxisName, position: number) => MotionService.MotionMoveTo(id, axis, position)
export const MotionSetAcceleration = (id: string, axis: AxisName, accel: number) => MotionService.MotionSetAcceleration(id, axis, accel)
export const MotionSetAxisDirection = (id: string, axis: AxisName, reverse: boolean) => MotionService.MotionSetAxisDirection(id, axis, reverse)
export const MotionSetDeceleration = (id: string, axis: AxisName, decel: number) => MotionService.MotionSetDeceleration(id, axis, decel)
export const MotionStop = (id: string, axis: AxisName) => MotionService.MotionStop(id, axis)
export const MotionStopAll = (id: string) => MotionService.MotionStopAll(id)
export const MotionWaitForComplete = (id: string, axis: AxisName, timeoutMs: number) => MotionService.MotionWaitForComplete(id, axis, timeoutMs)
export const PauseCalibration = () => CalibrationService.PauseCalibration()
export const PauseThreeHoleTraversal = (probeID = defaultProbeID) => ThreeHoleService.PauseThreeHoleTraversal(probeID)
export const ReadRecordingFile = (fileName: string) => DataService.ReadRecordingFile(fileName)
export const RemoveDeviceProfile = (id: string) => DeviceService.RemoveDeviceProfile(id)
export const RemoveMotionProfile = (id: string) => MotionService.RemoveMotionProfile(id)
export const ResumeCalibration = () => CalibrationService.ResumeCalibration()
export const ResumeThreeHoleTraversal = (probeID = defaultProbeID) => ThreeHoleService.ResumeThreeHoleTraversal(probeID)
export const SaveThreeHoleConfig = (probeIDOrConfig: string | ThreeHoleTraversalConfig, maybeConfig?: ThreeHoleTraversalConfig) => {
  if (typeof probeIDOrConfig === 'string') {
    const cfg = maybeConfig!
    if (probeIDOrConfig === 'probe2') return ConfigService.SaveThreeHoleProbe2Config(cfg)
    return ConfigService.SaveThreeHoleProbe1Config(cfg)
  }
  return ConfigService.SaveThreeHoleProbe1Config(probeIDOrConfig)
}
export const ScanDevices = () => DeviceService.ScanDevices()
export const SelectDataSavePath = () => ConfigService.SelectDataSavePath()
export const SelectThreeHoleCalibFiles = () => ThreeHoleService.SelectThreeHoleCalibFiles()
export const SetDataSavePath = (path: string) => ConfigService.SetDataSavePath(path)
export const SetPublishRate = (hz: number) => DataService.SetPublishRate(hz)
export const SetUnit = (id: string, unit: string) => DeviceService.SetUnit(id, unit)
export const StartAcquisition = (id: string) => DeviceService.StartAcquisition(id)
export const StartAcquisitionAll = () => DeviceService.StartAcquisitionAll()
export const StartCalibration = (config: CalibrationConfig) => CalibrationService.StartCalibration(config)
export const StartRecording = () => DataService.StartRecording()
export const StartThreeHoleRealtimeMonitor = (probeIDOrConfig: string | ThreeHoleTraversalConfig, maybeConfig?: ThreeHoleTraversalConfig) => {
  if (typeof probeIDOrConfig === 'string') return ThreeHoleService.StartThreeHoleRealtimeMonitor(probeIDOrConfig, maybeConfig!)
  return ThreeHoleService.StartThreeHoleRealtimeMonitor(defaultProbeID, probeIDOrConfig)
}
export const StartThreeHoleTraversal = (probeIDOrConfig: string | ThreeHoleTraversalConfig, maybeConfig?: ThreeHoleTraversalConfig) => {
  if (typeof probeIDOrConfig === 'string') return ThreeHoleService.StartThreeHoleTraversal(probeIDOrConfig, maybeConfig!)
  return ThreeHoleService.StartThreeHoleTraversal(defaultProbeID, probeIDOrConfig)
}
export const StopAcquisition = (id: string) => DeviceService.StopAcquisition(id)
export const StopAcquisitionAll = () => DeviceService.StopAcquisitionAll()
export const StopCalibration = () => CalibrationService.StopCalibration()
export const StopRecording = () => DataService.StopRecording()
export const StopThreeHoleRealtimeMonitor = (probeID = defaultProbeID) => ThreeHoleService.StopThreeHoleRealtimeMonitor(probeID)
export const StartThreeHoleRealtimeRecording = (probeID = defaultProbeID) => ThreeHoleService.StartThreeHoleRealtimeRecording(probeID)
export const StopThreeHoleRealtimeRecording = (probeID = defaultProbeID) => ThreeHoleService.StopThreeHoleRealtimeRecording(probeID)
export const IsThreeHoleRealtimeRecording = (probeID = defaultProbeID) => ThreeHoleService.IsThreeHoleRealtimeRecording(probeID)
export const StopThreeHoleTraversal = (probeID = defaultProbeID) => ThreeHoleService.StopThreeHoleTraversal(probeID)
export const UpdateDeviceProfile = (profile: DeviceProfile) => DeviceService.UpdateDeviceProfile(profile)
export const UpdateMotionProfile = (profile: MotionControllerProfile) => MotionService.UpdateMotionProfile(profile)

export const Shutdown = async () => undefined
export const Startup = async () => undefined
