<template>
  <div ref="host" class="toml-editor" aria-label="TOML 配置编辑器" />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { StreamLanguage, syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language'
import { toml } from '@codemirror/legacy-modes/mode/toml'
import { searchKeymap, highlightSelectionMatches } from '@codemirror/search'
import { Compartment, EditorState } from '@codemirror/state'
import {
  EditorView,
  drawSelection,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
} from '@codemirror/view'

const props = defineProps<{
  modelValue: string
  readonly?: boolean
  dark?: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const host = ref<HTMLElement>()
const themeCompartment = new Compartment()
let view: EditorView | undefined

const baseTheme = EditorView.theme({
  '&': {
    height: '100%',
    fontSize: '13px',
    backgroundColor: 'transparent',
  },
  '.cm-scroller': {
    fontFamily:
      '"SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace',
    lineHeight: '1.65',
  },
  '.cm-content': {
    padding: '14px 0',
  },
  '.cm-gutters': {
    backgroundColor: 'transparent',
    borderRight: '1px solid var(--frp-border)',
    color: 'var(--frp-muted)',
  },
  '.cm-activeLine': {
    backgroundColor: 'color-mix(in srgb, var(--frp-role-strong) 5%, transparent)',
  },
  '.cm-activeLineGutter': {
    backgroundColor: 'transparent',
    color: 'var(--frp-role-strong)',
  },
  '&.cm-focused': {
    outline: 'none',
  },
})

function createState(content: string) {
  return EditorState.create({
    doc: content,
    extensions: [
      lineNumbers(),
      highlightActiveLineGutter(),
      history(),
      drawSelection(),
      highlightActiveLine(),
      highlightSelectionMatches(),
      StreamLanguage.define(toml),
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap]),
      EditorState.readOnly.of(Boolean(props.readonly)),
      EditorView.editable.of(!props.readonly),
      baseTheme,
      themeCompartment.of(props.dark ? EditorView.theme({}, { dark: true }) : []),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          emit('update:modelValue', update.state.doc.toString())
        }
      }),
    ],
  })
}

onMounted(() => {
  if (!host.value) return
  view = new EditorView({
    state: createState(props.modelValue),
    parent: host.value,
  })
})

watch(
  () => props.modelValue,
  (value) => {
    if (!view) return
    const current = view.state.doc.toString()
    if (value !== current) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: value },
      })
    }
  },
)

watch(
  () => props.dark,
  (dark) => {
    view?.dispatch({
      effects: themeCompartment.reconfigure(
        dark ? EditorView.theme({}, { dark: true }) : [],
      ),
    })
  },
)

onBeforeUnmount(() => {
  view?.destroy()
})
</script>

<style scoped>
.toml-editor {
  height: 100%;
  min-height: 360px;
  overflow: hidden;
}

.toml-editor :deep(.cm-editor) {
  height: 100%;
}
</style>
