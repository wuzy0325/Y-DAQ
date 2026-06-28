<template>
  <div class="five-hole-test-view">
    <!-- 顶部工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button type="primary" :disabled="store.isRunning" @click="showSettingsDialog = true">
          <el-icon><Setting /></el-icon> 设置
        </el-button>
        <el-button
          type="info"
          plain
          :disabled="!hasAnyResults"
          @click="handleExportAllCSV"
        >
          导出 CSV
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

    <!-- 主内容区：左布点预览 + 右探针卡片堆叠 -->
    <div class="main-content">
      <!-- 左侧：布点预览与控制 -->
      <GlassCard title="布点预览与控制" icon="🗺️" class="preview-section">
        <template #actions>
          <span class="grid-hint">
            <span class="pulse-dot" />
            交互测点网格
          </span>
        </template>

        <!-- 状态图例 -->
        <div class="point-legend">
          <span class="legend-item"><span class="legend-dot pending" />待测</span>
          <span class="legend-item"><span class="legend-dot moving" />移动</span>
          <span class="legend-item"><span class="legend-dot acquiring" />采集</span>
          <span class="legend-item"><span class="legend-dot waiting" />等待</span>
          <span class="legend-item"><span class="legend-dot completed" />完成</span>
        </div>

        <!-- 网格绘图区 -->
        <div class="canvas-wrapper">
          <canvas ref="pointCanvasRef" class="point-canvas" width="440" height="320" />
          <div v-if="previewPoints.length > 10000" class="point-count-warning">
            ⚠ 布点数量 {{ previewPoints.length }} 较大，测试耗时可能很长
          </div>
        </div>

        <!-- 进度看板 -->
        <div class="progress-dashboard">
          <div class="progress-row">
            <span class="progress-label">总规划测试点:</span>
            <span class="progress-value mono-num">{{ previewPoints.length }} 点</span>
          </div>
          <div class="progress-row">
            <span class="progress-label">当前扫描进程:</span>
            <span class="progress-value mono-num accent">{{ completedPoints }} / {{ previewPoints.length }} ({{ progressPercentage }}%)</span>
          </div>
          <div class="progress-bar-track">
            <div class="progress-bar-fill" :style="{ width: progressPercentage + '%' }" />
          </div>
          <div class="progress-tip">
            💡 <span class="tip-strong">交互提示:</span> 点击网格内任意圆点可手动设定测定状态并读取该坐标处参数。
          </div>
        </div>
      </GlassCard>

      <!-- 右侧：探针卡片堆叠 -->
      <div class="probes-stack">
        <div
          v-for="probe in store.config.probes"
          :key="probe.probeId"
          class="probe-card-wrapper"
          :class="{ disabled: !probe.enabled }"
        >
          <GlassCard :title="probeLabels[probe.probeId] || probe.probeId" icon="🔬">
            <template #actions>
              <span class="calib-indicator" :class="{ ok: store.calibLoadedMap[probe.probeId] }">
                <span class="calib-dot" />
                <span class="calib-text">
                  {{ store.calibLoadedMap[probe.probeId] ? `已载入 ${store.calibFilesMap[probe.probeId]?.length || 0} 个校准文件` : '校准文件未载入' }}
                </span>
              </span>
              <el-tooltip :content="store.isRunning ? '运行中不可切换' : '启用/禁用探针'" placement="top">
                <el-switch v-model="probe.enabled" :disabled="store.isRunning" size="small" />
              </el-tooltip>
            </template>

            <!-- 原始压力测量 -->
            <div class="raw-section">
              <div class="section-label">
                <span class="label-dot indigo" /> 原始压力测量 (Raw Pressures)
              </div>
              <div class="raw-grid">
                <div class="raw-cell">
                  <div class="cell-label">P1</div>
                  <ValueDisplay :value="getProbeRaw(probe.probeId)?.p1" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P1)" color="#b829ff" />
                </div>
                <div class="raw-cell">
                  <div class="cell-label">P2</div>
                  <ValueDisplay :value="getProbeRaw(probe.probeId)?.p2" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P2)" color="#00f5ff" />
                </div>
                <div class="raw-cell">
                  <div class="cell-label">P3</div>
                  <ValueDisplay :value="getProbeRaw(probe.probeId)?.p3" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P3)" color="#14b8a6" />
                </div>
                <div class="raw-cell">
                  <div class="cell-label">P4</div>
                  <ValueDisplay :value="getProbeRaw(probe.probeId)?.p4" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P4)" color="#ffaa00" />
                </div>
                <div class="raw-cell">
                  <div class="cell-label">P5</div>
                  <ValueDisplay :value="getProbeRaw(probe.probeId)?.p5" :precision="getChPrecision(probe.probeId, FiveHoleChannelRole.P5)" color="#ff6b6b" />
                </div>
                <div class="raw-cell">
                  <div class="cell-label">P∞</div>
                  <ValueDisplay :value="getProbeRaw(probe.probeId)?.pAtm" :precision="getAtmPrecision('p')" color="#00ff88" />
                </div>
                <div class="raw-cell">
                  <div class="cell-label">T∞ (°C)</div>
                  <ValueDisplay :value="getProbeRaw(probe.probeId)?.tAtm" :precision="getAtmPrecision('t')" color="#00aaff" />
                </div>
              </div>
            </div>

            <!-- 插值计算结果 -->
            <div class="interp-section">
              <div class="section-label">
                <span class="label-dot emerald" /> 测值气动插值解析 (Interpolated Aerodynamics)
              </div>
              <div class="interp-grid">
                <div class="interp-cell">
                  <div class="interp-label"><span class="label-dot cyan" /> 攻角 α</div>
                  <ValueDisplay :value="getProbeInterp(probe.probeId)?.alphaProbe" :precision="2" color="#00f5ff" unit="°" />
                </div>
                <div class="interp-cell">
                  <div class="interp-label"><span class="label-dot magenta" /> 侧滑角 β</div>
                  <ValueDisplay :value="getProbeInterp(probe.probeId)?.betaProbe" :precision="2" color="#b829ff" unit="°" />
                </div>
                <div class="interp-cell">
                  <div class="interp-label"><span class="label-dot green" /> 马赫数 Ma</div>
                  <ValueDisplay :value="getProbeInterp(probe.probeId)?.machProbe" :precision="4" color="#00ff88" />
                </div>
                <div class="interp-cell">
                  <div class="interp-label"><span class="label-dot red" /> 速度 V</div>
                  <ValueDisplay :value="getProbeInterp(probe.probeId)?.velocityProbe" :precision="2" color="#ff6b6b" unit="m/s" />
                </div>
                <div class="interp-cell">
                  <div class="interp-label"><span class="label-dot orange" /> 总压 Pt</div>
                  <ValueDisplay :value="getProbeInterp(probe.probeId)?.ptProbe" :precision="3" color="#ffaa00" unit="Pa" />
                </div>
                <div class="interp-cell">
                  <div class="interp-label"><span class="label-dot teal" /> 静压 Ps</div>
                  <ValueDisplay :value="getProbeInterp(probe.probeId)?.psProbe" :precision="3" color="#14b8a6" unit="Pa" />
                </div>
              </div>
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
            <div class="form-row atm-source-row">
              <!-- P∞ 设备 + 通道 配对 -->
              <div class="atm-pair">
                <div class="form-group atm-device-group">
                  <label class="group-label">大气压 P∞ 设备</label>
                  <el-select v-model="store.config.pAtmDeviceId" placeholder="选择采集设备" size="small" clearable filterable style="width: 100%">
                    <el-option v-for="dev in deviceStore.profiles" :key="dev.id" :label="`${dev.name} (${dev.type})`" :value="dev.id" />
                  </el-select>
                </div>
                <div class="form-group atm-channel-group">
                  <label class="group-label">P∞ 通道</label>
                  <el-select
                    :model-value="store.config.pAtmChannel + 1"
                    size="small"
                    style="width:80px"
                    @update:model-value="store.config.pAtmChannel = ($event as number) - 1"
                  >
                    <el-option
                      v-for="n in getChannelOptions(store.config.pAtmDeviceId)"
                      :key="n"
                      :label="String(n)"
                      :value="n"
                    />
                  </el-select>
                </div>
              </div>
              <!-- T∞ 设备 + 通道 配对 -->
              <div class="atm-pair">
                <div class="form-group atm-device-group">
                  <label class="group-label">大气温度 T∞ 设备</label>
                  <el-select v-model="store.config.tAtmDeviceId" placeholder="选择采集设备" size="small" clearable filterable style="width: 100%">
                    <el-option v-for="dev in deviceStore.profiles" :key="dev.id" :label="`${dev.name} (${dev.type})`" :value="dev.id" />
                  </el-select>
                </div>
                <div class="form-group atm-channel-group">
                  <label class="group-label">T∞ 通道</label>
                  <el-select
                    :model-value="store.config.tAtmChannel + 1"
                    size="small"
                    style="width:80px"
                    @update:model-value="store.config.tAtmChannel = ($event as number) - 1"
                  >
                    <el-option
                      v-for="n in getChannelOptions(store.config.tAtmDeviceId)"
                      :key="n"
                      :label="String(n)"
                      :value="n"
                    />
                  </el-select>
                </div>
              </div>
            </div>
          </div>

          <!-- 每探针配置：3 个 tab 切换 -->
          <el-tabs type="card" class="probe-tabs">
            <el-tab-pane
              v-for="probe in store.config.probes"
              :key="probe.probeId"
              :label="probeLabels[probe.probeId] || probe.probeId"
            >
              <div class="settings-section probe-config-section">
                <div class="probe-config-header">
                  <el-switch v-model="probe.enabled" size="small" :disabled="store.isRunning" />
                  <span class="enable-label">启用</span>
                  <el-button size="small" type="primary" plain style="margin-left: auto" @click="store.selectCalibFiles(probe.probeId)">
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
                    <el-table-column label="通道" width="130" class-name="channel-label-cell">
                      <template #default="{ row }">
                        <span class="channel-label-text">{{ FiveHoleChannelRoleLabels[row.role as FiveHoleChannelRoleValue] || row.role }}</span>
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
                        <el-select
                          :model-value="row.channel + 1"
                          size="small"
                          style="width:85px"
                          @update:model-value="row.channel = ($event as number) - 1"
                        >
                          <el-option
                            v-for="n in getChannelOptions(row.deviceId)"
                            :key="n"
                            :label="String(n)"
                            :value="n"
                          />
                        </el-select>
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
import { ConfigService, FiveHoleService } from '@bindings/yx-daq/internal/app'
import { usePointPreviewCanvas } from '../composables/usePointPreviewCanvas'
import { useLayoutStepSync } from '../composables/useLayoutStepSync'
import { useTestRealtimeRecording } from '../composables/useTestRealtimeRecording'
import { useAutoSaveConfig } from '../composables/useAutoSaveConfig'
import {
  TraversalPattern,
  TraversalPatternLabels,
  FiveHoleChannelRole,
  FiveHoleChannelRoleLabels,
  AxisName,
  type FiveHoleChannelRoleValue,
  type AxisNameValue,
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

const hasAnyResults = computed(() =>
  store.config.probes.some(p => hasProbeResults(p.probeId))
)

// 统一导出所有探针 CSV
async function handleExportAllCSV() {
  const enabledProbes = store.config.probes.filter(p => p.enabled)
  let exported = 0
  for (const probe of enabledProbes) {
    if (hasProbeResults(probe.probeId)) {
      store.exportProbeCSV(probe.probeId)
      exported++
    }
  }
  if (exported === 0) {
    ElMessage.warning('暂无可导出的数据')
  } else {
    ElMessage.success(`已导出 ${exported} 个探针的 CSV 文件`)
  }
}

// ==================== 实时保存 ====================
const { isRecording, handleStartRecording, handleStopRecording } = useTestRealtimeRecording({
  startRecording: () => store.selectAndStartRealtimeRecording(),
  stopRecording: () => store.stopRealtimeRecording(),
})

async function browseSavePath() {
  try {
    const dir = await ConfigService.SelectDataSavePath() as string
    if (dir) {
      store.config.savePath = dir
    }
  } catch (e) {
    console.error('browseSavePath failed:', e)
  }
}

// ==================== 设置弹窗 ====================
const showSettingsDialog = ref(false)

// 通道号枚举选项（1-indexed，仅返回该设备实际启用的通道，避免选到无数据通道）
function getChannelOptions(deviceId: string): number[] {
  if (!deviceId) return []
  const profile = deviceStore.profiles.find(p => p.id === deviceId)
  if (!profile) return []
  // 仅保留 enabled=true 的通道；按 index 升序输出 1-indexed 显示值
  return profile.channels
    .filter(ch => ch.enabled)
    .map(ch => ch.index + 1)
    .sort((a, b) => a - b)
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
const { rectXStep, rectYStep, lineXStep, lineYStep } = useLayoutStepSync(
  computed(() => store.config.layout),
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
const { pointCanvasRef, previewPoints, drawPointCanvas } = usePointPreviewCanvas({
  layout: computed(() => store.config.layout),
  progress: computed(() => store.progress),
  isRunning: computed(() => store.isRunning),
  canvasSize: 'parent',
  padding: { left: 44, right: 16, top: 12, bottom: 36 },
  pointRadiusFactor: 120,
  supportCustomPattern: true,
})

// 进度看板数据
const completedPoints = computed(() => store.progress?.completedPoints ?? 0)
const progressPercentage = computed(() => {
  const total = previewPoints.value.length
  if (total === 0) return 0
  return ((completedPoints.value / total) * 100).toFixed(1)
})

watch(
  [previewPoints, pointCanvasRef, () => store.config.layout],
  () => {
    nextTick(drawPointCanvas)
  },
  { deep: true }
)

// ==================== 生命周期 ====================
const { markInitialized } = useAutoSaveConfig({
  config: computed(() => store.config),
  isRunning: computed(() => store.isRunning),
  saveConfig: () => store.saveConfig(),
  stopRealtimeMonitor: () => store.stopRealtimeMonitor(),
  startRealtimeMonitor: () => store.startRealtimeMonitor(),
})

let canvasResizeObserver: ResizeObserver | null = null

onMounted(async () => {
  store.startListening()
  await store.loadConfig()
  markInitialized()
  await store.startRealtimeMonitor()
  deviceStore.fetchProfiles()
  deviceStore.fetchStatuses()
  motionStore.fetchProfiles()
  motionStore.fetchStatuses()
  nextTick(drawPointCanvas)
  try {
    isRecording.value = await FiveHoleService.IsFiveHoleRealtimeRecording()
  } catch (e) {
    console.warn('IsFiveHoleRealtimeRecording failed:', e)
  }
  // 监听容器尺寸变化，自适应重绘 canvas
  const canvas = pointCanvasRef.value
  if (canvas?.parentElement) {
    canvasResizeObserver = new ResizeObserver(() => { nextTick(drawPointCanvas) })
    canvasResizeObserver.observe(canvas.parentElement)
  }
})

onUnmounted(() => {
  store.stopListening()
  store.stopRealtimeMonitor()
  if (canvasResizeObserver) {
    canvasResizeObserver.disconnect()
    canvasResizeObserver = null
  }
  // configSaveTimer 由 useAutoSaveConfig 内部 onUnmounted 自动清理
})
</script>

<style lang="scss" scoped>
.five-hole-test-view {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  min-height: 0;
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
  display: grid;
  grid-template-columns: 5fr 7fr;
  grid-template-rows: 1fr;
  gap: 16px;
  align-items: stretch;
  flex: 1;
  min-height: 0;

  @media (max-width: 1200px) {
    grid-template-columns: 1fr;
  }
}

// ==================== 左侧：布点预览与控制 ====================
.preview-section {
  display: flex;
  flex-direction: column;
  align-self: stretch;

  :deep(.glass-card) {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  :deep(.card-body) {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }
}

.grid-hint {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: rgba(255,255,255,0.5);
}

.pulse-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #3b82f6;
  box-shadow: 0 0 4px rgba(59,130,246,0.7);
  animation: pulse-fade 1.5s ease-in-out infinite;
}

@keyframes pulse-fade {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.point-legend {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 6px;
  margin-bottom: 10px;
  padding: 8px;
  background: rgba(0,0,0,0.25);
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.06);
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
  justify-content: center;
  font-size: 10px;
  color: rgba(255,255,255,0.55);
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;

  &.pending { background: rgba(255,255,255,0.35); border: 1px solid rgba(255,255,255,0.2); }
  &.moving { background: #ffaa00; box-shadow: 0 0 4px rgba(255,170,0,0.6); }
  &.acquiring { background: #00f5ff; box-shadow: 0 0 4px rgba(0,245,255,0.6); }
  &.waiting { background: #ff3366; box-shadow: 0 0 4px rgba(255,51,102,0.6); }
  &.completed { background: #00ff88; box-shadow: 0 0 4px rgba(0,255,136,0.5); }
}

.canvas-wrapper {
  position: relative;
  width: 100%;
  flex: 1;
  min-height: 0;
  background: #050810;
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.08);
  padding: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.point-canvas {
  display: block;
  margin: 0 auto;
  border-radius: 6px;
  background: transparent;
  max-width: 100%;
}

.point-count-warning {
  position: absolute;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 11px;
  color: #ff9800;
  text-align: center;
  background: rgba(0,0,0,0.7);
  padding: 3px 10px;
  border-radius: 4px;
}

// 进度看板
.progress-dashboard {
  margin-top: 12px;
  background: rgba(0,0,0,0.3);
  border: 1px solid rgba(255,255,255,0.08);
  padding: 12px;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 11px;
}

.progress-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.progress-label {
  color: rgba(255,255,255,0.5);
}

.progress-value {
  font-weight: 600;
  color: rgba(255,255,255,0.85);

  &.accent { color: #00ff88; }
}

.mono-num {
  font-family: 'JetBrains Mono', 'Fira Code', Consolas, monospace;
  font-variant-numeric: tabular-nums;
}

.progress-bar-track {
  width: 100%;
  height: 6px;
  background: rgba(255,255,255,0.08);
  border-radius: 3px;
  overflow: hidden;
}

.progress-bar-fill {
  height: 100%;
  background: linear-gradient(90deg, #00ff88, #66ffaa);
  border-radius: 3px;
  transition: width 0.3s ease;
  box-shadow: 0 0 8px rgba(0,255,136,0.5);
}

.progress-tip {
  margin-top: 6px;
  font-size: 10px;
  color: rgba(255,255,255,0.45);
  line-height: 1.5;
  background: rgba(0,0,0,0.4);
  padding: 6px 8px;
  border-radius: 4px;
  border: 1px solid rgba(255,255,255,0.04);

  .tip-strong {
    color: rgba(255,255,255,0.7);
    font-weight: 500;
  }
}

// ==================== 右侧：探针卡片堆叠 ====================
.probes-stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  min-height: 0;
}

.probe-card-wrapper {
  flex: 1;
  min-height: 0;

  :deep(.glass-card) {
    padding: 10px 12px;
    height: 100%;
    display: flex;
    flex-direction: column;
  }
  :deep(.card-header) {
    margin-bottom: 6px;
    padding-bottom: 4px;
    flex-shrink: 0;
  }
  :deep(.card-body) {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  :deep(.card-title) {
    font-size: 13px;
  }
  :deep(.card-icon) {
    font-size: 16px;
  }

  &.disabled {
    opacity: 0.5;
  }
}

.calib-indicator {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 11px;
  background: rgba(255,255,255,0.06);
  color: rgba(255,255,255,0.45);
  border: 1px solid rgba(255,255,255,0.08);
  margin-right: 6px;

  &.ok {
    background: rgba(0,255,136,0.1);
    color: #00ff88;
    border-color: rgba(0,255,136,0.2);
  }
}

.calib-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgba(255,255,255,0.3);
  flex-shrink: 0;

  .calib-indicator.ok & {
    background: #00ff88;
    box-shadow: 0 0 6px rgba(0,255,136,0.6);
  }
}

.calib-text {
  white-space: nowrap;
}

// 区块标签
.section-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  color: rgba(255,255,255,0.5);
  font-weight: 500;
  margin-bottom: 4px;
}

.label-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;

  &.indigo { background: #6366f1; box-shadow: 0 0 4px rgba(99,102,241,0.6); }
  &.emerald { background: #00ff88; box-shadow: 0 0 4px rgba(0,255,136,0.6); }
  &.cyan { background: #00f5ff; box-shadow: 0 0 4px rgba(0,245,255,0.6); }
  &.magenta { background: #b829ff; box-shadow: 0 0 4px rgba(184,41,255,0.6); }
  &.green { background: #00ff88; box-shadow: 0 0 4px rgba(0,255,136,0.6); }
  &.red { background: #ff6b6b; box-shadow: 0 0 4px rgba(255,107,107,0.6); }
  &.orange { background: #ffaa00; box-shadow: 0 0 4px rgba(255,170,0,0.6); }
  &.teal { background: #14b8a6; box-shadow: 0 0 4px rgba(20,184,166,0.6); }
}

// 原始压力测量区
.raw-section {
  margin-bottom: 10px;
}

.raw-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 6px;

  @media (max-width: 900px) {
    grid-template-columns: repeat(4, 1fr);
  }
}

.raw-cell {
  background: rgba(0,0,0,0.35);
  border: 1px solid rgba(255,255,255,0.06);
  padding: 8px 4px;
  border-radius: 8px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  transition: background 0.2s, border-color 0.2s;

  &:hover {
    background: rgba(0,0,0,0.55);
    border-color: rgba(255,255,255,0.12);
  }
}

.raw-cell .cell-label {
  font-size: 11px;
  color: rgba(255,255,255,0.5);
  font-weight: 600;
  margin-bottom: 2px;
}

// 缩小 ValueDisplay 在网格单元中的尺寸
.raw-cell :deep(.value-display),
.interp-cell :deep(.value-display) {
  .value {
    font-size: 16px;
  }
  .unit {
    font-size: 11px;
  }
}

// 插值结果区
.interp-section {
  // 无额外边距，由 header 控制
}

.interp-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 6px;

  @media (max-width: 900px) {
    grid-template-columns: repeat(3, 1fr);
  }
}

.interp-cell {
  background: rgba(0,0,0,0.25);
  border: 1px solid rgba(255,255,255,0.05);
  padding: 8px 6px;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  transition: background 0.2s, border-color 0.2s;

  &:hover {
    background: rgba(0,0,0,0.5);
    border-color: rgba(255,255,255,0.12);
  }
}

.interp-label {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: rgba(255,255,255,0.5);
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

  .enable-label {
    font-size: 12px;
    color: rgba(255,255,255,0.7);
  }
}

// 探针 tab 切换
.probe-tabs {
  :deep(.el-tabs__header) {
    margin-bottom: 10px;
    border-bottom: 1px solid rgba(255,255,255,0.1);
  }
  :deep(.el-tabs__nav) {
    border: none !important;
    display: inline-flex !important;
    width: auto !important;
  }
  :deep(.el-tabs__item) {
    font-size: 12px;
    color: rgba(255,255,255,0.55);
    background: rgba(255,255,255,0.04);
    border: 1px solid rgba(255,255,255,0.08) !important;
    border-radius: 4px 4px 0 0;
    margin-right: 4px;
    padding: 0 14px !important;
    height: 28px !important;
    line-height: 26px !important;
    flex: 0 0 auto !important;
    transition: all 0.2s ease;
    &.is-active {
      color: #00f5ff;
      font-weight: 600;
      background: rgba(0,245,255,0.12);
      border-color: rgba(0,245,255,0.5) !important;
      border-bottom-color: transparent !important;
      box-shadow: 0 -2px 8px rgba(0,245,255,0.2);
    }
    &:hover:not(.is-active) {
      color: rgba(255,255,255,0.85);
      background: rgba(255,255,255,0.08);
    }
  }
  :deep(.el-tabs__nav-wrap::after) {
    background-color: transparent;
  }
}

// P∞/T∞ 数据源配对布局：设备选择与通道选择紧邻成组
.atm-source-row {
  gap: 16px;
}
.atm-pair {
  display: flex;
  flex: 1;
  min-width: 0;
  gap: 8px;
  align-items: flex-end;
}
.atm-device-group {
  flex: 1;
  min-width: 0;
}
.atm-channel-group {
  flex-shrink: 0;
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
  // 通道标签单元格：禁止换行
  :deep(.channel-label-cell) {
    .cell {
      white-space: nowrap;
    }
  }
}

.channel-label-text {
  white-space: nowrap;
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
