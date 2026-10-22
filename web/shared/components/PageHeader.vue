<template>
  <header class="page-header">
    <div class="heading-copy">
      <div class="eyebrow">{{ eyebrow }}</div>
      <h1>{{ title }}</h1>
      <p v-if="description">{{ description }}</p>
    </div>
    <div class="page-actions">
      <slot />
    </div>
  </header>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    title: string
    description?: string
    eyebrow?: string
  }>(),
  { description: '', eyebrow: 'FRP CONTROL' },
)
</script>

<style scoped>
.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 22px;
}

.eyebrow {
  margin-bottom: 7px;
  color: var(--frp-role-strong);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.16em;
}

h1 {
  margin: 0;
  color: var(--frp-text);
  font-size: clamp(24px, 2.1vw, 32px);
  line-height: 1.2;
  letter-spacing: -0.02em;
}

p {
  max-width: 720px;
  margin: 8px 0 0;
  color: var(--frp-muted);
  font-size: 14px;
  line-height: 1.6;
}

.page-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 10px;
}

@media (max-width: 900px) {
  .page-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 14px;
  }
}
</style>
