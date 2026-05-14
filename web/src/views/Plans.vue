<template>
  <div class="plans-page">
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <span style="font-weight: 600">套餐管理</span>
          <el-button type="primary" @click="showCreateDialog">
            <el-icon><Plus /></el-icon> 创建套餐
          </el-button>
        </div>
      </template>

      <el-table :data="plans" v-loading="loading" stripe>
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column label="流量配额" min-width="120">
          <template #default="{ row }">
            {{ formatBytes(row.quota_bytes) }}
          </template>
        </el-table-column>
        <el-table-column prop="duration_days" label="有效期(天)" width="110" />
        <el-table-column prop="max_connections" label="最大连接数" width="110" />
        <el-table-column label="倍率" width="80">
          <template #default="{ row }">
            {{ row.traffic_rate }}x
          </template>
        </el-table-column>
        <el-table-column label="价格" width="100">
          <template #default="{ row }">
            ¥{{ (row.price_cents / 100).toFixed(2) }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'" size="small">
              {{ row.is_active ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="showEditDialog(row)">编辑</el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 创建/编辑套餐对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑套餐' : '创建套餐'" width="500px" @close="resetForm">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="100px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入套餐名称" />
        </el-form-item>
        <el-form-item label="流量配额(GB)" prop="quota_gb">
          <el-input-number v-model="form.quota_gb" :min="1" :max="10240" style="width: 100%" />
        </el-form-item>
        <el-form-item label="有效天数" prop="duration_days">
          <el-input-number v-model="form.duration_days" :min="1" :max="3650" style="width: 100%" />
        </el-form-item>
        <el-form-item label="最大连接数" prop="max_connections">
          <el-input-number v-model="form.max_connections" :min="1" :max="100" style="width: 100%" />
        </el-form-item>
        <el-form-item label="倍率" prop="traffic_rate">
          <el-input-number v-model="form.traffic_rate" :min="0.1" :max="10" :step="0.1" :precision="1" style="width: 100%" />
        </el-form-item>
        <el-form-item label="价格(元)" prop="price_yuan">
          <el-input-number v-model="form.price_yuan" :min="0" :max="99999" :precision="2" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="isEdit" label="启用状态">
          <el-switch v-model="form.is_active" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleSubmit">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { listPlans, createPlan, updatePlan, deletePlan } from '../api/plans'
import type { Plan } from '../types'
import type { FormInstance, FormRules } from 'element-plus'

const plans = ref<Plan[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const editingPlanId = ref('')
const formRef = ref<FormInstance>()

const form = reactive({
  name: '',
  quota_gb: 100,
  duration_days: 30,
  max_connections: 3,
  traffic_rate: 1.0,
  price_yuan: 0,
  is_active: true,
})

const formRules: FormRules = {
  name: [{ required: true, message: '请输入套餐名称', trigger: 'blur' }],
  quota_gb: [{ required: true, message: '请输入流量配额', trigger: 'blur' }],
  duration_days: [{ required: true, message: '请输入有效天数', trigger: 'blur' }],
  price_yuan: [{ required: true, message: '请输入价格', trigger: 'blur' }],
}

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

async function fetchPlans() {
  loading.value = true
  try {
    const res = await listPlans()
    plans.value = res.data || []
  } finally {
    loading.value = false
  }
}

function showCreateDialog() {
  isEdit.value = false
  editingPlanId.value = ''
  form.name = ''
  form.quota_gb = 100
  form.duration_days = 30
  form.max_connections = 3
  form.traffic_rate = 1.0
  form.price_yuan = 0
  form.is_active = true
  dialogVisible.value = true
}

function showEditDialog(plan: Plan) {
  isEdit.value = true
  editingPlanId.value = plan.id
  form.name = plan.name
  form.quota_gb = plan.quota_bytes / (1024 * 1024 * 1024)
  form.duration_days = plan.duration_days
  form.max_connections = plan.max_connections
  form.traffic_rate = plan.traffic_rate
  form.price_yuan = plan.price_cents / 100
  form.is_active = plan.is_active
  dialogVisible.value = true
}

function resetForm() {
  formRef.value?.resetFields()
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  submitLoading.value = true
  try {
    if (isEdit.value) {
      await updatePlan(editingPlanId.value, {
        name: form.name,
        quota_bytes: form.quota_gb * 1024 * 1024 * 1024,
        duration_days: form.duration_days,
        max_connections: form.max_connections,
        traffic_rate: form.traffic_rate,
        price_cents: Math.round(form.price_yuan * 100),
        is_active: form.is_active,
      })
      ElMessage.success('更新成功')
    } else {
      await createPlan({
        name: form.name,
        quota_bytes: form.quota_gb * 1024 * 1024 * 1024,
        duration_days: form.duration_days,
        max_connections: form.max_connections,
        traffic_rate: form.traffic_rate,
        price_cents: Math.round(form.price_yuan * 100),
      })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await fetchPlans()
  } finally {
    submitLoading.value = false
  }
}

async function handleDelete(plan: Plan) {
  await ElMessageBox.confirm(`确定要删除套餐「${plan.name}」吗？`, '确认删除', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'warning',
  })
  await deletePlan(plan.id)
  ElMessage.success('删除成功')
  await fetchPlans()
}

onMounted(fetchPlans)
</script>

<style scoped>
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
