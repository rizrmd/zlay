<script setup lang="ts">
import { onMounted } from 'vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useAuth } from '@/composables/useAuth'

const { user, isLoading, logout, checkAuth } = useAuth()

const handleLogout = async () => {
  await logout()
  window.location.href = '/login'
}

onMounted(async () => {
  const isAuthenticated = await checkAuth()
  if (!isAuthenticated) {
    window.location.href = '/login'
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
            <p><strong>Username:</strong> {{ user.username }}</p>
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
