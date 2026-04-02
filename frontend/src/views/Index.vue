<template>
  <div class="page">
    <n-card class="main-card" title="GPP 加速器" size="large">
      <template #header-extra>
        <n-space>
          <n-button text type="primary" @click="openManageDialog">订阅管理</n-button>
          <n-button text type="primary" @click="openImportDialog">+ 导入订阅</n-button>
        </n-space>
      </template>

      <n-card embedded class="status-card">
        <n-space vertical size="small">
          <div class="status-item">
            <n-text depth="3">当前状态</n-text>
            <n-tag :type="runningStatus.type" size="small" round>{{ runningStatus.label }}</n-tag>
          </div>
          <div class="status-item">
            <n-text depth="3">当前节点</n-text>
            <n-text>{{ currentPeerName }}</n-text>
          </div>
          <div class="status-item">
            <n-text depth="3">网络延迟</n-text>
            <n-gradient-text v-if="currentPing > 0" :type="pingColor(currentPing)">{{ currentPing }} ms</n-gradient-text>
            <n-text v-else depth="3">--</n-text>
          </div>
          <div class="status-item" v-if="showUpDowInfo">
            <n-text depth="3">流量统计</n-text>
            <n-gradient-text type="success">{{ formatBytes(down || 0) }}</n-gradient-text>
          </div>
        </n-space>
      </n-card>

      <n-space vertical size="medium" class="action-area">
        <n-button type="primary" size="large" :disabled="btnDisabled" class="full-width-btn" @click="!state ? start() : stop()">
          {{ btnText }}
        </n-button>

        <n-space justify="space-between">
          <n-button size="large" @click="openNodeDialog">节点选择</n-button>
          <n-button quaternary @click="refreshSub">更新订阅</n-button>
        </n-space>

        <n-space>
          <n-button quaternary @click="exportConfigFile">导出配置</n-button>
          <n-button quaternary @click="importConfigFile(true)">导入(合并)</n-button>
          <n-button quaternary @click="importConfigFile(false)">导入(覆盖)</n-button>
        </n-space>
      </n-space>

      <div class="version">v1.4.9</div>
    </n-card>

    <n-modal
      v-model:show="showImportModal"
      :mask-closable="false"
      preset="dialog"
      title="导入订阅"
      positive-text="导入并更新"
      negative-text="取消"
      @positive-click="submitImportSubscription"
    >
      <n-tabs v-model:value="importMode" type="line" animated>
        <n-tab-pane name="url" tab="订阅链接">
          <n-space vertical size="small">
            <n-text depth="3">支持 Clash YAML 订阅</n-text>
            <n-input v-model:value="subscriptionUrl" placeholder="粘贴订阅链接（https://...）" clearable />
          </n-space>
        </n-tab-pane>
        <n-tab-pane name="qrcode" tab="二维码内容">
          <n-space vertical size="small">
            <n-text depth="3">支持粘贴二维码识别出的文本（clash:// 或直接链接）</n-text>
            <n-input v-model:value="qrContent" type="textarea" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="粘贴二维码内容" />
          </n-space>
        </n-tab-pane>
      </n-tabs>

      <n-space v-if="recentImports.length" align="center" wrap style="margin-top: 8px;">
        <n-text depth="3">最近导入:</n-text>
        <n-tag v-for="(item, idx) in recentImports" :key="idx" size="small" round @click="useRecent(item)" style="cursor: pointer">
          {{ shortText(item) }}
        </n-tag>
      </n-space>
    </n-modal>

    <n-modal v-model:show="showManageModal" :mask-closable="false" preset="dialog" title="订阅管理" positive-text="关闭" @positive-click="() => true">
      <n-space vertical>
        <n-button secondary @click="refreshSub">全部更新</n-button>
        <n-empty v-if="subscriptions.length === 0" description="暂无订阅" />
        <n-card v-for="(item, idx) in subscriptions" :key="idx" size="small" embedded>
          <n-space justify="space-between" align="center">
            <n-ellipsis style="max-width: 210px;">{{ item }}</n-ellipsis>
            <n-popconfirm @positive-click="removeSubscription(item)">
              <template #trigger>
                <n-button text type="error">删除</n-button>
              </template>
              确认删除该订阅？
            </n-popconfirm>
          </n-space>
        </n-card>
      </n-space>
    </n-modal>

    <n-modal
      v-model:show="showModal"
      :mask-closable="false"
      preset="dialog"
      title="节点选择"
      positive-text="确认"
      negative-text="取消"
      @positive-click="submitCallback"
    >
      <n-select v-model:value="gameValue" filterable :options="gameOpt" placeholder="请选择 Game 节点" value-field="val" label-field="name" />
      <br />
      <n-select v-model:value="httpValue" filterable :options="httpOpt" placeholder="请选择 Http 节点" value-field="val" label-field="name" />
      <br />
      <n-input v-model:value="newUrl" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="导入单个节点链接" />
      <br />
      <n-input v-model:value="batchUrls" type="textarea" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="批量导入：每行一个节点链接" />
      <n-space justify="end" style="margin-top: 8px;">
        <n-button size="small" @click="importBatch">批量导入</n-button>
      </n-space>
    </n-modal>
  </div>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref, type Ref } from 'vue'
