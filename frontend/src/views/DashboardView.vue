<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { apiClient, type UserProfile } from '@/services/api'

const user = ref<UserProfile | null>(null)
const isLoading = ref(true)

const handleLogout = async () => {
  try {
    await apiClient.logout()
    window.location.href = '/login'
  } catch (error) {
    console.error('Logout failed:', error)
  }
}

onMounted(async () => {
  try {
    const response = await apiClient.getProfile()
    if (response.success) {
      user.value = response.user
    }
  } catch (error) {
    console.error('Failed to load profile:', error)
    window.location.href = '/login'
  } finally {
    isLoading.value = false
  }
})
</script>

<template>
  <div class="container mx-auto p-8">
    <div class="flex justify-between items-center mb-8">
      <h1 class="text-3xl font-bold">Dashboard</h1>
      <Button @click="handleLogout" variant="outline">Logout</Button>
    </div>

    <div v-if="isLoading" class="text-center">
      <p>Loading...</p>
    </div>

    <div v-else-if="user" class="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Welcome back!</CardTitle>
          <CardDescription>Your profile information</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="space-y-2">
            <p><strong>Email:</strong> {{ user.email }}</p>
            <p v-if="user.first_name"><strong>First Name:</strong> {{ user.first_name }}</p>
            <p v-if="user.last_name"><strong>Last Name:</strong> {{ user.last_name }}</p>
            <p>
              <strong>Member since:</strong> {{ new Date(user.created_at).toLocaleDateString() }}
            </p>
          </div>
        </CardContent>
      </Card>
    </div>

    <div v-else class="text-center">
      <p>Failed to load user profile</p>
    </div>
  </div>
</template>
