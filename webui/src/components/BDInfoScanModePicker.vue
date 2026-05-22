<template>
    <div class="variant-picker">
        <button
            type="button"
            class="ghost variant-option"
            :class="{ active: modelValue === 'auto' }"
            :disabled="busy"
            @click="$emit('update:modelValue', 'auto')"
        >
            智能推荐
        </button>
        <button
            type="button"
            class="ghost variant-option"
            :class="{ active: modelValue === 'playlists' }"
            :disabled="busy"
            @click="handleManual"
        >
            手动选取
        </button>
        <button
            type="button"
            class="ghost variant-option"
            :class="{ active: modelValue === 'whole' }"
            :disabled="busy"
            @click="$emit('update:modelValue', 'whole')"
        >
            完整扫描
        </button>
    </div>
</template>

<script setup>
const props = defineProps({
    modelValue: { type: String, default: "auto" },
    busy: { type: Boolean, default: false },
});

const emit = defineEmits(["update:modelValue"]);

const handleManual = () => {
    emit("update:modelValue", "playlists");
};
</script>