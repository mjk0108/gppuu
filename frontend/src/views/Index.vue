<template>
  <div class="page">
    <n-card class="main-card" title="GPP 加速器" size="large">
      <div class="status-wrap">
        <n-progress
          type="circle"
          :height="24"
          :status="percentageRef<=25?'error':percentageRef<=50?'warning':percentageRef<=75?'info':'success'"
          :percentage="percentageRef"
        >
          <n-space vertical size="small" style="text-align: center;">
            <span>{{ percentageRef === 100 ? '加速完成' : percentageRef === 0 ? '未开始' : '正在加速' }}</span>
            <div v-if="showGameHttpInfo">
              <p class="peer-line" @click="openNodeDialog">Game: {{ gamePeer ? gamePeer.name : '未选择' }}
                <n-gradient-text v-if="gamePeer" :type="pingColor(gamePeer.ping)"> {{ gamePeer.ping }}ms </n-gradient-text>
              </p>
              <p class="peer-line" @click="openNodeDialog">Http: {{ httpPeer ? httpPeer.name : '未选择' }}
                <n-gradient-text v-if="httpPeer" :type="pingColor(httpPeer.ping)"> {{ httpPeer.ping }}ms </n-gradient-text>
              </p>
            </div>
            <p v-if="showUpDowInfo">流量统计:
              <n-gradient-text v-if="down" type="success">{{ formatBytes(down) }}</n-gradient-text>
            </p>
          </n-space>
        </n-progress>
      </div>

      <n-space justify="center" class="action-row">
        <n-button type="primary" size="large" :disabled="btnDisabled" @click="!state ? start() : stop()">{{ btnText }}</n-button>
        <n-button size="large" @click="openNodeDialog">节点管理</n-button>
      </n-space>

      <n-space justify="center" class="tool-row">
        <n-button tertiary @click="refreshSub">刷新订阅</n-button>
        <n-button tertiary @click="exportConfigFile">导出配置</n-button>
        <n-button tertiary @click="importConfigFile(true)">导入(合并)</n-button>
        <n-button tertiary @click="importConfigFile(false)">导入(覆盖)</n-button>
      </n-space>

      <div class="version">v1.4.6</div>
    </n-card>

    <n-modal
      v-model:show="showModal"
      :mask-closable="false"
      preset="dialog"
      title="节点管理"
      positive-text="确认"
      negative-text="取消"
      @positive-click="submitCallback"
    >
      <n-select
        v-model:value="gameValue"
        filterable
        :options="gameOpt"
        placeholder="请选择 Game 节点"
        value-field="val"
        label-field="name"
      />
      <br />
      <n-select
        v-model:value="httpValue"
        filterable
        :options="httpOpt"
        placeholder="请选择 Http 节点"
        value-field="val"
        label-field="name"
      />
      <br />
      <n-input v-model:value="newUrl" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="导入单个节点或订阅地址" />
      <n-space v-if="recentImports.length" style="margin-top: 8px;" align="center" wrap>
        <n-text depth="3">最近导入:</n-text>
        <n-tag
          v-for="(item, idx) in recentImports"
          :key="idx"
          size="small"
          round
          @click="useRecent(item)"
          style="cursor:pointer;"
        >
          {{ shortText(item) }}
        </n-tag>
      </n-space>
      <br />
      <n-input v-model:value="batchUrls" type="textarea" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="批量导入：每行一个节点链接" />
      <n-space justify="end" style="margin-top: 8px;">
        <n-button size="small" @click="importBatch">批量导入</n-button>
      </n-space>
    </n-modal>
  </div>
</template>

<script lang="ts" setup>
import { ref, type Ref, onMounted, onBeforeUnmount } from 'vue'
import { Add, BatchAdd, ExportConfig, ImportConfig, List, RefreshSubscription, SetPeer, Start, Status, Stop } from '../../wailsjs/go/main/App'
import { SelectOption, SelectGroupOption, useMessage } from 'naive-ui'

const percentageRef = ref(0)
const state = ref(false)
const btnText = ref('开始加速')
const btnDisabled = ref(false)
const showModal = ref(false)
const gameOpt = ref<Array<SelectOption | SelectGroupOption>>([])
const httpOpt = ref<Array<SelectOption | SelectGroupOption>>([])
const gameValue = ref<string | undefined>()
const httpValue = ref<string | undefined>()

const gamePeer: Ref<any> = ref(null)
const httpPeer: Ref<any> = ref(null)
const down = ref<number>()

const showGameHttpInfo = ref(true)
const showUpDowInfo = ref(false)

const newUrl = ref<string>()
const batchUrls = ref<string>()
const recentImports = ref<string[]>([])

let timerHandle: number | null = null
const message = useMessage()

const pingColor = (ping: number) => (ping < 60 ? 'success' : ping < 100 ? 'warning' : 'error')
const formatBytes = (bytes: number) => {
  if (!bytes) return '0 KB'
  return bytes / 1024 > 1024 ? `${(bytes / 1024 / 1024).toFixed(2)} MB` : `${(bytes / 1024).toFixed(2)} KB`
}

onMounted(() => {
  loadRecentImports()
  getStatus()
  timerHandle = window.setInterval(getStatus, 1000)
})

onBeforeUnmount(() => {
  if (timerHandle !== null) {
    clearInterval(timerHandle)
    timerHandle = null
  }
})

const start = () => {
  btnDisabled.value = true
  showGameHttpInfo.value = false
  showUpDowInfo.value = true
  btnText.value = '加速中...'
  Start().then((res) => {
    if (res !== 'ok' && res !== 'running') {
      message.error(`加速失败: ${res}`)
      btnDisabled.value = false
      showUpDowInfo.value = false
      showGameHttpInfo.value = true
      return
    }
    state.value = true
    const anim = setInterval(() => {
      percentageRef.value += 10
      if (percentageRef.value >= 100) {
        percentageRef.value = 100
        clearInterval(anim)
        btnText.value = '结束加速'
        btnDisabled.value = false
      }
    }, 80)
  })
}

const stop = () => {
  Stop().then(() => {
    percentageRef.value = 0
    state.value = false
    showGameHttpInfo.value = true
    showUpDowInfo.value = false
    btnText.value = '开始加速'
  })
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

const openNodeDialog = () => {
  getList()
}

const getStatus = () => {
  Status().then((res) => {
    if (res.game_peer !== null || res.http_peer !== null) {
      gamePeer.value = res.game_peer
      httpPeer.value = res.http_peer
      down.value = res.down
      btnDisabled.value = false
      btnText.value = state.value ? '结束加速' : '开始加速'
      return
    }
    btnText.value = '没有节点'
    btnDisabled.value = true
  })
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
    message.success('订阅刷新成功')
    await getList()
  } else {
    message.error(`订阅刷新失败: ${res}`)
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
  newUrl.value = text
}
</script>

<style>
.page {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: #f6f8fb;
}

.main-card {
  width: 340px;
  border-radius: 14px;
}

.status-wrap {
  display: flex;
  justify-content: center;
  margin: 10px 0 18px;
}

.action-row {
  margin-top: 8px;
}

.tool-row {
  margin-top: 14px;
  flex-wrap: wrap;
}

.version {
  text-align: center;
  margin-top: 16px;
  color: #18a058;
  font-weight: 600;
}

.peer-line {
  cursor: pointer;
  margin: 2px 0;
}

.n-progress-content {
  width: 280px;
  height: 280px;
}

.n-progress-content svg {
  width: 280px;
  height: 280px;
}
</style>
