<template>
  <div class="page-stack">
    <PageHeader
      title="配置中心"
      :description="
        role === 'server'
          ? '在线校验并应用服务端 TOML 配置，重启失败会自动回滚。'
          : '管理主配置和 include 文件，校验后热加载或完整重启。'
      "
      eyebrow="CONFIGURATION"
    >
      <a-button @click="historyVisible = true">
        <template #icon><IconHistory /></template>
        配置历史
      </a-button>
      <a-button @click="validateCurrent">
        <template #icon><IconCheckCircle /></template>
        校验
      </a-button>
      <a-button type="primary" :loading="saving" @click="save(false)">
        <template #icon><IconSave /></template>
        保存
      </a-button>
    </PageHeader>

    <div v-if="loading" class="panel">
      <a-skeleton :animation="true" class="panel-body">
        <a-space direction="vertical" fill>
          <a-skeleton-line :rows="3" />
          <a-skeleton-line :rows="10" />
        </a-space>
      </a-skeleton>
    </div>

    <div v-else class="config-workspace">
      <aside class="config-files" aria-label="配置文件列表">
        <div class="config-pane-head">
          <strong>配置文件</strong>
          <a-tooltip content="新建 TOML 文件">
            <a-button size="mini" shape="circle" @click="openCreate">
              <IconPlus />
            </a-button>
          </a-tooltip>
        </div>
        <div class="file-list">
          <button
            v-for="file in files"
            :key="file.path"
            type="button"
            class="file-row"
            :class="{ active: currentPath === file.path }"
            @click="selectFile(file.path)"
          >
            <IconFile />
            <span>{{ file.path }}</span>
            <a-tag v-if="file.main" size="small" color="arcoblue">主配置</a-tag>
          </button>
        </div>
      </aside>

      <section class="editor-pane" aria-label="TOML 编辑器">
        <div class="editor-toolbar">
          <div class="editor-file">
            <IconCode />
            <span>{{ currentPath || '未选择文件' }}</span>
            <a-tag v-if="dirty" size="small" color="orange">未保存</a-tag>
          </div>
          <a-space>
            <a-button
              v-if="files.length > 1"
              size="small"
              @click="filesVisible = true"
            >
              文件
            </a-button>
            <a-button
              v-if="!isMain && currentPath"
              size="small"
              @click="openRename"
            >
              重命名
            </a-button>
            <a-button
              size="small"
              :disabled="isMain"
              @click="openDelete"
              status="danger"
            >
              删除
            </a-button>
            <a-button size="small" @click="resetEditor">还原</a-button>
            <a-button
              size="small"
              @click="sideVisible = true"
            >
              校验与历史
            </a-button>
          </a-space>
        </div>
        <div class="editor-body">
          <TomlEditor
            v-if="currentPath"
            v-model="content"
            :dark="isDark"
          />
          <div v-else class="empty-state">
            <div class="empty-icon"><IconFile /></div>
            <strong>请选择配置文件</strong>
            <p>从左侧文件列表选择主配置或 include 文件。</p>
          </div>
        </div>
        <div class="editor-toolbar">
          <span class="editor-hint">
            revision {{ revision.slice(0, 10) || '新文件' }}
          </span>
          <a-space>
            <a-button :loading="saving" @click="save(false)">保存配置</a-button>
            <a-button
              type="primary"
              :loading="saving"
              :disabled="!currentPath"
              @click="save(true)"
            >
              <template #icon><IconRefresh /></template>
              {{ role === 'server' ? '保存并重启' : '保存并应用' }}
            </a-button>
          </a-space>
        </div>
      </section>

      <aside class="config-side" aria-label="校验结果和配置历史">
        <div class="config-pane-head">
          <strong>校验结果</strong>
          <StatusBadge
            :status="validation?.valid ? 'ok' : 'error'"
            :label="validation ? (validation.valid ? '通过' : '未通过') : '待校验'"
          />
        </div>
        <div class="validation-list">
          <div
            v-if="validation"
            class="validation-item"
            :class="validation.valid ? 'success' : 'error'"
          >
            {{ validation.valid ? 'TOML 语法和配置校验通过' : '配置存在错误，请修正后重试' }}
          </div>
          <div
            v-for="error in validation?.errors || []"
            :key="error"
            class="validation-item error"
          >
            {{ error }}
          </div>
          <div
            v-for="warning in validation?.warnings || []"
            :key="warning"
            class="validation-item warning"
          >
            {{ warning }}
          </div>
          <div v-if="!validation" class="validation-item">
            编辑后点击“校验”或直接保存，服务端会执行严格校验。
          </div>
        </div>
        <div class="config-pane-head">
          <strong>最近版本</strong>
        </div>
        <div class="history-list">
          <a-empty v-if="history.length === 0" description="暂无历史版本" />
          <button
            v-for="entry in history.slice(0, 5)"
            :key="entry.name"
            class="file-row"
            type="button"
            @click="restoreHistory(entry)"
          >
            <IconHistory />
            <span>{{ formatDate(entry.createdAt) }}</span>
          </button>
        </div>
      </aside>
    </div>

    <a-drawer
      v-model:visible="filesVisible"
      title="配置文件"
      :width="380"
      unmount-on-close
    >
      <div class="file-list">
        <button
          v-for="file in files"
          :key="file.path"
          class="file-row"
          :class="{ active: currentPath === file.path }"
          type="button"
          @click="selectFile(file.path); filesVisible = false"
        >
          <IconFile />
          <span>{{ file.path }}</span>
        </button>
      </div>
    </a-drawer>

    <a-drawer
      v-model:visible="sideVisible"
      title="校验与历史"
      :width="440"
      unmount-on-close
    >
      <a-space direction="vertical" fill size="medium">
        <a-button long @click="validateCurrent">重新校验</a-button>
        <div
          v-for="error in validation?.errors || []"
          :key="error"
          class="validation-item error"
        >
          {{ error }}
        </div>
        <div
          v-for="warning in validation?.warnings || []"
          :key="warning"
          class="validation-item warning"
        >
          {{ warning }}
        </div>
        <div v-if="validation?.valid" class="validation-item success">
          TOML 语法和配置校验通过
        </div>
        <a-divider>历史版本</a-divider>
        <button
          v-for="entry in history"
          :key="entry.name"
          class="file-row"
          type="button"
          @click="restoreHistory(entry)"
        >
          <IconHistory />
          <span>{{ formatDate(entry.createdAt) }}</span>
        </button>
      </a-space>
    </a-drawer>

    <a-drawer
      v-model:visible="historyVisible"
      title="配置历史"
      :width="420"
      unmount-on-close
    >
      <div class="history-list">
        <a-empty v-if="history.length === 0" description="暂无历史版本" />
        <button
          v-for="entry in history"
          :key="entry.name"
          class="file-row"
          type="button"
          @click="restoreHistory(entry); historyVisible = false"
        >
          <IconHistory />
          <span>{{ formatDate(entry.createdAt) }}</span>
        </button>
      </div>
    </a-drawer>

    <a-modal
      v-model:visible="createVisible"
      title="新建配置文件"
      :ok-loading="creating"
      ok-text="创建"
      @ok="createFile"
    >
      <a-form :model="{ path: createPath }" layout="vertical">
        <a-form-item label="相对路径" required>
          <a-input
            v-model="createPath"
            placeholder="conf.d/web.toml"
            allow-clear
          />
        </a-form-item>
        <a-form-item v-if="role === 'client'">
          <a-checkbox v-model="createInclude">自动加入 includes 列表</a-checkbox>
        </a-form-item>
        <a-alert type="info" :show-icon="true">
          文件必须位于配置根目录内，并只包含代理或访问者配置。
        </a-alert>
      </a-form>
    </a-modal>

    <a-modal
      v-model:visible="renameVisible"
      title="重命名配置文件"
      :ok-loading="renaming"
      @ok="renameFile"
    >
      <a-form :model="{ path: renamePath }" layout="vertical">
        <a-form-item label="新相对路径" required>
          <a-input v-model="renamePath" placeholder="conf.d/web.toml" />
        </a-form-item>
        <a-checkbox v-model="renameIncludes">
          同步更新主配置 includes
        </a-checkbox>
      </a-form>
    </a-modal>

    <a-modal
      v-model:visible="restartVisible"
      title="应用配置"
      :ok-loading="restarting"
      ok-text="确认重启"
      @ok="restartService"
    >
      <a-alert type="warning" :show-icon="true">
        {{
          role === 'server'
            ? '服务端将完整重启，现有客户端连接会短暂中断。'
            : '客户端公共配置变化需要完整重启；仅代理变化通常可热加载。'
        }}
      </a-alert>
      <p class="restart-copy">配置已经保存。新进程启动失败时会自动回滚。</p>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { Message, Modal } from '@arco-design/web-vue'
