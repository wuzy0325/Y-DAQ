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
          <el-radio-group v-model="selectedAxisName" size="small" @change="onAxisChange">
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
              <el-input-number v-model="formData.lead" :precision="1" :step="1" :min="1" size="small" style="width: 100%" />
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
  inverted: false,
})

const currentAxis = computed(() => store.axisUIStates[selectedAxisName.value])

// 监听轴变化，更新表单
watch([() => selectedAxisName.value, () => store.axisUIStates], () => {
  const axis = store.axisUIStates[selectedAxisName.value]
  if (axis) {
    axisKind.value = axis.kind as any
    formData.value = {
      stepAngleDeg: axis.config.stepAngleDeg,
      microSteps: axis.config.microSteps,
      maxSpeed: axis.config.maxSpeed,
      lead: axis.config.lead,
      inverted: axis.config.inverted,
    }
    applyToAxes.value = []
  }
}, { immediate: true })

function open(axisName?: string) {
  if (axisName) selectedAxisName.value = axisName
  visible.value = true
}

function onAxisChange(name: string) {
  selectedAxisName.value = name
}

function onKindChange(newKind: string) {
  axisKind.value = newKind as any
  if (newKind === 'LINEAR') {
    formData.value.lead = 5.0
    formData.value.maxSpeed = 50
  } else {
    formData.value.lead = 4
    formData.value.maxSpeed = 30
  }
}

async function saveConfig() {
  saving.value = true
  try {
    const config: any = {
      stepAngleDeg: formData.value.stepAngleDeg,
      microSteps: formData.value.microSteps,
      maxSpeed: formData.value.maxSpeed,
      lead: formData.value.lead,
      inverted: formData.value.inverted,
      kind: axisKind.value,
    }

    // 保存当前轴
    store.updateAxisKind(selectedAxisName.value, axisKind.value)
    store.updateAxisConfig(selectedAxisName.value, config)

    // 批量应用
    for (const axisName of applyToAxes.value) {
      const targetAxis = store.axisUIStates[axisName]
      if (targetAxis && targetAxis.kind === axisKind.value) {
        store.updateAxisConfig(axisName, config)
      }
    }

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
