<template>
  <ConflictExplorer :can-execute="auth.isSuperAdmin" show-agent-filter />
</template>

<script setup>
import ConflictExplorer from '../components/ConflictExplorer.vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
</script>
