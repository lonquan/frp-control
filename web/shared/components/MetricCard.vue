<template>
  <article class="metric-card" :class="tone">
    <div class="metric-top">
      <span class="metric-label">{{ label }}</span>
      <span class="metric-icon" aria-hidden="true">
        <component :is="icon" />
      </span>
    </div>
    <div class="metric-value">{{ value }}</div>
    <div class="metric-foot">
      <span>{{ hint }}</span>
      <router-link v-if="to" :to="to">{{ action || '查看' }}</router-link>
    </div>
  </article>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    label: string
    value: string | number
    hint?: string
    tone?: 'blue' | 'green' | 'amber' | 'violet'
    icon: unknown
    to?: string
    action?: string
  }>(),
  { hint: '', tone: 'blue', to: '', action: '查看' },
)
</script>

<style scoped>
.metric-card {
  position: relative;
  min-height: 152px;
  overflow: hidden;
  border: 1px solid var(--frp-border);
  border-radius: 16px;
  padding: 18px 20px;
  background: var(--frp-surface);
  box-shadow: var(--frp-shadow);
}

.metric-card::after {
  position: absolute;
  right: -30px;
  bottom: -40px;
  width: 120px;
  height: 120px;
  border-radius: 50%;
  background: var(--metric-glow);
  content: '';
}

.metric-card.blue {
  --metric-color: #2563eb;
  --metric-glow: rgb(37 99 235 / 9%);
}

.metric-card.green {
  --metric-color: #0f766e;
  --metric-glow: rgb(15 118 110 / 10%);
}

.metric-card.amber {
  --metric-color: #b45309;
  --metric-glow: rgb(180 83 9 / 10%);
}

.metric-card.violet {
  --metric-color: #6d28d9;
  --metric-glow: rgb(109 40 217 / 9%);
}

.metric-top,
.metric-foot {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.metric-label {
  color: var(--frp-muted-strong);
  font-size: 13px;
  font-weight: 600;
}

.metric-icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 10px;
  background: var(--metric-glow);
  color: var(--metric-color);
  font-size: 18px;
}

.metric-value {
  position: relative;
  z-index: 1;
  margin-top: 15px;
  color: var(--frp-text);
  font-size: 30px;
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  letter-spacing: -0.03em;
}

.metric-foot {
  margin-top: 8px;
  color: var(--frp-muted);
  font-size: 12px;
}

.metric-foot a {
  color: var(--metric-color);
  font-weight: 600;
  text-decoration: none;
}
</style>