import {
  IconCheckCircle,
  IconCode,
  IconFile,
  IconHistory,
  IconPlus,
  IconRefresh,
  IconSave,
} from '@arco-design/web-vue/es/icon'
import {
  adminAPI,
  legacyRequest,
  type ConfigDocument,
  type ConfigFileInfo,
  type ConfigHistoryEntry,
  type ConfigValidation,
} from '../admin'
import { useDark } from '@vueuse/core'
import PageHeader from './PageHeader.vue'
import StatusBadge from './StatusBadge.vue'
import TomlEditor from './TomlEditor.vue'

const props = defineProps<{
  role: 'server' | 'client'
}>()

const isDark = useDark()
const loading = ref(true)
const saving = ref(false)
const restarting = ref(false)
const files = ref<ConfigFileInfo[]>([])
const currentPath = ref('')
const content = ref('')
const loadError = ref('')
const revision = ref('')
const dirty = ref(false)
const validation = ref<ConfigValidation>()
const history = ref<ConfigHistoryEntry[]>([])
const filesVisible = ref(false)
const sideVisible = ref(false)
const historyVisible = ref(false)
const createVisible = ref(false)
const createPath = ref('')
const createInclude = ref(true)
const creating = ref(false)
const restartVisible = ref(false)
const renameVisible = ref(false)
const renamePath = ref('')
const renameIncludes = ref(true)
const renaming = ref(false)

