<template>
    <div class="count-picker">
        <button
            v-for="n in maxCount"
            :key="n"
            type="button"
            class="ghost count-option"
            :class="{ active: modelValue === n }"
            :disabled="busy"
            @click="$emit('update:modelValue', n)"
        >
            {{ n }}
        </button>
    </div>
</template>

<script setup>
defineProps({
    modelValue: { type: Number, required: true },
    busy: { type: Boolean, required: true },
    maxCount: { type: Number, default: 10 },
});

defineEmits(["update:modelValue"]);
</script>
