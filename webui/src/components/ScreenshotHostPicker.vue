<template>
    <div class="host-picker">
        <button
            type="button"
            class="ghost host-option"
            :class="{ active: modelValue === 'pixhost' }"
            :disabled="busy"
            @click="$emit('update:modelValue', 'pixhost')"
        >
            Pixhost
        </button>
        <button
            type="button"
            class="ghost host-option"
            :class="{ active: modelValue === 'freeimage' }"
            :disabled="busy"
            @click="$emit('update:modelValue', 'freeimage')"
        >
            Freeimage
        </button>
        <button
            type="button"
            class="ghost host-option"
            :class="{ active: modelValue === 'local' }"
            :disabled="busy"
            @click="$emit('update:modelValue', 'local')"
        >
            本地
        </button>
    </div>
</template>

<script setup>
defineProps({
    modelValue: { type: String, required: true },
    busy: { type: Boolean, required: true },
});

defineEmits(["update:modelValue"]);
</script>