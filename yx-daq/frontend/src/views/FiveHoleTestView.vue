<template>
  <div class="five-hole-test-view">
    <!-- 顶部工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button type="primary" :disabled="store.isRunning" @click="showSettingsDialog = true">
          <el-icon><Setting /></el-icon> 设置
        </el-button>
        <el-tooltip
          v-if="!store.isRunning"
          :content="!store.allCalibLoaded ? '请先为所有启用的探针加载校准文件' : ''"
          :disabled="store.allCalibLoaded"
          placement="bottom"
        >
          <el-button type="success" :disabled="!store.allCalibLoaded" @click="store.startTest()">
            启动测试
          </el-button>
        </el-tooltip>
        <el-button v-if="store.isRunning && !store.isPaused" type="warning" @click="store.pauseTest()">暂停</el-button>
        <el-button v-if="store.isPaused" type="success" @click="store.resumeTest()">恢复</el-button>
        <el-button v-if="store.isRunning" type="danger" @click="store.stopTest()">停止</el-button>
      </div>
      <!-- 统一进度条 -->
      <div v-if="store.isRunning && store.progress" class="toolbar-progress">
        <el-progress
          :percentage="Math.round(store.progress.progress)"
          :stroke-width="10"
          color="#00f5ff"
        />
        <div class="test-progress-meta">
          <span class="meta-item">{{ store.progress.completedPoints }} / {{ store.progress.totalPoints }} 点</span>
          <span class="meta-item phase-badge" :class="store.progress.phase || 'acquiring'">{{ phaseLabel }}</span>
          <span class="meta-item">X={{ store.progress.currentX.toFixed(1) }}°  Y={{ store.progress.currentY.toFixed(1) }}°</span>
        </div>
      </div>
      <div class="toolbar-right">
        <span v-if="store.allCalibLoaded" class="calib-ok">校准就绪 ({{ store.enabledProbes.length }} 探针)</span>
        <span v-else class="calib-no">校准未就绪</span>
        <el-button
          :type="isRecording ? 'danger' : 'primary'"
          size="small"
          @click="isRecording ? handleStopRecording() : handleStartRecording()"
        >
          {{ isRecording ? '停止保存' : '实时保存' }}
        </el-button>
        <span class="status-text">{{ store.statusText }}</span>
      </div>
    </div>

    <!-- 错误信息 -->
    <div v-if="store.lastError" class="error-bar">
      <el-alert :title="store.lastError" type="error" :closable="true" show-icon @close="store.clearError()" />
    </div>

    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 共享布点预览 -->
      <GlassCard title="布点预览" icon="🗺️">
        <div class="point-legend">
          <span class="legend-item"><span class="legend-dot pending" />待测</span>
          <span class="legend-item"><span class="legend-dot moving" />移动</span>
          <span class="legend-item"><span class="legend-dot acquiring" />采集</span>
          <span class="legend-item"><span class="legend-dot waiting" />等待</span>
          <span class="legend-item"><span class="legend-dot completed" />完成</span>
        </div>
        <canvas ref="pointCanvasRef" class="point-canvas" width="440" height="320" />
        <div v-if="previewPoints.length > 10000" class="point-count-warning">
          ⚠ 布点数量 {{ previewPoints.length }} 较大，测试耗时可能很长
        </div>
      </GlassCard>

      <!-- 三探针面板 -->
      <div class="probes-row">
        <div
          v-for="probe in store.config.probes"
          :key="probe.probeId"
          class="probe-pane"
          :class="{ disabled: !probe.enabled }"
        >
          <GlassCard :title="probeLabels[probe.probeId] || probe.probeId" icon="🔬">
            <template #actions>
              <span class="calib-pill" :class="{ ok: store.calibLoadedMap[probe.probeId] }">
                {{ store.calibLoadedMap[probe.probeId] ? `校准 ${store.calibFilesMap[probe.probeId]?.length || 0}` : '无校准' }}
              </span>
              <el-tooltip :content="store.isRunning ? '运行中不可切换' : '启用/禁用探针'" placement="top">
                <el-switch v-model="probe.enabled" :disabled="store.isRunning" size="small" />
              </el-tooltip>
            </template>

            <!-- 实时压力值 -->
            <div class="realtime-section">
              <div class="section-label">原始压力</div>
              <div class="data-grid">
                <div class="data-item"><span class="label">P1</span><ValueDisplay :value="getProbeRaw(probe.probeId)?.p1" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P1)" color="#b829ff" /></div>
                <div class="data-item"><span class="label">P2</span><ValueDisplay :value="getProbeRaw(probe.probeId)?.p2" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P2)" color="#00f5ff" /></div>
                <div class="data-item"><span class="label">P3</span><ValueDisplay :value="getProbeRaw(probe.probeId)?.p3" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P3)" color="#00ff88" /></div>
                <div class="data-item"><span class="label">P4</span><ValueDisplay :value="getProbeRaw(probe.probeId)?.p4" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P4)" color="#ffaa00" /></div>
                <div class="data-item"><span class="label">P5</span><ValueDisplay :value="getProbeRaw(probe.probeId)?.p5" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P5)" color="#ff6b6b" /></div>
                <div class="data-item"><span class="label">P∞</span><ValueDisplay :value="getProbeRaw(probe.probeId)?.pAtm" :precision="getAtmPrecision('p')" /></div>
                <div class="data-item"><span class="label">T∞</span><ValueDisplay :value="getProbeRaw(probe.probeId)?.tAtm" :precision="getAtmPrecision('t')" unit="°C" /></div>
              </div>
            </div>

            <!-- 插值结果 -->
            <div class="interp-section">
              <div class="section-label">插值结果</div>
              <div class="data-grid">
                <div class="data-item"><span class="label">攻角 α</span><ValueDisplay :value="getProbeInterp(probe.probeId)?.alphaProbe" :precision="2" color="#00f5ff" unit="°" /></div>
                <div class="data-item"><span class="label">侧滑角 β</span><ValueDisplay :value="getProbeInterp(probe.probeId)?.betaProbe" :precision="2" color="#b829ff" unit="°" /></div>
                <div class="data-item"><span class="label">马赫数 Ma</span><ValueDisplay :value="getProbeInterp(probe.probeId)?.machProbe" :precision="4" color="#00ff88" /></div>
                <div class="data-item"><span class="label">速度 V</span><ValueDisplay :value="getProbeInterp(probe.probeId)?.velocityProbe" :precision="2" color="#ff6b6b" unit="m/s" /></div>
                <div class="data-item"><span class="label">总压 Pt</span><ValueDisplay :value="getProbeInterp(probe.probeId)?.ptProbe" :precision="3" color="#ffaa00" unit="Pa" /></div>
                <div class="data-item"><span class="label">静压 Ps</span><ValueDisplay :value="getProbeInterp(probe.probeId)?.psProbe" :precision="3" color="#00ff88" unit="Pa" /></div>
              </div>
            </div>

            <div class="probe-footer">
              <el-button
                type="primary"
                size="small"
                plain
                :disabled="!hasProbeResults(probe.probeId)"
                @click="store.exportProbeCSV(probe.probeId)"
              >
                导出 CSV
              </el-button>
            </div>
          </GlassCard>
        </div>
      </div>
    </div>

    <!-- ==================== 设置弹窗 ==================== -->
    <el-dialog v-model="showSettingsDialog" title="五孔测试设置" width="720px" :append-to-body="true" top="5vh" class="settings-dialog">
      <el-tabs>
        <!-- 测试配置 -->
        <el-tab-pane label="测试配置">
          <div class="settings-section">
            <div class="section-title">📝 基本信息</div>
            <el-form label-width="70px" size="small" class="compact-form">
              <el-form-item label="测试名称">
                <el-input v-model="store.config.name" placeholder="五孔移位测试" style="width: 320px" />
              </el-form-item>
            </el-form>
          </div>

          <div class="settings-section">
            <div class="section-title">📍 布点配置</div>
            <el-form label-width="70px" size="small" class="compact-form">
              <el-form-item label="布点模式">
                <el-select v-model="store.config.layout.pattern" style="width: 160px">
                  <el-option
                    v-for="(label, key) in TraversalPatternLabels"
                    :key="key"
                    :label="label"
                    :value="key"
                  />
                </el-select>
              </el-form-item>
            </el-form>

            <!-- 矩形布点 -->
            <template v-if="store.config.layout.pattern === TraversalPattern.RECTANGLE && store.config.layout.rectangle">
              <div class="form-row">
                <div class="form-group">
                  <label class="group-label">X范围</label>
                  <div class="range-inputs">
                    <el-input-number v-model="store.config.layout.rectangle.xMin" :step="5" size="small" style="width:90px" />
                    <span class="range-separator">~</span>
                    <el-input-number v-model="store.config.layout.rectangle.xMax" :step="5" size="small" style="width:90px" />
                  </div>
                </div>
                <div class="form-group">
                  <label class="group-label">Y范围</label>
                  <div class="range-inputs">
                    <el-input-number v-model="store.config.layout.rectangle.yMin" :step="5" size="small" style="width:90px" />
                    <span class="range-separator">~</span>
                    <el-input-number v-model="store.config.layout.rectangle.yMax" :step="5" size="small" style="width:90px" />
                  </div>
                </div>
              </div>
              <div class="form-row">
                <div class="form-group">
                  <label class="group-label">X步长</label>
                  <el-input-number v-model="rectXStep" :min="1" :step="1" size="small" style="width:100px" />
                </div>
                <div class="form-group">
                  <label class="group-label">Y步长</label>
                  <el-input-number v-model="rectYStep" :min="1" :step="1" size="small" style="width:100px" />
                </div>
              </div>
            </template>

            <!-- 直线布点 -->
            <template v-if="store.config.layout.pattern === TraversalPattern.LINE && store.config.layout.line">
              <div class="form-row">
                <div class="form-group">
                  <label class="group-label">起点</label>
                  <div class="point-inputs">
                    <el-input-number v-model="store.config.layout.line.startX" :step="5" size="small" style="width:90px" />
                    <el-input-number v-model="store.config.layout.line.startY" :step="5" size="small" style="width:90px" />
                  </div>
                </div>
                <div class="form-group">
                  <label class="group-label">终点</label>
                  <div class="point-inputs">
                    <el-input-number v-model="store.config.layout.line.endX" :step="5" size="small" style="width:90px" />
                    <el-input-number v-model="store.config.layout.line.endY" :step="5" size="small" style="width:90px" />
                  </div>
                </div>
              </div>
              <div class="form-row">
                <div class="form-group">
                  <label class="group-label">X步长</label>
                  <el-input-number v-model="lineXStep" :min="1" :step="1" size="small" style="width:100px" />
                </div>
                <div class="form-group">
                  <label class="group-label">Y步长</label>
                  <el-input-number v-model="lineYStep" :min="1" :step="1" size="small" style="width:100px" />
                </div>
              </div>
            </template>

            <!-- 自定义布点 -->
            <template v-if="store.config.layout.pattern === TraversalPattern.CUSTOM">
              <div class="custom-points-editor">
                <div
                  v-for="(pt, i) in customPoints"
                  :key="i"
                  class="custom-point-row"
                >
                  <span class="cp-index">{{ i + 1 }}</span>
                  <el-input-number v-model="pt.x" :step="1" size="small" style="width:110px" />
                  <el-input-number v-model="pt.y" :step="1" size="small" style="width:110px" />
                  <el-button size="small" type="danger" plain @click="removeCustomPoint(i)">删除</el-button>
                </div>
                <el-button size="small" type="primary" plain @click="addCustomPoint">+ 添加点</el-button>
              </div>
            </template>
          </div>

          <div class="settings-section">
            <div class="section-title">📊 采集参数</div>
            <div class="form-row">
              <div class="form-group">
                <label class="group-label">驻留时间</label>
                <el-input-number v-model="store.config.dwellTimeMs" :min="100" :step="100" size="small" style="width:100px" />
                <span class="unit-label">ms</span>
              </div>
              <div class="form-group">
                <label class="group-label">采样次数</label>
                <el-input-number v-model="store.config.samplesPerPoint" :min="1" :max="100" size="small" style="width:100px" />
              </div>
              <div class="form-group">
                <label class="group-label">采样间隔</label>
                <el-input-number v-model="store.config.sampleIntervalMs" :min="10" :step="10" size="small" style="width:100px" />
                <span class="unit-label">ms</span>
              </div>
              <div class="form-group">
                <label class="group-label">运动超时</label>
                <el-input-number v-model="store.config.motionTimeoutMs" :min="1000" :step="1000" size="small" style="width:100px" />
                <span class="unit-label">ms</span>
              </div>
            </div>
            <div class="form-row" style="margin-top: 8px">
              <div class="form-group" style="flex: 1">
                <label class="group-label">保存路径</label>
                <el-input v-model="store.config.savePath" placeholder="默认 ~/.yx-daq/recordings/" size="small" clearable>
                  <template #append>
                    <el-button :icon="FolderOpened" @click="browseSavePath" />
                  </template>
                </el-input>
              </div>
              <div class="form-group" style="flex: 1">
                <label class="group-label">文件名</label>
                <el-input v-model="store.config.saveFileName" placeholder="FiveHoleTraversal-xxx" size="small" clearable />
              </div>
            </div>
          </div>
        </el-tab-pane>

        <!-- 探针配置 -->
        <el-tab-pane label="探针配置">
          <!-- 全局 PAtm/TAtm 数据源 -->
          <div class="settings-section">
            <div class="section-title">🌐 大气压/温度数据源（全局共享）</div>
            <div class="form-row">
              <div class="form-group" style="flex:1">
                <label class="group-label">大气压 P∞ 设备</label>
                <el-select v-model="store.config.pAtmDeviceId" placeholder="选择采集设备" size="small" clearable filterable style="width: 200px">
                  <el-option v-for="dev in deviceStore.profiles" :key="dev.id" :label="`${dev.name} (${dev.type})`" :value="dev.id" />
                </el-select>
              </div>
              <div class="form-group">
                <label class="group-label">P∞ 通道</label>
                <el-input-number v-model="store.config.pAtmChannel" :min="0" :max="getMaxChannel(store.config.pAtmDeviceId)" size="small" controls-position="right" style="width:90px" />
              </div>
              <div class="form-group" style="flex:1">
                <label class="group-label">大气温度 T∞ 设备</label>
                <el-select v-model="store.config.tAtmDeviceId" placeholder="选择采集设备" size="small" clearable filterable style="width: 200px">
                  <el-option v-for="dev in deviceStore.profiles" :key="dev.id" :label="`${dev.name} (${dev.type})`" :value="dev.id" />
                </el-select>
              </div>
              <div class="form-group">
                <label class="group-label">T∞ 通道</label>
                <el-input-number v-model="store.config.tAtmChannel" :min="0" :max="getMaxChannel(store.config.tAtmDeviceId)" size="small" controls-position="right" style="width:90px" />
              </div>
            </div>
          </div>

          <!-- 每探针配置 -->
          <div
            v-for="probe in store.config.probes"
            :key="probe.probeId"
            class="settings-section probe-config-section"
          >
            <div class="probe-config-header">
              <div class="section-title" style="margin-bottom: 0; flex: 1">
                🔬 {{ probeLabels[probe.probeId] || probe.probeId }}
                <el-switch v-model="probe.enabled" size="small" style="margin-left: 10px" />
              </div>
              <el-button size="small" type="primary" plain @click="store.selectCalibFiles(probe.probeId)">
                选择校准文件
              </el-button>
            </div>
            <div class="calib-status-row">
              <span v-if="store.calibLoadedMap[probe.probeId]" class="calib-ok">
                ✓ 已加载 {{ store.calibFilesMap[probe.probeId]?.length || 0 }} 个校准文件
              </span>
              <span v-else class="calib-no">未加载校准文件</span>
            </div>

            <!-- 通道映射 P1-P5 -->
            <div class="channel-block">
              <div class="block-label">通道映射 (P1-P5)</div>
              <el-table :data="probe.probeChannels" size="small" class="channel-table" :header-cell-style="{background:'rgba(255,255,255,0.05)'}">
                <el-table-column label="通道" width="70">
                  <template #default="{ row }">
                    {{ FiveHoleChannelRoleLabels[row.role as FiveHoleChannelRoleValue] || row.role }}
                  </template>
                </el-table-column>
                <el-table-column label="采集设备">
                  <template #default="{ row }">
                    <el-select v-model="row.deviceId" placeholder="选择设备" size="small" clearable filterable style="width: 100%">
                      <el-option v-for="dev in deviceStore.profiles" :key="dev.id" :label="`${dev.name} (${dev.type})`" :value="dev.id" />
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="通道号" width="100">
                  <template #default="{ row }">
                    <el-input-number v-model="row.channel" :min="0" :max="getMaxChannel(row.deviceId)" size="small" controls-position="right" style="width:85px" />
                  </template>
                </el-table-column>
                <el-table-column label="启用" width="60" align="center">
                  <template #default="{ row }">
                    <el-switch v-model="row.enabled" size="small" />
                  </template>
                </el-table-column>
              </el-table>
            </div>

            <!-- 运动轴映射 -->
            <div class="channel-block">
              <div class="block-label">运动轴映射</div>
              <div class="form-row">
                <div class="form-group" style="flex:1">
                  <label class="group-label">α 轴 控制器</label>
                  <el-select v-model="probe.motionAlpha.controllerId" placeholder="选择运动控制器" size="small" clearable filterable style="width: 100%">
                    <el-option v-for="mc in motionStore.profiles" :key="mc.id" :label="`${mc.name} (${mc.type})`" :value="mc.id" />
                  </el-select>
                </div>
                <div class="form-group">
                  <label class="group-label">α 轴</label>
                  <el-select v-model="probe.motionAlpha.axis" size="small" style="width: 70px">
                    <el-option v-for="axis in getAxisOptions(probe.motionAlpha.controllerId)" :key="axis" :label="axis" :value="axis" />
                  </el-select>
                </div>
                <div class="form-group" style="flex:1">
                  <label class="group-label">β 轴 控制器</label>
                  <el-select v-model="probe.motionBeta.controllerId" placeholder="选择运动控制器" size="small" clearable filterable style="width: 100%">
                    <el-option v-for="mc in motionStore.profiles" :key="mc.id" :label="`${mc.name} (${mc.type})`" :value="mc.id" />
                  </el-select>
                </div>
                <div class="form-group">
                  <label class="group-label">β 轴</label>
                  <el-select v-model="probe.motionBeta.axis" size="small" style="width: 70px">
                    <el-option v-for="axis in getAxisOptions(probe.motionBeta.controllerId)" :key="axis" :label="axis" :value="axis" />
                  </el-select>
                </div>
              </div>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <el-button type="primary" @click="store.saveConfig(); showSettingsDialog = false">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { Setting, FolderOpened } from '@element-plus/icons-vue'
