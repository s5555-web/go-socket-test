<script setup>
import { computed, onMounted, reactive, ref } from 'vue'

const token = ref(localStorage.adminToken || '')
const loading = ref(false)
const error = ref('')
const stats = reactive({ users: 0, conversations: 0, messages: 0 })
const users = ref([])
const sites = ref([])
const credentials = reactive({ username: '', password: '' })
const siteForm = reactive({ name: '', allowed_origin: '', welcome_message: '您好，请问有什么可以帮您？' })
const loggedIn = computed(() => Boolean(token.value))

async function api(path, options = {}) {
  options.headers = { ...(options.headers || {}), 'Content-Type': 'application/json' }
  if (token.value) options.headers.Authorization = `Bearer ${token.value}`
  const response = await fetch(`/api${path}`, options)
  if (response.status === 204) return null
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw Error(data.error || '请求失败')
  return data
}

async function login() {
  loading.value = true
  error.value = ''
  try {
    const result = await api('/login', { method: 'POST', body: JSON.stringify(credentials) })
    if (!result.is_admin) throw Error('该账号不是管理员')
    token.value = result.token
    localStorage.adminToken = result.token
    await refresh()
  } catch (cause) {
    error.value = cause.message
    token.value = ''
    localStorage.removeItem('adminToken')
  } finally {
    loading.value = false
  }
}

async function refresh() {
  if (!token.value) return
  loading.value = true
  error.value = ''
  try {
    const [summary, userRows, siteRows] = await Promise.all([api('/stats'), api('/users'), api('/support-sites')])
    Object.assign(stats, summary)
    users.value = userRows
    sites.value = siteRows.map(item => ({ ...item }))
  } catch (cause) {
    error.value = cause.message
    if (/未授权|unauthorized|token/i.test(cause.message)) logout()
  } finally {
    loading.value = false
  }
}

function logout() {
  token.value = ''
  localStorage.removeItem('adminToken')
}

async function createSite() {
  try {
    await api('/support-sites', { method: 'POST', body: JSON.stringify(siteForm) })
    Object.assign(siteForm, { name: '', allowed_origin: '', welcome_message: '您好，请问有什么可以帮您？' })
    await refresh()
  } catch (cause) { error.value = cause.message }
}

async function saveSite(site, enabled = site.enabled) {
  try {
    await api(`/support-sites/${site.id}`, {
      method: 'PUT',
      body: JSON.stringify({ name: site.name, allowed_origin: site.allowed_origin, welcome_message: site.welcome_message, enabled })
    })
    await refresh()
  } catch (cause) { error.value = cause.message }
}

function snippet(site) {
  return `<script src="https://msg.trip-vn.com/assets/support-widget.js" data-site-key="${site.site_key}" data-title="在线客服" async><\/script>`
}

async function copySnippet(site) {
  await navigator.clipboard.writeText(snippet(site))
  window.alert('嵌入代码已复制')
}

async function setSupport(user) {
	try {
		const enabling = !user.is_support
		if (enabling && !user.support_site_ids?.length) throw Error('请先选择该客服负责的插件站点')
		await api(`/users/${user.id}/support`, { method: 'PUT', body: JSON.stringify({ is_support: enabling, site_ids: user.support_site_ids || [] }) })
		await refresh()
	} catch (cause) { error.value = cause.message }
}

async function saveSupportSites(user) {
	try {
		if (!user.support_site_ids?.length) throw Error('请至少选择一个插件站点')
		await api(`/users/${user.id}/support`, { method: 'PUT', body: JSON.stringify({ is_support: true, site_ids: user.support_site_ids }) })
		await refresh()
	} catch (cause) { error.value = cause.message }
}

async function deleteUser(user) {
  if (!window.confirm(`确定删除用户 ${user.display_name}？`)) return
  await api(`/users/${user.id}`, { method: 'DELETE' })
  await refresh()
}

const userColumns = [
  { field: 'id', title: 'ID', width: 80 },
  { field: 'username', title: '用户名', minWidth: 130 },
  { field: 'display_name', title: '昵称', minWidth: 130 },
  { field: 'about', title: '个人简介', minWidth: 190, showOverflow: true },
  { field: 'role', title: '角色', width: 130, slots: { default: 'role' } },
	{ field: 'support_site_ids', title: '客服插件站点', minWidth: 260, slots: { default: 'supportSites' } },
  { field: 'created_at', title: '注册时间', width: 190, formatter: ({ cellValue }) => new Date(cellValue).toLocaleString() },
	{ title: '操作', width: 250, fixed: 'right', slots: { default: 'actions' } }
]

onMounted(() => { if (token.value) refresh() })
</script>

