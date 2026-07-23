<template>
  <div class="page">
    <div class="topbar">
      <span class="topbar-title">配置文件</span>
      <div class="topbar-right">
        <div class="tabs" style="margin:0;border:none">
          <button class="tab-btn" :class="{ on: tab === 'generate' }" @click="tab = 'generate'">生成配置</button>
          <button class="tab-btn" :class="{ on: tab === 'upload'   }" @click="tab = 'upload'">上传配置</button>
        </div>
      </div>
    </div>
    <div class="page">

      <!-- ═══════════════════════ GENERATE ════════════════════════════ -->
      <template v-if="tab === 'generate'">
        <div style="display:flex;flex-direction:column;gap:14px">

          <div class="alert alert-info" style="display:flex;align-items:center;justify-content:space-between;gap:12px">
            <span>通过向导逐步配置各项参数，生成可用的 sing-box 配置。每个配置可独立绑定一个订阅。</span>
            <button class="btn btn-primary btn-sm" style="white-space:nowrap" @click="openWizard(null)">
              ＋ 新增配置
            </button>
          </div>

          <div v-if="!profilesStore.profiles.length" class="card">
            <div class="empty">
              点击右上角「新增配置」创建第一个向导配置。
            </div>
          </div>

          <div v-for="prof in profilesStore.profiles" :key="prof.id" class="card">
            <div class="flex items-center gap-3 mb-3">
              <span style="font-size:22px">📄</span>
              <div>
                <div style="font-weight:700">{{ prof.name }}</div>
                <div class="text-xs text-muted monospace">
                  <template v-if="prof.updatedAt">{{ fmtTime(prof.updatedAt) }}</template>
                </div>
              </div>
              <div class="ml-auto flex gap-2">
                <button class="btn btn-ghost btn-sm" @click="validateProfile(prof)"
                  :disabled="!prof.wizardConfig || validating===prof.id">
                  {{ validating===prof.id ? '验证中…' : '✓ 验证' }}
                </button>
                <button class="btn btn-ghost btn-sm" @click="openWizard(prof)">
                  ✎ 编辑配置
                </button>
                <button class="btn btn-danger btn-sm" @click="deleteProfile(prof.id)">
                  删除
                </button>
              </div>
            </div>
            <div v-if="prof.wizardConfig" class="alert alert-success text-xs">
              ✓ 已完成配置，可从仪表盘启动
            </div>
            <div v-else class="alert alert-warn text-xs">
              尚未完成向导，点击「编辑配置」继续
            </div>
            <!-- Validation results -->
            <template v-if="validationResults[prof.id]">
              <div v-if="validationResults[prof.id].ok" class="alert alert-success text-xs mt-2">
                ✓ 配置验证通过，所有引用均有效
              </div>
              <div v-else class="alert alert-error mt-2">
                <div class="text-xs font-bold mb-1">⚠ 配置存在引用错误：</div>
                <div v-for="(e,i) in validationResults[prof.id].errors" :key="i"
                  class="text-xs" style="margin-top:3px">
                  <code style="background:rgba(0,0,0,.1);padding:0 4px;border-radius:3px">{{ e.location }}</code>
                  {{ e.message }}
                </div>
              </div>
            </template>
          </div>

        </div>
      </template>

      <!-- ═══════════════════════ UPLOAD ═══════════════════════════════ -->
      <template v-if="tab === 'upload'">
        <div style="display:flex;flex-direction:column;gap:14px">

          <div class="card">
            <div class="card-title">上传配置文件</div>
            <label class="upload-drop" :class="{ over: dragOver }"
              @dragover.prevent="dragOver=true" @dragleave="dragOver=false" @drop.prevent="onDrop">
              <input type="file" accept=".json" style="display:none" ref="fileInput" @change="onFileChange" />
              <span style="font-size:36px">📁</span>
              <span style="font-size:13px;color:var(--text3)">拖拽或点击上传 sing-box JSON 配置（仅 .json）</span>
            </label>
            <div v-if="uploadErr" class="alert alert-error mt-2">{{ uploadErr }}</div>
            <div v-if="uploadOk"  class="alert alert-success mt-2">{{ uploadOk }}</div>
          </div>

          <div class="card">
            <div class="card-title-row" style="display:flex;align-items:center;justify-content:space-between;margin-bottom:10px">
              <span class="card-title" style="margin:0">已上传配置</span>
              <span style="font-size:12px;color:var(--text3)">{{ uploadedConfigs.length }} 个文件</span>
            </div>
            <div v-if="!uploadedConfigs.length" class="empty" style="padding:12px 0">暂无上传配置</div>
            <div v-else style="display:flex;flex-direction:column;gap:8px">
              <div v-for="cfg in uploadedConfigs" :key="cfg.filename" class="uc-item">
                <div class="uc-header">
                  <span class="uc-filename">{{ cfg.filename }}</span>
                  <div class="uc-actions">
                    <button class="btn btn-ghost btn-sm" @click="viewUploadedConfig(cfg)">查看/编辑</button>
                    <button class="btn btn-danger btn-sm" @click="deleteUploadedConfig(cfg.filename)">删除</button>
                  </div>
                </div>
                <div class="uc-meta">
                  <span>{{ fmtSize(cfg.size) }}</span>
                  <span>{{ fmtTime(cfg.updatedAt) }}</span>
                  <span v-if="cfg.inbounds && cfg.inbounds.length">{{ cfg.inbounds.length }} 个入站</span>
                </div>
              </div>
            </div>
          </div>

        </div>
      </template>

    </div>

    <!-- View/Edit JSON modal -->
    <div v-if="showJson" class="mask" @click.self="showJson=false">
      <div class="modal" style="max-width:720px;max-height:90vh;display:flex;flex-direction:column">
        <div class="modal-head">
          <span>{{ editingFilename }}</span>
          <button class="btn-icon" @click="showJson=false">✕</button>
        </div>
        <div style="flex:1;overflow:hidden;display:flex;flex-direction:column;padding:0">
          <textarea class="uc-editor" v-model="jsonText" spellcheck="false"></textarea>
          <div v-if="editErr" class="alert alert-error" style="margin:8px 16px 0">{{ editErr }}</div>
          <div v-if="editOk"  class="alert alert-success" style="margin:8px 16px 0">{{ editOk }}</div>
        </div>
        <div style="padding:10px 16px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:8px">
          <button class="btn btn-ghost btn-sm" @click="showJson=false">关闭</button>
          <button class="btn btn-primary btn-sm" :disabled="savingJson" @click="saveJsonEdit">
            {{ savingJson ? '保存中…' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <ProfileWizard v-if="showWizard"
      :profile="editingProfile"
      :subs="subsStore.subs"
      @close="showWizard=false"
      @saved="onWizardSaved" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api.js'
import { useSubsStore, useProfilesStore } from '../stores.js'
import ProfileWizard from '../components/ProfileWizard.vue'

const subsStore    = useSubsStore()
const profilesStore = useProfilesStore()
const tab = ref('generate')

// ── Upload ────────────────────────────────────────────────────────────────
const dragOver        = ref(false)
const uploadErr       = ref('')
const uploadOk        = ref('')
const uploadedConfigs = ref([])
const showJson        = ref(false)
const jsonText        = ref('')
const editingFilename = ref('')
const editErr         = ref('')
const editOk          = ref('')
const savingJson      = ref(false)
const fileInput       = ref(null)

function getAuthHeaders() {
  const token = localStorage.getItem('singa_token') || ''
  return token ? { 'X-Auth-Token': token } : {}
}

async function loadUploadedConfigs() {
  try { uploadedConfigs.value = await api('GET', '/config/list') } catch {}
}

async function uploadFile(file) {
  uploadErr.value = ''; uploadOk.value = ''
  if (!file?.name.endsWith('.json')) { uploadErr.value = '只允许上传 .json 文件'; return }
  try {
    const fd = new FormData(); fd.append('config', file)
    const r = await fetch('/api/config', { method:'POST', body:fd, headers: getAuthHeaders() })
    const d = await r.json()
    if (!r.ok) throw new Error(d.error)
    uploadOk.value = '✓ 上传成功：' + d.filename
    await loadUploadedConfigs()
  } catch (e) { uploadErr.value = '✕ ' + e.message }
}
function onFileChange(e) { uploadFile(e.target.files[0]); e.target.value = '' }
function onDrop(e)       { dragOver.value=false; uploadFile(e.dataTransfer.files[0]) }

async function viewUploadedConfig(cfg) {
  editErr.value = ''; editOk.value = ''
  editingFilename.value = cfg.filename
  try {
    const r = await fetch('/api/config/raw/' + encodeURIComponent(cfg.filename), { headers: getAuthHeaders() })
    const text = await r.text()
    jsonText.value = JSON.stringify(JSON.parse(text), null, 2)
  } catch { jsonText.value = '' }
  showJson.value = true
}

async function saveJsonEdit() {
  editErr.value = ''; editOk.value = ''
  try { JSON.parse(jsonText.value) } catch { editErr.value = 'JSON 格式错误'; return }
  savingJson.value = true
  try {
    const r = await fetch('/api/config/raw/' + encodeURIComponent(editingFilename.value), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...getAuthHeaders() },
      body: jsonText.value
    })
    const d = await r.json()
    if (!r.ok) throw new Error(d.error)
    editOk.value = '✓ 已保存'
    await loadUploadedConfigs()
  } catch (e) { editErr.value = '✕ ' + e.message }
  finally { savingJson.value = false }
}

