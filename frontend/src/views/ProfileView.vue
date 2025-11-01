<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useAuth } from '@/composables/useAuth'
import Navbar from '@/components/Navbar.vue'

const { user } = useAuth()
const isLoading = ref(false)

onMounted(async () => {
  // Router guard already handles authentication check
  isLoading.value = false
})

const getUserRole = () => {
  if (user.value?.username === 'root') {
    return 'Administrator'
  }
  return 'User'
}
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <Navbar />
    <div class="max-w-4xl mx-auto py-8 px-4">
      <div class="mb-8">
        <h1 class="text-3xl font-bold text-gray-900">Profile</h1>
        <p class="text-gray-600 mt-2">Manage your account information</p>
      </div>

      <div v-if="isLoading" class="text-center py-8">
        <p>Loading profile...</p>
      </div>

      <div v-else-if="user" class="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>Account Information</CardTitle>
            <CardDescription>Your personal account details</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label class="text-sm font-medium text-gray-700">Username</label>
                <p class="mt-1 text-lg">{{ user.username }}</p>
              </div>
              <div>
                <label class="text-sm font-medium text-gray-700">Role</label>
                <p class="mt-1 text-lg">{{ getUserRole() }}</p>
              </div>
              <div>
                <label class="text-sm font-medium text-gray-700">User ID</label>
                <p class="mt-1 text-lg font-mono">{{ user.id }}</p>
              </div>
              <div>
                <label class="text-sm font-medium text-gray-700">Member Since</label>
                <p class="mt-1 text-lg">{{ new Date(user.created_at).toLocaleDateString() }}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <div v-else class="text-center py-8">
        <p class="text-red-600">Failed to load profile information</p>
      </div>
    </div>
  </div>
</template>