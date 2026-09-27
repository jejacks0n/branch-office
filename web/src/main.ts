import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { bootstrapToken } from './token'
import './theme'

bootstrapToken()

createApp(App).mount('#app')