import { useDeviceStore } from '../stores/device'
import { useMotionStore } from '../stores/motion'
import { useFiveHoleTestStore } from '../stores/fiveHoleTest'
import { SelectDataSavePath, IsFiveHoleRealtimeRecording } from '../wails-compat/app'
import {
  TraversalPattern,
  TraversalPatternLabels,
  FiveHoleChannelRole,
  FiveHoleChannelRoleLabels,
  AxisName,
  getTotalChannelCount,
  type FiveHoleChannelRoleValue,
  type AxisNameValue,
  type DeviceTypeValue,
} from '../api/enums'
import GlassCard from '../components/GlassCard.vue'
import ValueDisplay from '../components/ValueDisplay.vue'
import type {
  FiveHoleRawData,
  FiveHoleInterpolationResult,
} from '../stores/fiveHoleTest/types'

const store = useFiveHoleTestStore()
const deviceStore = useDeviceStore()
const motionStore = useMotionStore()

const probeLabels: Record<string, string> = {
  probe1: '探针 1',
  probe2: '探针 2',
  probe3: '探针 3',
}

// ==================== 实时数据提取 ====================
function getProbeRealtime(probeId: string) {
  return store.realtime?.probeRealtime.find(p => p.probeId === probeId)
}
function getProbeRaw(probeId: string): FiveHoleRawData | undefined {
  return getProbeRealtime(probeId)?.rawData
}
function getProbeInterp(probeId: string): FiveHoleInterpolationResult | undefined {
  return getProbeRealtime(probeId)?.interpResult
}