const isMain = computed(
  () => files.value.find((item) => item.path === currentPath.value)?.main,
)

onMounted(async () => {
  try {
    const tree = await adminAPI.configTree()
    files.value = tree.files
    await selectFile(tree.files.find((item) => item.main)?.path || tree.files[0]?.path || '')
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '加载配置失败'
  } finally {
    loading.value = false
  }
})

async function refreshTree() {
  const tree = await adminAPI.configTree()
  files.value = tree.files
}

async function selectFile(path: string) {
  if (!path) return
  if (dirty.value && currentPath.value) {
    Modal.warning({
      title: '未保存的修改',
      content: '切换文件会丢失当前未保存内容。',
      hideCancel: true,
    })
    return
  }
  try {
    const document = await adminAPI.configFile(path)
    applyDocument(document)
    validation.value = undefined
    history.value = await adminAPI.configHistory(path)
    currentPath.value = path
    dirty.value = false
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '读取配置失败')
  }
}

function applyDocument(document: ConfigDocument) {
  currentPath.value = document.path
  content.value = document.content
  revision.value = document.revision
}

function resetEditor() {
  if (!currentPath.value) return
  selectFile(currentPath.value)
}

async function validateCurrent() {
  if (!currentPath.value) return
  try {
    validation.value = await adminAPI.validateConfig(
      currentPath.value,
      content.value,
    )
    if (validation.value.valid) {
      Message.success('配置校验通过')
    } else {
      Message.error('配置校验未通过')
    }
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '校验失败')
  }
}

