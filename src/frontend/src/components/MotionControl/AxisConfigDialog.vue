<template>
  <el-dialog v-model="visible" title="轴参数配置" width="600px" destroy-on-close :append-to-body="true">
    <div v-if="!currentAxis" class="empty-state">
      <el-empty description="请先选择要配置的轴" />
    </div>

    <template v-else>
      <!-- 顶部：轴与类型选择 -->
      <div class="config-panel config-header">
        <div class="header-row">
          <span class="header-label">选择轴</span>
          <el-radio-group v-model="selectedAxisName" size="small">
            <el-radio-button value="X">X轴</el-radio-button>
            <el-radio-button value="Y">Y轴</el-radio-button>
            <el-radio-button value="Z">Z轴</el-radio-button>
            <el-radio-button value="U">U轴</el-radio-button>
          </el-radio-group>
        </div>
        <div class="header-row">
          <span class="header-label">轴类型</span>
          <el-radio-group v-model="axisKind" size="small" @change="onKindChange">
            <el-radio-button value="LINEAR">平移轴</el-radio-button>
            <el-radio-button value="ROTARY">旋转轴</el-radio-button>
          </el-radio-group>
        </div>
      </div>

      <!-- 主体：电机参数 / 机械参数 -->
      <div class="config-body">
        <div class="config-panel config-column">
          <div class="section-title">电机参数</div>
          <el-form :model="formData" label-width="80px" class="compact-form">
            <el-form-item label="电机度数">
              <div class="field-control">
                <el-input-number
                  v-model="formData.stepAngleDeg"
                  class="field-input"
                  :precision="1"
                  :step="0.1"
                  :min="0.1"
                  :max="10"
                  size="small"
                  :controls="false"
                />
                <span class="field-unit">°/步</span>
              </div>
              <div class="form-hint">如 1.8° 步进电机</div>
            </el-form-item>
            <el-form-item label="细分数">
              <div class="field-control">
                <el-input-number
                  v-model="formData.microSteps"
                  class="field-input"
                  :precision="0"
                  :step="1"
                  :min="1"
                  size="small"
                  :controls="false"
                />
              </div>
              <div class="form-hint">驱动细分倍数，1 为整步</div>
            </el-form-item>
            <el-form-item label="驱动速度">
              <div class="field-control">
                <el-input-number
                  v-model="formData.maxSpeed"
                  class="field-input"
                  :precision="1"
                  :step="1"
                  :min="0.1"
                  :max="500"
                  size="small"
                  :controls="false"
                />
                <span class="field-unit">{{ speedUnit }}</span>
              </div>
            </el-form-item>
          </el-form>
        </div>

        <div class="config-panel config-column">
          <div class="section-title">机械参数</div>
          <el-form :model="formData" label-width="80px" class="compact-form">
            <el-form-item v-if="axisKind === 'LINEAR'" label="丝杆导程">
              <div class="field-control">
                <el-input-number
                  v-model="formData.lead"
                  class="field-input"
                  :precision="2"
                  :step="0.5"
                  :min="0.1"
                  :max="50"
                  size="small"
                  :controls="false"
                />
                <span class="field-unit">mm/转</span>
              </div>
              <div class="form-hint">电机转一圈移动距离</div>
            </el-form-item>
            <el-form-item v-else label="传动比">
              <div class="field-control">
                <el-input-number
                  v-model="formData.gearRatio"
                  class="field-input"
                  :precision="1"
                  :step="1"
                  :min="1"
                  size="small"
                  :controls="false"
                />
                <span class="field-unit">:1</span>
              </div>
              <div class="form-hint">减速比，如 10:1</div>
            </el-form-item>
            <el-form-item label="方向取反">
              <div class="field-control">
                <el-switch v-model="formData.inverted" size="small" />
              </div>
              <div class="form-hint">开启后反转电机运动方向</div>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- 底部：软限位 -->
      <div class="config-panel config-limits">
        <div class="limits-header">
          <div class="section-title">软限位</div>
          <el-switch
            v-model="formData.softLimit.enabled"
            size="small"
            active-text="启用"
            inactive-text="禁用"
          />
        </div>
        <el-form :model="formData" label-width="80px" class="compact-form limits-form">
          <el-form-item label="反向限位">
            <div class="field-control">
              <el-input-number
                v-model="formData.softLimit.min"
                class="field-input"
                :precision="2"
                :step="1"
                size="small"
                :controls="false"
                :disabled="!formData.softLimit.enabled"
              />
              <span class="field-unit">{{ limitUnit }}</span>
            </div>
          </el-form-item>
          <el-form-item label="正向限位">
            <div class="field-control">
              <el-input-number
                v-model="formData.softLimit.max"
                class="field-input"
                :precision="2"
                :step="1"
                size="small"
                :controls="false"
                :disabled="!formData.softLimit.enabled"
              />
              <span class="field-unit">{{ limitUnit }}</span>
            </div>
          </el-form-item>
        </el-form>
        <div class="form-hint">启用后超出范围的运动将被拒绝；保存后下发至控制器（EA25MC04：BL/FL 命令）</div>
      </div>

      <!-- 底部：批量应用 -->
      <div class="config-footer">
        <div class="footer-row">
          <span class="footer-label">批量应用到其他轴</span>
          <el-checkbox-group v-model="applyToAxes" size="small">
            <el-checkbox v-for="n in AXIS_NAMES" :key="n" :value="n" :disabled="selectedAxisName === n">
              {{ n }}轴
            </el-checkbox>
          </el-checkbox-group>
        </div>
        <div class="form-hint">仅应用到相同类型的轴</div>
      </div>
    </template>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="saveConfig">保存配置</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useMotionStore } from '../../stores/motion'