// 通道精度（按探针 + 角色）
function getChPrecision(probeId: string, role: FiveHoleChannelRoleValue): number {
  const probe = store.config.probes.find(p => p.probeId === probeId)
  if (!probe) return 3
  const ch = probe.probeChannels.find(c => c.role === role)
  if (!ch) return 3
  return getDeviceChannelPrecision(ch.deviceId, ch.channel)
}
// 全局 P∞/T∞ 精度
function getAtmPrecision(which: 'p' | 't'): number {
  const deviceId = which === 'p' ? store.config.pAtmDeviceId : store.config.tAtmDeviceId
  const channel = which === 'p' ? store.config.pAtmChannel : store.config.tAtmChannel
  return getDeviceChannelPrecision(deviceId, channel)
}
function getDeviceChannelPrecision(deviceId: string, channel: number): number {
  const profile = deviceStore.profiles.find(p => p.id === deviceId)
  if (!profile) return 3
  return profile.channels[channel]?.precision ?? 3
}

// CSV 导出可用性
function hasProbeResults(probeId: string): boolean {
  return (store.completeProbeDataPoints?.[probeId]?.length ?? 0) > 0
}

// ==================== 实时保存 ====================
const isRecording = ref(false)

async function handleStartRecording() {
  try {
    const filePath = await store.selectAndStartRealtimeRecording()
    if (!filePath) return // 用户取消
    isRecording.value = true
    ElMessage.success(`已开始保存: ${filePath}`)
  } catch (e: any) {
    ElMessage.error(`开始保存失败: ${e?.message || e}`)
  }
}