import {
  Add,
  BatchAdd,
  DeleteSubscription,
  ExportConfig,
  ImportConfig,
  List,
  ListSubscriptions,
  RefreshSubscription,
  SetPeer,
  Start,
  Status,
  Stop
} from '../../wailsjs/go/main/App'
import { SelectGroupOption, SelectOption, useMessage } from 'naive-ui'

const state = ref(false)
const btnText = ref('一键连接')
const btnDisabled = ref(false)

const showModal = ref(false)
const showImportModal = ref(false)
const showManageModal = ref(false)

const importMode = ref<'url' | 'qrcode'>('url')
const gameOpt = ref<Array<SelectOption | SelectGroupOption>>([])
const httpOpt = ref<Array<SelectOption | SelectGroupOption>>([])
const gameValue = ref<string | undefined>()
const httpValue = ref<string | undefined>()

const gamePeer: Ref<any> = ref(null)
const httpPeer: Ref<any> = ref(null)
const down = ref<number>()
const showUpDowInfo = ref(false)

const newUrl = ref<string>()
const batchUrls = ref<string>()
const subscriptionUrl = ref<string>('')
const qrContent = ref<string>('')
const recentImports = ref<string[]>([])
const subscriptions = ref<string[]>([])

let timerHandle: number | null = null
const message = useMessage()

const pingColor = (ping: number) => (ping < 60 ? 'success' : ping < 100 ? 'warning' : 'error')
const formatBytes = (bytes: number) => {
  if (!bytes) return '0 KB'
  return bytes / 1024 > 1024 ? `${(bytes / 1024 / 1024).toFixed(2)} MB` : `${(bytes / 1024).toFixed(2)} KB`
}

const currentPeerName = computed(() => gamePeer.value?.name || httpPeer.value?.name || '未选择节点')
const currentPing = computed(() => Number(gamePeer.value?.ping || httpPeer.value?.ping || 0))
const runningStatus = computed(() => {
  if (btnDisabled.value) return { label: '无可用节点', type: 'error' as const }
  if (state.value) return { label: '已连接', type: 'success' as const }
  return { label: '未连接', type: 'warning' as const }
})

onMounted(() => {
  loadRecentImports()
  getStatus()
  loadSubscriptions()
  timerHandle = window.setInterval(getStatus, 1000)
})

onBeforeUnmount(() => {
  if (timerHandle !== null) {
    clearInterval(timerHandle)
    timerHandle = null
  }
})

const start = async () => {
  btnDisabled.value = true
  showUpDowInfo.value = true
  btnText.value = '连接中...'
  const res = await Start()
  if (res !== 'ok' && res !== 'running') {
    message.error(`连接失败: ${res}`)
    btnText.value = '一键连接'
    btnDisabled.value = false
    showUpDowInfo.value = false
    return
  }
  state.value = true
  btnText.value = '断开连接'
  btnDisabled.value = false
  message.success('连接成功')
}

const stop = async () => {
  await Stop()
  state.value = false
  showUpDowInfo.value = false
  btnText.value = '一键连接'
  message.success('已断开连接')
}

const getList = async () => {
  showModal.value = true
  httpOpt.value = []
  gameOpt.value = []
  const res = await List()
  res.forEach((item) => {
    const option = { name: `${item.name}-${item.ping}ms`, val: item.name }
    if (!item.name.startsWith('http')) gameOpt.value.push(option)
    if (!item.name.startsWith('game')) httpOpt.value.push(option)
  })
}

const openNodeDialog = () => getList()

const openImportDialog = () => {
  subscriptionUrl.value = ''
  qrContent.value = ''
  importMode.value = 'url'
  showImportModal.value = true
}

const openManageDialog = async () => {
  await loadSubscriptions()
  showManageModal.value = true
}

const getStatus = () => {
  Status().then((res) => {
    state.value = !!res.running
    if (res.game_peer !== null || res.http_peer !== null) {
      gamePeer.value = res.game_peer
      httpPeer.value = res.http_peer
      down.value = res.down
      btnDisabled.value = false
      btnText.value = state.value ? '断开连接' : '一键连接'
      return
    }
    btnText.value = '无可用节点'
    btnDisabled.value = true
  })
}

