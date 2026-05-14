<template>
  <div class="users-page">
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <span style="font-weight: 600">用户管理</span>
          <el-button type="primary" @click="showCreateDialog">
            <el-icon><Plus /></el-icon> 创建用户
          </el-button>
        </div>
      </template>

      <div class="filter-row">
        <el-input
          v-model="searchKeyword"
          placeholder="搜索用户名/邮箱"
          clearable
          style="width: 250px"
          @clear="fetchUsers"
          @keyup.enter="fetchUsers"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select v-model="stateFilter" placeholder="状态筛选" clearable style="width: 150px" @change="fetchUsers">
          <el-option label="活跃" value="active" />
          <el-option label="暂停" value="suspended" />
          <el-option label="过期" value="expired" />
        </el-select>
      </div>

      <el-table :data="users" v-loading="loading" stripe>
        <el-table-column prop="username" label="用户名" min-width="100" />
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="stateTagType(row.state)" size="small">{{ stateLabel(row.state) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="配额使用" min-width="150">
          <template #default="{ row }">
            <el-progress
              :percentage="row.quota_bytes > 0 ? Math.min(Math.round(row.used_bytes / row.quota_bytes * 100), 100) : 0"
              :stroke-width="10"
              :color="progressColor(row)"
            />
            <span class="quota-text">{{ formatBytes(row.used_bytes) }} / {{ formatBytes(row.quota_bytes) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="有效期" min-width="110">
          <template #default="{ row }">
            {{ row.expires_at ? new Date(row.expires_at).toLocaleDateString('zh-CN') : '永久' }}
          </template>
        </el-table-column>
        <el-table-column label="套餐" min-width="100">
          <template #default="{ row }">
            {{ row.plan?.name || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="showTrafficDialog(row)">流量</el-button>
            <el-button type="warning" link size="small" @click="showEditDialog(row)">编辑</el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-row">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="size"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @size-change="fetchUsers"
          @current-change="fetchUsers"
        />
      </div>
    </el-card>

    <!-- 创建/编辑用户对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑用户' : '创建用户'" width="500px" @close="resetForm">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="80px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" :disabled="isEdit" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item v-if="!isEdit" label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="至少6位" />
        </el-form-item>
        <el-form-item label="套餐" prop="plan_id">
          <el-select v-model="form.plan_id" placeholder="选择套餐" clearable style="width: 100%">
            <el-option
              v-for="plan in plans"
              :key="plan.id"
              :label="plan.name"
              :value="plan.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="isEdit" label="状态" prop="state">
          <el-select v-model="form.state" style="width: 100%">
            <el-option label="活跃" value="active" />
            <el-option label="暂停" value="suspended" />
            <el-option label="过期" value="expired" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleSubmit">确认</el-button>
      </template>
    </el-dialog>

    <!-- 流量详情对话框 -->
    <el-dialog v-model="trafficDialogVisible" title="流量详情" width="700px">
      <div v-if="selectedUser">
        <p style="margin-bottom: 12px; color: #606266;">
          用户：{{ selectedUser.username }} — 已用 {{ formatBytes(selectedUser.used_bytes) }} / {{ formatBytes(selectedUser.quota_bytes) }}
        </p>
        <TrafficChart :data="userTrafficData" title="用户流量" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import { listUsers, createUser, updateUser, deleteUser, getUserTraffic } from '../api/users'
import { listPlans } from '../api/plans'
import TrafficChart from '../components/TrafficChart.vue'
import type { User, Plan, TrafficEntry, UpdateUserRequest } from '../types'
import type { FormInstance, FormRules } from 'element-plus'

const users = ref<User[]>([])
const plans = ref<Plan[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const size = ref(20)
const searchKeyword = ref('')
const stateFilter = ref('')

const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const editingUserId = ref('')
const formRef = ref<FormInstance>()

const trafficDialogVisible = ref(false)
const selectedUser = ref<User | null>(null)
const userTrafficData = ref<TrafficEntry[]>([])

const form = reactive({
  username: '',
  email: '',
  password: '',
  plan_id: '',
  state: 'active',
})

const formRules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' },
  ],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }, { min: 6, message: '密码至少6位', trigger: 'blur' }],
}

function stateTagType(state: string) {
  switch (state) {
    case 'active': return 'success'
    case 'suspended': return 'warning'
    case 'expired': return 'danger'
    default: return 'info'
  }
}

function stateLabel(state: string) {
  switch (state) {
    case 'active': return '活跃'
    case 'suspended': return '暂停'
    case 'expired': return '过期'
    default: return state
  }
}

function progressColor(row: User) {
  const pct = row.quota_bytes > 0 ? row.used_bytes / row.quota_bytes : 0
  if (pct > 0.9) return '#f56c6c'
  if (pct > 0.7) return '#e6a23c'
  return '#409eff'
}

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

async function fetchUsers() {
  loading.value = true
  try {
    const res = await listUsers({ page: page.value, size: size.value, state: stateFilter.value || undefined })
    users.value = res.data.items || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

async function fetchPlans() {
  const res = await listPlans()
  plans.value = res.data || []
}

function showCreateDialog() {
  isEdit.value = false
  editingUserId.value = ''
  form.username = ''
  form.email = ''
  form.password = ''
  form.plan_id = ''
  form.state = 'active'
  dialogVisible.value = true
}

function showEditDialog(user: User) {
  isEdit.value = true
  editingUserId.value = user.id
  form.username = user.username
  form.email = user.email
  form.password = ''
  form.plan_id = user.plan_id || ''
  form.state = user.state
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
      const data: UpdateUserRequest = {
        email: form.email,
        state: form.state,
        plan_id: form.plan_id || undefined,
      }
      if (form.password) {
        data.password = form.password
      }
      await updateUser(editingUserId.value, data)
      ElMessage.success('更新成功')
    } else {
      await createUser({
        username: form.username,
        email: form.email,
        password: form.password,
        plan_id: form.plan_id || undefined,
      })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await fetchUsers()
  } finally {
    submitLoading.value = false
  }
}

async function handleDelete(user: User) {
  await ElMessageBox.confirm(`确定要删除用户「${user.username}」吗？`, '确认删除', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'warning',
  })
  await deleteUser(user.id)
  ElMessage.success('删除成功')
  await fetchUsers()
}

async function showTrafficDialog(user: User) {
  selectedUser.value = user
  trafficDialogVisible.value = true
  try {
    const res = await getUserTraffic(user.id, 'week')
    userTrafficData.value = res.data.data || []
  } catch {
    userTrafficData.value = []
  }
}

onMounted(() => {
  fetchUsers()
  fetchPlans()
})
</script>

<style scoped>
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.filter-row {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.quota-text {
  font-size: 12px;
  color: #909399;
}
.pagination-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
