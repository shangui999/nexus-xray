<template>
  <div class="inbounds-page">
    <el-card shadow="hover">
      <template #header>
        <div class="card-header">
          <span style="font-weight: 600">入站配置</span>
          <div class="header-actions">
            <el-select v-model="nodeFilter" placeholder="按节点筛选" clearable style="width: 200px" @change="fetchInbounds">
              <el-option
                v-for="node in nodes"
                :key="node.id"
                :label="node.name"
                :value="node.id"
              />
            </el-select>
            <el-button type="primary" @click="showCreateDialog">
              <el-icon><Plus /></el-icon> 创建入站
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="inbounds" v-loading="loading" stripe>
        <el-table-column prop="tag" label="标签" min-width="120" />
        <el-table-column label="节点" min-width="120">
          <template #default="{ row }">
            {{ row.node?.name || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="protocol" label="协议" width="120" />
        <el-table-column prop="port" label="端口" width="80" />
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '启用' : '停用' }}
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

    <!-- 创建/编辑入站对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑入站' : '创建入站'" width="600px" @close="resetForm">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="100px">
        <el-form-item label="节点" prop="node_id">
          <el-select v-model="form.node_id" placeholder="选择节点" :disabled="isEdit" style="width: 100%">
            <el-option
              v-for="node in nodes"
              :key="node.id"
              :label="node.name"
              :value="node.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="标签" prop="tag">
          <el-input v-model="form.tag" placeholder="入站标签，例如: vless-in" />
        </el-form-item>
        <el-form-item label="协议" prop="protocol">
          <el-select v-model="form.protocol" placeholder="选择协议" style="width: 100%">
            <el-option label="VLESS" value="vless" />
            <el-option label="VMess" value="vmess" />
            <el-option label="Trojan" value="trojan" />
            <el-option label="Shadowsocks" value="shadowsocks" />
          </el-select>
        </el-form-item>
        <el-form-item label="端口" prop="port">
          <el-input-number v-model="form.port" :min="1" :max="65535" style="width: 100%" />
        </el-form-item>
        <el-form-item label="Settings" prop="settings_json">
          <el-input
            v-model="form.settings_json"
            type="textarea"
            :rows="6"
            placeholder="JSON 配置"
          />
        </el-form-item>
        <el-form-item label="Stream" prop="stream_json">
          <el-input
            v-model="form.stream_json"
            type="textarea"
            :rows="4"
            placeholder="JSON 传输层配置（可选）"
          />
        </el-form-item>
        <el-form-item v-if="isEdit" label="启用状态">
          <el-switch v-model="form.enabled" />
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
import { listInbounds, createInbound, updateInbound, deleteInbound } from '../api/inbounds'
import { listNodes } from '../api/nodes'
import type { Inbound, Node, UpdateInboundRequest } from '../types'
import type { FormInstance, FormRules } from 'element-plus'

const inbounds = ref<Inbound[]>([])
const nodes = ref<Node[]>([])
const loading = ref(false)
const nodeFilter = ref('')

const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const editingInboundId = ref('')
const formRef = ref<FormInstance>()

const form = reactive({
  node_id: '',
  tag: '',
  protocol: 'vless',
  port: 443,
  settings_json: '{}',
  stream_json: '',
  enabled: true,
})

const formRules: FormRules = {
  node_id: [{ required: true, message: '请选择节点', trigger: 'change' }],
  tag: [{ required: true, message: '请输入标签', trigger: 'blur' }],
  protocol: [{ required: true, message: '请选择协议', trigger: 'change' }],
  port: [{ required: true, message: '请输入端口', trigger: 'blur' }],
  settings_json: [
    { required: true, message: '请输入配置', trigger: 'blur' },
    {
      validator: (_rule: unknown, value: string, callback: (err?: Error) => void) => {
        try {
          JSON.parse(value)
          callback()
        } catch {
          callback(new Error('JSON 格式不正确'))
        }
      },
      trigger: 'blur',
    },
  ],
}

async function fetchInbounds() {
  loading.value = true
  try {
    const res = await listInbounds({ node_id: nodeFilter.value || undefined })
    inbounds.value = res.data || []
  } finally {
    loading.value = false
  }
}

async function fetchNodes() {
  const res = await listNodes()
  nodes.value = res.data || []
}

function showCreateDialog() {
  isEdit.value = false
  editingInboundId.value = ''
  form.node_id = ''
  form.tag = ''
  form.protocol = 'vless'
  form.port = 443
  form.settings_json = '{}'
  form.stream_json = ''
  form.enabled = true
  dialogVisible.value = true
}

function showEditDialog(inbound: Inbound) {
  isEdit.value = true
  editingInboundId.value = inbound.id
  form.node_id = inbound.node_id
  form.tag = inbound.tag
  form.protocol = inbound.protocol
  form.port = inbound.port
  form.settings_json = JSON.stringify(inbound.settings, null, 2)
  form.stream_json = inbound.stream_settings ? JSON.stringify(inbound.stream_settings, null, 2) : ''
  form.enabled = inbound.enabled
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
    const settings = JSON.parse(form.settings_json)
    const streamSettings = form.stream_json ? JSON.parse(form.stream_json) : undefined

    if (isEdit.value) {
      const data: UpdateInboundRequest = {
        protocol: form.protocol,
        port: form.port,
        settings,
        stream_settings: streamSettings,
        tag: form.tag,
        enabled: form.enabled,
      }
      await updateInbound(editingInboundId.value, data)
      ElMessage.success('更新成功')
    } else {
      await createInbound({
        node_id: form.node_id,
        protocol: form.protocol,
        port: form.port,
        settings,
        stream_settings: streamSettings,
        tag: form.tag,
      })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await fetchInbounds()
  } finally {
    submitLoading.value = false
  }
}

async function handleDelete(inbound: Inbound) {
  await ElMessageBox.confirm(`确定要删除入站「${inbound.tag}」吗？`, '确认删除', {
    confirmButtonText: '删除',
    cancelButtonText: '取消',
    type: 'warning',
  })
  await deleteInbound(inbound.id)
  ElMessage.success('删除成功')
  await fetchInbounds()
}

onMounted(() => {
  fetchInbounds()
  fetchNodes()
})
</script>

<style scoped>
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.header-actions {
  display: flex;
  gap: 12px;
}
</style>
