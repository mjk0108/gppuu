<template>
  <div class="page apple-bg">
    <n-card class="main-card glass" :bordered="false">
      <div class="hero">
        <div>
          <div class="app-title">GPP 加速器</div>
          <div class="app-subtitle">简洁 · 稳定 · 一键连接</div>
        </div>
        <n-space>
          <n-button type="primary" class="import-btn" @click="openImportDialog">+ 导入节点</n-button>
        </n-space>
      </div>

      <div class="status-panel">
        <div class="status-row">
          <span class="label">状态</span>
          <n-tag :type="runningStatus.type" round size="small">{{ runningStatus.label }}</n-tag>
        </div>
        <div class="status-row">
          <span class="label">游戏代理节点</span>
          <n-ellipsis style="max-width: 320px">{{ gameNodeName }}</n-ellipsis>
        </div>
        <div class="status-row">
          <span class="label">直连/HTTP节点</span>
          <n-ellipsis style="max-width: 320px">{{ httpNodeName }}</n-ellipsis>
        </div>
        <div class="status-row">
          <span class="label">网络延迟</span>
          <n-gradient-text v-if="currentPing > 0" :type="pingColor(currentPing)">{{ currentPing }} ms</n-gradient-text>
          <span v-else class="value-muted">--</span>
        </div>
        <div class="status-row" v-if="showUpDowInfo">
          <span class="label">流量统计</span>
          <n-gradient-text type="success">{{ formatBytes(down || 0) }}</n-gradient-text>
        </div>
      </div>

      <div class="quick-actions">
        <n-button class="pill-btn" @click="openNodeDialog">节点选择</n-button>
        <n-button class="pill-btn" @click="openNodeDialog">导入节点</n-button>
        <n-button class="pill-btn" @click="openRuleDialog">规则设置</n-button>
        <n-button class="pill-btn" @click="exportConfigFile">导出配置</n-button>
      </div>

      <div class="main-cta-wrap">
        <n-button
          type="primary"
          size="large"
          class="main-cta"
          :disabled="btnDisabled"
          @click="!state ? start() : stop()"
        >
          {{ btnText }}
        </n-button>
      </div>

      <div class="footer-row">
        <n-button text @click="importConfigFile(true)">导入(合并)</n-button>
        <n-button text @click="importConfigFile(false)">导入(覆盖)</n-button>
        <span class="version">v1.5.0</span>
      </div>
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
            <n-text depth="3">支持 Clash YAML 与机场常见 base64 订阅</n-text>
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
            <n-ellipsis style="max-width: 260px;">{{ item }}</n-ellipsis>
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
      v-model:show="showRuleModal"
      :mask-closable="false"
      preset="dialog"
      title="规则设置（游戏加速）"
      positive-text="保存规则"
      negative-text="取消"
      @positive-click="saveRules"
    >
      <n-space vertical>
        <n-text depth="3">示例：商店走代理，下载直连</n-text>
        <n-input
          v-model:value="ruleText"
          type="textarea"
          :autosize="{ minRows: 8, maxRows: 14 }"
          placeholder="# DIRECT domain_suffix steamcontent.com,cm.steampowered.com&#10;# PROXY domain_suffix steampowered.com,steamcommunity.com"
        />
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
  GetRuleText,
  ImportConfig,
  List,
  ListSubscriptions,
  RefreshSubscription,
  SaveRuleText,
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
const showRuleModal = ref(false)

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
const ruleText = ref<string>('')
const recentImports = ref<string[]>([])
const subscriptions = ref<string[]>([])

let timerHandle: number | null = null
const message = useMessage()

const pingColor = (ping: number) => (ping < 60 ? 'success' : ping < 100 ? 'warning' : 'error')
const formatBytes = (bytes: number) => {
  if (!bytes) return '0 KB'
  return bytes / 1024 > 1024 ? `${(bytes / 1024 / 1024).toFixed(2)} MB` : `${(bytes / 1024).toFixed(2)} KB`
}

const gameNodeName = computed(() => gamePeer.value?.name || '未选择')
const httpNodeName = computed(() => httpPeer.value?.name || '未选择')
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
  message.info('订阅导入已下线，请使用节点导入')
  openNodeDialog()
}

const openRuleDialog = async () => {
  ruleText.value = await GetRuleText()
  showRuleModal.value = true
}

const saveRules = async () => {
  const res = await SaveRuleText(ruleText.value)
  if (res === 'ok') {
    message.success('规则已保存，重新连接后生效')
    return true
  }
  message.error(`规则保存失败: ${res}`)
  return false
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
  await getList()
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
  message.info('订阅更新已下线，请改用节点导入')
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
    // 只选一个时，自动双写，避免用户重复选择
    if (gameValue.value === undefined && httpValue.value !== undefined) {
      gameValue.value = httpValue.value
      message.info('已自动将游戏节点设置为同一节点')
    }
    if (httpValue.value === undefined && gameValue.value !== undefined) {
      httpValue.value = gameValue.value
      message.info('已自动将直连/HTTP节点设置为同一节点')
    }
    if (gameValue.value === undefined || httpValue.value === undefined) {
      message.error('请选择节点')
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
  return t.length > 26 ? `${t.slice(0, 26)}...` : t
}

const useRecent = (text: string) => {
  if (importMode.value === 'qrcode') qrContent.value = text
  else subscriptionUrl.value = text
}
</script>

<style>
:root {
  --apple-bg-1: #e9efff;
  --apple-bg-2: #f7faff;
  --apple-primary: #4f7cff;
}

.page {
  height: 100vh;
  width: 100vw;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 18px;
  overflow: hidden;
  box-sizing: border-box;
}

.apple-bg {
  background: radial-gradient(circle at 12% 12%, #dce8ff 0, transparent 45%),
    radial-gradient(circle at 85% 0%, #e8f0ff 0, transparent 40%),
    linear-gradient(180deg, var(--apple-bg-1), var(--apple-bg-2));
}

.main-card {
  width: min(920px, 96vw);
  height: min(700px, 92vh);
  border-radius: 24px;
  overflow: hidden;
}

.glass {
  background: rgba(255, 255, 255, 0.72);
  box-shadow: 0 20px 60px rgba(31, 63, 120, 0.15);
  backdrop-filter: blur(10px);
}

.hero {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.app-title {
  font-size: 34px;
  font-weight: 700;
  line-height: 1.2;
  color: #1f2a44;
}

.app-subtitle {
  margin-top: 6px;
  color: #6b7890;
  font-size: 14px;
}

.import-btn {
  border-radius: 14px;
  height: 38px;
  background: linear-gradient(135deg, #5b86ff, #4f7cff);
}

.ghost-btn {
  border-radius: 14px;
}

.status-panel {
  border-radius: 18px;
  padding: 16px 18px;
  background: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(120, 140, 180, 0.18);
}

.status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 34px;
}

.label {
  color: #6b7890;
  font-size: 14px;
}

.value-muted {
  color: #9aa5ba;
}

.quick-actions {
  margin-top: 14px;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}

.pill-btn {
  border-radius: 14px;
  height: 40px;
  background: rgba(255, 255, 255, 0.86);
}

.main-cta-wrap {
  margin-top: 20px;
}

.main-cta {
  width: 100%;
  height: 52px;
  border-radius: 16px;
  font-size: 16px;
  font-weight: 600;
  background: linear-gradient(135deg, #5b86ff, #4f7cff);
}

.footer-row {
  margin-top: 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.version {
  color: #8c97ab;
  font-size: 12px;
}
</style>
