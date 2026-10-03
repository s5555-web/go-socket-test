import { createApp } from 'vue'
import VxeUI from 'vxe-pc-ui'
import VxeUITable from 'vxe-table'
import 'vxe-pc-ui/es/style.css'
import 'vxe-table/lib/style.css'
import './style.css'
import App from './App.vue'

createApp(App).use(VxeUI).use(VxeUITable).mount('#app')