import { getAxisSoftLimitDefaults } from '../../stores/motion/helpers'
import type { AxisKind } from '../../stores/motion/types'

const store = useMotionStore()

const AXIS_NAMES = ['X', 'Y', 'Z', 'U']

const visible = ref(false)
const saving = ref(false)
const applyToAxes = ref<string[]>([])
const selectedAxisName = ref('X')
const axisKind = ref<'LINEAR' | 'ROTARY'>('LINEAR')

const formData = ref({
  stepAngleDeg: 1.8,
  microSteps: 16,
  maxSpeed: 50,
  lead: 5.0,
  gearRatio: 4,
  inverted: false,
  softLimit: {
    enabled: false,
    min: -100,
    max: 100,
  },
})

// 各轴编辑草稿：切换轴时暂存未保存的编辑，支持多轴改完一次保存
interface AxisDraft {
  kind: 'LINEAR' | 'ROTARY'
  data: typeof formData.value
}
const drafts = ref<Record<string, AxisDraft>>({})

const currentAxis = computed(() => store.axisUIStates[selectedAxisName.value])
const limitUnit = computed(() => (axisKind.value === 'LINEAR' ? 'mm' : '°'))
const speedUnit = computed(() => (axisKind.value === 'LINEAR' ? 'mm/s' : '°/s'))

// 兼容旧配置：softLimit 缺失或为 0/0（历史数据零值）时回填按轴类型的默认范围
function normalizeSoftLimit(
  raw: { enabled: boolean; min: number; max: number } | undefined,
  kind: AxisKind,
) {
  const defaults = getAxisSoftLimitDefaults(kind)
  if (!raw) return { enabled: false, ...defaults }
  const isUnset = raw.min === 0 && raw.max === 0
  return {
    enabled: raw.enabled ?? false,
    min: isUnset ? defaults.min : raw.min,
    max: isUnset ? defaults.max : raw.max,
  }
}

// 从 store 初始化某轴草稿（打开对话框/首次切到某轴时）
function loadDraft(axisName: string) {
  const axis = store.axisUIStates[axisName]
  if (!axis) return
  const kind = axis.kind as 'LINEAR' | 'ROTARY'
  const draft: AxisDraft = {
    kind,
    data: {
      stepAngleDeg: axis.config.stepAngleDeg,
      microSteps: axis.config.microSteps,
      maxSpeed: axis.config.maxSpeed,
      lead: axis.config.lead,
      gearRatio: axis.config.gearRatio || 1,
      inverted: axis.config.inverted,
      softLimit: normalizeSoftLimit(axis.config.softLimit, kind),
    },
  }
  drafts.value[axisName] = draft
  axisKind.value = draft.kind
  formData.value = { ...draft.data, softLimit: { ...draft.data.softLimit } }
}

// 切换轴：先暂存当前轴编辑，再载入目标轴草稿（保留其未保存的编辑）
// visible 守卫：open() 初始化触发的轴切换不暂存（此时 formData 是上次会话的残留）
watch(selectedAxisName, (newName, oldName) => {
  if (!visible.value) return
  if (oldName && drafts.value[oldName]) {
    drafts.value[oldName] = {
      kind: axisKind.value,
      data: { ...formData.value, softLimit: { ...formData.value.softLimit } },
    }
  }
  if (newName) loadDraft(newName)
})

function open(axisName?: string) {
  drafts.value = {}
  applyToAxes.value = []
  if (axisName) selectedAxisName.value = axisName
  loadDraft(selectedAxisName.value)
  visible.value = true
}

