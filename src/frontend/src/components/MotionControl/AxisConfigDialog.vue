<template>
  <el-dialog v-model="visible" title="轴参数配置" width="600px" destroy-on-close :append-to-body="true">
    <div v-if="!currentAxis" class="empty-state">
      <el-empty description="请先选择要配置的轴" />
    </div>

    <template v-else>
      <!-- 顶部：轴选择和类型选择 -->
      <div class="config-header">
        <div class="header-item">
          <span class="header-label">选择轴</span>
          <el-radio-group v-model="selectedAxisName" size="small">
            <el-radio-button value="X">X轴</el-radio-button>
            <el-radio-button value="Y">Y轴</el-radio-button>
            <el-radio-button value="Z">Z轴</el-radio-button>
            <el-radio-button value="U">U轴</el-radio-button>
          </el-radio-group>
        </div>
        <div class="header-item">
          <span class="header-label">轴类型</span>
          <el-radio-group v-model="axisKind" size="small" @change="onKindChange">
            <el-radio-button value="LINEAR">平移轴</el-radio-button>
            <el-radio-button value="ROTARY">旋转轴</el-radio-button>
          </el-radio-group>
        </div>
      </div>

      <!-- 主体：双列布局 -->
      <div class="config-body">
        <!-- 左侧：电机参数 -->
        <div class="config-column">
          <div class="section-title">电机参数</div>
          <el-form :model="formData" label-width="80px" class="compact-form">
            <el-form-item label="电机度数">
              <el-input-number v-model="formData.stepAngleDeg" :precision="1" :step="0.1" :min="0.1" :max="10" size="small" style="width: 100%" />
              <div class="form-hint">°/步，如 1.8° 步进电机</div>
            </el-form-item>
            <el-form-item label="细分数">
              <el-select v-model="formData.microSteps" size="small" style="width: 100%">
                <el-option v-for="n in [1,2,4,8,16,32,64,128,256]" :key="n" :label="`${n}${n===1?' (整步)':n===2?' (半步)':''}`" :value="n" />
              </el-select>
            </el-form-item>
            <el-form-item label="驱动速度">
              <el-input-number v-model="formData.maxSpeed" :precision="1" :step="1" :min="0.1" :max="500" size="small" style="width: 100%" />
              <div class="form-hint">{{ axisKind === 'LINEAR' ? 'mm/s' : '°/s' }}</div>
            </el-form-item>
          </el-form>
        </div>

        <!-- 右侧：机械参数 -->
        <div class="config-column">
          <div class="section-title">机械参数</div>
          <el-form :model="formData" label-width="80px" class="compact-form">
            <el-form-item v-if="axisKind === 'LINEAR'" label="丝杆导程">
              <el-input-number v-model="formData.lead" :precision="2" :step="0.5" :min="0.1" :max="50" size="small" style="width: 100%" />
              <div class="form-hint">mm/转，电机转一圈移动距离</div>
            </el-form-item>
            <el-form-item v-else label="传动比">
              <el-input-number v-model="formData.gearRatio" :precision="1" :step="1" :min="1" size="small" style="width: 100%" />
              <div class="form-hint">减速比，如 10:1</div>
            </el-form-item>
            <el-form-item label="方向取反">
              <el-switch v-model="formData.inverted" size="small" active-text="是" inactive-text="否" />
              <div class="form-hint">反转电机运动方向</div>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- 底部：批量应用 -->
      <div class="config-footer">
        <div class="footer-label">批量应用到其他轴:</div>
        <el-checkbox-group v-model="applyToAxes" size="small">
          <el-checkbox v-for="n in ['X','Y','Z','U']" :key="n" :value="n" :label="`${n}轴`" :disabled="selectedAxisName === n" />
        </el-checkbox-group>
        <span class="apply-hint">(仅应用到相同类型轴)</span>
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

const store = useMotionStore()

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
})

// 各轴编辑草稿：切换轴时暂存未保存的编辑，支持多轴改完一次保存
interface AxisDraft {
  kind: 'LINEAR' | 'ROTARY'
  data: typeof formData.value
}
const drafts = ref<Record<string, AxisDraft>>({})

const currentAxis = computed(() => store.axisUIStates[selectedAxisName.value])

// 从 store 初始化某轴草稿（打开对话框/首次切到某轴时）
function loadDraft(axisName: string) {
  const axis = store.axisUIStates[axisName]
  if (!axis) return
  const draft: AxisDraft = {
    kind: axis.kind as 'LINEAR' | 'ROTARY',
    data: {
      stepAngleDeg: axis.config.stepAngleDeg,
      microSteps: axis.config.microSteps,
      maxSpeed: axis.config.maxSpeed,
      lead: axis.config.lead,
      gearRatio: axis.config.gearRatio || 1,
      inverted: axis.config.inverted,
    },
  }
  drafts.value[axisName] = draft
  axisKind.value = draft.kind
  formData.value = { ...draft.data }
}

// 切换轴：先暂存当前轴编辑，再载入目标轴草稿（保留其未保存的编辑）
// visible 守卫：open() 初始化触发的轴切换不暂存（此时 formData 是上次会话的残留）
watch(selectedAxisName, (newName, oldName) => {
  if (!visible.value) return
  if (oldName && drafts.value[oldName]) {
    drafts.value[oldName] = { kind: axisKind.value, data: { ...formData.value } }
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
    drafts.value[selectedAxisName.value] = { kind: axisKind.value, data: { ...formData.value } }

    const buildConfig = (draft: AxisDraft) => ({
      stepAngleDeg: draft.data.stepAngleDeg,
      microSteps: draft.data.microSteps,
      maxSpeed: draft.data.maxSpeed,
      lead: draft.kind === 'LINEAR' ? draft.data.lead : 0,
      gearRatio: draft.kind === 'ROTARY' ? draft.data.gearRatio : 1,
      inverted: draft.data.inverted,
      kind: draft.kind,
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
        updates[axisName] = { ...currentConfig }
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
.empty-state { padding: 30px; }

.config-header {
  display: flex;
  gap: 24px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255,255,255,0.06);

  .header-item {
    display: flex;
    align-items: center;
    gap: 10px;
    .header-label {
      font-size: 13px;
      font-weight: 500;
      color: rgba(255,255,255,0.5);
      white-space: nowrap;
    }
  }
}

.config-body {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
  margin-bottom: 16px;
}

.config-column {
  .section-title {
    font-size: 13px;
    font-weight: 600;
    color: #00f5ff;
    margin-bottom: 12px;
    padding-left: 8px;
    border-left: 3px solid #00f5ff;
  }
}

.compact-form {
  :deep(.el-form-item) { margin-bottom: 12px; &:last-child { margin-bottom: 0; } }
  :deep(.el-form-item__label) { color: rgba(255,255,255,0.5); font-size: 12px; }
}

.form-hint {
  font-size: 11px;
  color: rgba(255,255,255,0.25);
  margin-top: 3px;
}

.config-footer {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(255,255,255,0.06);

  .footer-label {
    font-size: 12px;
    font-weight: 500;
    color: rgba(255,255,255,0.5);
    white-space: nowrap;
  }

  .apply-hint {
    font-size: 11px;
    color: rgba(255,255,255,0.25);
    font-style: italic;
    margin-left: auto;
  }
}
</style>