async function handleStopRecording() {
  try {
    await store.stopRealtimeRecording()
    isRecording.value = false
    ElMessage.success('已停止保存')
  } catch (e: any) {
    ElMessage.error(`停止保存失败: ${e?.message || e}`)
  }
}

async function browseSavePath() {
  try {
    const dir = await SelectDataSavePath() as string
    if (dir) {
      store.config.savePath = dir
    }
  } catch (e) {
    console.error('browseSavePath failed:', e)
  }
}

// ==================== 设置弹窗 ====================
const showSettingsDialog = ref(false)

// 设备最大通道号
function getMaxChannel(deviceId: string): number {
  if (!deviceId) return 17
  const profile = deviceStore.profiles.find(p => p.id === deviceId)
  if (!profile) return 17
  return getTotalChannelCount(profile.type as DeviceTypeValue) - 1
}

// 运动控制器可用轴
function getAxisOptions(controllerId: string): AxisNameValue[] {
  if (!controllerId) return [AxisName.X, AxisName.Y]
  const profile = motionStore.profiles.find(p => p.id === controllerId)
  if (profile?.axes?.length) {
    const axes = profile.axes.filter(a => a.enabled).map(a => a.name as AxisNameValue)
    return axes.length > 0 ? axes : [AxisName.X, AxisName.Y]
  }
  return [AxisName.X, AxisName.Y]
}

