<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-3">
          <p class="text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('availableChannels.catalog.scopeNotice') }}</p>
          <div class="flex flex-wrap items-end gap-3">
            <label class="min-w-0 flex-1 sm:min-w-64">
              <span class="mb-1 block text-sm font-medium">{{ t('availableChannels.catalog.search') }}</span>
              <input v-model="searchQuery" type="search" :placeholder="t('availableChannels.searchPlaceholder')" class="input" />
            </label>
            <label class="w-full sm:w-56">
              <span class="mb-1 block text-sm font-medium">{{ t('availableChannels.catalog.group') }}</span>
              <select v-model="groupId" class="input" data-testid="catalog-group">
                <option :value="null">{{ t('availableChannels.catalog.allGroups') }}</option>
                <option v-for="group in catalog?.groups ?? []" :key="group.id" :value="group.id">{{ group.name }}</option>
              </select>
            </label>
            <button class="btn btn-secondary" :disabled="loading" @click="loadCatalog">{{ t('common.refresh') }}</button>
          </div>
          <p v-if="catalog?.user_rate_status === 'unavailable'" role="status" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">{{ t('availableChannels.catalog.rateUnavailable') }}</p>
        </div>
      </template>
      <template #table>
        <div class="table-wrapper p-4" :aria-busy="loading">
          <p v-if="loading" role="status" class="py-12 text-center text-gray-500">{{ t('common.loading') }}</p>
          <div v-else-if="errorMessage" role="alert" class="space-y-3 py-12 text-center">
            <p>{{ errorMessage }}</p><button class="btn btn-secondary" @click="loadCatalog">{{ t('availableChannels.catalog.retry') }}</button>
          </div>
          <template v-else-if="catalog">
            <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('availableChannels.catalog.modelCount', { count: filteredModels.length }) }}</p>
            <div v-if="filteredModels.length" class="grid min-w-0 grid-cols-1 items-start gap-4 xl:grid-cols-2">
              <ModelCatalogCard v-for="model in filteredModels" :key="model.key" :model="model" :user-rate-status="catalog.user_rate_status" />
            </div>
            <p v-else class="py-12 text-center text-gray-500 dark:text-gray-400">{{ t(searchQuery.trim() || groupId !== null ? 'availableChannels.catalog.noMatches' : 'availableChannels.catalog.empty') }}</p>
            <p v-if="emptyGroups.length" class="mt-4 break-words text-xs text-gray-500 dark:text-gray-400">{{ t('availableChannels.catalog.emptyGroups', { groups: emptyGroups.map(group => group.name).join(' · ') }) }}</p>
          </template>
        </div>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import ModelCatalogCard from '@/components/channels/ModelCatalogCard.vue'
import userChannelsAPI, { type AvailableModelCatalog } from '@/api/channels'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { collectCatalogModels, filterCatalogModels } from '@/utils/modelCatalog'

const { t } = useI18n()
const authStore = useAuthStore()
const catalog = ref<AvailableModelCatalog | null>(null)
const loading = ref(false)
const errorMessage = ref('')
const searchQuery = ref('')
const groupId = ref<number | null>(null)
const models = computed(() => catalog.value ? collectCatalogModels(catalog.value) : [])
const filteredModels = computed(() => filterCatalogModels(models.value, searchQuery.value, groupId.value))
const emptyGroups = computed(() => catalog.value?.groups.filter(group => !group.models.length) ?? [])
let requestId = 0
let controller: AbortController | undefined

async function loadCatalog() {
  const currentRequest = ++requestId
  const userId = authStore.user?.id
  controller?.abort()
  const requestController = new AbortController()
  controller = requestController
  // 每次刷新先清除旧报价；失败或主体切换时不能继续显示上一份目录。
  catalog.value = null
  errorMessage.value = ''
  loading.value = userId !== undefined
  if (userId === undefined) return
  try {
    const result = await userChannelsAPI.getCatalog({ signal: requestController.signal })
    if (currentRequest !== requestId || userId !== authStore.user?.id || requestController.signal.aborted) return
    catalog.value = result
    if (groupId.value !== null && !result.groups.some(group => group.id === groupId.value)) groupId.value = null
  } catch (err: unknown) {
    if (currentRequest !== requestId || userId !== authStore.user?.id || requestController.signal.aborted) return
    errorMessage.value = extractApiErrorMessage(err, t('common.error'))
  } finally {
    if (currentRequest === requestId) loading.value = false
  }
}

watch(() => authStore.user?.id, () => {
  searchQuery.value = ''
  groupId.value = null
  void loadCatalog()
}, { immediate: true, flush: 'sync' })
onBeforeUnmount(() => { ++requestId; controller?.abort() })
</script>