async function deleteUploadedConfig(filename) {
  if (!confirm('确定删除 ' + filename + '？')) return
  try {
    await api('DELETE', '/config/' + encodeURIComponent(filename))
    await loadUploadedConfigs()
  } catch (e) { alert('删除失败：' + e.message) }
}

function fmtSize(bytes) {
  if (!bytes) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(1) + ' MB'
}

// ── Generate / wizard ─────────────────────────────────────────────────────
const showWizard    = ref(false)
const editingProfile = ref(null)
const validating = ref('')  // profile id being validated
const validationResults = ref({})  // { [profId]: { ok, errors } }

function openWizard(prof) {
  editingProfile.value = prof
  showWizard.value = true
}

async function onWizardSaved(prof) {
  showWizard.value = false
  await profilesStore.load()
  // Auto-validate after saving
  const saved = profilesStore.profiles.find(p => p.id === (prof?.id || editingProfile.value?.id))
  if (saved?.wizardConfig) {
    await validateProfile(saved)
  }
}

async function validateProfile(prof) {
  if (!prof.wizardConfig) return
  validating.value = prof.id
  try {
    const res = await api('POST', '/profiles/validate', { wizardConfig: prof.wizardConfig })
    validationResults.value = { ...validationResults.value, [prof.id]: res }
  } catch (e) {
    validationResults.value = { ...validationResults.value, [prof.id]: {
      ok: false, errors: [{ location: 'network', message: e.message }]
    }}
  } finally {
    validating.value = ''
  }
}