// 阶段标签
const phaseLabel = computed(() => {
  const map: Record<string, string> = {
    starting: '启动中',
    moving: '移动中',
    waiting: '等待中',
    acquiring: '采集中',
    acquired: '已采集',
  }
  return map[store.progress?.phase || ''] || '采集中'
})

// 步长快捷设置（同步到 xSteps/ySteps 分段）
const rectXStep = ref(5)
const rectYStep = ref(5)
const lineXStep = ref(5)
const lineYStep = ref(5)

watch(
  () => {
    const r = store.config.layout.rectangle
    if (!r) return null
    return { xMin: r.xMin, xMax: r.xMax, yMin: r.yMin, yMax: r.yMax, xs: rectXStep.value, ys: rectYStep.value }
  },
  (val) => {
    if (!val) return
    const r = store.config.layout.rectangle
    if (!r) return
    r.xSteps = [{ start: val.xMin, end: val.xMax, step: val.xs }]
    r.ySteps = [{ start: val.yMin, end: val.yMax, step: val.ys }]
  },
  { immediate: true }
)

watch(
  () => {
    const l = store.config.layout.line
    if (!l) return null
    return { startX: l.startX, startY: l.startY, endX: l.endX, endY: l.endY, xs: lineXStep.value, ys: lineYStep.value }
  },
  (val) => {
    if (!val) return
    const l = store.config.layout.line
    if (!l) return
    l.xSteps = [{ start: l.startX, end: l.endX, step: val.xs }]
    l.ySteps = [{ start: l.startY, end: l.endY, step: val.ys }]
  },
  { immediate: true }
)

// 自定义点编辑器（双向绑定到 store.config.layout.customPoints）
const customPoints = computed({
  get: () => store.config.layout.customPoints ?? [],
  set: (val) => { store.config.layout.customPoints = val },
})

function addCustomPoint() {
  if (!store.config.layout.customPoints) store.config.layout.customPoints = []
  store.config.layout.customPoints.push({ id: `pt-${Date.now()}`, x: 0, y: 0 })
}
function removeCustomPoint(index: number) {
  if (!store.config.layout.customPoints) return
  store.config.layout.customPoints.splice(index, 1)
}

// ==================== 布点预览 Canvas ====================
const pointCanvasRef = ref<HTMLCanvasElement>()

type PointState = 'pending' | 'moving' | 'acquiring' | 'waiting' | 'completed'