const normalizeSubscriptionInput = (raw: string) => {
  let text = raw.trim()
  if (!text) return ''
  if (text.startsWith('clash://')) {
    const payload = text.replace(/^clash:\/\//i, '')
    try {
      text = atob(payload)
    } catch {
      return ''
    }
  }
  return text.trim()
}

const submitImportSubscription = async () => {
  const source = importMode.value === 'url' ? subscriptionUrl.value : qrContent.value
  const url = normalizeSubscriptionInput(source)
  if (!url) {
    message.warning('请先输入订阅内容')
    return false
  }
  if (!/^https?:\/\//i.test(url)) {
    message.error('未识别到有效链接，请检查二维码内容')
    return false
  }

  const before = await List()
  const addRes = await Add(url)
  if (addRes !== 'ok') {
    message.error(`订阅保存失败: ${addRes}`)
    return false
  }

  const refreshRes = await RefreshSubscription()
  if (refreshRes !== 'ok') {
    message.error(`订阅更新失败: ${refreshRes}`)
    return false
  }

  saveRecentImport(url)
  await loadSubscriptions()
  const after = await List()
  const addedCount = Math.max(after.length - before.length, 0)
  message.success(addedCount > 0 ? `导入成功，新增 ${addedCount} 个节点` : '订阅更新成功')
  getStatus()
  return true
}

const importBatch = async () => {
  if (!batchUrls.value?.trim()) {
    message.warning('请先粘贴批量节点')
    return
  }
  const result = await BatchAdd(batchUrls.value)
  saveRecentImport(batchUrls.value)
  message.success(result)
  batchUrls.value = undefined
  await getList()
}

const refreshSub = async () => {
  const res = await RefreshSubscription()
  if (res === 'ok') {
    message.success('订阅更新成功')
    await getList()
  } else {
    message.error(`订阅更新失败: ${res}`)
  }
}

const loadSubscriptions = async () => {
  try {
    subscriptions.value = await ListSubscriptions()
  } catch {
    subscriptions.value = []
  }
}

const removeSubscription = async (addr: string) => {
  const res = await DeleteSubscription(addr)
  if (res === 'ok') {
    message.success('订阅已删除')
    await loadSubscriptions()
  } else {
    message.error(`删除失败: ${res}`)
  }
}

const exportConfigFile = async () => {
  const res = await ExportConfig()
  if (res === 'cancel') return
  if (res.endsWith('.json')) {
    message.success(`已导出: ${res}`)
    return
  }
  message.error(`导出失败: ${res}`)
}

const importConfigFile = async (merge: boolean) => {
  const res = await ImportConfig(merge)
  if (res === 'cancel') return
  if (res === 'ok') {
    message.success(merge ? '导入并合并成功' : '导入并覆盖成功')
    getStatus()
    await loadSubscriptions()
    await getList()
    return
  }
  message.error(`导入失败: ${res}`)
}

const submitCallback = () => {
  if (newUrl.value !== undefined && gameValue.value !== undefined && httpValue.value !== undefined) {
    message.error('只能选择一种方式')
    newUrl.value = undefined
    gameValue.value = undefined
    httpValue.value = undefined
    return
  }
  if (newUrl.value !== undefined) {
    Add(newUrl.value).then((res) => {
      if (res === 'ok') {
        saveRecentImport(newUrl.value as string)
        message.success('导入连接成功')
        newUrl.value = undefined
      } else {
        message.error(`导入连接失败: ${res}`)
      }
    })
  }
  if (gameValue.value !== undefined || httpValue.value !== undefined) {
    if (gameValue.value === undefined) {
      message.error('请选择 Game 节点')
      httpValue.value = undefined
      return
    }
    if (httpValue.value === undefined) {
      message.error('请选择 Http 节点')
      gameValue.value = undefined
      return
    }
    SetPeer(gameValue.value, httpValue.value).then((res) => {
      if (res === 'ok') {
        message.success('设置节点成功')
        gameValue.value = undefined
        httpValue.value = undefined
      } else {
        message.error('设置节点失败')
      }
    })
  }
}

const RECENT_IMPORT_KEY = 'gpp_recent_imports_v1'
const loadRecentImports = () => {
  try {
    const raw = localStorage.getItem(RECENT_IMPORT_KEY)
    if (!raw) return
    const arr = JSON.parse(raw)
    if (Array.isArray(arr)) {
      recentImports.value = arr.filter((x) => typeof x === 'string').slice(0, 8)
    }
  } catch {
    recentImports.value = []
  }
}

const saveRecentImport = (text: string) => {
  const val = text?.trim()
  if (!val) return
  const merged = [val, ...recentImports.value.filter((x) => x !== val)].slice(0, 8)
  recentImports.value = merged
  localStorage.setItem(RECENT_IMPORT_KEY, JSON.stringify(merged))
}

const shortText = (text: string) => {
  const t = text.replace(/\s+/g, ' ')
  return t.length > 22 ? `${t.slice(0, 22)}...` : t
}

const useRecent = (text: string) => {
  if (importMode.value === 'qrcode') qrContent.value = text
  else subscriptionUrl.value = text
}
</script>

<style>
.page {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: #f3f5f9;
  padding: 20px;
}

.main-card {
  width: 380px;
  border-radius: 14px;
}

.status-card {
  border-radius: 12px;
  margin-bottom: 14px;
}

.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.action-area {
  margin-top: 8px;
}

.full-width-btn {
  width: 100%;
  height: 44px;
}

.version {
  text-align: center;
  margin-top: 14px;
  color: #18a058;
  font-weight: 600;
}
</style>