async function deleteProfile(id) {
  if (!confirm('确定删除此配置？')) return
  await profilesStore.remove(id)
}

// ── Helpers ───────────────────────────────────────────────────────────────

function fmtTime(iso) {
  if (!iso) return ''
  return new Date(iso).toLocaleString('zh-CN', { month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit' })
}

onMounted(() => {
  subsStore.load()
  profilesStore.load()
  loadUploadedConfigs()
})
</script>

<style scoped>
.upload-drop {
  display: flex; flex-direction: column; align-items: center; gap: 10px;
  padding: 32px 20px; border: 2px dashed var(--border2); border-radius: var(--radius);
  cursor: pointer; transition: all .15s; margin-top: 8px; background: var(--surface2);
}
.upload-drop:hover, .upload-drop.over {
  border-color: var(--accent); background: var(--accent-bg);
}
.uc-item {
  border: 1.5px solid var(--border); border-radius: var(--radius);
  padding: 10px 12px; background: var(--surface2);
}
.uc-header {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
}
.uc-filename {
  font-weight: 600; font-size: 13px; font-family: var(--mono);
  color: var(--text1); word-break: break-all; flex: 1;
}
.uc-actions { display: flex; gap: 6px; flex-shrink: 0; }
.uc-meta {
  display: flex; gap: 10px; margin-top: 5px;
  font-size: 11px; color: var(--text3); flex-wrap: wrap;
}
.uc-editor {
  width: 100%; min-height: 420px; flex: 1; resize: vertical;
  font-family: var(--mono); font-size: 12px; line-height: 1.5;
  padding: 14px; background: #0f1117; color: #e2e8f0;
  border: none; outline: none; box-sizing: border-box;
}
@media (max-width: 640px) {
  .upload-drop { padding: 22px 14px; }
  .uc-header { flex-direction: column; align-items: flex-start; }
}
</style>