async function save(applyAfterSave: boolean) {
  if (!currentPath.value) return
  saving.value = true
  try {
    const document = await adminAPI.saveConfigFile(
      currentPath.value,
      content.value,
      revision.value,
    )
    applyDocument(document)
    dirty.value = false
    validation.value = await adminAPI.validateConfig(
      currentPath.value,
      content.value,
    )
    history.value = await adminAPI.configHistory(currentPath.value)
    Message.success('配置已保存')
    if (applyAfterSave) {
      restartVisible.value = true
    }
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function restartService() {
  restarting.value = true
  try {
    if (props.role === 'client') {
      await legacyRequest<void>('/api/reload?strictConfig=true')
      Message.success('客户端配置已热加载')
      restartVisible.value = false
      return
    }
    await adminAPI.restart()
    restartVisible.value = false
    Message.success('服务正在重启，请稍候刷新')
    await waitForService()
  } catch (error) {
    Message.warning(
      `${error instanceof Error ? error.message : '应用失败'}，可尝试完整重启`,
    )
  } finally {
    restarting.value = false
  }
}

async function waitForService() {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    await new Promise((resolve) => window.setTimeout(resolve, 1_000))
    try {
      const response = await fetch('/healthz', { cache: 'no-store' })
      if (response.ok) {
        Message.success('服务已恢复')
        window.location.reload()
        return
      }
    } catch {
      // Keep polling while the process restarts.
    }
  }
  Message.warning('服务尚未恢复，请检查 Supervisor 状态和宿主机日志')
}

function openCreate() {
  createPath.value = props.role === 'client' ? 'conf.d/new-proxy.toml' : ''
  createVisible.value = true
}

async function createFile() {
  if (!createPath.value.trim()) {
    Message.warning('请输入文件路径')
    return
  }
  creating.value = true
  try {
    const document = await adminAPI.createConfigFile(
      createPath.value.trim(),
      '',
      createInclude.value,
    )
    await refreshTree()
    applyDocument(document)
    createVisible.value = false
    Message.success('配置文件已创建')
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '创建失败')
  } finally {
    creating.value = false
  }
}

function openRename() {
  renamePath.value = currentPath.value
  renameIncludes.value = true
  renameVisible.value = true
}

async function renameFile() {
  if (!renamePath.value.trim() || renamePath.value === currentPath.value) return
  renaming.value = true
  try {
    const document = await adminAPI.renameConfigFile(
      currentPath.value,
      renamePath.value.trim(),
      renameIncludes.value,
    )
    await refreshTree()
    applyDocument(document)
    renameVisible.value = false
    Message.success('配置文件已重命名')
  } catch (error) {
    Message.error(error instanceof Error ? error.message : '重命名失败')
  } finally {
    renaming.value = false
  }
}

function openDelete() {
  if (!isMain.value) {
    Modal.confirm({
      title: '删除配置文件',
      content: `确认删除 ${currentPath.value}？删除前会自动保存历史快照。`,
      okText: '删除',
      okButtonProps: { status: 'danger' },
      onOk: async () => {
        await adminAPI.deleteConfigFile(currentPath.value)
        await refreshTree()
        const next = files.value.find((item) => item.main) || files.value[0]
        if (next) await selectFile(next.path)
        Message.success('配置文件已删除')
      },
    })
  }
}

async function restoreHistory(entry: ConfigHistoryEntry) {
  Modal.confirm({
    title: '恢复历史版本',
    content: `将 ${entry.path} 恢复到 ${formatDate(entry.createdAt)} 的版本？`,
    onOk: async () => {
      const restored = await adminAPI.restoreConfigHistory(
        entry.path,
        entry.name,
      )
      await selectFile(restored.path)
      Message.success('历史版本已恢复')
    },
  })
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value))
}

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return window.confirm('当前配置有未保存修改，确定离开吗？')
})
</script>

<style scoped>
.editor-hint {
  color: var(--frp-muted);
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 11px;
}

.restart-copy {
  margin: 14px 0 0;
  color: var(--frp-muted-strong);
}
</style>
