import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  adminAPI,
  setCSRFToken,
  setUnauthorizedHandler,
  type SessionInfo,
} from './admin'

export const useSessionStore = defineStore('admin-session', () => {
  const ready = ref(false)
  const authenticated = ref(false)
  const username = ref('')
  const role = ref('')
  const expiresAt = ref('')
  const redirectTo = ref('/')

  const displayName = computed(() => username.value || '管理员')

  function applySession(session: SessionInfo) {
    authenticated.value = true
    username.value = session.username
    role.value = session.role
    expiresAt.value = session.expiresAt
    setCSRFToken(session.csrfToken)
  }

  function clearSession() {
    authenticated.value = false
    username.value = ''
    role.value = ''
    expiresAt.value = ''
    setCSRFToken('')
  }

  async function restore() {
    setUnauthorizedHandler(() => {
      clearSession()
    })
    try {
      applySession(await adminAPI.session())
    } catch {
      clearSession()
    } finally {
      ready.value = true
    }
  }

  async function login(user: string, password: string) {
    applySession(await adminAPI.login(user, password))
  }

  async function logout() {
    try {
      await adminAPI.logout()
    } finally {
      clearSession()
    }
  }

  return {
    ready,
    authenticated,
    username,
    role,
    expiresAt,
    redirectTo,
    displayName,
    restore,
    login,
    logout,
    clearSession,
  }
})
