import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'ant-design-vue/dist/reset.css'
import App from './App.vue'
import { usePanelStore } from './stores/panel'
import './style.css'

const pinia = createPinia()
const app = createApp(App)
app.use(pinia)

const panel = usePanelStore(pinia)

app.config.errorHandler = (err, instance, info) => {
  const detail = `${String(info || 'vue error')}: ${err instanceof Error ? err.message : String(err)}`
  panel.recordClientError(detail, 'vue')
  console.error(err, instance, info)
}

window.addEventListener('error', (event) => {
  panel.recordClientError(String(event.message || 'window error'), 'window')
})

window.addEventListener('unhandledrejection', (event) => {
  const reason = event.reason instanceof Error ? event.reason.message : String(event.reason || 'unhandled rejection')
  panel.recordClientError(reason, 'promise')
})

app.mount('#app')