function onKindChange(newKind: string | number | boolean | undefined) {
  if (typeof newKind !== 'string') return
  axisKind.value = newKind as 'LINEAR' | 'ROTARY'
  if (newKind === 'LINEAR') {
    formData.value.lead = 5.0
    formData.value.maxSpeed = 50
  } else {
    formData.value.gearRatio = 4
    formData.value.maxSpeed = 30
  }
}

async function saveConfig() {
  saving.value = true
  try {
    // 暂存当前轴的编辑
    drafts.value[selectedAxisName.value] = {
      kind: axisKind.value,
      data: { ...formData.value, softLimit: { ...formData.value.softLimit } },
    }

    // 校验软限位：启用时上限必须大于下限
    for (const [axisName, draft] of Object.entries(drafts.value)) {
      const sl = draft.data.softLimit
      if (sl.enabled && sl.max <= sl.min) {
        ElMessage.error(`${axisName} 轴软限位上限必须大于下限`)
        return
      }
    }

    const buildConfig = (draft: AxisDraft) => ({
      stepAngleDeg: draft.data.stepAngleDeg,
      microSteps: draft.data.microSteps,
      maxSpeed: draft.data.maxSpeed,
      lead: draft.kind === 'LINEAR' ? draft.data.lead : 0,
      gearRatio: draft.kind === 'ROTARY' ? draft.data.gearRatio : 1,
      inverted: draft.data.inverted,
      kind: draft.kind,
      softLimit: { ...draft.data.softLimit },
    })

    // 收集所有编辑过的轴，多轴一次提交
    const updates: Record<string, ReturnType<typeof buildConfig>> = {}
    for (const [axisName, draft] of Object.entries(drafts.value)) {
      updates[axisName] = buildConfig(draft)
    }

    // 批量应用到其他同类型轴
    const currentConfig = buildConfig(drafts.value[selectedAxisName.value])
    for (const axisName of applyToAxes.value) {
      const targetAxis = store.axisUIStates[axisName]
      if (targetAxis && targetAxis.kind === axisKind.value) {
        updates[axisName] = { ...currentConfig, softLimit: { ...currentConfig.softLimit } }
      }
    }

    await store.updateAxesConfig(updates)
    ElMessage.success('配置保存成功')
    visible.value = false
  } catch (e) {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

defineExpose({ open })
</script>

<style scoped lang="scss">
.empty-state { padding: $spacing-2xl; }

.config-panel {
  padding: $spacing-md;
  background: $bg-tertiary;
  border: 1px solid $glass-border-light;
  border-radius: $border-radius-sm;
}

.config-header {
  display: flex;
  flex-direction: column;
  gap: $spacing-md;
  margin-bottom: $spacing-md;

  .header-row {
    display: flex;
    align-items: center;
    gap: $spacing-md;
  }

  .header-label {
    flex-shrink: 0;
    width: 48px;
    font-size: $font-size-sm;
    color: $text-tertiary;
  }
}

.config-body {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: start;
  gap: $spacing-md;
  margin-bottom: $spacing-md;
}

.section-title {
  font-size: $font-size-sm;
  font-weight: 600;
  color: $color-accent;
  line-height: 1.3;
  margin-bottom: $spacing-md;
  padding-left: $spacing-sm;
  border-left: 3px solid $color-accent;
}

.compact-form {
  :deep(.el-form-item) {
    margin-bottom: $spacing-md;
    &:last-child { margin-bottom: 0; }
  }
  :deep(.el-form-item__label) {
    color: $text-tertiary;
    font-size: 12px;
  }

  .field-control {
    display: flex;
    align-items: center;
    gap: $spacing-xs;
    width: 100%;

    .field-input {
      flex: 1;
      min-width: 0;
    }

    .field-unit {
      flex-shrink: 0;
      font-size: $font-size-xs;
      color: $text-muted;
    }
  }

  .form-hint { width: 100%; }
}

.form-hint {
  font-size: $font-size-xs;
  line-height: 1.5;
  color: $text-muted;
  margin-top: 2px;
}

.config-limits {
  margin-bottom: $spacing-md;

  .limits-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: $spacing-md;

    .section-title { margin-bottom: 0; }
  }

  .limits-form {
    display: flex;
    gap: $spacing-xl;
    margin-bottom: $spacing-sm;

    :deep(.el-form-item) {
      flex: 1;
      margin-bottom: 0;
    }
  }
}

.config-footer {
  padding-top: $spacing-md;
  border-top: 1px solid $glass-border-light;

  .footer-row {
    display: flex;
    align-items: center;
    gap: $spacing-md;
  }

  .footer-label {
    font-size: $font-size-sm;
    color: $text-tertiary;
    white-space: nowrap;
  }
}
</style>
