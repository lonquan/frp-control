import { cloneVNode, defineComponent, h } from 'vue'

export const SlotStub = defineComponent({
  props: {
    title: String,
    visible: {
      type: Boolean,
      default: true,
    },
  },
  setup(props, { slots }) {
    return () =>
      props.visible
        ? h('div', [
            props.title ? h('strong', props.title) : null,
            slots.default?.(),
            slots.extra?.(),
          ])
        : null
  },
})

export const DrawerStub = defineComponent({
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
  },
  setup(props, { slots }) {
    return () =>
      props.visible
        ? h('section', { class: 'drawer-stub' }, [
            slots.default?.(),
            h('footer', slots.footer?.()),
          ])
        : null
  },
})

export const ButtonStub = defineComponent({
  props: {
    disabled: Boolean,
    loading: Boolean,
  },
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.disabled,
          'data-loading': props.loading ? 'true' : 'false',
          onClick: (event: Event) => emit('click', event),
        },
        slots.default?.(),
      )
  },
})

export const SwitchStub = defineComponent({
  props: {
    modelValue: Boolean,
    loading: Boolean,
  },
  emits: ['change'],
  setup(props, { emit }) {
    return () =>
      h('button', {
        class: 'switch-stub',
        'data-model-value': props.modelValue ? 'true' : 'false',
        'data-loading': props.loading ? 'true' : 'false',
        onClick: () => emit('change', !props.modelValue),
      })
  },
})

export const StatusBadgeStub = defineComponent({
  props: {
    status: String,
    label: String,
  },
  setup(props) {
    return () => h('span', props.label || props.status)
  },
})

export const InputStub = defineComponent({
  props: {
    modelValue: String,
    placeholder: String,
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        value: props.modelValue || '',
        placeholder: props.placeholder,
        onInput: (event: Event) =>
          emit('update:modelValue', (event.target as HTMLInputElement).value),
      })
  },
})

export const TableColumnStub = defineComponent({
  props: {
    title: String,
    record: {
      type: Object,
      default: undefined,
    },
  },
  setup(props, { slots }) {
    return () => slots.cell?.({ record: props.record }) || slots.default?.()
  },
})

export const TableStub = defineComponent({
  props: {
    data: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['row-click'],
  setup(props, { slots }) {
    const columns = slots.columns?.() || []
    return () =>
      h(
        'div',
        { class: 'table-stub' },
        props.data.map((record: any) =>
          h(
            'div',
            {
              class: 'record-row',
              key: record.key || record.name,
              onClick: () => undefined,
            },
            columns.map((column, index) =>
              cloneVNode(column, {
                record,
                key: `${record.key || record.name}-${index}`,
              }),
            ),
          ),
        ),
      )
  },
})

export const directoryStubs = {
  PageHeader: SlotStub,
  StatusBadge: StatusBadgeStub,
  ProxyEditor: true,
  VisitorEditor: true,
  'a-button': ButtonStub,
  'a-alert': SlotStub,
  'a-input': InputStub,
  'a-select': SlotStub,
  'a-option': true,
  'a-tag': SlotStub,
  'a-tooltip': SlotStub,
  'a-space': SlotStub,
  'a-table': TableStub,
  'a-table-column': TableColumnStub,
  'a-popconfirm': SlotStub,
  'a-drawer': DrawerStub,
  'a-switch': SwitchStub,
}
