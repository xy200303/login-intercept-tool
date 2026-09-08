<template>
  <table class="sk-table" aria-hidden="true">
    <tbody>
      <tr v-for="row in rows" :key="row">
        <td v-for="col in cols" :key="col"><div class="sk" :style="{ width: 45 + ((row * 37 + col * 53) % 50) + '%' }"></div></td>
      </tr>
    </tbody>
  </table>
</template>

<script setup>
defineProps({
  rows: { type: Number, default: 6 },
  cols: { type: Number, default: 6 },
})
</script>
