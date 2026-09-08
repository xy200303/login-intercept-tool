import { reactive } from 'vue'

let seq = 0
export const toasts = reactive([])

function push(type, message, timeout) {
  const id = ++seq
  toasts.push({ id, type, message })
  setTimeout(() => {
    const index = toasts.findIndex((item) => item.id === id)
    if (index >= 0) toasts.splice(index, 1)
  }, timeout)
}

export const toast = {
  success: (message) => push('success', message, 3500),
  error: (message) => push('error', message, 5000),
  info: (message) => push('info', message, 3500),
}