<template>
  <main v-if="!loggedIn" class="login-page">
    <vxe-card class="login-card" title="Signal Web 管理后台">
      <vxe-form :data="credentials" @submit="login">
        <vxe-form-item title="用户名" field="username" span="24"><vxe-input v-model="credentials.username" clearable /></vxe-form-item>
        <vxe-form-item title="密码" field="password" span="24"><vxe-password-input v-model="credentials.password" clearable /></vxe-form-item>
        <vxe-form-item span="24"><vxe-button type="submit" status="primary" :loading="loading" content="登录" class="full-button" /></vxe-form-item>
      </vxe-form>
      <vxe-alert v-if="error" status="error" :content="error" />
    </vxe-card>
  </main>

  <div v-else class="admin-shell">
    <header class="topbar"><div><strong>Signal Web</strong><span>管理后台</span></div><vxe-space><vxe-button icon="vxe-icon-refresh" circle @click="refresh" /><vxe-button status="danger" content="退出" @click="logout" /></vxe-space></header>
    <main class="content">
      <vxe-loading v-if="loading" />
      <vxe-alert v-if="error" status="error" :content="error" closable @close="error=''" />
      <section class="stats-grid">
        <vxe-card><span>用户</span><b>{{ stats.users }}</b></vxe-card>
        <vxe-card><span>私聊/群聊会话</span><b>{{ stats.conversations }}</b></vxe-card>
        <vxe-card><span>旧消息表记录</span><b>{{ stats.messages }}</b></vxe-card>
      </section>

      <vxe-card title="客服插件站点" class="panel">
        <p class="hint">授权域名填写完整来源；多个来源用逗号分隔，测试环境可填写 *。</p>
        <vxe-form :data="siteForm" title-width="90" @submit="createSite">
          <vxe-form-item title="站点名称" field="name" :span="8"><vxe-input v-model="siteForm.name" /></vxe-form-item>
          <vxe-form-item title="授权域名" field="allowed_origin" :span="16"><vxe-input v-model="siteForm.allowed_origin" placeholder="https://www.example.com" /></vxe-form-item>
          <vxe-form-item title="欢迎语" field="welcome_message" :span="20"><vxe-textarea v-model="siteForm.welcome_message" autosize /></vxe-form-item>
          <vxe-form-item :span="4"><vxe-button type="submit" status="primary" content="创建插件" /></vxe-form-item>
        </vxe-form>
        <div v-if="sites.length" class="site-grid">
          <vxe-card v-for="site in sites" :key="site.id" :title="site.name">
            <template #extra><vxe-tag :status="site.enabled ? 'success' : 'error'" :content="site.enabled ? '启用' : '停用'" /></template>
            <vxe-form :data="site" title-width="80">
              <vxe-form-item title="名称" field="name" span="24"><vxe-input v-model="site.name" /></vxe-form-item>
              <vxe-form-item title="授权域名" field="allowed_origin" span="24"><vxe-input v-model="site.allowed_origin" /></vxe-form-item>
              <vxe-form-item title="欢迎语" field="welcome_message" span="24"><vxe-textarea v-model="site.welcome_message" autosize /></vxe-form-item>
            </vxe-form>
            <pre>{{ snippet(site) }}</pre>
            <vxe-space wrap><vxe-button status="primary" content="保存" @click="saveSite(site)" /><vxe-button content="复制嵌入代码" @click="copySnippet(site)" /><vxe-button :status="site.enabled ? 'danger' : 'success'" :content="site.enabled ? '停用' : '启用'" @click="saveSite(site, !site.enabled)" /></vxe-space>
          </vxe-card>
        </div>
        <vxe-empty v-else content="尚未创建客服插件站点" />
      </vxe-card>

      <vxe-card title="用户与客服账号" class="panel">
		<p class="hint">为客服选择负责的插件站点；权限变更会实时同步到已登录的 Chat 客户端。</p>
		<vxe-grid border stripe show-overflow :data="users" :columns="userColumns" :column-config="{ resizable: true }" :scroll-x="{ enabled: true }">
		  <template #role="{ row }"><vxe-space><vxe-tag v-if="row.is_admin" status="primary" content="管理员" /><vxe-tag :status="row.is_support ? 'success' : 'info'" :content="row.is_support ? '客服' : '普通用户'" /></vxe-space></template>
		  <template #supportSites="{ row }"><vxe-select v-model="row.support_site_ids" multiple clearable placeholder="选择插件站点"><vxe-option v-for="site in sites" :key="site.id" :value="site.id" :label="site.name" :disabled="!site.enabled" /></vxe-select></template>
		  <template #actions="{ row }"><vxe-space><vxe-button v-if="row.is_support" size="mini" status="primary" content="保存站点" @click="saveSupportSites(row)" /><vxe-button size="mini" :status="row.is_support ? 'warning' : 'success'" :content="row.is_support ? '取消客服' : '设为客服'" @click="setSupport(row)" /><vxe-button v-if="!row.is_admin" size="mini" status="danger" content="删除" @click="deleteUser(row)" /></vxe-space></template>
        </vxe-grid>
      </vxe-card>
    </main>
  </div>
</template>