const previewPoints = computed(() => {
  const layout = store.config.layout
  const points: { x: number; y: number; state: PointState }[] = []

  if (layout.pattern === 'rectangle' && layout.rectangle) {
    const r = layout.rectangle
    const xValues = expandSteps(r.xSteps)
    const yValues = expandSteps(r.ySteps)
    if (xValues.length === 0) { xValues.push(r.xMin, r.xMax) }
    if (yValues.length === 0) { yValues.push(r.yMin, r.yMax) }
    for (const x of xValues) {
      for (const y of yValues) {
        points.push({ x, y, state: 'pending' })
      }
    }
  } else if (layout.pattern === 'line' && layout.line) {
    const l = layout.line
    const xValues = expandSteps(l.xSteps)
    const yValues = expandSteps(l.ySteps)
    if (xValues.length === 0 && yValues.length === 0) {
      points.push({ x: l.startX, y: l.startY, state: 'pending' })
      points.push({ x: l.endX, y: l.endY, state: 'pending' })
    } else {
      if (yValues.length === 0) { yValues.push(l.startY) }
      if (xValues.length === 0) { xValues.push(l.startX) }
      for (const x of xValues) {
        for (const y of yValues) {
          points.push({ x, y, state: 'pending' })
        }
      }
    }
  } else if (layout.pattern === 'custom' && layout.customPoints) {
    for (const pt of layout.customPoints) {
      points.push({ x: pt.x, y: pt.y, state: 'pending' })
    }
  }

  // 统一进度：已完成点数（所有启用探针均已完成的点）
  const completedCount = store.progress?.completedPoints ?? 0
  for (let i = 0; i < points.length && i < completedCount; i++) {
    points[i].state = 'completed'
  }
  // 标记当前正在处理的点
  const currentPoint = store.progress
  if (currentPoint && store.isRunning && completedCount < points.length) {
    const phase = currentPoint.phase
    points[completedCount].state = (phase === 'moving' || phase === 'waiting' || phase === 'acquiring') ? phase : 'acquiring'
  }

  return points
})

// expandSteps 展开分段步长为具体数值列表
// 与后端 point_generator.go::expandStepSegments 算法保持一致：
// - 先 skip start>end / step<0；step==0 时 push (start, end)
// - 用整数步数计算 n = floor((end-start)/step + 0.5)，避免浮点累加误差
// - 不再额外四舍五入数值，与后端 seg.Start+float64(i)*seg.Step 完全对齐
function expandSteps(steps: { start: number; end: number; step: number }[]): number[] {
  const values: number[] = []
  const MAX_POINTS = 50000
  for (const seg of steps) {
    if (seg.start > seg.end) continue
    if (seg.step === 0) {
      values.push(seg.start, seg.end)
      continue
    }
    if (seg.step < 0) continue
    const n = Math.floor((seg.end - seg.start) / seg.step + 0.5)
    if (n < 0 || n > MAX_POINTS) continue
    for (let i = 0; i <= n; i++) {
      values.push(seg.start + i * seg.step)
    }
  }
  return values
}

const CANVAS_W = 440
const CANVAS_H = 320

