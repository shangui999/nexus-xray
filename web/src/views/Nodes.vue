<template>
  <div class="nodes-page">
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <span style="font-weight: 600">节点列表</span>
          <el-button type="primary" @click="showAddDialog">
            <el-icon><Plus /></el-icon> 添加节点
          </el-button>
        </div>
      </template>

      <el-table :data="nodes" v-loading="loading" stripe>
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="address" label="地址" min-width="180" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'online' ? 'success' : row.status === 'error' ? 'warning' : 'danger'" size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最近心跳" min-width="180">
          <template #default="{ row }">
            {{ row.last_heartbeat ? new Date(row.last_heartbeat).toLocaleString('zh-CN') : '无' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button type="danger" link size="small" @click="handleDelete(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 添加节点对话框 -->
    <el-dialog v-model="addDialogVisible" title="添加节点" width="500px" @close="resetAddForm">
      <el-form ref="addFormRef" :model="addForm" :rules="addRules" label-width="80px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="addForm.name" placeholder="请输入节点名称" />
        </el-form-item>
        <el-form-item label="地址" prop="address">
          <el-input v-model="addForm.address" placeholder="例如: 192.168.1.1" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="addLoading" @click="handleAdd">确认</el-button>
      </template>
    </el-dialog>

    <!-- 安装命令对话框 -->
    <el-dialog v-model="installDialogVisible" title="安装命令" width="600px">
      <el-alert type="success" :closable="false" style="margin-bottom: 16px">
        节点创建成功！请在目标服务器上执行以下命令安装 Agent：
      </el-alert>
      <el-input
        v-model="installCommand"
        type="textarea"
        :rows="3"
        readonly
      />
      <template #footer>
        <el-button type="primary" @click="copyCommand">复制命令</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { listNodes, createNode, deleteNode } from '../api/nodes'
import type { Node, CreateNodeResponse } from '../types'
import type { FormInstance, FormRules } from 'element-plus'

const nodes = ref<Node[]>([])
const loading = ref(false)
const addDialogVisible = ref(false)
const addLoading = ref(false)
const installDialogVisible = ref(false)
const installCommand = ref('')
const addFormRef = ref<FormInstance>()

const addForm = reactive({
  name: '',
  address: '',
})

const addRules: FormRules = {
  name: [{ required: true, message: '请输入节点名称', trigger: 'blur' }],
  address: [{ required: true, message: '请输入节点地址', trigger: 'blur' }],
}

async function fetchNodes() {
  loading.value = true
  try {
    const res = await listNodes()
    nodes.value = res.data || []
  } finally {
    loading.value = false
  }
}

function showAddDialog() {
  addDialogVisible.value = true
}

function resetAddForm() {
  addForm.name = ''
  addForm.address = ''
  addFormRef.value?.resetFields()
}

async function handleAdd() {
  const valid = await addFormRef.value?.validate().catch(() => false)
  if (!valid) return

  addLoading.value = true
  try {
    const res = await createNode({ name: addForm.name, address: addForm.address })
    const data = res.data as CreateNodeResponse
    addDialogVisible.value = false
    installCommand.value = data.install_command
    installDialogVisible.value = true
    await fetchNodes()
  } finally {
    addLoading.value = false
  }
}

async function handleDelete(node: Node) {
  await ElMessageBox.confirm(`确定要删除节点「${node.name}」吗？`, '确认删除', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'warning',
  })
  await deleteNode(node.id)
  ElMessage.success('删除成功')
  await fetchNodes()
}

async function copyCommand() {
  try {
    await navigator.clipboard.writeText(installCommand.value)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

onMounted(fetchNodes)
</script>

<style scoped>
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