function drawPointCanvas() {
  const canvas = pointCanvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = window.devicePixelRatio || 1
  const w = CANVAS_W
  const h = CANVAS_H
  canvas.width = w * dpr
  canvas.height = h * dpr
  canvas.style.width = w + 'px'
  canvas.style.height = h + 'px'
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)

  ctx.clearRect(0, 0, w, h)

  const points = previewPoints.value
  if (points.length === 0) {
    ctx.fillStyle = 'rgba(255,255,255,0.3)'
    ctx.font = '14px sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText('请先配置布点参数', w / 2, h / 2)
    return
  }

  const xs = points.map(p => p.x)
  const ys = points.map(p => p.y)
  const xMin = Math.min(...xs)
  const xMax = Math.max(...xs)
  const yMin = Math.min(...ys)
  const yMax = Math.max(...ys)
  const xRange = xMax - xMin || 1
  const yRange = yMax - yMin || 1

  const padLeft = 44
  const padRight = 16
  const padTop = 12
  const padBottom = 36
  const plotW = w - padLeft - padRight
  const plotH = h - padTop - padBottom

  const toCanvasX = (x: number) => padLeft + ((x - xMin) / xRange) * plotW
  const toCanvasY = (y: number) => padTop + plotH - ((y - yMin) / yRange) * plotH

  function niceStep(range: number, targetTicks: number): number {
    const rough = range / targetTicks
    const mag = Math.pow(10, Math.floor(Math.log10(rough)))
    const norm = rough / mag
    let nice: number
    if (norm <= 1.5) nice = 1
    else if (norm <= 3.5) nice = 2
    else if (norm <= 7.5) nice = 5
    else nice = 10
    return nice * mag
  }
  function formatTick(value: number, step: number): string {
    if (step >= 1) return Math.round(value).toString()
    const decimals = Math.max(0, -Math.floor(Math.log10(step)))
    return value.toFixed(decimals)
  }

  const xStep = niceStep(xRange, 6)
  const yStep = niceStep(yRange, 6)

  ctx.strokeStyle = 'rgba(255,255,255,0.06)'
  ctx.lineWidth = 1
  for (let x = Math.ceil(xMin / xStep) * xStep; x <= xMax + xStep * 0.01; x += xStep) {
    const cx = toCanvasX(x)
    ctx.beginPath()
    ctx.moveTo(cx, padTop)
    ctx.lineTo(cx, h - padBottom)
    ctx.stroke()
  }
  for (let y = Math.ceil(yMin / yStep) * yStep; y <= yMax + yStep * 0.01; y += yStep) {
    const cy = toCanvasY(y)
    ctx.beginPath()
    ctx.moveTo(padLeft, cy)
    ctx.lineTo(w - padRight, cy)
    ctx.stroke()
  }

  ctx.strokeStyle = 'rgba(255,255,255,0.15)'
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.moveTo(padLeft, padTop)
  ctx.lineTo(padLeft, h - padBottom)
  ctx.lineTo(w - padRight, h - padBottom)
  ctx.stroke()

  ctx.fillStyle = 'rgba(255,255,255,0.55)'
  ctx.font = '10px sans-serif'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  for (let x = Math.ceil(xMin / xStep) * xStep; x <= xMax + xStep * 0.01; x += xStep) {
    ctx.fillText(formatTick(x, xStep), toCanvasX(x), h - padBottom + 5)
  }
  ctx.textAlign = 'right'
  ctx.textBaseline = 'middle'
  for (let y = Math.ceil(yMin / yStep) * yStep; y <= yMax + yStep * 0.01; y += yStep) {
    ctx.fillText(formatTick(y, yStep), padLeft - 6, toCanvasY(y))
  }

  ctx.fillStyle = 'rgba(255,255,255,0.45)'
  ctx.font = '11px sans-serif'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  ctx.fillText('X (mm)', padLeft + plotW / 2, h - padBottom + 18)
  ctx.save()
  ctx.translate(12, padTop + plotH / 2)
  ctx.rotate(-Math.PI / 2)
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  ctx.fillText('Y (mm)', 0, 0)
  ctx.restore()

  const stateColors: Record<PointState, string> = {
    pending: 'rgba(255,255,255,0.35)',
    moving: '#ffaa00',
    acquiring: '#00f5ff',
    waiting: '#ff3366',
    completed: '#00ff88',
  }
  const stateGlow: Record<PointState, string> = {
    pending: 'transparent',
    moving: 'rgba(255,170,0,0.4)',
    acquiring: 'rgba(0,245,255,0.4)',
    waiting: 'rgba(255,51,102,0.4)',
    completed: 'rgba(0,255,136,0.3)',
  }
  const stateBorder: Record<PointState, string> = {
    pending: 'rgba(255,255,255,0.2)',
    moving: 'rgba(255,170,0,0.6)',
    acquiring: 'rgba(0,245,255,0.6)',
    waiting: 'rgba(255,51,102,0.6)',
    completed: 'rgba(0,255,136,0.6)',
  }

  const pointRadius = Math.max(2.5, Math.min(5, 120 / Math.sqrt(points.length)))
  for (const pt of points) {
    const cx = toCanvasX(pt.x)
    const cy = toCanvasY(pt.y)
    const color = stateColors[pt.state]
    const glow = stateGlow[pt.state]
    const border = stateBorder[pt.state]

    if (glow !== 'transparent') {
      ctx.shadowColor = glow
      ctx.shadowBlur = 8
    } else {
      ctx.shadowColor = 'transparent'
      ctx.shadowBlur = 0
    }

    ctx.beginPath()
    ctx.arc(cx, cy, pointRadius, 0, Math.PI * 2)
    ctx.fillStyle = color
    ctx.fill()

    ctx.strokeStyle = border
    ctx.lineWidth = 1
    ctx.stroke()

    ctx.shadowColor = 'transparent'
    ctx.shadowBlur = 0
  }
}

watch(
  [previewPoints, pointCanvasRef, () => store.config.layout],
  () => {
    nextTick(drawPointCanvas)
  },
  { deep: true }
)

// ==================== 生命周期 ====================
const isInitializing = ref(true)

onMounted(async () => {
  store.startListening()
  await store.loadConfig()
  isInitializing.value = false
  await store.startRealtimeMonitor()
  deviceStore.fetchProfiles()
  deviceStore.fetchStatuses()
  motionStore.fetchProfiles()
  motionStore.fetchStatuses()
  nextTick(drawPointCanvas)
  try {
    isRecording.value = await IsFiveHoleRealtimeRecording()
  } catch (e) {
    console.warn('IsFiveHoleRealtimeRecording failed:', e)
  }
})

onUnmounted(() => {
  store.stopListening()
  store.stopRealtimeMonitor()
  // 清理防抖定时器，避免组件卸载后残留回调触发后端监控重启
  if (configSaveTimer) {
    clearTimeout(configSaveTimer)
    configSaveTimer = null
  }
})

// 配置变更时自动保存（防抖），并重启实时监控以应用最新配置
let configSaveTimer: number | null = null
watch(() => store.config, () => {
  if (isInitializing.value) return
  if (configSaveTimer) clearTimeout(configSaveTimer)
  configSaveTimer = window.setTimeout(() => {
    store.saveConfig()
    if (!store.isRunning) {
      store.stopRealtimeMonitor()
      store.startRealtimeMonitor()
    }
  }, 500)
}, { deep: true })
</script>

<style lang="scss" scoped>
.five-hole-test-view {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: rgba(255,255,255,0.04);
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.08);
  gap: 12px;
}

.toolbar-left, .toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.toolbar-progress {
  flex: 1;
  max-width: 420px;
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 6px;
  padding: 6px 10px;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.toolbar-progress :deep(.el-progress-bar__outer) {
  border-radius: 3px;
  background-color: rgba(255,255,255,0.06) !important;
}

.toolbar-progress :deep(.el-progress__text) {
  font-size: 11px;
  color: #fff;
  min-width: 32px;
}

.calib-ok { font-size: 12px; color: #00ff88; }
.calib-no { font-size: 12px; color: rgba(255,255,255,0.4); }

.status-text {
  font-size: 12px;
  color: rgba(255,255,255,0.65);
  padding: 2px 8px;
  background: rgba(0,0,0,0.2);
  border-radius: 4px;
}

.error-bar { margin: 0; }

.test-progress-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 11px;
  color: rgba(255,255,255,0.5);
}

.test-progress-meta .meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.phase-badge {
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  color: #fff;
}
.phase-badge.moving   { background: rgba(255,170,0,0.35); color: #ffcc66; }
.phase-badge.waiting  { background: rgba(255,51,102,0.35); color: #ff7799; }
.phase-badge.acquiring{ background: rgba(0,245,255,0.25); color: #66f5ff; }

.main-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

// 布点预览
.point-legend {
  display: flex;
  gap: 10px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.point-count-warning {
  margin-top: 6px;
  font-size: 11px;
  color: #ff9800;
  text-align: center;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 10px;
  color: rgba(255,255,255,0.55);
}

.legend-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;

  &.pending { background: rgba(255,255,255,0.35); border: 1px solid rgba(255,255,255,0.2); }
  &.moving { background: #ffaa00; box-shadow: 0 0 3px rgba(255,170,0,0.5); }
  &.acquiring { background: #00f5ff; box-shadow: 0 0 3px rgba(0,245,255,0.5); }
  &.waiting { background: #ff3366; box-shadow: 0 0 3px rgba(255,51,102,0.5); }
  &.completed { background: #00ff88; box-shadow: 0 0 3px rgba(0,255,136,0.4); }
}

.point-canvas {
  display: block;
  margin: 0 auto;
  border-radius: 8px;
  background: rgba(0,0,0,0.15);
}

// 三探针面板
.probes-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.probe-pane {
  flex: 1 1 33%;
  min-width: 280px;
  display: flex;
  flex-direction: column;

  &.disabled {
    opacity: 0.5;
  }
}

.calib-pill {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 8px;
  background: rgba(255,255,255,0.08);
  color: rgba(255,255,255,0.5);
  margin-right: 6px;

  &.ok {
    background: rgba(0,255,136,0.15);
    color: #00ff88;
  }
}

.realtime-section, .interp-section {
  margin-bottom: 10px;
}

.section-label {
  font-size: 11px;
  color: rgba(255,255,255,0.4);
  text-transform: uppercase;
  letter-spacing: 1px;
  margin-bottom: 6px;
}

.data-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.data-item {
  background: rgba(255,255,255,0.04);
  border-radius: 6px;
  padding: 5px 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 70px;
  flex: 1 1 70px;
}

.data-item .label {
  font-size: 10px;
  color: rgba(255,255,255,0.5);
  margin-bottom: 2px;
}

.probe-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}

// ==================== 设置弹窗 ====================
.settings-section {
  margin-bottom: 16px;
  padding: 12px;
  background: rgba(255,255,255,0.03);
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.06);
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255,255,255,0.85);
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(255,255,255,0.08);
}

.form-row {
  display: flex;
  gap: 20px;
  margin-bottom: 12px;
  flex-wrap: wrap;
  &:last-child { margin-bottom: 0; }
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group-label {
  font-size: 11px;
  color: rgba(255,255,255,0.55);
  font-weight: 500;
}

.range-inputs, .point-inputs {
  display: flex;
  align-items: center;
  gap: 6px;
}

.range-separator {
  color: rgba(255,255,255,0.4);
  font-size: 12px;
}

.unit-label {
  font-size: 11px;
  color: rgba(255,255,255,0.4);
  margin-left: 4px;
}

.compact-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

// 自定义点编辑器
.custom-points-editor {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.custom-point-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cp-index {
  font-size: 11px;
  color: rgba(255,255,255,0.45);
  width: 24px;
  text-align: right;
}

// 探针配置区
.probe-config-section {
  border-left: 3px solid rgba(0,245,255,0.3);
}

.probe-config-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}

.calib-status-row {
  margin-bottom: 10px;
  font-size: 11px;
}

.channel-block {
  margin-top: 10px;
}

.block-label {
  font-size: 11px;
  color: rgba(255,255,255,0.5);
  margin-bottom: 6px;
}

.channel-table {
  :deep(th) {
    font-size: 11px;
    color: rgba(255,255,255,0.7) !important;
    font-weight: 600;
    padding: 6px 4px !important;
  }
  :deep(td) {
    font-size: 11px;
    color: rgba(255,255,255,0.6);
    padding: 4px 4px !important;
  }
}

// 弹窗全局样式优化
:deep(.settings-dialog) {
  .el-dialog__header {
    margin-right: 0;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(255,255,255,0.08);
  }
  .el-dialog__title {
    font-size: 14px;
    font-weight: 600;
    color: rgba(255,255,255,0.9);
  }
  .el-dialog__body {
    padding: 16px 20px;
  }
  .el-tabs__nav-wrap::after {
    background: rgba(255,255,255,0.08);
  }
  .el-tabs__item {
    font-size: 12px;
    color: rgba(255,255,255,0.55);
    &.is-active {
      color: #00f5ff;
    }
  }
  .el-tabs__active-bar {
    background: #00f5ff;
  }
}
</style>
